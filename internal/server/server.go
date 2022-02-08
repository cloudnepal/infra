package server

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/handlers"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/crypto/acme/autocert"
	"gopkg.in/square/go-jose.v2"
	"gorm.io/gorm"

	"github.com/infrahq/infra/internal"
	"github.com/infrahq/infra/internal/certs"
	"github.com/infrahq/infra/internal/config"
	"github.com/infrahq/infra/internal/logging"
	"github.com/infrahq/infra/internal/server/data"
	"github.com/infrahq/infra/internal/server/models"
	timer "github.com/infrahq/infra/internal/timer"
	"github.com/infrahq/infra/secrets"
)

type Options struct {
	Import                  *config.Config   `yaml:"import"`
	Secrets                 []SecretProvider `yaml:"secrets" validate:"dive"`
	Keys                    []KeyProvider    `yaml:"keys" validate:"dive"`
	TLSCache                string           `yaml:"tlsCache"`
	AdminAccessKey          string           `yaml:"adminAccessKey"`
	AccessKey               string           `yaml:"accessKey"`
	DBFile                  string           `yaml:"dbFile" `
	DBEncryptionKey         string           `yaml:"dbEncryptionKey"`
	DBEncryptionKeyProvider string           `yaml:"dbEncryptionKeyProvider"`
	DBHost                  string           `yaml:"dbHost" `
	DBPort                  int              `yaml:"dbPort"`
	DBName                  string           `yaml:"dbName"`
	DBUser                  string           `yaml:"dbUser"`
	DBPassword              string           `yaml:"dbPassword"`
	DBParameters            string           `yaml:"dbParameters"`
	EnableTelemetry         bool             `yaml:"enableTelemetry"`
	EnableCrashReporting    bool             `yaml:"enableCrashReporting"`
	SessionDuration         time.Duration    `yaml:"sessionDuration"`
}

type Server struct {
	options Options
	db      *gorm.DB
	tel     *Telemetry
	secrets map[string]secrets.SecretStorage
	keys    map[string]secrets.SymmetricKeyProvider
}

func Run(options Options) (err error) {
	server := &Server{
		options: options,
	}

	if err := validate.Struct(options); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	if err, ok := server.configureSentry(); ok {
		defer recoverWithSentryHub(sentry.CurrentHub())
	} else if err != nil {
		return fmt.Errorf("configure sentry: %w", err)
	}

	if err := server.importSecrets(); err != nil {
		return fmt.Errorf("secrets config: %w", err)
	}

	if err := server.importSecretKeys(); err != nil {
		return fmt.Errorf("key config: %w", err)
	}

	driver, err := server.getDatabaseDriver()
	if err != nil {
		return fmt.Errorf("driver: %w", err)
	}

	server.db, err = data.NewDB(driver)
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}

	settings, err := data.InitializeSettings(server.db)
	if err != nil {
		return fmt.Errorf("settings: %w", err)
	}

	if err = server.loadDBKey(); err != nil {
		return fmt.Errorf("loading database key: %w", err)
	}

	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetContext("serverId", settings.ID)
	})

	if err := server.configureTelemetry(); err != nil {
		return fmt.Errorf("configuring telemetry: %w", err)
	}

	if err := SetupMetrics(server.db); err != nil {
		return fmt.Errorf("configuring metrics: %w", err)
	}

	if err := os.MkdirAll(server.options.TLSCache, os.ModePerm); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	if err := server.importAccessKeys(); err != nil {
		return fmt.Errorf("importing access keys: %w", err)
	}

	// TODO: this should instead happen after runserver and we should wait for the server to close
	go func() {
		if err := server.importConfig(); err != nil {
			logging.S.Error(fmt.Errorf("import config: %w", err))
		}
	}()

	if err := server.runServer(); err != nil {
		return fmt.Errorf("running server: %w", err)
	}

	return logging.L.Sync()
}

func (s *Server) configureTelemetry() error {
	var err error

	s.tel, err = NewTelemetry(s.db)
	if err != nil {
		return err
	}

	s.tel.SetEnabled(s.options.EnableTelemetry)

	telemetryTimer := timer.NewTimer()
	telemetryTimer.Start(60*time.Minute, func() {
		if err := s.tel.EnqueueHeartbeat(); err != nil {
			logging.S.Debug(err)
		}
	})

	return nil
}

func serve(server *http.Server) {
	if err := server.ListenAndServe(); err != nil {
		logging.S.Errorf("server: %w", err)
	}
}

func (s *Server) runServer() error {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	router.Use(gin.Recovery())
	router.GET("/.well-known/jwks.json", func(c *gin.Context) {
		settings, err := data.GetSettings(s.db)
		if err != nil {
			sendAPIError(c, fmt.Errorf("could not get JWKs"))
			return
		}

		var pubKey jose.JSONWebKey
		if err := pubKey.UnmarshalJSON(settings.PublicJWK); err != nil {
			sendAPIError(c, fmt.Errorf("could not get JWKs"))
			return
		}

		c.JSON(http.StatusOK, struct {
			Keys []jose.JSONWebKey `json:"keys"`
		}{
			[]jose.JSONWebKey{pubKey},
		})
	})

	router.GET("/healthz", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	NewAPIMux(s, router.Group("/v1"))

	sentryHandler := sentryhttp.New(sentryhttp.Options{})

	metrics := gin.New()
	metrics.GET("/metrics", func(c *gin.Context) {
		promhttp.Handler().ServeHTTP(c.Writer, c.Request)
	})

	metricsServer := &http.Server{
		Addr:     ":9090",
		Handler:  handlers.CustomLoggingHandler(io.Discard, metrics, logging.ZapLogFormatter),
		ErrorLog: logging.StandardErrorLog(),
	}

	go serve(metricsServer)

	plaintextServer := &http.Server{
		Addr:     ":80",
		Handler:  handlers.CustomLoggingHandler(io.Discard, sentryHandler.Handle(router), logging.ZapLogFormatter),
		ErrorLog: logging.StandardErrorLog(),
	}

	go serve(plaintextServer)

	if err := os.MkdirAll(s.options.TLSCache, os.ModePerm); err != nil {
		return fmt.Errorf("create tls cache: %w", err)
	}

	manager := &autocert.Manager{
		Prompt: autocert.AcceptTOS,
		Cache:  autocert.DirCache(s.options.TLSCache),
	}

	tlsConfig := manager.TLSConfig()
	tlsConfig.GetCertificate = certs.SelfSignedOrLetsEncryptCert(manager, func() string {
		return ""
	})

	tlsServer := &http.Server{
		Addr:      ":443",
		TLSConfig: tlsConfig,
		Handler:   handlers.CustomLoggingHandler(io.Discard, sentryHandler.Handle(router), logging.ZapLogFormatter),
		ErrorLog:  logging.StandardErrorLog(),
	}

	if err := tlsServer.ListenAndServeTLS("", ""); err != nil {
		return err
	}

	return nil
}

// configureSentry returns ok:true when sentry is configured and initialized, or false otherwise. It can be used to know if `defer recoverWithSentryHub(sentry.CurrentHub())` can be called
func (s *Server) configureSentry() (err error, ok bool) {
	if s.options.EnableCrashReporting && internal.CrashReportingDSN != "" {
		err := sentry.Init(sentry.ClientOptions{
			Dsn:              internal.CrashReportingDSN,
			AttachStacktrace: true,
			Release:          internal.Version,
			BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
				event.ServerName = ""
				event.Request = nil
				hint.Request = nil
				return event
			},
		})
		if err != nil {
			return err, false
		}

		return nil, true
	}

	return nil, false
}

func (s *Server) getDatabaseDriver() (gorm.Dialector, error) {
	postgres, err := s.getPostgresConnectionString()
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}

	if postgres != "" {
		return data.NewPostgresDriver(postgres)
	}

	return data.NewSQLiteDriver(s.options.DBFile)
}

// getPostgresConnectionString parses postgres configuration options and returns the connection string
func (s *Server) getPostgresConnectionString() (string, error) {
	var pgConn strings.Builder

	if s.options.DBHost != "" {
		// config has separate postgres parameters set, combine them into a connection DSN now
		fmt.Fprintf(&pgConn, "host=%s ", s.options.DBHost)

		if s.options.DBUser != "" {
			fmt.Fprintf(&pgConn, "user=%s ", s.options.DBUser)

			if s.options.DBPassword != "" {
				pass, err := s.GetSecret(s.options.DBPassword)
				if err != nil {
					return "", fmt.Errorf("postgres secret: %w", err)
				}

				fmt.Fprintf(&pgConn, "password=%s ", pass)
			}
		}

		if s.options.DBPort > 0 {
			fmt.Fprintf(&pgConn, "port=%d ", s.options.DBPort)
		}

		if s.options.DBName != "" {
			fmt.Fprintf(&pgConn, "dbname=%s ", s.options.DBName)
		}

		if s.options.DBParameters != "" {
			fmt.Fprintf(&pgConn, "%s", s.options.DBParameters)
		}
	}

	return strings.TrimSpace(pgConn.String()), nil
}

func secretKindAndName(secret string) (kind string, name string, err error) {
	if !strings.Contains(secret, ":") {
		return "plaintext", secret, nil
	}

	parts := strings.SplitN(secret, ":", 2)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("unexpected secret provider format %q. Expecting <kind>:<secret name>, eg env:ACCESS_TOKEN", name)
	}

	kind = parts[0]
	name = parts[1]

	return kind, name, nil
}

// GetSecret implements the secret definition scheme for Infra.
// eg plaintext:pass123, or kubernetes:infra-okta/clientSecret
// it's an abstraction around all secret providers
func (s *Server) GetSecret(name string) (string, error) {
	kind, name, err := secretKindAndName(name)
	if err != nil {
		return "", err
	}

	secretProvider, found := s.secrets[kind]
	if !found {
		return "", fmt.Errorf("secret provider %q not found in configuration for field %q", kind, name)
	}

	b, err := secretProvider.GetSecret(name)
	if err != nil {
		return "", fmt.Errorf("getting secret: %w", err)
	}

	if b == nil {
		return "", nil
	}

	return string(b), nil
}

func (s *Server) SetSecret(name string, value string) error {
	kind, name, err := secretKindAndName(name)
	if err != nil {
		return err
	}

	secretProvider, found := s.secrets[kind]
	if !found {
		return fmt.Errorf("secret provider %q not found in configuration for field %q", kind, name)
	}

	err = secretProvider.SetSecret(name, []byte(value))
	if err != nil {
		return fmt.Errorf("setting secret: %w", err)
	}

	return nil
}

var dbKeyName = "dbkey"

// load encrypted db key from database
func (s *Server) loadDBKey() error {
	key, ok := s.keys[s.options.DBEncryptionKeyProvider]
	if !ok {
		return fmt.Errorf("key provider %s not configured", s.options.DBEncryptionKeyProvider)
	}

	keyRec, err := data.GetEncryptionKey(s.db, data.ByName(dbKeyName))
	if err != nil {
		if errors.Is(err, internal.ErrNotFound) {
			return s.createDBKey(key, s.options.DBEncryptionKey)
		}

		return err
	}

	sKey, err := key.DecryptDataKey(s.options.DBEncryptionKey, keyRec.Encrypted)
	if err != nil {
		return err
	}

	models.SymmetricKey = sKey

	return nil
}

// creates db key
func (s *Server) createDBKey(provider secrets.SymmetricKeyProvider, rootKeyId string) error {
	sKey, err := provider.GenerateDataKey(rootKeyId)
	if err != nil {
		return err
	}

	key := &models.EncryptionKey{
		Name:      dbKeyName,
		Encrypted: sKey.Encrypted,
		Algorithm: sKey.Algorithm,
		RootKeyID: sKey.RootKeyID,
	}

	_, err = data.CreateEncryptionKey(s.db, key)
	if err != nil {
		return err
	}

	models.SymmetricKey = sKey

	return nil
}
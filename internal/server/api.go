package server

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gopkg.in/segmentio/analytics-go.v3"

	"github.com/infrahq/infra/internal"
	"github.com/infrahq/infra/internal/access"
	"github.com/infrahq/infra/internal/api"
	"github.com/infrahq/infra/internal/logging"
	"github.com/infrahq/infra/internal/server/authn"
	"github.com/infrahq/infra/internal/server/models"
)

type API struct {
	t      *Telemetry
	server *Server
}

func NewAPIMux(server *Server, router *gin.RouterGroup) {
	a := API{
		t:      server.tel,
		server: server,
	}

	a.registerRoutes(router)
}

func sendAPIError(c *gin.Context, err error) {
	code := http.StatusInternalServerError
	message := "internal server error" // don't leak any info by default

	switch {
	case errors.Is(err, internal.ErrUnauthorized):
		code = http.StatusUnauthorized
		message = "unauthorized"
	case errors.Is(err, internal.ErrForbidden):
		code = http.StatusForbidden
		message = "forbidden"
	case errors.Is(err, internal.ErrDuplicate):
		code = http.StatusConflict
		message = err.Error()
	case errors.Is(err, internal.ErrNotFound):
		code = http.StatusNotFound
		message = err.Error()
	case errors.Is(err, internal.ErrBadRequest):
		code = http.StatusBadRequest
		message = err.Error()
	case errors.Is(err, (*validator.InvalidValidationError)(nil)):
		code = http.StatusBadRequest
		message = err.Error()
	}

	logging.Logger(c).Debugw(err.Error(), "statusCode", code)

	c.JSON(code, &api.Error{
		Code:    int32(code),
		Message: message,
	})
	c.Abort()
}

func (a *API) ListUsers(c *gin.Context, r *api.ListUsersRequest) ([]api.User, error) {
	users, err := access.ListUsers(c, r.Email, r.ProviderID)
	if err != nil {
		return nil, err
	}

	results := make([]api.User, len(users))
	for i, u := range users {
		results[i] = *u.ToAPI()
	}

	return results, nil
}

func (a *API) GetUser(c *gin.Context, r *api.Resource) (*api.User, error) {
	user, err := access.GetUser(c, r.ID)
	if err != nil {
		return nil, err
	}

	return user.ToAPI(), nil
}

func (a *API) CreateUser(c *gin.Context, r *api.CreateUserRequest) (*api.User, error) {
	user := &models.User{
		Email:      r.Email,
		ProviderID: r.ProviderID,
	}

	err := access.CreateUser(c, user)
	if err != nil {
		return nil, err
	}

	return user.ToAPI(), nil
}

func (a *API) ListUserGrants(c *gin.Context, r *api.Resource) ([]api.Grant, error) {
	grants, err := access.ListUserGrants(c, r.ID)
	if err != nil {
		return nil, err
	}

	results := make([]api.Grant, len(grants))
	for i, g := range grants {
		results[i] = g.ToAPI()
	}

	return results, nil
}

func (a *API) ListUserGroups(c *gin.Context, r *api.Resource) ([]api.Group, error) {
	groups, err := access.ListUserGroups(c, r.ID)
	if err != nil {
		return nil, err
	}

	results := make([]api.Group, len(groups))
	for i, g := range groups {
		results[i] = *g.ToAPI()
	}

	return results, nil
}

func (a *API) ListGroups(c *gin.Context, r *api.ListGroupsRequest) ([]api.Group, error) {
	groups, err := access.ListGroups(c, r.Name, r.ProviderID)
	if err != nil {
		return nil, err
	}

	results := make([]api.Group, len(groups))
	for i, g := range groups {
		results[i] = *g.ToAPI()
	}

	return results, nil
}

func (a *API) GetGroup(c *gin.Context, r *api.Resource) (*api.Group, error) {
	group, err := access.GetGroup(c, r.ID)
	if err != nil {
		return nil, err
	}

	return group.ToAPI(), nil
}

func (a *API) CreateGroup(c *gin.Context, r *api.CreateGroupRequest) (*api.Group, error) {
	group := &models.Group{
		Name:       r.Name,
		ProviderID: r.ProviderID,
	}

	err := access.CreateGroup(c, group)
	if err != nil {
		return nil, err
	}

	return group.ToAPI(), nil
}

func (a *API) ListGroupGrants(c *gin.Context, r *api.Resource) ([]api.Grant, error) {
	grants, err := access.ListGroupGrants(c, r.ID)
	if err != nil {
		return nil, err
	}

	results := make([]api.Grant, len(grants))
	for i, d := range grants {
		results[i] = d.ToAPI()
	}

	return results, nil
}

// caution: this endpoint is unauthenticated, do not return sensitive info
func (a *API) ListProviders(c *gin.Context, r *api.ListProvidersRequest) ([]api.Provider, error) {
	providers, err := access.ListProviders(c, r.Name)
	if err != nil {
		return nil, err
	}

	results := make([]api.Provider, len(providers))
	for i, p := range providers {
		results[i] = *p.ToAPI()
	}

	return results, nil
}

// caution: this endpoint is unauthenticated, do not return sensitive info
func (a *API) GetProvider(c *gin.Context, r *api.Resource) (*api.Provider, error) {
	provider, err := access.GetProvider(c, r.ID)
	if err != nil {
		return nil, err
	}

	return provider.ToAPI(), nil
}

var (
	dashAdminRemover = regexp.MustCompile(`(.*)\-admin(\.okta\.com)`)
	protocolRemover  = regexp.MustCompile(`http[s]?://`)
)

func cleanupURL(url string) string {
	url = strings.TrimSpace(url)
	url = dashAdminRemover.ReplaceAllString(url, "$1$2")
	url = protocolRemover.ReplaceAllString(url, "")
	return url
}

func (a *API) CreateProvider(c *gin.Context, r *api.CreateProviderRequest) (*api.Provider, error) {
	provider := &models.Provider{
		Name:         r.Name,
		URL:          cleanupURL(r.URL),
		ClientID:     r.ClientID,
		ClientSecret: models.EncryptedAtRest(r.ClientSecret),
	}

	err := access.CreateProvider(c, provider)
	if err != nil {
		return nil, err
	}

	result := provider.ToAPI()
	return result, nil
}

func (a *API) UpdateProvider(c *gin.Context, r *api.UpdateProviderRequest) (*api.Provider, error) {
	provider := &models.Provider{
		Model: models.Model{
			ID: r.ID,
		},
		Name:         r.Name,
		URL:          cleanupURL(r.URL),
		ClientID:     r.ClientID,
		ClientSecret: models.EncryptedAtRest(r.ClientSecret),
	}

	err := access.SaveProvider(c, provider)
	if err != nil {
		return nil, err
	}

	return provider.ToAPI(), nil
}

func (a *API) DeleteProvider(c *gin.Context, r *api.Resource) error {
	return access.DeleteProvider(c, r.ID)
}

func (a *API) ListDestinations(c *gin.Context, r *api.ListDestinationsRequest) ([]api.Destination, error) {
	destinations, err := access.ListDestinations(c, r.UniqueID, r.Name)
	if err != nil {
		return nil, err
	}

	results := make([]api.Destination, len(destinations))
	for i, d := range destinations {
		results[i] = *d.ToAPI()
	}

	return results, nil
}

func (a *API) CreateMachine(c *gin.Context, r *api.CreateMachineRequest) (*api.Machine, error) {
	machine := &models.Machine{}
	if err := machine.FromAPI(r); err != nil {
		return nil, err
	}

	err := access.CreateMachine(c, machine)
	if err != nil {
		return nil, err
	}

	return machine.ToAPI(), nil
}

func (a *API) ListMachines(c *gin.Context, r *api.ListMachinesRequest) ([]api.Machine, error) {
	machines, err := access.ListMachines(c, r.Name)
	if err != nil {
		return nil, err
	}

	results := make([]api.Machine, len(machines))

	for i, k := range machines {
		results[i] = *(k.ToAPI())
	}

	return results, nil
}

func (a *API) DeleteMachine(c *gin.Context, r *api.Resource) error {
	return access.DeleteMachine(c, r.ID)
}

func (a *API) GetDestination(c *gin.Context, r *api.Resource) (*api.Destination, error) {
	destination, err := access.GetDestination(c, r.ID)
	if err != nil {
		return nil, err
	}

	return destination.ToAPI(), nil
}

func (a *API) CreateDestination(c *gin.Context, r *api.CreateDestinationRequest) (*api.Destination, error) {
	destination := &models.Destination{
		Name:          r.Name,
		UniqueID:      r.UniqueID,
		ConnectionURL: r.Connection.URL,
		ConnectionCA:  r.Connection.CA,
	}

	err := access.CreateDestination(c, destination)
	if err != nil {
		return nil, fmt.Errorf("create destination: %w", err)
	}

	return destination.ToAPI(), nil
}

func (a *API) UpdateDestination(c *gin.Context, r *api.UpdateDestinationRequest) (*api.Destination, error) {
	destination := &models.Destination{
		Model: models.Model{
			ID: r.ID,
		},
		Name:          r.Name,
		UniqueID:      r.UniqueID,
		ConnectionURL: r.Connection.URL,
		ConnectionCA:  r.Connection.CA,
	}

	if err := access.SaveDestination(c, destination); err != nil {
		return nil, fmt.Errorf("update destination: %w", err)
	}

	return destination.ToAPI(), nil
}

func (a *API) DeleteDestination(c *gin.Context, r *api.Resource) error {
	return access.DeleteDestination(c, r.ID)
}

func (a *API) CreateToken(c *gin.Context, r *api.CreateTokenRequest) (*api.CreateTokenResponse, error) {
	err := a.updateUserInfo(c)
	if err != nil {
		return nil, err
	}

	token, err := access.CreateUserToken(c)
	if err != nil {
		return nil, err
	}

	return &api.CreateTokenResponse{Token: token.Token, Expires: token.Expires}, nil
}

func (a *API) ListAccessKeys(c *gin.Context, r *api.EmptyRequest) ([]api.AccessKey, error) {
	accessKeys, err := access.ListAccessKeys(c)
	if err != nil {
		return nil, err
	}

	results := make([]api.AccessKey, len(accessKeys))

	for i, a := range accessKeys {
		results[i] = api.AccessKey{
			ID:          a.ID,
			Name:        a.Name,
			Permissions: strings.Split(a.Permissions, " "),
			Created:     a.CreatedAt,
			Expires:     a.ExpiresAt,
		}
	}

	return results, nil
}

func (a *API) DeleteAccessKey(c *gin.Context, r *api.Resource) error {
	return access.DeleteAccessKey(c, r.ID)
}

func (a *API) CreateAccessKey(c *gin.Context, r *api.CreateAccessKeyRequest) (*api.CreateAccessKeyResponse, error) {
	accessKey := &models.AccessKey{
		Name:        r.Name,
		Permissions: strings.Join(r.Permissions, " "),
		ExpiresAt:   time.Now().Add(r.Ttl),
	}

	raw, err := access.CreateAccessKey(c, accessKey)
	if err != nil {
		return nil, err
	}

	return &api.CreateAccessKeyResponse{
		ID:        accessKey.ID,
		Created:   accessKey.CreatedAt,
		AccessKey: raw,
	}, nil
}

func (a *API) ListGrants(c *gin.Context, r *api.ListGrantsRequest) ([]api.Grant, error) {
	grants, err := access.ListGrants(c, r.Identity, r.Resource, r.Privilege)
	if err != nil {
		return nil, err
	}

	results := make([]api.Grant, len(grants))
	for i, r := range grants {
		results[i] = r.ToAPI()
	}

	return results, nil
}

func (a *API) GetGrant(c *gin.Context, r *api.Resource) (*api.Grant, error) {
	grant, err := access.GetGrant(c, r.ID)
	if err != nil {
		return nil, err
	}

	result := grant.ToAPI()
	return &result, nil
}

func (a *API) CreateGrant(c *gin.Context, r *api.CreateGrantRequest) (*api.Grant, error) {
	grant := &models.Grant{
		Resource:  r.Resource,
		Privilege: r.Privilege,
		Identity:  r.Identity,
	}

	err := access.CreateGrant(c, grant)
	if err != nil {
		return nil, err
	}

	result := grant.ToAPI()
	return &result, nil
}

func (a *API) DeleteGrant(c *gin.Context, r *api.Resource) error {
	return access.DeleteGrant(c, r.ID)
}

func (a *API) Login(c *gin.Context, r *api.LoginRequest) (*api.LoginResponse, error) {
	provider, err := access.GetProvider(c, r.ProviderID)
	if err != nil {
		return nil, err
	}

	oidc, err := a.providerClient(c, provider)
	if err != nil {
		return nil, err
	}

	user, token, err := access.ExchangeAuthCodeForAccessKey(c, r.Code, provider, oidc, a.server.options.SessionDuration)
	if err != nil {
		return nil, err
	}

	setAuthCookie(c, token, a.server.options.SessionDuration)

	if a.t != nil {
		if err := a.t.Enqueue(analytics.Track{Event: "infra.login", UserId: user.ID.String()}); err != nil {
			logging.S.Debug(err)
		}
	}

	return &api.LoginResponse{ID: user.ID, Name: user.Email, Token: token}, nil
}

func (a *API) Logout(c *gin.Context, r *api.EmptyRequest) (*api.EmptyResponse, error) {
	err := access.DeleteAllUserAccessKeys(c)
	if err != nil {
		return nil, err
	}

	deleteAuthCookie(c)

	return nil, nil
}

func (a *API) Version(c *gin.Context, r *api.EmptyRequest) (*api.Version, error) {
	return &api.Version{Version: internal.Version}, nil
}

// updateUserInfo calls the identity provider used to authenticate this user session to update their current information
func (a *API) updateUserInfo(c *gin.Context) error {
	// TODO: filter out machine users instead of returning nil
	user := access.CurrentUser(c)
	if user == nil {
		return nil
	}

	providerTokens, err := access.RetrieveUserProviderTokens(c)
	if err != nil {
		return err
	}

	provider, err := access.GetProvider(c, providerTokens.ProviderID)
	if err != nil {
		return fmt.Errorf("user info provider: %w", err)
	}

	oidc, err := a.providerClient(c, provider)
	if err != nil {
		return fmt.Errorf("update provider client: %w", err)
	}

	// check if the access token needs to be refreshed
	newAccessToken, newExpiry, err := oidc.RefreshAccessToken(providerTokens)
	if err != nil {
		return fmt.Errorf("refresh provider access: %w", err)
	}

	if newAccessToken != string(providerTokens.AccessToken) {
		logging.S.Debugf("access token for user at provider %s was refreshed", providerTokens.ProviderID)

		providerTokens.AccessToken = models.EncryptedAtRest(newAccessToken)
		providerTokens.ExpiresAt = *newExpiry

		if err := access.UpdateProviderToken(c, providerTokens); err != nil {
			return fmt.Errorf("update access token before JWT: %w", err)
		}
	}

	// get current identity provider groups
	info, err := oidc.GetUserInfo(providerTokens)
	if err != nil {
		if errors.Is(err, internal.ErrForbidden) {
			err := access.DeleteAllUserAccessKeys(c)
			if err != nil {
				logging.S.Errorf("failed to revoke invalid user session: %w", err)
			}
			deleteAuthCookie(c)
		}
		return fmt.Errorf("update user info: %w", err)
	}

	return access.UpdateUserInfo(c, info, user, provider)
}

func (a *API) providerClient(c *gin.Context, provider *models.Provider) (authn.OIDC, error) {
	if val, ok := c.Get("oidc"); ok {
		// oidc is added to the context during unit tests
		oidc, _ := val.(authn.OIDC)
		return oidc, nil
	}

	clientSecret, err := a.server.GetSecret(string(provider.ClientSecret))
	if err != nil {
		logging.S.Debugf("could not get client secret: %w", err)
		return nil, fmt.Errorf("error loading provider client")
	}

	return authn.NewOIDC(provider.URL, provider.ClientID, clientSecret), nil
}
package connector

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
	"gotest.tools/v3/assert"

	"github.com/infrahq/infra/internal/claims"
)

func TestGetClaims(t *testing.T) {
	type testCase struct {
		name        string
		setup       func(t *testing.T, req *http.Request)
		key         *jose.JSONWebKey
		expectedErr string
		expected    func(t *testing.T, claims claims.Custom)
	}

	pub, priv := generateJWK(t)

	run := func(t *testing.T, tc testCase) {
		if tc.key == nil {
			tc.key = &jose.JSONWebKey{}
		}
		req := httptest.NewRequest(http.MethodGet, "/apis", nil)
		if tc.setup != nil {
			tc.setup(t, req)
		}

		actual, err := getClaims(req, tc.key)
		if tc.expectedErr != "" {
			assert.ErrorContains(t, err, tc.expectedErr)
			return
		}
		assert.NilError(t, err)

		if tc.expected != nil {
			tc.expected(t, actual)
		}
	}

	testCases := []testCase{
		{
			name:        "no auth header",
			expectedErr: "no bearer token found",
		},
		{
			name: "no token",
			setup: func(t *testing.T, req *http.Request) {
				req.Header.Set("Authorization", "username:password")
			},
			expectedErr: "invalid jwt signature",
		},
		//{
		//	name: "invalid JWK",
		//	setup: func(t *testing.T, req *http.Request) {
		//		req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwiZW1haWwiOiJ0ZXN0QHRlc3QuY29tIiwiaWF0IjoxNTE2MjM5MDIyfQ.j7o5o8GBkybaYXdFJIi8O6mPF50E-gJWZ3reLfMQD68")
		//	},
		//	expectedErr: "TODO",
		//},
		{
			name: "invalid JWT",
			setup: func(t *testing.T, req *http.Request) {
				req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwiZW1haWwiOiJ0ZXN0QHRlc3QuY29tIiwiaWF0IjoxNTE2MjM5MDIyfQ.j7o5o8GBkybaYXdFJIi8O6mPF50E-gJWZ3reLfMQD68")
			},
			key:         pub,
			expectedErr: "error in cryptographic primitive",
		},
		{
			name: "expired JWT",
			setup: func(t *testing.T, req *http.Request) {
				j := generateJWT(t, priv, "test@example.com", time.Now().Add(-1*time.Hour))
				req.Header.Set("Authorization", "Bearer "+j)
			},
			key:         pub,
			expectedErr: "token is expired",
		},
		{
			name: "valid JWT",
			setup: func(t *testing.T, req *http.Request) {
				j := generateJWT(t, priv, "test@example.com", time.Now().Add(time.Hour))
				req.Header.Set("Authorization", "Bearer "+j)
			},
			key: pub,
			expected: func(t *testing.T, actual claims.Custom) {
				expected := claims.Custom{
					Name:   "test@example.com",
					Groups: []string{"developers"},
				}
				assert.DeepEqual(t, actual, expected)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func generateJWK(t *testing.T) (pub *jose.JSONWebKey, priv *jose.JSONWebKey) {
	t.Helper()
	pubkey, key, err := ed25519.GenerateKey(rand.Reader)
	assert.NilError(t, err)

	priv = &jose.JSONWebKey{Key: key, KeyID: "", Algorithm: string(jose.ED25519), Use: "sig"}
	thumb, err := priv.Thumbprint(crypto.SHA256)
	assert.NilError(t, err)

	kid := base64.URLEncoding.EncodeToString(thumb)
	priv.KeyID = kid
	pub = &jose.JSONWebKey{Key: pubkey, KeyID: kid, Algorithm: string(jose.ED25519), Use: "sig"}
	return pub, priv
}

func generateJWT(t *testing.T, priv *jose.JSONWebKey, email string, expiry time.Time) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.EdDSA, Key: priv}, (&jose.SignerOptions{}).WithType("JWT"))
	assert.NilError(t, err)

	cl := jwt.Claims{
		Issuer:   "InfraHQ",
		Expiry:   jwt.NewNumericDate(expiry),
		IssuedAt: jwt.NewNumericDate(time.Now()),
	}

	custom := claims.Custom{
		Name:   email,
		Groups: []string{"developers"},
	}

	raw, err := jwt.Signed(signer).Claims(cl).Claims(custom).CompactSerialize()
	assert.NilError(t, err)
	return raw
}

func TestCertCache_Certificate(t *testing.T) {
	testCACertPEM, err := os.ReadFile("./_testdata/test-ca-cert.pem")
	assert.NilError(t, err)

	testCAKeyPEM, err := os.ReadFile("./_testdata/test-ca-key.pem")
	assert.NilError(t, err)

	t.Run("no cached certificate adds empty certificate", func(t *testing.T) {
		certCache := NewCertCache(testCACertPEM, testCAKeyPEM)

		cert, err := certCache.Certificate()

		assert.NilError(t, err)
		assert.Equal(t, len(certCache.hosts), 1)
		assert.Equal(t, certCache.hosts[0], "")
		assert.Assert(t, cert != nil)
	})

	t.Run("cached certificate is returned when the host is set", func(t *testing.T) {
		certCache := NewCertCache(testCACertPEM, testCAKeyPEM)
		_, err := certCache.AddHost("test-host")
		assert.NilError(t, err)

		cert, err := certCache.Certificate()

		assert.NilError(t, err)
		assert.Equal(t, len(certCache.hosts), 1)
		assert.Equal(t, certCache.hosts[0], "test-host")

		parsedCert, err := x509.ParseCertificate(cert.Certificate[0])
		assert.NilError(t, err)
		assert.Equal(t, len(parsedCert.DNSNames), 1)
		assert.Equal(t, parsedCert.DNSNames[0], "test-host")
	})
}

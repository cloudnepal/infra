package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/infrahq/infra/api"
	"github.com/infrahq/infra/uid"
)

func TestKeysAddCmd(t *testing.T) {
	setup := func(t *testing.T) chan api.CreateAccessKeyRequest {
		requestCh := make(chan api.CreateAccessKeyRequest, 1)

		handler := func(resp http.ResponseWriter, req *http.Request) {
			// the command does a lookup for machine ID
			if requestMatches(req, http.MethodGet, "/v1/identities") {
				resp.WriteHeader(http.StatusOK)
				err := json.NewEncoder(resp).Encode([]*api.Identity{
					{ID: uid.ID(12345678)},
				})
				assert.Check(t, err)
				return
			}

			if !requestMatches(req, http.MethodPost, "/v1/access-keys") {
				resp.WriteHeader(http.StatusBadRequest)
				return
			}

			var createRequest api.CreateAccessKeyRequest
			err := json.NewDecoder(req.Body).Decode(&createRequest)
			assert.Check(t, err)

			resp.WriteHeader(http.StatusOK)
			err = json.NewEncoder(resp).Encode(&api.CreateAccessKeyResponse{
				AccessKey: "the-access-key",
			})
			assert.Check(t, err)
			requestCh <- createRequest
			close(requestCh)
		}

		srv := httptest.NewTLSServer(http.HandlerFunc(handler))
		t.Cleanup(srv.Close)

		cfg := ClientConfig{
			Version: "0.3",
			Hosts: []ClientHostConfig{
				{
					AccessKey:     "the-access-key",
					Host:          srv.Listener.Addr().String(),
					Current:       true,
					SkipTLSVerify: true,
					Expires:       api.Time(time.Now().Add(time.Minute)),
				},
			},
		}
		err := writeConfig(&cfg)
		assert.NilError(t, err)

		return requestCh
	}

	t.Run("all flags", func(t *testing.T) {
		ch := setup(t)

		ctx := context.Background()
		err := Run(ctx, "keys", "add", "--ttl=400h", "--extension-deadline=5h", "the-name", "my-machine")
		assert.NilError(t, err)

		req := <-ch
		expected := api.CreateAccessKeyRequest{
			IdentityID:        uid.ID(12345678),
			Name:              "the-name",
			TTL:               api.Duration(400 * time.Hour),
			ExtensionDeadline: api.Duration(5 * time.Hour),
		}
		assert.DeepEqual(t, expected, req)
	})
}

func requestMatches(req *http.Request, method string, path string) bool {
	return req.Method == method && req.URL.Path == path
}
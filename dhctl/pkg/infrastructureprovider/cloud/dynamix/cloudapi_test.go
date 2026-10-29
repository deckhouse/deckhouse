// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dynamix

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The paths the platform serves, spelled out here rather than taken from the
// client's own constants: a client that drifts off the protocol has to fail
// these tests, not move the fake along with it.
const (
	wireTokenPath             = "/v1/oauth/access_token"
	wireAccountListPath       = "/restmachine/cloudapi/account/list"
	wireStoragePolicyListPath = "/restmachine/cloudapi/storage_policy/list"
)

// fakeAPI answers the SSO endpoint and the two cloudapi lists the way the
// platform does: parameters form-encoded in the body even for GET, names
// matched by substring, everything wrapped under "data".
type fakeAPI struct {
	t *testing.T

	accounts []Account
	policies []StoragePolicy

	// statusFor, when set for a path, replaces the answer on that path.
	statusFor map[string]int

	mu       sync.Mutex
	requests []recordedRequest
}

type recordedRequest struct {
	method string
	path   string
	auth   string
	params url.Values
}

func newFakeAPI(t *testing.T) (*fakeAPI, *Client) {
	t.Helper()
	api := &fakeAPI{t: t, statusFor: map[string]int{}}
	server := httptest.NewServer(http.HandlerFunc(api.serve))
	t.Cleanup(server.Close)

	client, err := NewClient(ProviderConfig{
		ControllerURL: server.URL + "/",
		OAuth2URL:     server.URL,
		AppID:         "app-id",
		AppSecret:     "app-secret",
	})
	require.NoError(t, err)
	return api, client
}

func (f *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	params, _ := url.ParseQuery(string(body))

	f.mu.Lock()
	f.requests = append(f.requests, recordedRequest{method: r.Method, path: r.URL.Path, auth: r.Header.Get("Authorization"), params: params})
	f.mu.Unlock()

	if status, ok := f.statusFor[r.URL.Path]; ok {
		http.Error(w, fmt.Sprintf("status %d", status), status)
		return
	}

	switch r.URL.Path {
	case wireTokenPath:
		_, _ = io.WriteString(w, "token\n")
	case wireAccountListPath:
		var data []Account
		for _, a := range f.accounts {
			if strings.Contains(a.Name, params.Get("name")) {
				data = append(data, a)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	case wireStoragePolicyListPath:
		var data []StoragePolicy
		for _, p := range f.policies {
			if strings.Contains(p.Name, params.Get("name")) {
				data = append(data, p)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeAPI) recorded() []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedRequest(nil), f.requests...)
}

func TestClientSpeaksTheWireShapeOfThePlatform(t *testing.T) {
	api, client := newFakeAPI(t)
	api.policies = []StoragePolicy{{Name: "fast", Status: StoragePolicyStatusEnabled}}

	_, err := client.AccountStoragePoliciesNamed(t.Context(), 42, "fast")
	require.NoError(t, err)
	_, err = client.StoragePoliciesNamed(t.Context(), "fast")
	require.NoError(t, err)

	requests := api.recorded()
	require.Len(t, requests, 3, "the token is fetched once per client")

	assert.Equal(t, wireTokenPath, requests[0].path)
	assert.Equal(t, "client_credentials", requests[0].params.Get("grant_type"))

	scoped := requests[1]
	assert.Equal(t, http.MethodGet, scoped.method)
	assert.Equal(t, wireStoragePolicyListPath, scoped.path, "a trailing slash on controllerUrl does not double up")
	assert.Equal(t, "bearer token", scoped.auth)
	assert.Equal(t, "42", scoped.params.Get("account_id"), "GET parameters travel form-encoded in the body")

	assert.False(t, requests[2].params.Has("account_id"), "StoragePoliciesNamed asks across accounts")
}

func TestClientMatchesNamesExactly(t *testing.T) {
	api, client := newFakeAPI(t)
	api.accounts = []Account{{ID: 1, Name: "acc"}, {ID: 2, Name: "acc_staging"}}
	api.policies = []StoragePolicy{{Name: "fast"}, {Name: "fast-ssd"}}

	accounts, err := client.AccountsNamed(t.Context(), "acc")
	require.NoError(t, err)
	assert.Equal(t, []Account{{ID: 1, Name: "acc"}}, accounts)

	policies, err := client.StoragePoliciesNamed(t.Context(), "fast")
	require.NoError(t, err)
	assert.Equal(t, []StoragePolicy{{Name: "fast"}}, policies)

	policies, err = client.StoragePoliciesNamed(t.Context(), "fas")
	require.NoError(t, err)
	assert.Empty(t, policies, "a prefix is not the name")
}

func TestClientReadsAMissingStoragePolicyRouteAsAPlatformOlderThan46(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusNotImplemented} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			api, client := newFakeAPI(t)
			api.statusFor[wireStoragePolicyListPath] = status

			_, err := client.StoragePoliciesNamed(t.Context(), "fast")
			require.ErrorIs(t, err, ErrStoragePolicyAPIMissing)
			assert.False(t, IsTransient(err), "the version is a verdict, not a blip")

			_, err = client.AccountStoragePoliciesNamed(t.Context(), 42, "fast")
			require.ErrorIs(t, err, ErrStoragePolicyAPIMissing)
		})
	}
}

// The meaning of a missing route belongs to the storage policy API: the same
// status on another endpoint says nothing about the platform version.
func TestClientDoesNotReadAMissingAccountRouteAsTheVersion(t *testing.T) {
	api, client := newFakeAPI(t)
	api.statusFor[wireAccountListPath] = http.StatusNotFound

	_, err := client.AccountsNamed(t.Context(), "acc")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrStoragePolicyAPIMissing)
	assert.ErrorContains(t, err, "answered 404")
}

func TestClientSeparatesBlipsFromVerdicts(t *testing.T) {
	t.Run("a 5xx is worth another attempt", func(t *testing.T) {
		api, client := newFakeAPI(t)
		api.statusFor[wireStoragePolicyListPath] = http.StatusServiceUnavailable

		_, err := client.StoragePoliciesNamed(t.Context(), "fast")
		assert.True(t, IsTransient(err))
	})

	t.Run("a 4xx is the platform's answer", func(t *testing.T) {
		api, client := newFakeAPI(t)
		api.statusFor[wireAccountListPath] = http.StatusForbidden

		_, err := client.AccountsNamed(t.Context(), "acc")
		assert.False(t, IsTransient(err))
		assert.ErrorContains(t, err, "answered 403: status 403")
	})

	t.Run("refused credentials are a verdict", func(t *testing.T) {
		api, client := newFakeAPI(t)
		api.statusFor[wireTokenPath] = http.StatusUnauthorized

		_, err := client.AccountsNamed(t.Context(), "acc")
		require.ErrorIs(t, err, ErrCredentialsRefused)
		assert.False(t, IsTransient(err))
	})

	t.Run("an SSO hiccup is a blip", func(t *testing.T) {
		api, client := newFakeAPI(t)
		api.statusFor[wireTokenPath] = http.StatusBadGateway

		_, err := client.AccountsNamed(t.Context(), "acc")
		assert.True(t, IsTransient(err))
		assert.NotErrorIs(t, err, ErrCredentialsRefused)
	})

	t.Run("an unreachable platform is a blip", func(t *testing.T) {
		server := httptest.NewServer(http.NotFoundHandler())
		serverURL := server.URL
		server.Close()
		client, err := NewClient(ProviderConfig{ControllerURL: serverURL, OAuth2URL: serverURL})
		require.NoError(t, err)

		_, err = client.AccountsNamed(t.Context(), "acc")
		assert.True(t, IsTransient(err))
		assert.ErrorContains(t, err, "reach "+serverURL)
	})
}

func TestNewClientNamesTheFieldItCannotUse(t *testing.T) {
	tests := []struct {
		name     string
		provider ProviderConfig
		field    string
		problem  string
	}{
		{
			name:     "no controllerUrl",
			provider: ProviderConfig{OAuth2URL: "https://sso.example.com"},
			field:    "provider.controllerUrl",
			problem:  "is not set",
		},
		{
			name:     "relative controllerUrl",
			provider: ProviderConfig{ControllerURL: "controller.example.com", OAuth2URL: "https://sso.example.com"},
			field:    "provider.controllerUrl",
			problem:  "must be an absolute URL",
		},
		{
			name:     "no oAuth2Url",
			provider: ProviderConfig{ControllerURL: "https://controller.example.com"},
			field:    "provider.oAuth2Url",
			problem:  "is not set",
		},
		{
			name:     "oAuth2Url that does not parse",
			provider: ProviderConfig{ControllerURL: "https://controller.example.com", OAuth2URL: "https://sso\x7f.example.com"},
			field:    "provider.oAuth2Url",
			problem:  "is not a valid URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.provider)

			var configErr *ConfigError
			require.ErrorAs(t, err, &configErr)
			assert.Equal(t, tt.field, configErr.Field)
			assert.Contains(t, configErr.Problem, tt.problem)
		})
	}
}

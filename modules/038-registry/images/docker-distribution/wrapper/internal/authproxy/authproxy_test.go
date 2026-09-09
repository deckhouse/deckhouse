/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package authproxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/registry-distribution/internal/config"
)

// TestATokenRequestReachesTheServiceAndNothingElseDoes is the whole of what this mounts.
func TestATokenRequestReachesTheServiceAndNothingElseDoes(t *testing.T) {
	var asked struct {
		path       string
		host       string
		query      string
		forwarded  string
		authorized string
	}

	service := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		asked.path = request.URL.Path
		asked.host = request.Host
		asked.query = request.URL.RawQuery
		asked.forwarded = request.Header.Get("X-Forwarded-For")
		asked.authorized = request.Header.Get("Authorization")
		_, _ = writer.Write([]byte(`{"token":"issued"}`))
	}))
	t.Cleanup(service.Close)

	registry := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("the registry"))
	})

	handler, err := Handler(&config.AuthProxy{URL: service.URL + "/auth"}, registry)
	require.NoError(t, err)

	front := httptest.NewServer(handler)
	t.Cleanup(front.Close)

	t.Run("the token path", func(t *testing.T) {
		request, err := http.NewRequest(http.MethodGet,
			front.URL+Path+"?service=Deckhouse+registry&scope=repository:system/deckhouse:pull", nil)
		require.NoError(t, err)
		request.SetBasicAuth("registry-ro", "password")

		response, err := http.DefaultClient.Do(request)
		require.NoError(t, err)
		t.Cleanup(func() { _ = response.Body.Close() })

		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)

		assert.Equal(t, `{"token":"issued"}`, string(body))
		assert.Equal(t, "/auth", asked.path,
			"the service serves its own path, whatever path the client was told to ask this registry for")
		assert.Contains(t, asked.query, "scope=repository:system/deckhouse:pull",
			"the query is the request: it says which repository and which actions")
		assert.NotEmpty(t, asked.authorized,
			"the client's own credentials travel to the service that verifies them")
		assert.Equal(t, front.Listener.Addr().String(), asked.host,
			"the address the client used, because the service puts it in the token")
		assert.NotEmpty(t, asked.forwarded)
	})

	t.Run("everything else is the registry's", func(t *testing.T) {
		response, err := http.Get(front.URL + "/v2/system/deckhouse/manifests/v1")
		require.NoError(t, err)
		t.Cleanup(func() { _ = response.Body.Close() })

		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		assert.Equal(t, "the registry", string(body))
	})
}

// TestWithoutATokenServiceTheRegistryIsUntouched: a cluster whose registry needs no tokens must not
// be given a path that answers a challenge with an address refusing connections.
func TestWithoutATokenServiceTheRegistryIsUntouched(t *testing.T) {
	registry := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("the registry"))
	})

	handler, err := Handler(nil, registry)
	require.NoError(t, err)

	front := httptest.NewServer(handler)
	t.Cleanup(front.Close)

	response, err := http.Get(front.URL + Path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = response.Body.Close() })

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, "the registry", string(body))
}

func TestHandlerRefusesAnUnparseableService(t *testing.T) {
	_, err := Handler(&config.AuthProxy{URL: "://not-a-url"}, http.NotFoundHandler())
	require.Error(t, err)
}

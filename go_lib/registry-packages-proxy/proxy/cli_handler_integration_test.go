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

package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	crregistry "github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	v1remote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
)

// newRealRegistry starts an in-process OCI registry speaking the actual
// distribution protocol, 404 NAME_UNKNOWN for unknown repositories included.
func newRealRegistry(t *testing.T) (host string) {
	t.Helper()
	srv := httptest.NewServer(crregistry.New())
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://")
}

func pushPluginImage(t *testing.T, host, repo, tag string) {
	t.Helper()
	img, err := random.Image(256, 1)
	require.NoError(t, err)
	ref, err := name.NewTag(host+"/"+repo+":"+tag, name.WeakValidation)
	require.NoError(t, err)
	require.NoError(t, v1remote.Write(ref, img))
}

// TestCLIHandler_RealRegistryProbing drives the whole pipeline - the HTTP
// routes, the probing, the production DefaultClient and a real distribution
// registry - for both supported layouts and for an absent image. The fake
// registry client is deliberately not used here.
func TestCLIHandler_RealRegistryProbing(t *testing.T) {
	listTags := func(t *testing.T, srv *httptest.Server, urlPath string) (int, cliTagsResponse) {
		t.Helper()
		resp, err := http.Get(srv.URL + urlPath)
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		_ = resp.Body.Close()
		var tags cliTagsResponse
		if resp.StatusCode == http.StatusOK {
			require.NoError(t, json.Unmarshal(body, &tags))
		}
		return resp.StatusCode, tags
	}

	t.Run("mirrored layout is served from the cluster repo", func(t *testing.T) {
		host := newRealRegistry(t)
		// `d8 mirror push <host>/dest/ee` uploads the plugin right under the
		// target; the pre-probing proxy looked at <host>/dest and answered 404.
		pushPluginImage(t, host, "dest/ee/deckhouse-cli/plugins/foo", "v0.1.0")

		getter := &fakeCLIGetter{cfg: &registry.ClientConfig{Repository: host + "/dest/ee", Scheme: "http"}}
		p := newTestProxy(t, &registry.DefaultClient{}, getter, nil)
		srv := newCLITestServer(t, p)

		status, tags := listTags(t, srv, "/v1/images/deckhouse-cli/plugins/foo/tags")
		require.Equal(t, http.StatusOK, status)
		assert.Equal(t, []string{"v0.1.0"}, tags.Tags)

		assert.Equal(t, http.StatusOK, getStatus(t, srv, "/v1/images/deckhouse-cli/plugins/foo/manifests/v0.1.0"))
		assert.Equal(t, http.StatusOK, getStatus(t, srv, "/v1/images/deckhouse-cli/plugins/foo/images/v0.1.0"))
	})

	t.Run("official layout falls through to the trimmed root", func(t *testing.T) {
		host := newRealRegistry(t)
		// The official registry publishes CLI artifacts above the edition:
		// the first probe of <host>/deckhouse/ee/... meets a genuine 404.
		pushPluginImage(t, host, "deckhouse/deckhouse-cli/plugins/foo", "v0.1.0")

		getter := &fakeCLIGetter{cfg: &registry.ClientConfig{Repository: host + "/deckhouse/ee", Scheme: "http"}}
		p := newTestProxy(t, &registry.DefaultClient{}, getter, nil)
		srv := newCLITestServer(t, p)

		status, tags := listTags(t, srv, "/v1/images/deckhouse-cli/plugins/foo/tags")
		require.Equal(t, http.StatusOK, status)
		assert.Equal(t, []string{"v0.1.0"}, tags.Tags)

		assert.Equal(t, http.StatusOK, getStatus(t, srv, "/v1/images/deckhouse-cli/plugins/foo/images/v0.1.0"))
	})

	t.Run("absent image answers 404", func(t *testing.T) {
		host := newRealRegistry(t)
		getter := &fakeCLIGetter{cfg: &registry.ClientConfig{Repository: host + "/dest/ee", Scheme: "http"}}
		p := newTestProxy(t, &registry.DefaultClient{}, getter, nil)
		srv := newCLITestServer(t, p)

		assert.Equal(t, http.StatusNotFound, getStatus(t, srv, "/v1/images/deckhouse-cli/plugins/foo/tags"))
	})
}

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

package upstream

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/registry-distribution/internal/config"
)

func TestRewriteMapsTheScopeAndNothingElse(t *testing.T) {
	rewriter := &Rewriter{Local: "/v2/system/deckhouse", Remote: "/v2/deckhouse/ee"}

	cases := map[string]string{
		// The scope itself and everything under it.
		"/v2/system/deckhouse":                               "/v2/deckhouse/ee",
		"/v2/system/deckhouse/manifests/v1.77.0":             "/v2/deckhouse/ee/manifests/v1.77.0",
		"/v2/system/deckhouse/modules/sds/manifests/v0.6.10": "/v2/deckhouse/ee/modules/sds/manifests/v0.6.10",
		"/v2/system/deckhouse/blobs/sha256:aa":               "/v2/deckhouse/ee/blobs/sha256:aa",
		// The version endpoint, which every client checks first and the cache pings to establish a
		// challenge. Rewriting it would point the challenge at a repository instead of the API.
		"/v2/": "/v2/",
		// A name that merely starts with the same letters is a different repository.
		"/v2/system/deckhouse-extra/manifests/v1": "/v2/system/deckhouse-extra/manifests/v1",
		// Outside the scope: forwarded as it came. The cache asks about nothing else, and refusing
		// here would turn a routing question into an outage.
		"/v2/library/nginx/manifests/latest": "/v2/library/nginx/manifests/latest",
	}

	for path, want := range cases {
		assert.Equalf(t, want, rewriter.rewrite(path), "path %s", path)
	}
}

// TestRewriteIsATransparentForwarderWhenThePathsAgree covers the mirror that serves the same layout:
// there is nothing to map, and the rewriter must not invent a mapping.
func TestRewriteIsATransparentForwarderWhenThePathsAgree(t *testing.T) {
	same := &Rewriter{Local: "/v2/system/deckhouse", Remote: "/v2/system/deckhouse"}
	assert.Equal(t, "/v2/system/deckhouse/manifests/v1",
		same.rewrite("/v2/system/deckhouse/manifests/v1"))

	none := &Rewriter{Local: "/v2/system/deckhouse"}
	assert.Equal(t, "/v2/system/deckhouse/manifests/v1",
		none.rewrite("/v2/system/deckhouse/manifests/v1"))
}

// TestTheRewriterReachesTheUpstreamUnderItsOwnName is the whole hop, end to end: what the cache
// sends, what the upstream receives, and what comes back.
func TestTheRewriterReachesTheUpstreamUnderItsOwnName(t *testing.T) {
	var received struct {
		path  string
		host  string
		user  string
		token bool
	}

	real := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		received.path = request.URL.Path
		received.host = request.Host
		user, password, ok := request.BasicAuth()
		received.user, received.token = user, ok && password == "secret"
		writer.Header().Set("Docker-Content-Digest", "sha256:cafe")
		_, _ = writer.Write([]byte("a manifest"))
	}))
	t.Cleanup(real.Close)

	target, err := url.Parse(real.URL)
	require.NoError(t, err)

	rewriter := &Rewriter{
		Local:    "/v2/system/deckhouse",
		Remote:   "/v2/deckhouse/ee",
		Target:   *target,
		Username: "robot",
		Password: "secret",
	}

	front := httptest.NewServer(rewriter.Handler())
	t.Cleanup(front.Close)

	response, err := http.Get(front.URL + "/v2/system/deckhouse/manifests/v1.77.0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = response.Body.Close() })

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "a manifest", string(body))
	assert.Equal(t, "sha256:cafe", response.Header.Get("Docker-Content-Digest"))

	assert.Equal(t, "/v2/deckhouse/ee/manifests/v1.77.0", received.path,
		"the upstream must be asked under its own path")
	assert.Equal(t, target.Host, received.host,
		"the Host header follows the URL, or a registry serving several names answers for the wrong one")
	assert.Equal(t, "robot", received.user)
	assert.True(t, received.token, "the credentials for the upstream are added here")
}

// TestAnUnreachableUpstreamIsAGatewayFailure keeps the two failures apart: the cache has to see that
// the hop to the upstream failed, not that this registry is broken.
func TestAnUnreachableUpstreamIsAGatewayFailure(t *testing.T) {
	// A port nothing listens on: the address is well-formed, the dial is refused.
	rewriter := &Rewriter{
		Local:  "/v2/system/deckhouse",
		Remote: "/v2/deckhouse/ee",
		Target: url.URL{Scheme: "http", Host: "127.0.0.1:1"},
	}

	front := httptest.NewServer(rewriter.Handler())
	t.Cleanup(front.Close)

	response, err := http.Get(front.URL + "/v2/system/deckhouse/manifests/v1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = response.Body.Close() })

	assert.Equal(t, http.StatusBadGateway, response.StatusCode)
}

func TestNewIsSilentWithoutAnUpstream(t *testing.T) {
	// An air-gapped cluster: the store is authoritative and there is nothing to rewrite towards.
	rewriter, err := New(&config.Wrapper{Scope: "system/deckhouse"}, "127.0.0.1:5004", nil)
	require.NoError(t, err)
	assert.Nil(t, rewriter, "no upstream means no loopback listener at all")
}

func TestNewRefusesAnUnusableCertificateAuthority(t *testing.T) {
	broken := filepath.Join(t.TempDir(), "ca.crt")
	require.NoError(t, os.WriteFile(broken, []byte("this is not a certificate"), 0o600))

	_, err := New(&config.Wrapper{
		Scope:    "system/deckhouse",
		Upstream: &config.Upstream{Address: "registry.example.com", CA: broken},
	}, "127.0.0.1:5004", nil)

	require.Error(t, err, "starting with an authority that verifies nothing would fail every miss instead")
	assert.Contains(t, err.Error(), "no certificate")
}

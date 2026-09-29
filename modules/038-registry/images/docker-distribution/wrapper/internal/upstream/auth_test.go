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
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bearerUpstream wants a token for the upstream's own repository name and ignores everything else;
// the token service counts how often it is asked.
func bearerUpstream(t *testing.T, exchanges *atomic.Int32) *httptest.Server {
	t.Helper()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/auth" {
			exchanges.Add(1)
			if user, pass, ok := request.BasicAuth(); !ok || user != "robot" || pass != "secret" {
				http.Error(writer, "bad credentials", http.StatusUnauthorized)
				return
			}
			_, _ = fmt.Fprintf(writer, `{"token":"t-%s"}`, request.URL.Query().Get("scope"))
			return
		}

		scope := "repository:" + repositoryOf(request.URL.Path) + ":pull"
		if request.Header.Get("Authorization") != "Bearer t-"+scope {
			writer.Header().Set("Www-Authenticate",
				fmt.Sprintf(`Bearer realm="%s/auth",service="Docker registry",scope="%s"`, server.URL, scope))
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = writer.Write([]byte("a manifest"))
	}))
	t.Cleanup(server.Close)
	return server
}

func frontFor(t *testing.T, real *httptest.Server, username, password string) *httptest.Server {
	t.Helper()
	target, err := url.Parse(real.URL)
	require.NoError(t, err)

	rewriter := &Rewriter{
		Local:    "/v2/system/deckhouse",
		Remote:   "/v2/deckhouse/ee",
		Target:   *target,
		Username: username,
		Password: password,
	}
	front := httptest.NewServer(rewriter.Handler())
	t.Cleanup(front.Close)
	return front
}

func get(t *testing.T, address, authorization string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, address, nil)
	require.NoError(t, err)
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

func TestATokenIsObtainedUnderTheUpstreamsNameAndKept(t *testing.T) {
	var exchanges atomic.Int32
	front := frontFor(t, bearerUpstream(t, &exchanges), "robot", "secret")

	for range 3 {
		// Whatever the cache sends is its own idea of authentication, and never the upstream's.
		response := get(t, front.URL+"/v2/system/deckhouse/modules/upmeter/manifests/v1.0.9", "Bearer from-the-cache")
		assert.Equal(t, http.StatusOK, response.StatusCode)
	}
	assert.Equal(t, int32(1), exchanges.Load(), "one token serves every request for the repository")

	response := get(t, front.URL+"/v2/system/deckhouse/modules/console/manifests/v1.65.1", "")
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, int32(2), exchanges.Load(), "and another repository needs a token of its own")
}

func TestARefusedTokenReachesTheCacheWithoutAChallenge(t *testing.T) {
	var exchanges atomic.Int32
	front := frontFor(t, bearerUpstream(t, &exchanges), "robot", "wrong")

	response := get(t, front.URL+"/v2/system/deckhouse/modules/upmeter/manifests/v1.0.9", "")
	assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	assert.Empty(t, response.Header.Get("Www-Authenticate"),
		"a challenge would send the cache to the upstream's token service under the cluster's names")
}

func TestTheVersionCheckIsAnsweredWithoutAChallenge(t *testing.T) {
	var exchanges atomic.Int32
	front := frontFor(t, bearerUpstream(t, &exchanges), "robot", "secret")

	response := get(t, front.URL+"/v2/", "")
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Empty(t, response.Header.Get("Www-Authenticate"))
	assert.Equal(t, "registry/2.0", response.Header.Get("Docker-Distribution-Api-Version"))
	assert.Zero(t, exchanges.Load())
}

func TestRepositoryOf(t *testing.T) {
	cases := map[string]string{
		"/v2/deckhouse/ee/manifests/v1.77.0":         "deckhouse/ee",
		"/v2/deckhouse/ee/modules/upmeter/tags/list": "deckhouse/ee/modules/upmeter",
		"/v2/deckhouse/ee/blobs/sha256:aa":           "deckhouse/ee",
		"/v2/deckhouse/ee/referrers/sha256:aa":       "deckhouse/ee",
		"/v2/team/manifests/app/manifests/latest":    "team/manifests/app",
		"/v2/team/blobs/app/blobs/sha256:aa":         "team/blobs/app",
		"/v2/":                                       "",
		"/v2/_catalog":                               "",
		"/somewhere/else":                            "",
	}
	for path, want := range cases {
		assert.Equalf(t, want, repositoryOf(path), "path %s", path)
	}
}

func TestParseChallengeParameters(t *testing.T) {
	parameters := parseChallengeParameters(
		`realm="https://registry.example.com/auth", service="Docker registry",` +
			`scope="repository:a:pull repository:b:pull",error=insufficient_scope, quote="a\"b"`)

	assert.Equal(t, map[string]string{
		"realm":   "https://registry.example.com/auth",
		"service": "Docker registry",
		"scope":   "repository:a:pull repository:b:pull",
		"error":   "insufficient_scope",
		"quote":   `a"b`,
	}, parameters)
}

func TestCredentialsAreNotSentInTheClearToAnotherHost(t *testing.T) {
	a := newAuthenticator(nil, url.URL{Scheme: "http", Host: "upstream:5000"}, "robot", "secret", nil)

	assert.True(t, a.mayReceiveCredentials(&url.URL{Scheme: "https", Host: "auth.example.com"}))
	assert.True(t, a.mayReceiveCredentials(&url.URL{Scheme: "http", Host: "upstream:5000"}))
	assert.False(t, a.mayReceiveCredentials(&url.URL{Scheme: "http", Host: "auth.example.com"}))
}

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

package probe

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// registryStub is a registry just complete enough to exercise the probe: the
// version endpoint, an optional authentication challenge and a token endpoint,
// and the sentinel tag list.
type registryStub struct {
	// auth selects the challenge: "", "bearer" or "basic".
	auth string

	// credentials that the stub accepts. Empty means it accepts any.
	username, password string

	// tags served for the sentinel repository. A nil slice means the repository
	// does not exist.
	tags []string

	// versionStatus overrides the status of /v2/.
	versionStatus int

	server *httptest.Server

	// requestedScopes records what the token endpoint was asked for.
	requestedScopes []string
}

func (s *registryStub) start(t *testing.T) *registryStub {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/v2/", func(w http.ResponseWriter, req *http.Request) {
		if strings.HasSuffix(req.URL.Path, "/tags/list") {
			s.handleTags(w, req)
			return
		}

		if s.versionStatus != 0 {
			w.WriteHeader(s.versionStatus)
			return
		}
		switch s.auth {
		case "bearer":
			w.Header().Set("WWW-Authenticate",
				fmt.Sprintf(`Bearer realm="%s/token",service="registry"`, s.server.URL))
			w.WriteHeader(http.StatusUnauthorized)
		case "basic":
			w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
			w.WriteHeader(http.StatusUnauthorized)
		default:
			w.WriteHeader(http.StatusOK)
		}
	})

	mux.HandleFunc("/token", func(w http.ResponseWriter, req *http.Request) {
		s.requestedScopes = append(s.requestedScopes, req.URL.Query().Get("scope"))

		if !s.credentialsOK(req) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"issued-token"}`))
	})

	s.server = httptest.NewServer(mux)
	t.Cleanup(s.server.Close)
	return s
}

func (s *registryStub) handleTags(w http.ResponseWriter, req *http.Request) {
	if s.auth == "basic" && !s.credentialsOK(req) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if s.auth == "bearer" && req.Header.Get("Authorization") != "Bearer issued-token" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if s.tags == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	quoted := make([]string, 0, len(s.tags))
	for _, tag := range s.tags {
		quoted = append(quoted, `"`+tag+`"`)
	}
	_, _ = fmt.Fprintf(w, `{"name":"release-channel","tags":[%s]}`, strings.Join(quoted, ","))
}

func (s *registryStub) credentialsOK(req *http.Request) bool {
	if s.username == "" && s.password == "" {
		return true
	}
	username, password, ok := req.BasicAuth()
	return ok && username == s.username && password == s.password
}

func (s *registryStub) endpoint(path string) *registryv1alpha1.Upstream {
	parsed, _ := url.Parse(s.server.URL)
	return &registryv1alpha1.Upstream{
		Endpoint: registryv1alpha1.Endpoint{
			Scheme: registryv1alpha1.SchemeHTTP,
			Host:   parsed.Host,
			Path:   path,
		},
	}
}

func probeIt(t *testing.T, upstream *registryv1alpha1.Upstream) error {
	t.Helper()

	prober := &Registry{Timeout: 5 * time.Second}
	return prober.Probe(context.Background(), upstream)
}

func TestProbeAcceptsAnAnonymousRegistry(t *testing.T) {
	stub := (&registryStub{tags: []string{"stable", "early-access"}}).start(t)

	require.NoError(t, probeIt(t, stub.endpoint("/deckhouse/ce")))
}

func TestProbeAcceptsABearerRegistry(t *testing.T) {
	stub := (&registryStub{
		auth: "bearer", username: "license-token", password: "key",
		tags: []string{"stable"},
	}).start(t)

	upstream := stub.endpoint("/deckhouse/ee")
	upstream.Auth = &registryv1alpha1.Auth{Username: "license-token", Password: "key"}

	require.NoError(t, probeIt(t, upstream))

	// The token is requested for the sentinel repository under the configured path,
	// so a wrong path fails here rather than silently succeeding on a broad scope.
	require.NotEmpty(t, stub.requestedScopes)
	assert.Equal(t, "repository:deckhouse/ee/release-channel:pull", stub.requestedScopes[0])
}

func TestProbeAcceptsABasicRegistry(t *testing.T) {
	stub := (&registryStub{
		auth: "basic", username: "user", password: "password",
		tags: []string{"stable"},
	}).start(t)

	upstream := stub.endpoint("/deckhouse/ee")
	upstream.Auth = &registryv1alpha1.Auth{Username: "user", Password: "password"}

	require.NoError(t, probeIt(t, upstream))
}

// TestProbeRejectsWrongCredentials is the license case: the address is right, the
// registry is up, and the credentials are not accepted. This is what must stop the
// switch instead of taking the cluster down.
func TestProbeRejectsWrongCredentials(t *testing.T) {
	stub := (&registryStub{
		auth: "bearer", username: "license-token", password: "the-right-key",
		tags: []string{"stable"},
	}).start(t)

	upstream := stub.endpoint("/deckhouse/ee")
	upstream.Auth = &registryv1alpha1.Auth{Username: "license-token", Password: "the-wrong-key"}

	err := probeIt(t, upstream)
	require.Error(t, err)

	failure, ok := AsFailure(err)
	require.True(t, ok)
	assert.Equal(t, FailureAuth, failure.Kind)
	assert.Equal(t, registryv1alpha1.ReasonAuthFailed, failure.Reason())
}

func TestProbeRejectsWrongBasicCredentials(t *testing.T) {
	stub := (&registryStub{
		auth: "basic", username: "user", password: "right",
		tags: []string{"stable"},
	}).start(t)

	upstream := stub.endpoint("/deckhouse/ee")
	upstream.Auth = &registryv1alpha1.Auth{Username: "user", Password: "wrong"}

	failure, ok := AsFailure(probeIt(t, upstream))
	require.True(t, ok)
	assert.Equal(t, FailureAuth, failure.Kind)
}

func TestProbeRejectsMissingCredentials(t *testing.T) {
	stub := (&registryStub{auth: "bearer", username: "u", password: "p", tags: []string{"stable"}}).start(t)

	failure, ok := AsFailure(probeIt(t, stub.endpoint("/deckhouse/ee")))
	require.True(t, ok)
	assert.Equal(t, FailureAuth, failure.Kind)
	assert.Contains(t, failure.Message, "no credentials are configured")
}

// TestProbeRejectsAReachableButWrongRegistry is the other half of the point: a
// registry that answers and accepts the credentials but holds no Deckhouse content
// at this path. A typo in the path looks exactly like this.
func TestProbeRejectsAReachableButWrongRegistry(t *testing.T) {
	stub := (&registryStub{tags: nil}).start(t)

	failure, ok := AsFailure(probeIt(t, stub.endpoint("/wrong/path")))
	require.True(t, ok)
	assert.Equal(t, FailureSentinel, failure.Kind)
	assert.Equal(t, registryv1alpha1.ReasonSentinelMissing, failure.Reason())
	assert.Contains(t, failure.Message, "/wrong/path")
}

func TestProbeRejectsAnEmptySentinelRepository(t *testing.T) {
	// An empty repository would pass an existence check and then serve nothing.
	stub := (&registryStub{tags: []string{}}).start(t)

	failure, ok := AsFailure(probeIt(t, stub.endpoint("/deckhouse/ee")))
	require.True(t, ok)
	assert.Equal(t, FailureSentinel, failure.Kind)
	assert.Contains(t, failure.Message, "is empty")
}

func TestProbeRejectsAnUnreachableRegistry(t *testing.T) {
	upstream := &registryv1alpha1.Upstream{
		Endpoint: registryv1alpha1.Endpoint{
			Scheme: registryv1alpha1.SchemeHTTP,
			// Reserved by RFC 6761 for exactly this, and guaranteed not to resolve.
			Host: "registry.invalid",
			Path: "/deckhouse/ee",
		},
	}

	failure, ok := AsFailure(probeIt(t, upstream))
	require.True(t, ok)
	assert.Equal(t, FailureUnreachable, failure.Kind)
	assert.Equal(t, registryv1alpha1.ReasonUnreachable, failure.Reason())
}

func TestProbeRejectsANonRegistryEndpoint(t *testing.T) {
	// Something is listening and answering, but it is not a registry API. A load
	// balancer pointed at the wrong backend behaves this way.
	stub := (&registryStub{versionStatus: http.StatusServiceUnavailable}).start(t)

	failure, ok := AsFailure(probeIt(t, stub.endpoint("/deckhouse/ee")))
	require.True(t, ok)
	assert.Equal(t, FailureUnreachable, failure.Kind)
	assert.Contains(t, failure.Message, "503")
}

// TestProbeAcceptsNoUpstream covers air-gap: there is nothing to verify, and the
// transition is gated on cache completeness instead.
func TestProbeAcceptsNoUpstream(t *testing.T) {
	require.NoError(t, probeIt(t, nil))
}

// TestProbeIgnoresBrokenMirrors keeps redundancy from becoming a single point of
// failure: mirrors serve the same content and exist for failover, so one being
// down is not a reason to refuse a switch the cluster would survive.
func TestProbeIgnoresBrokenMirrors(t *testing.T) {
	stub := (&registryStub{tags: []string{"stable"}}).start(t)

	upstream := stub.endpoint("/deckhouse/ce")
	upstream.Mirrors = []registryv1alpha1.Endpoint{
		{Scheme: registryv1alpha1.SchemeHTTP, Host: "mirror.invalid", Path: "/deckhouse/ce"},
	}

	require.NoError(t, probeIt(t, upstream))
}

func TestProbeHonoursTheTimeout(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(slow.Close)

	parsed, _ := url.Parse(slow.URL)
	prober := &Registry{Timeout: 100 * time.Millisecond}

	started := time.Now()
	err := prober.Probe(context.Background(), &registryv1alpha1.Upstream{
		Endpoint: registryv1alpha1.Endpoint{
			Scheme: registryv1alpha1.SchemeHTTP, Host: parsed.Host, Path: "/deckhouse/ee",
		},
	})

	require.Error(t, err)
	// A probe that hangs must not hold the reconciliation: everything else the
	// controller does still has to happen.
	assert.Less(t, time.Since(started), time.Second)
}

func TestProbeRejectsAnInvalidCertificateAuthority(t *testing.T) {
	stub := (&registryStub{tags: []string{"stable"}}).start(t)

	upstream := stub.endpoint("/deckhouse/ee")
	upstream.CA = "this is not a certificate"

	failure, ok := AsFailure(probeIt(t, upstream))
	require.True(t, ok)
	assert.Equal(t, FailureUnreachable, failure.Kind)
	assert.Contains(t, failure.Message, "cannot build a client")
}

func TestSentinelRepositoryPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "/deckhouse/ee", want: "deckhouse/ee/release-channel"},
		{path: "deckhouse/ee/", want: "deckhouse/ee/release-channel"},
		{path: "", want: "release-channel"},
		{path: "/", want: "release-channel"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want,
				sentinelRepository(&registryv1alpha1.Endpoint{Path: tt.path}))
		})
	}
}

func TestParseChallenge(t *testing.T) {
	tests := []struct {
		name        string
		header      string
		wantNil     bool
		wantScheme  string
		wantRealm   string
		wantService string
	}{
		{name: "empty", header: "", wantNil: true},
		{
			name:        "bearer with a service",
			header:      `Bearer realm="https://auth.example.com/token",service="registry.example.com"`,
			wantScheme:  schemeBearer,
			wantRealm:   "https://auth.example.com/token",
			wantService: "registry.example.com",
		},
		{
			name:       "bearer without a service",
			header:     `Bearer realm="https://auth.example.com/token"`,
			wantScheme: schemeBearer,
			wantRealm:  "https://auth.example.com/token",
		},
		{name: "basic", header: `Basic realm="registry"`, wantScheme: schemeBasic},
		{
			name:       "lowercase scheme",
			header:     `bearer realm="https://auth/token"`,
			wantScheme: schemeBearer,
			wantRealm:  "https://auth/token",
		},
		// A bearer challenge with no realm gives nothing to exchange against.
		{name: "bearer without a realm", header: `Bearer service="registry"`, wantNil: true},
		{name: "an unsupported scheme", header: `Negotiate`, wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseChallenge(tt.header)

			if tt.wantNil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tt.wantScheme, got.scheme)
			assert.Equal(t, tt.wantRealm, got.realm)
			assert.Equal(t, tt.wantService, got.service)
		})
	}
}

func TestChallengeTokenURL(t *testing.T) {
	ch := &challenge{
		scheme:  schemeBearer,
		realm:   "https://auth.example.com/token",
		service: "registry.example.com",
	}

	got, err := ch.tokenURL("repository:deckhouse/ee/release-channel:pull")
	require.NoError(t, err)

	parsed, err := url.Parse(got)
	require.NoError(t, err)
	assert.Equal(t, "auth.example.com", parsed.Host)
	assert.Equal(t, "/token", parsed.Path)
	assert.Equal(t, "repository:deckhouse/ee/release-channel:pull", parsed.Query().Get("scope"))
	assert.Equal(t, "registry.example.com", parsed.Query().Get("service"))

	_, err = (&challenge{realm: "/not-absolute"}).tokenURL("scope")
	assert.Error(t, err, "a relative realm cannot be used to obtain a token")
}

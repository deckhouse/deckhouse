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

package proxy

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// registryStub is a registry just complete enough to be proxied to: it records what it
// was asked for and answers what the test told it to.
type registryStub struct {
	name string

	// challenge selects the authentication it demands: "", "basic" or "bearer".
	challenge string

	// username and password it accepts.
	username, password string

	// status of the manifest response, and its body.
	status int
	body   string

	// delay before answering, to exercise the timeout.
	delay time.Duration

	server *httptest.Server

	// requests records the paths asked for, and headers records the manifest request's
	// headers.
	requests atomic.Value
	headers  atomic.Value
	tokens   atomic.Int32
}

func (s *registryStub) start(t *testing.T) *registryStub {
	t.Helper()

	s.requests.Store([]string{})
	mux := http.NewServeMux()

	mux.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/v2/" {
			if s.challenge == "" {
				writer.WriteHeader(http.StatusOK)
				return
			}
			s.demandAuth(writer)
			return
		}

		s.requests.Store(append(s.requests.Load().([]string), request.URL.Path))
		s.headers.Store(request.Header.Clone())

		if s.delay > 0 {
			time.Sleep(s.delay)
		}

		if !s.authorized(request) {
			// A real registry names its scheme on every 401, not only on the version
			// endpoint, which is how a client knows what to answer with.
			s.demandAuth(writer)
			return
		}

		status := s.status
		if status == 0 {
			status = http.StatusOK
		}
		writer.Header().Set("Docker-Content-Digest", "sha256:"+s.name)
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(s.body))
	})

	mux.HandleFunc("/token", func(writer http.ResponseWriter, request *http.Request) {
		s.tokens.Add(1)

		username, password, ok := request.BasicAuth()
		if !ok || username != s.username || password != s.password {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = writer.Write([]byte(`{"token":"issued-` + s.name + `","expires_in":300}`))
	})

	s.server = httptest.NewServer(mux)
	t.Cleanup(s.server.Close)
	return s
}

// demandAuth answers with the challenge this registry expects.
func (s *registryStub) demandAuth(writer http.ResponseWriter) {
	switch s.challenge {
	case "bearer":
		writer.Header().Set("WWW-Authenticate",
			fmt.Sprintf(`Bearer realm="%s/token",service="registry"`, s.server.URL))
	case "basic":
		writer.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
	}
	writer.WriteHeader(http.StatusUnauthorized)
}

func (s *registryStub) authorized(request *http.Request) bool {
	switch s.challenge {
	case "bearer":
		return request.Header.Get("Authorization") == "Bearer issued-"+s.name
	case "basic":
		username, password, ok := request.BasicAuth()
		return ok && username == s.username && password == s.password
	default:
		return true
	}
}

func (s *registryStub) host(t *testing.T) string {
	t.Helper()

	parsed, err := url.Parse(s.server.URL)
	require.NoError(t, err)
	return parsed.Host
}

func (s *registryStub) asked() []string {
	return s.requests.Load().([]string)
}

func newServer(spec *registryv1alpha1.RegistryNodeSpec) *Server {
	return &Server{
		Log:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		Layout:         LayoutFunc(func() *registryv1alpha1.RegistryNodeSpec { return spec }),
		Self:           self,
		ForwardTimeout: 5 * time.Second,
	}
}

func pull(t *testing.T, server *Server, namespace, path string) *http.Response {
	t.Helper()

	front := httptest.NewServer(server.Handler())
	t.Cleanup(front.Close)

	request, err := http.NewRequest(http.MethodGet, front.URL+path+"?ns="+url.QueryEscape(namespace), nil)
	require.NoError(t, err)
	request.Header.Set("Accept", "application/vnd.oci.image.manifest.v1+json")

	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

// pullWithHeaders sends a request with the given headers and returns what the target
// actually received.
func pullWithHeaders(
	t *testing.T, server *Server, namespace, path string, headers map[string]string, target *registryStub,
) http.Header {
	t.Helper()

	front := httptest.NewServer(server.Handler())
	t.Cleanup(front.Close)

	request, err := http.NewRequest(http.MethodGet,
		front.URL+path+"?ns="+url.QueryEscape(namespace), nil)
	require.NoError(t, err)
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	t.Cleanup(func() { _ = response.Body.Close() })

	return target.headers.Load().(http.Header)
}

func bodyOf(t *testing.T, response *http.Response) string {
	t.Helper()

	content, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return string(content)
}

func insecureBackend(name registryv1alpha1.BackendName, stub *registryStub, t *testing.T, path string,
	auth *registryv1alpha1.Auth,
) registryv1alpha1.Backend {
	return registryv1alpha1.Backend{
		Name: name,
		Endpoint: registryv1alpha1.Endpoint{
			Scheme: registryv1alpha1.SchemeHTTP,
			Host:   stub.host(t),
			Path:   path,
			Auth:   auth,
		},
	}
}

func TestServeAnswersThePingItself(t *testing.T) {
	// The runtime checks the endpoint before using it. Making that depend on a registry
	// being reachable would take the node down with the registry.
	response := pull(t, newServer(&registryv1alpha1.RegistryNodeSpec{}), constant.Host, "/v2/")

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "registry/2.0", response.Header.Get("Docker-Distribution-Api-Version"))
}

func TestServeUsesTheCacheFirst(t *testing.T) {
	cache := (&registryStub{name: "cache", body: "from-the-cache"}).start(t)
	upstream := (&registryStub{name: "upstream", body: "from-the-upstream"}).start(t)

	spec := &registryv1alpha1.RegistryNodeSpec{
		Cache: true,
		Backends: []registryv1alpha1.Backend{
			insecureBackend(registryv1alpha1.BackendStorage, cache, t, constant.Path, nil),
			insecureBackend(registryv1alpha1.BackendUpstream, upstream, t, "/deckhouse/ee", nil),
		},
	}

	response := pull(t, newServer(spec), constant.Host, "/v2/system/deckhouse/one/manifests/v1")

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "from-the-cache", bodyOf(t, response))
	assert.Equal(t, []string{"/v2/system/deckhouse/one/manifests/v1"}, cache.asked())
	assert.Empty(t, upstream.asked(), "the upstream must not be touched while the cache answers")

	// Response headers travel back, since the runtime verifies the digest against them.
	assert.Equal(t, "sha256:cache", response.Header.Get("Docker-Content-Digest"))
}

// TestServeFallsBackToTheUpstream is what the agent is on the pull path for: a cache
// that is still filling, or briefly broken, must not stop a node from pulling.
func TestServeFallsBackToTheUpstream(t *testing.T) {
	cache := (&registryStub{name: "cache", status: http.StatusInternalServerError}).start(t)
	upstream := (&registryStub{name: "upstream", body: "from-the-upstream"}).start(t)

	spec := &registryv1alpha1.RegistryNodeSpec{
		Cache: true,
		Backends: []registryv1alpha1.Backend{
			insecureBackend(registryv1alpha1.BackendStorage, cache, t, constant.Path, nil),
			insecureBackend(registryv1alpha1.BackendUpstream, upstream, t, "/deckhouse/ee", nil),
		},
	}

	response := pull(t, newServer(spec), constant.Host, "/v2/system/deckhouse/one/manifests/v1")

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "from-the-upstream", bodyOf(t, response))
	// And the prefix was swapped for the upstream, which serves the same image
	// elsewhere.
	assert.Equal(t, []string{"/v2/deckhouse/ee/one/manifests/v1"}, upstream.asked())
}

// TestServeFallsBackWhenTheCacheIsUnreachable covers the harder failure: not an error
// response but nothing answering at all.
func TestServeFallsBackWhenTheCacheIsUnreachable(t *testing.T) {
	upstream := (&registryStub{name: "upstream", body: "from-the-upstream"}).start(t)

	spec := &registryv1alpha1.RegistryNodeSpec{
		Cache: true,
		Backends: []registryv1alpha1.Backend{
			{
				Name: registryv1alpha1.BackendStorage,
				Endpoint: registryv1alpha1.Endpoint{
					Scheme: registryv1alpha1.SchemeHTTP,
					Host:   "storage.invalid",
					Path:   constant.Path,
				},
			},
			insecureBackend(registryv1alpha1.BackendUpstream, upstream, t, "/deckhouse/ee", nil),
		},
	}

	response := pull(t, newServer(spec), constant.Host, "/v2/system/deckhouse/one/manifests/v1")

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "from-the-upstream", bodyOf(t, response))
}

// TestServeDoesNotRetryAMissingImage draws a line the cache depends on: a 404 means
// the image is genuinely absent, and asking every backend for it would turn every
// missing tag into a fan-out. Filling on a miss is the cache's job, not the agent's.
func TestServeDoesNotRetryAMissingImage(t *testing.T) {
	cache := (&registryStub{name: "cache", status: http.StatusNotFound}).start(t)
	upstream := (&registryStub{name: "upstream", body: "from-the-upstream"}).start(t)

	spec := &registryv1alpha1.RegistryNodeSpec{
		Backends: []registryv1alpha1.Backend{
			insecureBackend(registryv1alpha1.BackendStorage, cache, t, constant.Path, nil),
			insecureBackend(registryv1alpha1.BackendUpstream, upstream, t, "/deckhouse/ee", nil),
		},
	}

	response := pull(t, newServer(spec), constant.Host, "/v2/system/deckhouse/one/manifests/v1")

	assert.Equal(t, http.StatusNotFound, response.StatusCode)
	assert.Empty(t, upstream.asked())
}

func TestServeEveryTargetFailing(t *testing.T) {
	cache := (&registryStub{name: "cache", status: http.StatusBadGateway}).start(t)
	upstream := (&registryStub{name: "upstream", status: http.StatusServiceUnavailable}).start(t)

	spec := &registryv1alpha1.RegistryNodeSpec{
		Backends: []registryv1alpha1.Backend{
			insecureBackend(registryv1alpha1.BackendStorage, cache, t, constant.Path, nil),
			insecureBackend(registryv1alpha1.BackendUpstream, upstream, t, "/deckhouse/ee", nil),
		},
	}

	response := pull(t, newServer(spec), constant.Host, "/v2/system/deckhouse/one/manifests/v1")

	// The last target's own failure is passed through rather than replaced, so the
	// runtime's error message says something about the registry.
	assert.Equal(t, http.StatusServiceUnavailable, response.StatusCode)
	assert.NotEmpty(t, upstream.asked())
}

// TestServeAuthenticatesOnTheClientsBehalf is why the agent cannot simply pass a
// challenge along: the runtime asked the AGENT for the image, so a challenge forwarded
// to it would be answered against the agent's address with credentials the runtime does
// not have.
func TestServeAuthenticatesOnTheClientsBehalf(t *testing.T) {
	upstream := (&registryStub{
		name: "upstream", challenge: "bearer",
		username: "license-token", password: "the-license-key",
		body: "authenticated",
	}).start(t)

	spec := &registryv1alpha1.RegistryNodeSpec{
		Backends: []registryv1alpha1.Backend{
			insecureBackend(registryv1alpha1.BackendUpstream, upstream, t, "/deckhouse/ee",
				&registryv1alpha1.Auth{Username: "license-token", Password: "the-license-key"}),
		},
	}

	response := pull(t, newServer(spec), constant.Host, "/v2/system/deckhouse/one/manifests/v1")

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "authenticated", bodyOf(t, response))
}

func TestServeAuthenticatesWithBasicCredentials(t *testing.T) {
	upstream := (&registryStub{
		name: "upstream", challenge: "basic", username: "user", password: "pass", body: "authenticated",
	}).start(t)

	spec := &registryv1alpha1.RegistryNodeSpec{
		Backends: []registryv1alpha1.Backend{
			insecureBackend(registryv1alpha1.BackendUpstream, upstream, t, "/deckhouse/ee",
				&registryv1alpha1.Auth{Username: "user", Password: "pass"}),
		},
	}

	response := pull(t, newServer(spec), constant.Host, "/v2/system/deckhouse/one/manifests/v1")
	assert.Equal(t, http.StatusOK, response.StatusCode)
}

// TestServeCachesTokens matters because a pull is a sequence of requests — manifest,
// config, every layer — and paying for a token exchange on each would multiply the
// latency of every image on the node.
func TestServeCachesTokens(t *testing.T) {
	upstream := (&registryStub{
		name: "upstream", challenge: "bearer", username: "u", password: "p", body: "ok",
	}).start(t)

	spec := &registryv1alpha1.RegistryNodeSpec{
		Backends: []registryv1alpha1.Backend{
			insecureBackend(registryv1alpha1.BackendUpstream, upstream, t, "/deckhouse/ee",
				&registryv1alpha1.Auth{Username: "u", Password: "p"}),
		},
	}
	server := newServer(spec)

	for range 3 {
		response := pull(t, server, constant.Host, "/v2/system/deckhouse/one/manifests/v1")
		require.Equal(t, http.StatusOK, response.StatusCode)
	}

	assert.EqualValues(t, 1, upstream.tokens.Load(), "the token has to be reused across a pull")
}

// TestAttemptCredentialRelay pins down which credentials reach which target.
//
// Exercised at the attempt rather than end to end, because a pass-through target is
// reached over HTTPS by design — the same assumption containerd makes about a registry
// nothing is configured for — and a self-signed stub would fail the handshake before
// any of this mattered.
func TestAttemptCredentialRelay(t *testing.T) {
	tests := []struct {
		name string
		kind Kind
		// relayed says whether the client's own Authorization must reach the target.
		relayed bool
	}{
		{
			name: "a configured target gets what the cluster holds for it",
			kind: KindPrimary,
			// The pod's credentials belong to whoever wrote the imagePullSecret; sending
			// them to the Deckhouse upstream would hand them to a party they were never
			// meant for.
			relayed: false,
		},
		{
			name:    "a route gets what the cluster holds for it",
			kind:    KindRoute,
			relayed: false,
		},
		{
			name: "an unconfigured registry gets the pod's own",
			kind: KindPassThrough,
			// The only way a private third-party image can be pulled once the agent owns
			// the fallback. The credentials reach the agent at all because the kubelet
			// leaves the server address unset, so containerd's per-host check is skipped.
			relayed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := (&registryStub{name: "target", body: "ok"}).start(t)

			original, err := http.NewRequest(http.MethodGet, "https://ignored/v2/one/manifests/v1", nil)
			require.NoError(t, err)
			original.Header.Set("Authorization", "Bearer the-pods-own-token")
			original.Header.Set("Cookie", "session=secret")
			original.Header.Set("Accept", "application/vnd.oci.image.manifest.v1+json")

			server := newServer(nil)
			response, err := server.attempt(context.Background(), original, &Target{
				Name:   "target",
				Scheme: registryv1alpha1.SchemeHTTP,
				Host:   target.host(t),
				Path:   "/v2/one/manifests/v1",
			}, "one", tt.kind)
			require.NoError(t, err)
			t.Cleanup(func() { _ = response.Body.Close() })

			headers := target.headers.Load().(http.Header)
			if tt.relayed {
				assert.Equal(t, "Bearer the-pods-own-token", headers.Get("Authorization"))
			} else {
				assert.Empty(t, headers.Get("Authorization"))
			}

			// Never, in either case: it says nothing to a registry and everything about the
			// client.
			assert.Empty(t, headers.Get("Cookie"))
			// And what the protocol needs always travels, since content negotiation decides
			// which manifest comes back.
			assert.Contains(t, headers.Get("Accept"), "oci.image.manifest")
		})
	}
}

// TestServeRelaysTheChallenge is the other half of a private pull: the client answers
// the target's challenge, and it can only do that if the challenge reaches it.
func TestServeRelaysTheChallenge(t *testing.T) {
	upstream := (&registryStub{
		name: "upstream", challenge: "basic", username: "user", password: "pass",
	}).start(t)

	spec := &registryv1alpha1.RegistryNodeSpec{
		Backends: []registryv1alpha1.Backend{
			// No credentials configured, so the agent has nothing of its own to offer and
			// the target's 401 comes straight back.
			insecureBackend(registryv1alpha1.BackendUpstream, upstream, t, "/deckhouse/ee", nil),
		},
	}

	response := pull(t, newServer(spec), constant.Host, "/v2/system/deckhouse/one/manifests/v1")

	assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	assert.Contains(t, response.Header.Get("WWW-Authenticate"), "Basic")
}

func TestServePassesThroughAnUnconfiguredRegistry(t *testing.T) {
	other := (&registryStub{name: "other", body: "somebody-elses-image"}).start(t)

	spec := &registryv1alpha1.RegistryNodeSpec{
		Backends: []registryv1alpha1.Backend{{
			Name:     registryv1alpha1.BackendStorage,
			Endpoint: registryv1alpha1.Endpoint{Host: constant.Host, Path: constant.Path},
		}},
	}

	// The agent is on the path of every pull on the node, so a registry nobody
	// configured has to keep working. Pass-through assumes HTTPS, so this exercises the
	// routing rather than the transfer.
	decision, err := Resolve(other.host(t), "/v2/library/nginx/manifests/latest", spec, self)
	require.NoError(t, err)
	require.Equal(t, KindPassThrough, decision.Kind)
	assert.Equal(t, "/v2/library/nginx/manifests/latest", decision.Targets[0].Path)
}

func TestServeRefusesWhatItCannotRoute(t *testing.T) {
	server := newServer(&registryv1alpha1.RegistryNodeSpec{})

	// No namespace: something other than the runtime is talking to the agent.
	front := httptest.NewServer(server.Handler())
	t.Cleanup(front.Close)

	response, err := http.Get(front.URL + "/v2/one/manifests/v1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = response.Body.Close() })
	assert.Equal(t, http.StatusBadGateway, response.StatusCode)

	// Naming the agent itself, which would loop.
	looping := pull(t, server, self, "/v2/one/manifests/v1")
	assert.Equal(t, http.StatusBadGateway, looping.StatusCode)
	assert.Contains(t, bodyOf(t, looping), "loop")
}

// TestServeHonoursTheForwardTimeout keeps a hung registry from holding the pull for as
// long as it likes: a fallback that never gets tried is not a fallback.
func TestServeHonoursTheForwardTimeout(t *testing.T) {
	slow := (&registryStub{name: "slow", delay: 2 * time.Second}).start(t)
	fast := (&registryStub{name: "fast", body: "from-the-fast-one"}).start(t)

	spec := &registryv1alpha1.RegistryNodeSpec{
		Backends: []registryv1alpha1.Backend{
			insecureBackend(registryv1alpha1.BackendStorage, slow, t, constant.Path, nil),
			insecureBackend(registryv1alpha1.BackendUpstream, fast, t, "/deckhouse/ee", nil),
		},
	}

	server := newServer(spec)
	server.ForwardTimeout = 200 * time.Millisecond

	started := time.Now()
	response := pull(t, server, constant.Host, "/v2/system/deckhouse/one/manifests/v1")

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "from-the-fast-one", bodyOf(t, response))
	assert.Less(t, time.Since(started), 2*time.Second)
}

// TestServeStreamsWithoutBuffering matters because a layer can be gigabytes and the
// agent runs on every node.
func TestServeStreamsWithoutBuffering(t *testing.T) {
	const size = 1 << 20

	blobs := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = io.Copy(writer, io.LimitReader(zeroes{}, size))
	}))
	t.Cleanup(blobs.Close)

	parsed, err := url.Parse(blobs.URL)
	require.NoError(t, err)

	spec := &registryv1alpha1.RegistryNodeSpec{
		Backends: []registryv1alpha1.Backend{{
			Name: registryv1alpha1.BackendStorage,
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP, Host: parsed.Host, Path: constant.Path,
			},
		}},
	}

	response := pull(t, newServer(spec), constant.Host, "/v2/system/deckhouse/one/blobs/sha256:abc")
	require.Equal(t, http.StatusOK, response.StatusCode)

	transferred, err := io.Copy(io.Discard, response.Body)
	require.NoError(t, err)
	assert.EqualValues(t, size, transferred)
}

type zeroes struct{}

func (zeroes) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func TestParseChallenge(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		wantNil    bool
		wantScheme string
		wantRealm  string
	}{
		{name: "empty", header: "", wantNil: true},
		{
			name:       "bearer",
			header:     `Bearer realm="https://auth.example.com/token",service="registry"`,
			wantScheme: schemeBearer,
			wantRealm:  "https://auth.example.com/token",
		},
		{name: "basic", header: `Basic realm="registry"`, wantScheme: schemeBasic},
		{name: "lowercase", header: `bearer realm="https://auth/token"`, wantScheme: schemeBearer, wantRealm: "https://auth/token"},
		// Nothing to exchange against.
		{name: "bearer without a realm", header: `Bearer service="registry"`, wantNil: true},
		{name: "unsupported", header: "Negotiate", wantNil: true},
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
		})
	}
}

func TestChallengeTokenURL(t *testing.T) {
	got, err := (&challenge{
		scheme: schemeBearer, realm: "https://auth.example.com/token", service: "registry.example.com",
	}).tokenURL("repository:deckhouse/ee/one:pull")
	require.NoError(t, err)

	parsed, err := url.Parse(got)
	require.NoError(t, err)
	assert.Equal(t, "repository:deckhouse/ee/one:pull", parsed.Query().Get("scope"))
	assert.Equal(t, "registry.example.com", parsed.Query().Get("service"))

	_, err = (&challenge{realm: "/relative"}).tokenURL("scope")
	assert.Error(t, err)
}

func TestServerClientRejectsABadCertificateAuthority(t *testing.T) {
	_, err := newServer(nil).client("this is not a certificate")
	require.Error(t, err)

	// And a good one is cached, since building it parses a bundle and that would
	// otherwise happen on every layer of every image.
	server := newServer(nil)
	first, err := server.client("")
	require.NoError(t, err)
	second, err := server.client("")
	require.NoError(t, err)
	assert.Same(t, first, second)
}

func TestServingReportsTheListener(t *testing.T) {
	server := newServer(nil)
	assert.False(t, server.Serving())
}

func TestLayoutFunc(t *testing.T) {
	spec := &registryv1alpha1.RegistryNodeSpec{Cache: true}
	assert.Same(t, spec, LayoutFunc(func() *registryv1alpha1.RegistryNodeSpec { return spec }).Current())
}

func TestSplitAPIPathOnTheProxiedShape(t *testing.T) {
	// Sanity: the paths the stubs are asked for are the ones routing produced.
	repository, remainder, err := splitAPIPath("/v2/deckhouse/ee/one/blobs/sha256:abc")
	require.NoError(t, err)
	assert.Equal(t, "deckhouse/ee/one", repository)
	assert.True(t, strings.HasPrefix(remainder, "/blobs/"))
}

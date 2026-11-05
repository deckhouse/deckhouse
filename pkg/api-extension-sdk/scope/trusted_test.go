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

package scope

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/authenticatorfactory"
	"k8s.io/apiserver/pkg/authentication/request/headerrequest"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"

	"github.com/deckhouse/deckhouse/pkg/api-extension-sdk/internal/testcerts"
)

const proxyName = "front-proxy-client"

// swapCA is a CAContentProvider whose CA a test can replace, standing in for
// the ConfigMap-backed provider of DelegatingAuthenticationOptions.
type swapCA struct {
	mu   sync.Mutex
	pool *x509.CertPool
}

func (s *swapCA) set(p *x509.CertPool) { s.mu.Lock(); s.pool = p; s.mu.Unlock() }

func (s *swapCA) Name() string                             { return "swap" }
func (s *swapCA) CurrentCABundleContent() []byte           { return nil }
func (s *swapCA) AddListener(dynamiccertificates.Listener) {}
func (s *swapCA) VerifyOptions() (x509.VerifyOptions, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pool == nil {
		return x509.VerifyOptions{}, false
	}
	return x509.VerifyOptions{Roots: s.pool, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}, true
}

func requestHeaderConfig(ca CAProviderLike, allowed ...string) *authenticatorfactory.RequestHeaderConfig {
	return &authenticatorfactory.RequestHeaderConfig{
		UsernameHeaders:     headerrequest.StaticStringSlice{"X-Remote-User"},
		UIDHeaders:          headerrequest.StaticStringSlice{"X-Remote-Uid"},
		GroupHeaders:        headerrequest.StaticStringSlice{"X-Remote-Group"},
		ExtraHeaderPrefixes: headerrequest.StaticStringSlice{"X-Remote-Extra-"},
		CAContentProvider:   ca,
		AllowedClientNames:  headerrequest.StaticStringSlice(allowed),
	}
}

type CAProviderLike = dynamiccertificates.CAContentProvider

// seen is what the wrapped handler observed.
type seen struct {
	called       bool
	decision     Decision
	hasDecision  bool
	scopeHeaders []string
	remoteUser   string
}

func recorder(out *seen) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out.called = true
		out.decision, out.hasDecision = FromContext(r.Context())
		for k := range r.Header {
			if _, ok := isScopeHeader(k); ok {
				out.scopeHeaders = append(out.scopeHeaders, k)
			}
		}
		out.remoteUser = r.Header.Get("X-Remote-User")
		w.WriteHeader(http.StatusOK)
	})
}

type certs struct {
	proxyCA, otherCA                                       *testcerts.CA
	proxy, proxyOtherCN, proxyServerOnly, otherCAProxyName *testcerts.Leaf
}

func newCerts(t *testing.T) certs {
	proxyCA := testcerts.NewCA(t, "front-proxy-ca")
	otherCA := testcerts.NewCA(t, "kubernetes-ca")
	return certs{
		proxyCA:          proxyCA,
		otherCA:          otherCA,
		proxy:            proxyCA.ClientCert(t, proxyName),
		proxyOtherCN:     proxyCA.ClientCert(t, "someone-else"),
		proxyServerOnly:  proxyCA.ServerOnlyCert(t, proxyName),
		otherCAProxyName: otherCA.ClientCert(t, proxyName),
	}
}

type req struct {
	cert    *testcerts.Leaf
	headers map[string][]string // raw keys, not canonicalized
}

func (q req) build() *http.Request {
	r := httptest.NewRequest(http.MethodGet, "https://ext/apis/g/v1/gizmos", nil)
	for k, vv := range q.headers {
		r.Header[k] = vv
	}
	if q.cert != nil {
		r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{q.cert.Cert}}
	} else {
		r.TLS = &tls.ConnectionState{}
	}
	return r
}

func hdr(kv ...string) map[string][]string {
	out := map[string][]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		out[kv[i]] = append(out[kv[i]], kv[i+1])
	}
	return out
}

type flavour struct {
	name string
	// build returns the handler, and rotate swaps the trusted CA to ca.
	build func(t *testing.T, next http.Handler, ca *testcerts.CA, allowed []string) (h http.Handler, rotate func(*testcerts.CA))
}

var flavours = []flavour{
	{
		name: "WithTrusted",
		build: func(t *testing.T, next http.Handler, ca *testcerts.CA, allowed []string) (http.Handler, func(*testcerts.CA)) {
			p := &swapCA{}
			p.set(ca.Pool())
			return WithTrusted(next, requestHeaderConfig(p, allowed...)), func(c *testcerts.CA) { p.set(c.Pool()) }
		},
	},
	{
		name: "NewTrustedHandler",
		build: func(t *testing.T, next http.Handler, ca *testcerts.CA, allowed []string) (http.Handler, func(*testcerts.CA)) {
			src := &mutableSource{}
			src.set(TrustConfig{Roots: ca.Pool(), AllowedNames: allowed, UsernameHeaders: []string{"X-Remote-User"}})
			return NewTrustedHandler(next, src), func(c *testcerts.CA) {
				src.set(TrustConfig{Roots: c.Pool(), AllowedNames: allowed, UsernameHeaders: []string{"X-Remote-User"}})
			}
		},
	},
}

type mutableSource struct {
	mu  sync.Mutex
	cfg *TrustConfig
}

func (m *mutableSource) set(c TrustConfig) { m.mu.Lock(); m.cfg = &c; m.mu.Unlock() }
func (m *mutableSource) TrustConfig() (TrustConfig, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cfg == nil {
		return TrustConfig{}, false
	}
	return *m.cfg, true
}

func TestTrustedHandler(t *testing.T) {
	c := newCerts(t)
	trusted := func(extra ...string) map[string][]string {
		return hdr(append([]string{"X-Remote-User", "alice"}, extra...)...)
	}
	type want struct {
		status     int
		decision   *Decision // nil: no decision
		remoteUser string
		notCalled  bool
	}
	ns := func(names ...string) *Decision { return &Decision{Mode: ModeFiltered, Namespaces: sets.New(names...)} }
	narrowed := func(names ...string) *Decision { return &Decision{Mode: ModeNarrowed, Namespaces: sets.New(names...)} }

	cases := []struct {
		name    string
		req     req
		allowed []string
		want    want
	}{
		{"trusted filtered", req{c.proxy, trusted(HeaderMode, "filtered", HeaderNamespaces, "a,b")}, nil,
			want{status: 200, decision: ns("a", "b"), remoteUser: "alice"}},
		{"trusted narrowed", req{c.proxy, trusted(HeaderMode, "narrowed", HeaderNamespaces, "kube-system")}, nil,
			want{status: 200, decision: narrowed("kube-system"), remoteUser: "alice"}},
		{"trusted mode without namespaces is the empty set", req{c.proxy, trusted(HeaderMode, "filtered")}, nil,
			want{status: 200, decision: ns(), remoteUser: "alice"}},
		{"trusted empty namespaces is the empty set", req{c.proxy, trusted(HeaderMode, "filtered", HeaderNamespaces, "")}, nil,
			want{status: 200, decision: ns(), remoteUser: "alice"}},
		{"trusted without scope headers", req{c.proxy, trusted()}, nil,
			want{status: 200, remoteUser: "alice"}},
		{"trusted lower-case spelling is read and stripped", req{c.proxy, trusted("x-deckhouse-scope-mode", "filtered", "x_deckhouse_scope_namespaces", "a")}, nil,
			want{status: 200, decision: ns("a"), remoteUser: "alice"}},

		{"bearer token, no client certificate", req{nil, hdr("Authorization", "Bearer x", "X-Remote-User", "admin", HeaderMode, "filtered", HeaderNamespaces, "a")}, nil,
			want{status: 200, remoteUser: "admin"}},
		{"client certificate of another CA", req{c.otherCAProxyName, trusted(HeaderMode, "filtered", HeaderNamespaces, "a")}, nil,
			want{status: 200, remoteUser: "alice"}},
		{"front-proxy certificate without X-Remote-User", req{c.proxy, hdr(HeaderMode, "filtered", HeaderNamespaces, "a")}, nil,
			want{status: 200}},
		{"front-proxy CA, common name not allowed", req{c.proxyOtherCN, trusted(HeaderMode, "filtered", HeaderNamespaces, "a")}, nil,
			want{status: 200, remoteUser: "alice"}},
		{"front-proxy CA, server-auth-only certificate", req{c.proxyServerOnly, trusted(HeaderMode, "filtered", HeaderNamespaces, "a")}, nil,
			want{status: 200, remoteUser: "alice"}},
		{"empty allowed names trust nothing", req{c.proxy, trusted(HeaderMode, "filtered", HeaderNamespaces, "a")}, []string{},
			want{status: 200, remoteUser: "alice"}},
		{"allowed names of empty strings trust nothing", req{c.proxy, trusted(HeaderMode, "filtered", HeaderNamespaces, "a")}, []string{""},
			want{status: 200, remoteUser: "alice"}},
		{"untrusted spellings are all stripped", req{c.otherCAProxyName, trusted(
			"x-deckhouse-scope-mode", "filtered", "X_Deckhouse_Scope_Mode", "filtered", "x_DECKHOUSE_scope_namespaces", "a", HeaderNamespaces, "b")}, nil,
			want{status: 200, remoteUser: "alice"}},
		{"untrusted unknown mode is ignored, not refused", req{c.otherCAProxyName, trusted(HeaderMode, "bogus")}, nil,
			want{status: 200, remoteUser: "alice"}},

		{"trusted unknown mode", req{c.proxy, trusted(HeaderMode, "Filtered", HeaderNamespaces, "a")}, nil, want{status: 400, notCalled: true}},
		{"trusted invalid namespace name", req{c.proxy, trusted(HeaderMode, "filtered", HeaderNamespaces, "a,B_c")}, nil, want{status: 400, notCalled: true}},
		{"trusted empty entry", req{c.proxy, trusted(HeaderMode, "filtered", HeaderNamespaces, "a,,b")}, nil, want{status: 400, notCalled: true}},
		{"trusted namespaces without mode", req{c.proxy, trusted(HeaderNamespaces, "a")}, nil, want{status: 400, notCalled: true}},
		{"trusted two mode values", req{c.proxy, trusted(HeaderMode, "filtered", HeaderMode, "narrowed")}, nil, want{status: 400, notCalled: true}},
		{"trusted canonical and underscore spelling together", req{c.proxy, trusted(HeaderMode, "narrowed", "X_Deckhouse_Scope_Mode", "narrowed")}, nil, want{status: 400, notCalled: true}},
		{"trusted two namespace values", req{c.proxy, trusted(HeaderMode, "filtered", HeaderNamespaces, "a", HeaderNamespaces, "b")}, nil, want{status: 400, notCalled: true}},
	}
	for _, f := range flavours {
		for _, tc := range cases {
			t.Run(f.name+"/"+tc.name, func(t *testing.T) {
				allowed := tc.allowed
				if allowed == nil {
					allowed = []string{proxyName}
				}
				var got seen
				h, _ := f.build(t, recorder(&got), c.proxyCA, allowed)
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, tc.req.build())

				if rec.Code != tc.want.status {
					t.Fatalf("status %d, want %d: %s", rec.Code, tc.want.status, rec.Body.String())
				}
				if tc.want.notCalled {
					if got.called {
						t.Fatal("next handler called for a refused request")
					}
					var status map[string]any
					if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil || status["kind"] != "Status" {
						t.Fatalf("body is not a Status: %s", rec.Body.String())
					}
					return
				}
				if len(got.scopeHeaders) != 0 {
					t.Errorf("scope headers reached the next handler: %v", got.scopeHeaders)
				}
				if got.remoteUser != tc.want.remoteUser {
					t.Errorf("X-Remote-User %q, want %q (the trust check must not strip it)", got.remoteUser, tc.want.remoteUser)
				}
				switch {
				case tc.want.decision == nil && got.hasDecision:
					t.Errorf("unexpected decision %+v", got.decision)
				case tc.want.decision != nil && !got.hasDecision:
					t.Errorf("no decision, want %+v", *tc.want.decision)
				case tc.want.decision != nil:
					if got.decision.Mode != tc.want.decision.Mode || !got.decision.Namespaces.Equal(tc.want.decision.Namespaces) {
						t.Errorf("decision %v %v, want %v %v", got.decision.Mode, sets.List(got.decision.Namespaces), tc.want.decision.Mode, sets.List(tc.want.decision.Namespaces))
					}
				}
			})
		}
	}
}

func TestTrustedHandlerCARotation(t *testing.T) {
	oldCA := testcerts.NewCA(t, "old")
	newCA := testcerts.NewCA(t, "new")
	oldCert, newCert := oldCA.ClientCert(t, proxyName), newCA.ClientCert(t, proxyName)
	h := hdr("X-Remote-User", "alice", HeaderMode, "filtered", HeaderNamespaces, "a")

	for _, f := range flavours {
		t.Run(f.name, func(t *testing.T) {
			var got seen
			handler, rotate := f.build(t, recorder(&got), oldCA, []string{proxyName})
			check := func(cert *testcerts.Leaf, want bool) {
				t.Helper()
				got = seen{}
				handler.ServeHTTP(httptest.NewRecorder(), req{cert, h}.build())
				if got.hasDecision != want {
					t.Fatalf("trusted=%v, want %v", got.hasDecision, want)
				}
			}
			check(oldCert, true)
			check(newCert, false)
			rotate(newCA)
			check(oldCert, false)
			check(newCert, true)
		})
	}
}

func TestWithTrustedWithoutConfig(t *testing.T) {
	c := newCerts(t)
	h := hdr("X-Remote-User", "alice", HeaderMode, "filtered", HeaderNamespaces, "a")
	for name, rh := range map[string]*authenticatorfactory.RequestHeaderConfig{
		"nil config":          nil,
		"nil CA provider":     requestHeaderConfig(nil, proxyName),
		"provider without CA": requestHeaderConfig(&swapCA{}, proxyName),
		"nil allowed names":   {UsernameHeaders: headerrequest.StaticStringSlice{"X-Remote-User"}, CAContentProvider: staticCA(t, c.proxyCA)},
	} {
		t.Run(name, func(t *testing.T) {
			var got seen
			rec := httptest.NewRecorder()
			WithTrusted(recorder(&got), rh).ServeHTTP(rec, req{c.proxy, h}.build())
			if rec.Code != 200 || got.hasDecision || len(got.scopeHeaders) != 0 {
				t.Fatalf("status %d decision %v headers %v", rec.Code, got.hasDecision, got.scopeHeaders)
			}
		})
	}
}

func TestNewTrustedHandlerWithoutConfig(t *testing.T) {
	c := newCerts(t)
	h := hdr("X-Remote-User", "alice", HeaderMode, "filtered", HeaderNamespaces, "a")
	for name, src := range map[string]TrustSource{
		"nil source":            nil,
		"source not ready":      &mutableSource{},
		"no roots":              StaticTrustSource{AllowedNames: []string{proxyName}, UsernameHeaders: []string{"X-Remote-User"}},
		"no username headers":   StaticTrustSource{Roots: c.proxyCA.Pool(), AllowedNames: []string{proxyName}},
		"other username header": StaticTrustSource{Roots: c.proxyCA.Pool(), AllowedNames: []string{proxyName}, UsernameHeaders: []string{"X-Other-User"}},
	} {
		t.Run(name, func(t *testing.T) {
			var got seen
			rec := httptest.NewRecorder()
			NewTrustedHandler(recorder(&got), src).ServeHTTP(rec, req{c.proxy, h}.build())
			if rec.Code != 200 || got.hasDecision || len(got.scopeHeaders) != 0 {
				t.Fatalf("status %d decision %v headers %v", rec.Code, got.hasDecision, got.scopeHeaders)
			}
		})
	}
}

func staticCA(t *testing.T, ca *testcerts.CA) dynamiccertificates.CAContentProvider {
	p, err := dynamiccertificates.NewStaticCAContent("test", ca.PEM())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestWithTrustedStaticCAContent(t *testing.T) {
	c := newCerts(t)
	var got seen
	h := WithTrusted(recorder(&got), requestHeaderConfig(staticCA(t, c.proxyCA), proxyName))
	h.ServeHTTP(httptest.NewRecorder(), req{c.proxy, hdr("X-Remote-User", "alice", HeaderMode, "filtered", HeaderNamespaces, "a")}.build())
	if !got.hasDecision || !got.decision.Namespaces.Equal(sets.New("a")) {
		t.Fatalf("decision %+v %v", got.decision, got.hasDecision)
	}
}

func TestDecisionAllows(t *testing.T) {
	d := Decision{Mode: ModeFiltered, Namespaces: sets.New("a")}
	if !d.Allows("a") || d.Allows("b") || d.Allows("") {
		t.Fatal("Allows")
	}
	if (Decision{Mode: ModeFiltered}).Allows("a") {
		t.Fatal("nil set allows")
	}
}

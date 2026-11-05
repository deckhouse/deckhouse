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

package toy

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"
	"time"

	"k8s.io/apiserver/pkg/authorization/authorizer"

	"github.com/deckhouse/deckhouse/modules/140-user-authz/docs/internal/scope-aggregated-api-e2e/internal/testcerts"
	"github.com/deckhouse/deckhouse/pkg/api-extension-sdk/scope"
)

const base = "/apis/scopetoy.deckhouse.io/v1"

// stubAuthorizer stands in for the delegated SubjectAccessReview:
//   - system:masters: everything;
//   - reader-all: list/get/watch gizmos everywhere;
//   - reader-ns1: list/get/watch gizmos in ns1;
//   - everyone else: no opinion (403).
var stubAuthorizer = authorizer.AuthorizerFunc(func(_ context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
	u := a.GetUser()
	if u == nil {
		return authorizer.DecisionNoOpinion, "", nil
	}
	if slices.Contains(u.GetGroups(), "system:masters") {
		return authorizer.DecisionAllow, "stub: masters", nil
	}
	if a.IsResourceRequest() && a.GetResource() == "gizmos" && slices.Contains([]string{"list", "get", "watch"}, a.GetVerb()) {
		switch {
		case u.GetName() == "reader-all":
			return authorizer.DecisionAllow, "stub: reader-all", nil
		case u.GetName() == "reader-ns1" && a.GetNamespace() == "ns1":
			return authorizer.DecisionAllow, "stub: reader-ns1", nil
		}
	}
	return authorizer.DecisionNoOpinion, "", nil
})

type env struct {
	addr string
	// Client certificates.
	proxy, proxyOtherCN, foreign, admin, denied *testcerts.Leaf
}

func startServer(t *testing.T) *env {
	t.Helper()
	dir := t.TempDir()
	proxyCA := testcerts.NewCA(t, "front-proxy-ca")
	clientCA := testcerts.NewCA(t, "kubernetes-ca")
	foreignCA := testcerts.NewCA(t, "foreign-ca")
	write := func(name string, data []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	o := NewOptions()
	o.SecureServing.Listener = ln
	o.SecureServing.BindPort = ln.Addr().(*net.TCPAddr).Port
	o.SecureServing.ServerCert.CertDirectory = filepath.Join(dir, "certs")
	o.Authentication.RemoteKubeConfigFileOptional = true
	o.Authentication.SkipInClusterLookup = true
	o.Authentication.ClientCert.ClientCA = write("client-ca.crt", clientCA.PEM())
	o.Authentication.RequestHeader.ClientCAFile = write("front-proxy-ca.crt", proxyCA.PEM())
	o.Authentication.RequestHeader.AllowedNames = []string{"front-proxy-client"}
	o.Authentication.RequestHeader.UsernameHeaders = []string{"X-Remote-User"}
	o.Authentication.RequestHeader.GroupHeaders = []string{"X-Remote-Group"}
	o.Authentication.RequestHeader.ExtraHeaderPrefixes = []string{"X-Remote-Extra-"}
	o.Authorizer = stubAuthorizer
	o.Seed = []string{"ns1", "ns2", "ns3"}

	srv, err := o.New()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := srv.Run(ctx); err != nil {
			t.Errorf("run: %v", err)
		}
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	e := &env{
		addr:         ln.Addr().String(),
		proxy:        proxyCA.ClientCert(t, "front-proxy-client"),
		proxyOtherCN: proxyCA.ClientCert(t, "not-the-proxy"),
		foreign:      foreignCA.ClientCert(t, "front-proxy-client"),
		admin:        clientCA.ClientCert(t, "admin", "system:masters"),
		denied:       clientCA.ClientCert(t, "denied-user"),
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		if code, _ := e.do(t, e.admin, http.MethodGet, "/readyz", nil, nil); code == http.StatusOK {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("server not ready")
		}
		time.Sleep(100 * time.Millisecond)
	}
	return e
}

func (e *env) client(cert *testcerts.Leaf) *http.Client {
	tlsCfg := &tls.Config{InsecureSkipVerify: true} //nolint:gosec // self-signed test server
	if cert != nil {
		c := cert.TLS()
		tlsCfg.Certificates = []tls.Certificate{c}
	}
	return &http.Client{Transport: &http.Transport{TLSClientConfig: tlsCfg, ForceAttemptHTTP2: true}, Timeout: 30 * time.Second}
}

func (e *env) request(t *testing.T, method, path string, headers map[string]string, body []byte) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, "https://"+e.addr+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header[k] = []string{v} // raw keys: spellings matter
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func (e *env) do(t *testing.T, cert *testcerts.Leaf, method, path string, headers map[string]string, body []byte) (int, []byte) {
	t.Helper()
	resp, err := e.client(cert).Do(e.request(t, method, path, headers, body))
	if err != nil {
		return 0, []byte(err.Error())
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func names(t *testing.T, body []byte) []string {
	t.Helper()
	var l GizmoList
	if err := json.Unmarshal(body, &l); err != nil {
		t.Fatalf("decode list: %v: %s", err, body)
	}
	out := []string{}
	for _, g := range l.Items {
		out = append(out, g.Namespace+"/"+g.Name)
	}
	sort.Strings(out)
	return out
}

func proxied(user string, kv ...string) map[string]string {
	h := map[string]string{"X-Remote-User": user}
	for i := 0; i+1 < len(kv); i += 2 {
		h[kv[i]] = kv[i+1]
	}
	return h
}

func TestToyServer(t *testing.T) {
	e := startServer(t)
	all := []string{"ns1/gizmo-ns1", "ns2/gizmo-ns2", "ns3/gizmo-ns3"}

	lists := []struct {
		name   string
		cert   *testcerts.Leaf
		path   string
		header map[string]string
		code   int
		want   []string
	}{
		{"filtered denied user sees the set", e.proxy, base + "/gizmos",
			proxied("denied-user", scope.HeaderMode, "filtered", scope.HeaderNamespaces, "ns1,ns2"), 200, []string{"ns1/gizmo-ns1", "ns2/gizmo-ns2"}},
		{"filtered empty set serves nothing", e.proxy, base + "/gizmos",
			proxied("denied-user", scope.HeaderMode, "filtered", scope.HeaderNamespaces, ""), 200, []string{}},
		{"filtered mode without namespaces serves nothing", e.proxy, base + "/gizmos",
			proxied("denied-user", scope.HeaderMode, "filtered"), 200, []string{}},
		{"filtered namespaced list outside the set is empty", e.proxy, base + "/namespaces/ns3/gizmos",
			proxied("denied-user", scope.HeaderMode, "filtered", scope.HeaderNamespaces, ""), 200, []string{}},
		{"filtered namespaced list in the set", e.proxy, base + "/namespaces/ns1/gizmos",
			proxied("denied-user", scope.HeaderMode, "filtered", scope.HeaderNamespaces, "ns1"), 200, []string{"ns1/gizmo-ns1"}},
		{"denied user without scope", e.proxy, base + "/gizmos", proxied("denied-user"), 403, nil},
		{"narrowed allowed user", e.proxy, base + "/gizmos",
			proxied("reader-all", scope.HeaderMode, "narrowed", scope.HeaderNamespaces, "ns2"), 200, []string{"ns2/gizmo-ns2"}},
		{"narrowed denied user is authorized as usual", e.proxy, base + "/gizmos",
			proxied("denied-user", scope.HeaderMode, "narrowed", scope.HeaderNamespaces, "ns2"), 403, nil},
		{"narrowed namespace-scoped reader", e.proxy, base + "/namespaces/ns1/gizmos",
			proxied("reader-ns1", scope.HeaderMode, "narrowed", scope.HeaderNamespaces, "ns1"), 200, []string{"ns1/gizmo-ns1"}},
		{"allowed user without scope sees all", e.proxy, base + "/gizmos", proxied("reader-all"), 200, all},
		{"trusted unknown mode is refused", e.proxy, base + "/gizmos",
			proxied("denied-user", scope.HeaderMode, "everything", scope.HeaderNamespaces, "ns1"), 400, nil},
		{"trusted invalid namespace is refused", e.proxy, base + "/gizmos",
			proxied("denied-user", scope.HeaderMode, "filtered", scope.HeaderNamespaces, "ns1,NS2"), 400, nil},

		// Spoofed headers on requests not sent by the front proxy.
		{"spoof with a regular admin client certificate is ignored", e.admin, base + "/gizmos",
			map[string]string{scope.HeaderMode: "filtered", scope.HeaderNamespaces: "ns1"}, 200, all},
		{"spoof with lower-case and underscore spellings is ignored", e.admin, base + "/gizmos",
			map[string]string{"x-deckhouse-scope-mode": "filtered", "X_Deckhouse_Scope_Namespaces": "ns1"}, 200, all},
		{"spoof with a regular denied client certificate stays denied", e.denied, base + "/gizmos",
			map[string]string{scope.HeaderMode: "filtered", scope.HeaderNamespaces: "ns1"}, 403, nil},
		{"spoof with a certificate of a foreign CA stays denied", e.foreign, base + "/gizmos",
			proxied("admin", "X-Remote-Group", "system:masters", scope.HeaderMode, "filtered", scope.HeaderNamespaces, "ns1"), 403, nil},
		{"spoof with front-proxy CA but a disallowed CN stays denied", e.proxyOtherCN, base + "/gizmos",
			proxied("admin", "X-Remote-Group", "system:masters", scope.HeaderMode, "filtered", scope.HeaderNamespaces, "ns1"), 401, nil},
		{"front-proxy certificate without X-Remote-User stays denied", e.proxy, base + "/gizmos",
			map[string]string{scope.HeaderMode: "filtered", scope.HeaderNamespaces: "ns1"}, 401, nil},
		{"bearer token without certificate stays denied", nil, base + "/gizmos",
			map[string]string{"Authorization": "Bearer made-up", scope.HeaderMode: "filtered", scope.HeaderNamespaces: "ns1"}, 403, nil},
	}
	for _, tc := range lists {
		t.Run(tc.name, func(t *testing.T) {
			code, body := e.do(t, tc.cert, http.MethodGet, tc.path, tc.header, nil)
			if code != tc.code {
				t.Fatalf("status %d, want %d: %s", code, tc.code, body)
			}
			if tc.want != nil {
				if got := names(t, body); !slices.Equal(got, tc.want) {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}

	t.Run("table output is filtered the same", func(t *testing.T) {
		h := proxied("denied-user", scope.HeaderMode, "filtered", scope.HeaderNamespaces, "ns1,ns3",
			"Accept", "application/json;as=Table;v=v1;g=meta.k8s.io,application/json")
		code, body := e.do(t, e.proxy, http.MethodGet, base+"/gizmos", h, nil)
		if code != 200 {
			t.Fatalf("%d %s", code, body)
		}
		var table struct {
			Kind string
			Rows []struct{ Cells []any }
		}
		if err := json.Unmarshal(body, &table); err != nil || table.Kind != "Table" {
			t.Fatalf("not a table: %v %s", err, body)
		}
		var got []string
		for _, r := range table.Rows {
			got = append(got, fmt.Sprint(r.Cells[0]))
		}
		sort.Strings(got)
		if !slices.Equal(got, []string{"gizmo-ns1", "gizmo-ns3"}) {
			t.Fatalf("rows %v", got)
		}
	})

	gets := []struct {
		name   string
		path   string
		header map[string]string
		code   int
	}{
		{"filtered get in the set", base + "/namespaces/ns1/gizmos/gizmo-ns1",
			proxied("denied-user", scope.HeaderMode, "filtered", scope.HeaderNamespaces, "ns1"), 200},
		{"filtered get outside the set", base + "/namespaces/ns3/gizmos/gizmo-ns3",
			proxied("denied-user", scope.HeaderMode, "filtered", scope.HeaderNamespaces, "ns1"), 404},
		{"narrowed get with the empty set", base + "/namespaces/ns1/gizmos/gizmo-ns1",
			proxied("reader-all", scope.HeaderMode, "narrowed", scope.HeaderNamespaces, ""), 404},
		{"get without scope by allowed user", base + "/namespaces/ns3/gizmos/gizmo-ns3", proxied("reader-all"), 200},
	}
	for _, tc := range gets {
		t.Run(tc.name, func(t *testing.T) {
			code, body := e.do(t, e.proxy, http.MethodGet, tc.path, tc.header, nil)
			if code != tc.code {
				t.Fatalf("status %d, want %d: %s", code, tc.code, body)
			}
		})
	}

	t.Run("404 outside the set equals a missing object", func(t *testing.T) {
		h := proxied("denied-user", scope.HeaderMode, "filtered", scope.HeaderNamespaces, "ns1")
		_, outside := e.do(t, e.proxy, http.MethodGet, base+"/namespaces/ns3/gizmos/gizmo-ns3", h, nil)
		_, missing := e.do(t, e.proxy, http.MethodGet, base+"/namespaces/ns1/gizmos/gizmo-ns3", h, nil)
		var a, b map[string]any
		_ = json.Unmarshal(outside, &a)
		_ = json.Unmarshal(missing, &b)
		if a["message"] != b["message"] || a["reason"] != "NotFound" {
			t.Fatalf("outside %s, missing %s", outside, missing)
		}
	})

	t.Run("filtered create is not bypassed", func(t *testing.T) {
		body := []byte(`{"apiVersion":"scopetoy.deckhouse.io/v1","kind":"Gizmo","metadata":{"name":"sneaky"}}`)
		code, resp := e.do(t, e.proxy, http.MethodPost, base+"/namespaces/ns1/gizmos",
			proxied("denied-user", scope.HeaderMode, "filtered", scope.HeaderNamespaces, "ns1"), body)
		if code != 403 {
			t.Fatalf("status %d: %s", code, resp)
		}
	})

	t.Run("watch is filtered per event", func(t *testing.T) {
		code, body := e.do(t, e.admin, http.MethodGet, base+"/gizmos", nil, nil)
		if code != 200 {
			t.Fatalf("%d %s", code, body)
		}
		var l GizmoList
		_ = json.Unmarshal(body, &l)

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		req := e.request(t, http.MethodGet, base+"/gizmos?watch=1&resourceVersion="+l.ResourceVersion,
			proxied("denied-user", scope.HeaderMode, "filtered", scope.HeaderNamespaces, "ns1"), nil).WithContext(ctx)
		resp, err := e.client(e.proxy).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("watch status %d: %s", resp.StatusCode, b)
		}

		for _, ns := range []string{"ns2", "ns1"} {
			body := []byte(fmt.Sprintf(`{"apiVersion":"scopetoy.deckhouse.io/v1","kind":"Gizmo","metadata":{"name":"watched-%s"},"spec":{"color":"red"}}`, ns))
			if code, resp := e.do(t, e.admin, http.MethodPost, base+"/namespaces/"+ns+"/gizmos", nil, body); code != 201 {
				t.Fatalf("create in %s: %d %s", ns, code, resp)
			}
		}
		if code, resp := e.do(t, e.admin, http.MethodDelete, base+"/namespaces/ns1/gizmos/watched-ns1", nil, nil); code != 200 {
			t.Fatalf("delete: %d %s", code, resp)
		}

		dec := json.NewDecoder(resp.Body)
		var got []string
		for len(got) < 2 {
			var ev struct {
				Type   string
				Object Gizmo
			}
			if err := dec.Decode(&ev); err != nil {
				t.Fatalf("decode event after %v: %v", got, err)
			}
			got = append(got, ev.Type+" "+ev.Object.Namespace+"/"+ev.Object.Name)
		}
		want := []string{"ADDED ns1/watched-ns1", "DELETED ns1/watched-ns1"}
		if !slices.Equal(got, want) {
			t.Fatalf("events %v, want %v", got, want)
		}
	})
}

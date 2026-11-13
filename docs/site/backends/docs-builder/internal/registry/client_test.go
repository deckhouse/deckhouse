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

package registry

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	ggcrregistry "github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/docs-builder/internal/metrics"
)

// pushModule puts a module image and its release tags into the registry at
// host and returns the digest of the module image.
func pushModule(t *testing.T, host string, opts []name.Option, ropts []remote.Option, module, version string, channels ...string) string {
	t.Helper()

	img, err := random.Image(256, 1)
	if err != nil {
		t.Fatal(err)
	}

	ref, err := name.ParseReference(host+"/modules/"+module+":"+version, opts...)
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(ref, img, ropts...); err != nil {
		t.Fatal(err)
	}

	for _, tag := range append([]string{version}, channels...) {
		rel, err := name.ParseReference(host+"/modules/"+module+"/release:"+tag, opts...)
		if err != nil {
			t.Fatal(err)
		}
		if err := remote.Write(rel, img, ropts...); err != nil {
			t.Fatal(err)
		}
	}

	digest, err := img.Digest()
	if err != nil {
		t.Fatal(err)
	}

	return digest.String()
}

func TestClientOverHTTP(t *testing.T) {
	srv := httptest.NewServer(ggcrregistry.New())
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	want := pushModule(t, host, []name.Option{name.Insecure}, nil, "echo", "v1.2.3", "stable", "alpha")

	client, err := NewClient(Source{Repo: host + "/modules/", Scheme: "HTTP"})
	if err != nil {
		t.Fatal(err)
	}

	tags, err := client.ListTags(context.Background(), "echo")
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(tags)
	if !slices.Equal(tags, []string{"alpha", "stable", "v1.2.3"}) {
		t.Fatalf("tags = %v", tags)
	}

	img, err := client.Image(context.Background(), "echo", "v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := img.Digest(); got.String() != want {
		t.Fatalf("digest = %s, want %s", got, want)
	}

	if _, err := client.Image(context.Background(), "echo", "v9.9.9"); err == nil {
		t.Fatal("want an error for a missing version")
	}

	release, err := client.ReleaseImage(context.Background(), "echo", "v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := release.Digest(); got.String() != want {
		t.Fatalf("release digest = %s, want %s", got, want)
	}
}

func TestClientOverHTTPSWithCA(t *testing.T) {
	srv := httptest.NewTLSServer(ggcrregistry.New())
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "https://")
	pushModule(t, host, nil, []remote.Option{remote.WithTransport(srv.Client().Transport)}, "echo", "v1.2.3")

	ca := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}))

	trusted, err := NewClient(Source{Repo: host + "/modules", CA: ca})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := trusted.ListTags(context.Background(), "echo"); err != nil {
		t.Fatalf("with CA: %v", err)
	}

	untrusted, err := NewClient(Source{Repo: host + "/modules"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := untrusted.ListTags(context.Background(), "echo"); err == nil {
		t.Fatal("without CA: want a certificate error")
	}
}

// The credentials are picked as deckhouse-controller picks them, so a source
// that works in the cluster works for the documentation too.
func TestAuthenticator(t *testing.T) {
	cfg := func(json string) string { return base64.StdEncoding.EncodeToString([]byte(json)) }
	userPass := base64.StdEncoding.EncodeToString([]byte("user:pass"))
	entry := `{"auth":"` + userPass + `"}`
	fromConfig := &authn.AuthConfig{Username: "user", Password: "pass", Auth: userPass}

	tests := []struct {
		name    string
		src     Source
		want    *authn.AuthConfig
		wantErr bool
	}{
		{"empty means anonymous", Source{}, &authn.AuthConfig{}, false},
		{"credentials for the host", Source{DockerCfg: cfg(`{"auths":{"registry.example.io":` + entry + `}}`)}, fromConfig, false},
		{"a URL of the host", Source{DockerCfg: cfg(`{"auths":{"https://registry.example.io":` + entry + `}}`)}, fromConfig, false},
		{"a repository on the host", Source{DockerCfg: cfg(`{"auths":{"registry.example.io/deckhouse":` + entry + `}}`)}, fromConfig, false},
		{"no credentials for the host", Source{DockerCfg: cfg(`{"auths":{"other.example.io":{"auth":"x"}}}`)}, nil, true},
		{"another port of the host", Source{DockerCfg: cfg(`{"auths":{"registry.example.io:5000":` + entry + `}}`)}, nil, true},
		{"a broken entry of another registry", Source{DockerCfg: cfg(`{"auths":{"other.example.io":{"auth":"%"},"registry.example.io":` + entry + `}}`)}, fromConfig, false},
		{"not base64 is taken as it is", Source{DockerCfg: `{"auths":{"registry.example.io":` + entry + `}}`}, fromConfig, false},
		{"neither base64 nor json", Source{DockerCfg: "{not base64"}, nil, true},
		{"masked by kube-apiserver", Source{DockerCfg: "<omitted>"}, nil, true},
		{"a login masked by kube-apiserver", Source{DockerCfg: "<omitted>", Login: "<omitted>", Password: "<omitted>"}, nil, true},
		{"not json", Source{DockerCfg: cfg("nope")}, nil, true},
		{"a login over the docker config", Source{DockerCfg: cfg(`{"auths":{"registry.example.io":{"auth":"x"}}}`), Login: "user", Password: "pass"}, &authn.AuthConfig{Username: "user", Password: "pass"}, false},
		{"a password without a login means anonymous", Source{Password: "pass"}, &authn.AuthConfig{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, err := authenticator("registry.example.io/deckhouse/modules", tt.src)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			got, err := auth.Authorization()
			if err != nil {
				t.Fatal(err)
			}
			if *got != *tt.want {
				t.Fatalf("auth = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestTransportWithCAInvalid(t *testing.T) {
	if _, err := transportWithCA("not a certificate"); err == nil {
		t.Fatal("want an error")
	}
}

func TestClientsReuseClientPerSource(t *testing.T) {
	clients := NewClients()
	src := Source{Name: "deckhouse", Repo: "registry.example.io/deckhouse/modules"}

	first, err := clients.For(src)
	if err != nil {
		t.Fatal(err)
	}
	again, err := clients.For(src)
	if err != nil {
		t.Fatal(err)
	}
	if first != again {
		t.Fatal("the same source got a new client")
	}

	changed := src
	changed.Scheme = "HTTP"
	other, err := clients.For(changed)
	if err != nil {
		t.Fatal(err)
	}
	if other == first {
		t.Fatal("a source with new registry settings kept the old client")
	}

	broken := src
	broken.DockerCfg = "not base64"
	if _, err := clients.For(broken); err == nil {
		t.Fatal("want an error for broken credentials")
	}
	if c, _ := clients.For(changed); c != other {
		t.Fatal("a failed client replaced the cached one")
	}

	// A fallback is compared by what it is, not by its address.
	upstream := func(repo string) *Source { return &Source{Name: "deckhouse (upstream)", Repo: repo} }
	chained := src
	chained.Fallback = upstream("registry.example.io/deckhouse/ee/modules")
	first, err = clients.For(chained)
	if err != nil {
		t.Fatal(err)
	}
	chained.Fallback = upstream("registry.example.io/deckhouse/ee/modules")
	if c, _ := clients.For(chained); c != first {
		t.Fatal("the same source with the same fallback got a new client")
	}
	chained.Fallback = upstream("registry.example.io/deckhouse/ce/modules")
	if c, _ := clients.For(chained); c == first {
		t.Fatal("a source whose fallback changed kept the old client")
	}
}

// fakeRegistry is a registry that answers status to every request instead of
// serving it while status is set, and counts the requests.
type fakeRegistry struct {
	srv      *httptest.Server
	host     string
	status   atomic.Int32
	requests atomic.Int32
}

func newFakeRegistry(t *testing.T) *fakeRegistry {
	t.Helper()

	r := &fakeRegistry{}
	reg := ggcrregistry.New()
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.requests.Add(1)
		if status := int(r.status.Load()); status != 0 {
			http.Error(w, http.StatusText(status), status)
			return
		}
		reg.ServeHTTP(w, req)
	}))
	t.Cleanup(r.srv.Close)
	r.host = strings.TrimPrefix(r.srv.URL, "http://")

	return r
}

// storeThenUpstream is the client of a source read from store, and from
// upstream where store fails.
func storeThenUpstream(t *testing.T, store, upstream *fakeRegistry) *Client {
	t.Helper()

	client, err := NewClient(Source{Name: "src (store)", Repo: store.host + "/modules", Scheme: "HTTP",
		Fallback: &Source{Name: "src (upstream)", Repo: upstream.host + "/modules", Scheme: "HTTP"}})
	if err != nil {
		t.Fatal(err)
	}

	return client
}

// The fallback is read only where the registry before it gives no answer,
// as the node agent reads the store and the upstream: the tags and a "not
// found" of the store are final.
func TestClientFallback(t *testing.T) {
	tests := []struct {
		name   string
		status int  // what the store answers to every request, 0 to serve
		down   bool // the store is not reached
		// fallback means the upstream answers instead of the store.
		fallback bool
	}{
		{name: "the store answers"},
		// 501: a server error the client does not retry.
		{name: "the store fails", status: http.StatusNotImplemented, fallback: true},
		{name: "the store refuses the credentials", status: http.StatusUnauthorized, fallback: true},
		{name: "the store forbids", status: http.StatusForbidden, fallback: true},
		{name: "the store is not reached", down: true, fallback: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, upstream := newFakeRegistry(t), newFakeRegistry(t)
			// The store holds v1.0.0 only, each registry an image of its own.
			cached := pushModule(t, store.host, []name.Option{name.Insecure}, nil, "echo", "v1.0.0")
			released := pushModule(t, upstream.host, []name.Option{name.Insecure}, nil, "echo", "v1.0.0")
			newer := pushModule(t, upstream.host, []name.Option{name.Insecure}, nil, "echo", "v1.1.0")
			upstream.requests.Store(0)
			store.status.Store(int32(tt.status))
			if tt.down {
				store.srv.Close()
			}
			client := storeThenUpstream(t, store, upstream)

			wantTags, wantDigest := []string{"v1.0.0"}, cached
			if tt.fallback {
				wantTags, wantDigest = []string{"v1.0.0", "v1.1.0"}, released
			}

			tags, err := client.ListTags(context.Background(), "echo")
			slices.Sort(tags)
			if err != nil || !slices.Equal(tags, wantTags) {
				t.Fatalf("tags %v, %v; want %v", tags, err, wantTags)
			}
			if got, err := client.ReleaseDigest(context.Background(), "echo", "v1.0.0"); err != nil || got != wantDigest {
				t.Fatalf("digest %q, %v; want %q", got, err, wantDigest)
			}

			got, err := client.ReleaseDigest(context.Background(), "echo", "v1.1.0")
			if tt.fallback {
				if err != nil || got != newer {
					t.Fatalf("digest of v1.1.0 %q, %v; want %q of the upstream", got, err, newer)
				}
				return
			}
			var terr *transport.Error
			if !errors.As(err, &terr) || terr.StatusCode != http.StatusNotFound {
				t.Fatalf("digest of v1.1.0 %q, %v; want the 404 of the store", got, err)
			}
			if n := upstream.requests.Load(); n != 0 {
				t.Fatalf("%d requests to the upstream, want none while the store answers", n)
			}
		})
	}
}

// Where the fallback fails too, its answer is the answer, and the error tells
// how the store failed. A caller that gave up does not make the fallback be
// asked.
func TestClientFallbackErrors(t *testing.T) {
	store, upstream := newFakeRegistry(t), newFakeRegistry(t)
	store.status.Store(http.StatusForbidden)
	client := storeThenUpstream(t, store, upstream)

	_, err := client.ListTags(context.Background(), "echo")
	var terr *transport.Error
	if !errors.As(err, &terr) || terr.StatusCode != http.StatusNotFound || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err = %v; want the 404 of the upstream, telling the 403 of the store", err)
	}

	upstream.requests.Store(0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = client.ListTags(ctx, "echo")
	if err == nil || strings.Contains(err.Error(), upstream.host) || upstream.requests.Load() != 0 {
		t.Fatalf("err = %v, %d requests to the upstream; want the error of the store alone", err, upstream.requests.Load())
	}
}

func tarLayer(t *testing.T, files map[string]string) v1.Layer {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
	})
	if err != nil {
		t.Fatal(err)
	}

	return layer
}

func TestReleaseVersion(t *testing.T) {
	docs := tarLayer(t, map[string]string{"docs/README.md": strings.Repeat("x", 4096)})

	img, err := mutate.AppendLayers(empty.Image, docs, tarLayer(t, map[string]string{"module.yaml": "name: echo", "version.json": `{"version": "v1.70.5"}`}))
	if err != nil {
		t.Fatal(err)
	}
	version, err := ReleaseVersion(img)
	if err != nil {
		t.Fatal(err)
	}
	if version != "v1.70.5" {
		t.Fatalf("version = %q, want v1.70.5", version)
	}

	img, err = mutate.AppendLayers(empty.Image, docs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReleaseVersion(img); !errors.Is(err, ErrNoVersion) {
		t.Fatalf("err = %v, want ErrNoVersion for an image without version.json", err)
	}
}

func TestRequestKind(t *testing.T) {
	tests := []struct {
		method, url, want string
	}{
		{http.MethodGet, "https://r.example/v2/", "ping"},
		{http.MethodGet, "https://r.example/auth?scope=repository:m/echo/release:pull&service=Docker+registry", "token"},
		{http.MethodHead, "https://r.example/v2/m/echo/release/manifests/stable", "manifest_head"},
		{http.MethodGet, "https://r.example/v2/m/echo/release/manifests/sha256:abc", "manifest"},
		{http.MethodGet, "https://r.example/v2/m/echo/release/blobs/sha256:abc", "blob"},
		{http.MethodGet, "https://r.example/v2/m/echo/release/tags/list", "tags"},
		{http.MethodGet, "https://storage.example/docker/registry/v2/blobs/sha256/ab/abc/data?X-Amz-Signature=x", "other"},
	}
	for _, tt := range tests {
		req, err := http.NewRequest(tt.method, tt.url, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := requestKind(req); got != tt.want {
			t.Errorf("%s %s: %q, want %q", tt.method, tt.url, got, tt.want)
		}
	}
}

func TestClientCountsRequestsAndKeepsTheConnection(t *testing.T) {
	srv := httptest.NewServer(ggcrregistry.New())
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	want := pushModule(t, host, []name.Option{name.Insecure}, nil, "echo", "v1.2.3", "stable")

	ms := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())
	if err := metrics.RegisterMetrics(ms); err != nil {
		t.Fatal(err)
	}
	clients := NewClients()
	clients.Metrics = ms
	client, err := clients.For(Source{Name: "src", Repo: host + "/modules", Scheme: "HTTP"})
	if err != nil {
		t.Fatal(err)
	}

	for range 2 {
		got, err := client.ReleaseDigest(context.Background(), "echo", "stable")
		if err != nil || got != want {
			t.Fatalf("digest %q, %v; want %q", got, err, want)
		}
	}

	rec := httptest.NewRecorder()
	ms.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	scrape := rec.Body.String()
	// /v2/ is checked once for both: the client keeps it.
	for _, line := range []string{
		`docs_builder_ondemand_registry_requests_total{code="2xx",request="ping"} 1`,
		`docs_builder_ondemand_registry_requests_total{code="2xx",request="manifest_head"} 2`,
	} {
		if !strings.Contains(scrape, line) {
			t.Errorf("metrics have no %s:\n%s", line, grepLines(scrape, "registry_requests"))
		}
	}
}

// A registry that took a request and does not answer fails it after
// responseHeaderTimeout instead of holding the caller; the next request gets
// a new connection.
func TestClientGivesUpOnASilentRegistry(t *testing.T) {
	old := responseHeaderTimeout
	responseHeaderTimeout = 100 * time.Millisecond
	t.Cleanup(func() { responseHeaderTimeout = old })

	reg := ggcrregistry.New()
	var silent atomic.Bool
	gone := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodHead && silent.Load() {
			select {
			case <-req.Context().Done():
			case <-gone:
			}
			return
		}
		reg.ServeHTTP(w, req)
	}))
	defer srv.Close()
	defer close(gone)

	host := strings.TrimPrefix(srv.URL, "http://")
	want := pushModule(t, host, []name.Option{name.Insecure}, nil, "echo", "v1.2.3", "stable")
	client, err := NewClient(Source{Name: "src", Repo: host + "/modules", Scheme: "HTTP"})
	if err != nil {
		t.Fatal(err)
	}

	digest := func() (string, error) {
		t.Helper()
		type result struct {
			digest string
			err    error
		}
		done := make(chan result, 1)
		go func() {
			digest, err := client.ReleaseDigest(context.Background(), "echo", "stable")
			done <- result{digest, err}
		}()
		select {
		case got := <-done:
			return got.digest, got.err
		case <-time.After(30 * time.Second):
			t.Fatal("the client still waits for the registry")
			return "", nil
		}
	}

	silent.Store(true)
	if _, err := digest(); err == nil {
		t.Fatal("want an error from a registry that does not answer")
	}
	silent.Store(false)
	if got, err := digest(); err != nil || got != want {
		t.Fatalf("digest %q, %v once the registry answers; want %q", got, err, want)
	}
}

func grepLines(s, sub string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, sub) {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

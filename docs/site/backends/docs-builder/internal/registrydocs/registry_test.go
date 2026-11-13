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

package registrydocs

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	ggcrregistry "github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/registry"
)

// testRegistry is an in-memory registry with two module sources, src1 and src2.
type testRegistry struct {
	t    *testing.T
	host string
	// failing are request path prefixes the registry refuses (403): an
	// error other than a missing image, which the client does not retry.
	failing atomic.Pointer[[]string]
	// noHeadDigest answers HEAD without Docker-Content-Digest, as some
	// registries do; goneByDigest answers 404 to a manifest asked by digest.
	noHeadDigest, goneByDigest atomic.Bool
	// silent picks the requests the registry sends nothing more for, after
	// what silent wrote, until the client gives up or the test ends (gone).
	silent atomic.Pointer[func(w http.ResponseWriter, req *http.Request) bool]
	gone   chan struct{}

	mu       sync.Mutex
	requests []string
}

// takeRequests returns "METHOD path" of the requests since the last call.
func (r *testRegistry) takeRequests() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := r.requests
	r.requests = nil

	return out
}

func newTestRegistry(t *testing.T) *testRegistry {
	t.Helper()

	r := &testRegistry{t: t, gone: make(chan struct{})}
	reg := ggcrregistry.New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		r.requests = append(r.requests, req.Method+" "+req.URL.Path)
		r.mu.Unlock()
		if r.goneByDigest.Load() && strings.Contains(req.URL.Path, "/manifests/sha256:") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"errors":[{"code":"MANIFEST_UNKNOWN","message":"manifest unknown"}]}`)
			return
		}
		if r.noHeadDigest.Load() && req.Method == http.MethodHead {
			w = noDigestWriter{w}
		}
		if failing := r.failing.Load(); failing != nil {
			for _, prefix := range *failing {
				if strings.HasPrefix(req.URL.Path, prefix) {
					http.Error(w, "refused", http.StatusForbidden)
					return
				}
			}
		}
		if silent := r.silent.Load(); silent != nil && (*silent)(w, req) {
			select {
			case <-req.Context().Done():
			case <-r.gone:
			}
			return
		}
		reg.ServeHTTP(w, req)
	}))
	t.Cleanup(srv.Close)
	// Before srv.Close, which waits for the requests.
	t.Cleanup(func() { close(r.gone) })
	r.host = strings.TrimPrefix(srv.URL, "http://")

	return r
}

// noDigestWriter drops Docker-Content-Digest from a response.
type noDigestWriter struct{ http.ResponseWriter }

func (w noDigestWriter) WriteHeader(code int) {
	w.Header().Del("Docker-Content-Digest")
	w.ResponseWriter.WriteHeader(code)
}

// pushIndex puts under <source>/<path> an index of one linux/amd64 image
// with the given files, as a multi-platform build does.
func (r *testRegistry) pushIndex(path string, files ...entry) {
	r.t.Helper()

	img, err := mutate.AppendLayers(empty.Image, layer(r.t, files...))
	if err != nil {
		r.t.Fatal(err)
	}
	idx := mutate.AppendManifests(empty.Index, mutate.IndexAddendum{
		Add:        img,
		Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: "amd64"}},
	})

	ref, err := name.ParseReference(r.host+"/"+path, name.Insecure)
	if err != nil {
		r.t.Fatal(err)
	}
	if err := remote.WriteIndex(ref, idx); err != nil {
		r.t.Fatal(err)
	}
}

// push puts an image with the given files under <source>/<path>.
func (r *testRegistry) push(path string, files ...entry) {
	r.t.Helper()

	img, err := mutate.AppendLayers(empty.Image, layer(r.t, files...))
	if err != nil {
		r.t.Fatal(err)
	}

	ref, err := name.ParseReference(r.host+"/"+path, name.Insecure)
	if err != nil {
		r.t.Fatal(err)
	}

	if err := remote.Write(ref, img); err != nil {
		r.t.Fatal(err)
	}
}

// untag removes the tag <source>/<path>; the image stays by its digest.
func (r *testRegistry) untag(path string) {
	r.t.Helper()

	ref, err := name.ParseReference(r.host+"/"+path, name.Insecure)
	if err != nil {
		r.t.Fatal(err)
	}

	if err := remote.Delete(ref); err != nil {
		r.t.Fatal(err)
	}
}

func (r *testRegistry) sources() *registry.Sources {
	objects := []runtime.Object{&unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "Module",
		"metadata":   map[string]any{"name": "echo"},
		"properties": map[string]any{"availableSources": []any{"src1", "src2"}},
	}}}
	for _, src := range []string{"src1", "src2"} {
		objects = append(objects, &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "deckhouse.io/v1alpha1",
			"kind":       "ModuleSource",
			"metadata":   map[string]any{"name": src},
			"spec":       map[string]any{"registry": map[string]any{"repo": r.host + "/" + src, "scheme": "HTTP"}},
		}})
	}

	return registry.NewSources(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), objects...), log.NewNop())
}

// storeThenUpstream is one source read from the store under store/, and from
// the upstream under upstream/ where the store fails: the in-cluster registry
// of the registry module with a cache, as registry.Sources gives it.
func (r *testRegistry) storeThenUpstream() fixedSources {
	return fixedSources{{Name: "deckhouse (store)", Repo: r.host + "/store", Scheme: "HTTP",
		Fallback: &registry.Source{Name: "deckhouse (upstream)", Repo: r.host + "/upstream", Scheme: "HTTP"}}}
}

// fixedSources gives every module the same sources.
type fixedSources []registry.Source

func (s fixedSources) ForModule(context.Context, string) ([]registry.Source, error) {
	return s, nil
}

// release is the content of a release image of the version.
func release(version string) []entry {
	return []entry{{name: "version.json", body: `{"version": "` + version + `"}`}}
}

// errOf is the lookup of the module echo with its error only.
func errOf[T any](lookup func(ctx context.Context, module string) (T, error)) func() error {
	return func() error {
		_, err := lookup(context.Background(), "echo")
		return err
	}
}

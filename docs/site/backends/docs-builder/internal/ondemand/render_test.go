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

package ondemand

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// readmeOf returns the docs/README.md of the archive, or "" without docs.
func readmeOf(t *testing.T, path string) string {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return ""
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Name == "docs/README.md" {
			b, err := io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
			return string(b)
		}
	}
}

// errTransient marks a test case that expects a registry error that is
// neither nil nor ErrVersionNotFound.
var errTransient = errors.New("transient")

func TestFetchDocs(t *testing.T) {
	docs := func(from string) []entry {
		return []entry{{name: "module.yaml", body: "name: echo"}, {name: "docs/README.md", body: from}}
	}
	noDocs := []entry{{name: "module.yaml", body: "name: echo"}}

	tests := []struct {
		name    string
		images  map[string][]entry // path in the registry -> files
		failing []string           // request paths the registry refuses
		readme  string             // docs/README.md of the fetched archive
		wantErr error
	}{
		{
			name:   "the release image has docs",
			images: map[string][]entry{"src1/echo/release:v1.0.0": docs("src1 release"), "src1/echo:v1.0.0": docs("src1 module")},
			readme: "src1 release",
		},
		{
			name:   "an older release image without docs falls back to the module image",
			images: map[string][]entry{"src1/echo/release:v1.0.0": noDocs, "src1/echo:v1.0.0": docs("src1 module")},
			readme: "src1 module",
		},
		{
			name:   "no release image",
			images: map[string][]entry{"src1/echo:v1.0.0": docs("src1 module")},
			readme: "src1 module",
		},
		{
			name:   "the first source has neither image",
			images: map[string][]entry{"src2/echo/release:v1.0.0": docs("src2 release")},
			readme: "src2 release",
		},
		{
			name:   "no docs in any image",
			images: map[string][]entry{"src1/echo/release:v1.0.0": noDocs, "src1/echo:v1.0.0": noDocs},
			readme: "",
		},
		{
			// A failed request may hide the docs: not the final "no docs".
			name:    "no docs in the release image, the module image fails",
			images:  map[string][]entry{"src1/echo/release:v1.0.0": noDocs, "src1/echo:v1.0.0": docs("src1 module")},
			failing: []string{"/v2/src1/echo/manifests/", "/v2/src2/"},
			wantErr: errTransient,
		},
		{
			// A layer that fails to download may hold the docs: not the final "no docs".
			name:    "the layers fail to download",
			images:  map[string][]entry{"src1/echo/release:v1.0.0": docs("src1 release"), "src1/echo:v1.0.0": docs("src1 module")},
			failing: []string{"/v2/src1/echo/blobs/", "/v2/src1/echo/release/blobs/"},
			wantErr: errTransient,
		},
		{
			name:    "the version does not exist",
			images:  map[string][]entry{"src1/echo:v9.9.9": docs("other version")},
			wantErr: ErrVersionNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry(t)
			for path, files := range tt.images {
				r.push(path, files...)
			}
			r.failing.Store(&tt.failing)

			path := filepath.Join(t.TempDir(), "docs.tar")
			_, err := fetchDocs(context.Background(), r.sources(), registry.NewClients(), "echo", "v1.0.0", path, log.NewNop())
			if tt.wantErr == errTransient {
				if err == nil || errors.Is(err, ErrVersionNotFound) {
					t.Fatalf("err = %v, want a registry error", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}

			if got := readmeOf(t, path); got != tt.readme {
				t.Fatalf("docs/README.md = %q, want %q", got, tt.readme)
			}
		})
	}
}

// A version the store does not hold is not found, although the upstream
// behind it refuses docs-builder: the store fetches what it does not hold
// from the upstream itself, so its "not found" is final.
func TestFetchDocsNotInTheStore(t *testing.T) {
	r := newTestRegistry(t)
	r.push("store/echo/release:v1.0.0", release("v1.0.0")...)
	r.failing.Store(&[]string{"/v2/upstream/"})

	_, err := fetchDocs(context.Background(), r.storeThenUpstream(), registry.NewClients(), "echo", "v9.9.9", filepath.Join(t.TempDir(), "docs.tar"), log.NewNop())
	if !errors.Is(err, ErrVersionNotFound) {
		t.Fatalf("err = %v, want ErrVersionNotFound", err)
	}
}

func TestReason(t *testing.T) {
	for err, want := range map[error]string{
		fmt.Errorf("hugo build: %w", context.DeadlineExceeded):                           ReasonTimeout,
		fmt.Errorf("%w: source deckhouse: unauthorized", errFetch):                       ReasonRegistry,
		fmt.Errorf("%w: %w", errFetch, fmt.Errorf("pull: %w", context.DeadlineExceeded)): ReasonTimeout,
		errors.New("hugo build: template error"):                                         ReasonBuild,
	} {
		if got := reason(err); got != want {
			t.Errorf("reason(%v) = %q, want %q", err, got, want)
		}
	}
}

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
	"archive/tar"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/ondemand"
	"github.com/flant/docs-builder/internal/registry"
)

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
// neither nil nor ondemand.ErrVersionNotFound.
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
			wantErr: ondemand.ErrVersionNotFound,
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
				if err == nil || errors.Is(err, ondemand.ErrVersionNotFound) {
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
	if !errors.Is(err, ondemand.ErrVersionNotFound) {
		t.Fatalf("err = %v, want ondemand.ErrVersionNotFound", err)
	}
}

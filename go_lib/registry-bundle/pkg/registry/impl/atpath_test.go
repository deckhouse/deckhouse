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

package impl

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/errs"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/registry/mocks"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/types"
)

func newAtPathRegistry(t *testing.T, rootRepo string, inner *mocks.MockRegistry) *AtPathRegistry {
	t.Helper()
	reg, err := NewAtPathRegistry(rootRepo, inner)
	if err != nil {
		t.Fatalf("NewAtPathRegistry: %v", err)
	}
	return reg.(*AtPathRegistry)
}

func TestNewAtPathRegistry(t *testing.T) {
	t.Run("nil inner registry", func(t *testing.T) {
		_, err := NewAtPathRegistry("root", nil)
		if err == nil {
			t.Fatal("expected error for nil inner registry")
		}
	})

	t.Run("empty root passes through", func(t *testing.T) {
		_, err := NewAtPathRegistry("", mocks.NopRegistry())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("valid root", func(t *testing.T) {
		_, err := NewAtPathRegistry("myroot", mocks.NopRegistry())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestAtPathRegistry_Fetch(t *testing.T) {
	content := "blob content"
	dgst := digest.FromString(content)

	newRegistry := func(rootRepo, innerRepo string) *AtPathRegistry {
		inner := mocks.NopRegistry()
		inner.FetchFunc = func(_ context.Context, repo string, d digest.Digest) (io.ReadCloser, error) {
			if repo != innerRepo {
				return nil, errs.ErrUnknownRepository
			}
			if d != dgst {
				return nil, errs.ErrBlobNotFound
			}
			return io.NopCloser(strings.NewReader(content)), nil
		}
		return newAtPathRegistry(t, rootRepo, inner)
	}

	tests := []struct {
		name    string
		reg     *AtPathRegistry
		repo    string
		dgst    digest.Digest
		wantErr error
		wantOK  bool
	}{
		{
			name:   "exact root repo",
			reg:    newRegistry("root", ""),
			repo:   "root",
			dgst:   dgst,
			wantOK: true,
		},
		{
			name:   "sub-repo under root",
			reg:    newRegistry("root", "sub"),
			repo:   "root/sub",
			dgst:   dgst,
			wantOK: true,
		},
		{
			name:    "wrong root",
			reg:     newRegistry("root", ""),
			repo:    "other",
			dgst:    dgst,
			wantErr: errs.ErrUnknownRepository,
		},
		{
			name:   "empty root passes repo through",
			reg:    newRegistry("", "myrepo"),
			repo:   "myrepo",
			dgst:   dgst,
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc, err := tt.reg.Fetch(t.Context(), tt.repo, tt.dgst)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Fetch error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Fetch unexpected error: %v", err)
			}
			defer rc.Close()
			if !tt.wantOK {
				t.Fatal("expected no result")
			}
		})
	}
}

func TestAtPathRegistry_Exists(t *testing.T) {
	content := "blob content"
	dgst := digest.FromString(content)

	newRegistry := func(rootRepo, innerRepo string) *AtPathRegistry {
		inner := mocks.NopRegistry()
		inner.ExistsFunc = func(_ context.Context, repo string, d digest.Digest) (bool, int64, error) {
			if repo != innerRepo {
				return false, 0, errs.ErrUnknownRepository
			}
			if d != dgst {
				return false, 0, nil
			}
			return true, int64(len(content)), nil
		}
		return newAtPathRegistry(t, rootRepo, inner)
	}

	tests := []struct {
		name      string
		reg       *AtPathRegistry
		repo      string
		dgst      digest.Digest
		wantErr   error
		wantExist bool
		wantSize  int64
	}{
		{
			name:      "exact root repo",
			reg:       newRegistry("root", ""),
			repo:      "root",
			dgst:      dgst,
			wantExist: true,
			wantSize:  int64(len(content)),
		},
		{
			name:      "sub-repo under root",
			reg:       newRegistry("root", "sub"),
			repo:      "root/sub",
			dgst:      dgst,
			wantExist: true,
			wantSize:  int64(len(content)),
		},
		{
			name:    "wrong root",
			reg:     newRegistry("root", ""),
			repo:    "other",
			dgst:    dgst,
			wantErr: errs.ErrUnknownRepository,
		},
		{
			name:      "empty root passes repo through",
			reg:       newRegistry("", "myrepo"),
			repo:      "myrepo",
			dgst:      dgst,
			wantExist: true,
			wantSize:  int64(len(content)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, size, err := tt.reg.Exists(t.Context(), tt.repo, tt.dgst)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Exists error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Exists unexpected error: %v", err)
			}
			if exists != tt.wantExist {
				t.Errorf("exists = %v, want %v", exists, tt.wantExist)
			}
			if size != tt.wantSize {
				t.Errorf("size = %d, want %d", size, tt.wantSize)
			}
		})
	}
}

func TestAtPathRegistry_Resolve(t *testing.T) {
	content := `{"schemaVersion":2}`
	dgst := digest.FromString(content)
	desc := types.ShortDescriptor{
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Digest:    dgst,
		Size:      int64(len(content)),
	}

	newRegistry := func(rootRepo, innerRepo string) *AtPathRegistry {
		inner := mocks.NopRegistry()
		inner.ResolveFunc = func(_ context.Context, repo, _ string) (types.ShortDescriptor, io.ReadCloser, error) {
			if repo != innerRepo {
				return types.ShortDescriptor{}, nil, errs.ErrUnknownRepository
			}
			return desc, io.NopCloser(strings.NewReader(content)), nil
		}
		return newAtPathRegistry(t, rootRepo, inner)
	}

	tests := []struct {
		name    string
		reg     *AtPathRegistry
		repo    string
		ref     string
		wantErr error
		wantOK  bool
	}{
		{
			name:   "exact root repo",
			reg:    newRegistry("root", ""),
			repo:   "root",
			ref:    "latest",
			wantOK: true,
		},
		{
			name:   "sub-repo under root",
			reg:    newRegistry("root", "sub"),
			repo:   "root/sub",
			ref:    "latest",
			wantOK: true,
		},
		{
			name:    "wrong root",
			reg:     newRegistry("root", ""),
			repo:    "other",
			ref:     "latest",
			wantErr: errs.ErrUnknownRepository,
		},
		{
			name:   "empty root passes repo through",
			reg:    newRegistry("", "myrepo"),
			repo:   "myrepo",
			ref:    "latest",
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, rc, err := tt.reg.Resolve(t.Context(), tt.repo, tt.ref)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Resolve error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve unexpected error: %v", err)
			}
			defer rc.Close()
			if d.Digest != dgst {
				t.Errorf("digest = %v, want %v", d.Digest, dgst)
			}
		})
	}
}

func TestAtPathRegistry_Predecessors(t *testing.T) {
	dgst := digest.FromString("manifest")
	ref := ocispec.Descriptor{
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Digest:    digest.FromString("ref"),
		Size:      10,
	}

	newRegistry := func(rootRepo, innerRepo string) *AtPathRegistry {
		inner := mocks.NopRegistry()
		inner.PredecessorsFunc = func(_ context.Context, repo string, d digest.Digest) ([]ocispec.Descriptor, error) {
			if repo != innerRepo {
				return nil, errs.ErrUnknownRepository
			}
			if d != dgst {
				return nil, nil
			}
			return []ocispec.Descriptor{ref}, nil
		}
		return newAtPathRegistry(t, rootRepo, inner)
	}

	tests := []struct {
		name      string
		reg       *AtPathRegistry
		repo      string
		dgst      digest.Digest
		wantErr   error
		wantCount int
	}{
		{
			name:      "exact root repo",
			reg:       newRegistry("root", ""),
			repo:      "root",
			dgst:      dgst,
			wantCount: 1,
		},
		{
			name:      "sub-repo under root",
			reg:       newRegistry("root", "sub"),
			repo:      "root/sub",
			dgst:      dgst,
			wantCount: 1,
		},
		{
			name:    "wrong root",
			reg:     newRegistry("root", ""),
			repo:    "other",
			dgst:    dgst,
			wantErr: errs.ErrUnknownRepository,
		},
		{
			name:      "empty root passes repo through",
			reg:       newRegistry("", "myrepo"),
			repo:      "myrepo",
			dgst:      dgst,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			descs, err := tt.reg.Predecessors(t.Context(), tt.repo, tt.dgst)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Predecessors error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Predecessors unexpected error: %v", err)
			}
			if len(descs) != tt.wantCount {
				t.Errorf("count = %d, want %d", len(descs), tt.wantCount)
			}
		})
	}
}

func TestAtPathRegistry_SortedTags(t *testing.T) {
	newRegistry := func(rootRepo, innerRepo string, tags []string) *AtPathRegistry {
		inner := mocks.NopRegistry()
		inner.SortedTagsFunc = func(_ context.Context, repo, last string) ([]string, error) {
			if repo != innerRepo {
				return nil, errs.ErrUnknownRepository
			}
			if last == "" {
				return tags, nil
			}
			for i, tag := range tags {
				if tag > last {
					return tags[i:], nil
				}
			}
			return nil, nil
		}
		return newAtPathRegistry(t, rootRepo, inner)
	}

	tests := []struct {
		name     string
		reg      *AtPathRegistry
		repo     string
		last     string
		wantErr  error
		wantTags []string
	}{
		{
			name:     "exact root repo",
			reg:      newRegistry("root", "", []string{"v1", "v2"}),
			repo:     "root",
			wantTags: []string{"v1", "v2"},
		},
		{
			name:     "sub-repo under root",
			reg:      newRegistry("root", "sub", []string{"v1", "v2"}),
			repo:     "root/sub",
			wantTags: []string{"v1", "v2"},
		},
		{
			name:    "wrong root",
			reg:     newRegistry("root", "", []string{"v1"}),
			repo:    "other",
			wantErr: errs.ErrUnknownRepository,
		},
		{
			name:     "empty root passes repo through",
			reg:      newRegistry("", "myrepo", []string{"v1", "v2"}),
			repo:     "myrepo",
			wantTags: []string{"v1", "v2"},
		},
		{
			name:     "with last",
			reg:      newRegistry("root", "", []string{"v1", "v2", "v3"}),
			repo:     "root",
			last:     "v1",
			wantTags: []string{"v2", "v3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags, err := tt.reg.SortedTags(t.Context(), tt.repo, tt.last)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("SortedTags error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("SortedTags unexpected error: %v", err)
			}
			if len(tags) != len(tt.wantTags) {
				t.Errorf("tags = %v, want %v", tags, tt.wantTags)
			}
		})
	}
}

func TestAtPathRegistry_SortedRepos(t *testing.T) {
	tests := []struct {
		name       string
		rootRepo   string
		innerRepos []string
		wantRepos  []string
	}{
		{
			name:       "empty root returns inner names unchanged",
			rootRepo:   "",
			innerRepos: []string{"a", "b", "c"},
			wantRepos:  []string{"a", "b", "c"},
		},
		{
			name:       "root prefixes all inner names",
			rootRepo:   "root",
			innerRepos: []string{"a", "b"},
			wantRepos:  []string{"root/a", "root/b"},
		},
		{
			name:       "inner empty string maps to root itself",
			rootRepo:   "root",
			innerRepos: []string{"", "sub"},
			wantRepos:  []string{"root", "root/sub"},
		},
		{
			name:       "no inner repos",
			rootRepo:   "root",
			innerRepos: []string{},
			wantRepos:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner := mocks.NopRegistry()
			inner.SortedReposFunc = func() []string { return tt.innerRepos }
			reg := newAtPathRegistry(t, tt.rootRepo, inner)

			repos := reg.SortedRepos()
			if len(repos) != len(tt.wantRepos) {
				t.Fatalf("repos = %v, want %v", repos, tt.wantRepos)
			}
			for i, r := range repos {
				if r != tt.wantRepos[i] {
					t.Errorf("repos[%d] = %q, want %q", i, r, tt.wantRepos[i])
				}
			}
		})
	}
}

func TestAtPathRegistry_StripRootRepo(t *testing.T) {
	tests := []struct {
		name     string
		rootRepo string
		repo     string
		wantSub  string
		wantOK   bool
	}{
		{
			name:     "empty root returns repo unchanged",
			rootRepo: "",
			repo:     "anything",
			wantSub:  "anything",
			wantOK:   true,
		},
		{
			name:     "exact match returns empty sub",
			rootRepo: "root",
			repo:     "root",
			wantSub:  "",
			wantOK:   true,
		},
		{
			name:     "prefix match returns sub path",
			rootRepo: "root",
			repo:     "root/sub",
			wantSub:  "sub",
			wantOK:   true,
		},
		{
			name:     "deep sub path",
			rootRepo: "a/b",
			repo:     "a/b/c/d",
			wantSub:  "c/d",
			wantOK:   true,
		},
		{
			name:     "no match returns false",
			rootRepo: "root",
			repo:     "other",
			wantSub:  "",
			wantOK:   false,
		},
		{
			name:     "partial prefix does not match",
			rootRepo: "root",
			repo:     "rootmore",
			wantSub:  "",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &AtPathRegistry{rootRepo: tt.rootRepo}
			sub, ok := reg.stripRootRepo(tt.repo)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if sub != tt.wantSub {
				t.Errorf("sub = %q, want %q", sub, tt.wantSub)
			}
		})
	}
}

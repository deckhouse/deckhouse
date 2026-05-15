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
	"slices"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/errs"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/registry"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/store"
	storemocks "github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/store/mocks"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/types"
)

// resolverImpl is a test implementation of StoreResolver backed by a map.
type resolverImpl struct {
	stores map[string]store.Store
}

func (r *resolverImpl) StoreByRepo(repo string, fn func(ok bool, st store.Store)) {
	st, ok := r.stores[repo]
	fn(ok, st)
}

func (r *resolverImpl) SortedRepos() []string {
	repos := make([]string, 0, len(r.stores))

	for repo := range r.stores {
		repos = append(repos, repo)
	}

	slices.Sort(repos)
	return repos
}

func newStoreRegistry(t *testing.T, repos map[string]store.Store) registry.Registry {
	t.Helper()
	reg, err := NewStoreRegistry(&resolverImpl{stores: repos})
	if err != nil {
		t.Fatalf("NewStoreRegistry: %v", err)
	}
	return reg
}

func TestNewStoreRegistry(t *testing.T) {
	t.Run("nil resolver", func(t *testing.T) {
		_, err := NewStoreRegistry(nil)
		if err == nil {
			t.Fatal("expected error for nil resolver")
		}
	})

	t.Run("valid resolver", func(t *testing.T) {
		_, err := NewStoreRegistry(&resolverImpl{stores: map[string]store.Store{}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestStoreRegistry_Fetch(t *testing.T) {
	content := "blob content"
	dgst := digest.FromString(content)

	newRegistry := func(repo string) registry.Registry {
		st := storemocks.NopStore()
		st.FetchFunc = func(_ context.Context, d digest.Digest) (io.ReadCloser, error) {
			if d != dgst {
				return nil, errs.ErrBlobNotFound
			}
			return io.NopCloser(strings.NewReader(content)), nil
		}
		return newStoreRegistry(t, map[string]store.Store{repo: st})
	}

	tests := []struct {
		name    string
		reg     registry.Registry
		repo    string
		dgst    digest.Digest
		wantErr error
		wantOK  bool
	}{
		{
			name:   "existing repo",
			reg:    newRegistry("myrepo"),
			repo:   "myrepo",
			dgst:   dgst,
			wantOK: true,
		},
		{
			name:    "unknown repo",
			reg:     newRegistry("myrepo"),
			repo:    "unknown",
			dgst:    dgst,
			wantErr: errs.ErrUnknownRepository,
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
			body, _ := io.ReadAll(rc)
			if string(body) != content {
				t.Errorf("body = %q, want %q", string(body), content)
			}
		})
	}
}

func TestStoreRegistry_Exists(t *testing.T) {
	content := "blob content"
	dgst := digest.FromString(content)

	newRegistry := func(repo string) registry.Registry {
		st := storemocks.NopStore()
		st.ExistsFunc = func(_ context.Context, d digest.Digest) (bool, int64, error) {
			if d != dgst {
				return false, 0, nil
			}
			return true, int64(len(content)), nil
		}
		return newStoreRegistry(t, map[string]store.Store{repo: st})
	}

	tests := []struct {
		name      string
		reg       registry.Registry
		repo      string
		dgst      digest.Digest
		wantErr   error
		wantExist bool
		wantSize  int64
	}{
		{
			name:      "existing repo",
			reg:       newRegistry("myrepo"),
			repo:      "myrepo",
			dgst:      dgst,
			wantExist: true,
			wantSize:  int64(len(content)),
		},
		{
			name:    "unknown repo",
			reg:     newRegistry("myrepo"),
			repo:    "unknown",
			dgst:    dgst,
			wantErr: errs.ErrUnknownRepository,
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

func TestStoreRegistry_Resolve(t *testing.T) {
	content := `{"schemaVersion":2}`
	dgst := digest.FromString(content)
	desc := types.ShortDescriptor{
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Digest:    dgst,
		Size:      int64(len(content)),
	}

	newRegistry := func(repo string) registry.Registry {
		st := storemocks.NopStore()
		st.ResolveFunc = func(_ context.Context, _ string) (types.ShortDescriptor, io.ReadCloser, error) {
			return desc, io.NopCloser(strings.NewReader(content)), nil
		}
		return newStoreRegistry(t, map[string]store.Store{repo: st})
	}

	tests := []struct {
		name    string
		reg     registry.Registry
		repo    string
		ref     string
		wantErr error
		wantOK  bool
	}{
		{
			name:   "existing repo",
			reg:    newRegistry("myrepo"),
			repo:   "myrepo",
			ref:    "latest",
			wantOK: true,
		},
		{
			name:    "unknown repo",
			reg:     newRegistry("myrepo"),
			repo:    "unknown",
			ref:     "latest",
			wantErr: errs.ErrUnknownRepository,
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

func TestStoreRegistry_Predecessors(t *testing.T) {
	dgst := digest.FromString("manifest")
	ref := ocispec.Descriptor{
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Digest:    digest.FromString("ref"),
		Size:      10,
	}

	newRegistry := func(repo string) registry.Registry {
		st := storemocks.NopStore()
		st.PredecessorsFunc = func(_ context.Context, d digest.Digest) ([]ocispec.Descriptor, error) {
			if d != dgst {
				return nil, nil
			}
			return []ocispec.Descriptor{ref}, nil
		}
		return newStoreRegistry(t, map[string]store.Store{repo: st})
	}

	tests := []struct {
		name      string
		reg       registry.Registry
		repo      string
		dgst      digest.Digest
		wantErr   error
		wantCount int
	}{
		{
			name:      "existing repo",
			reg:       newRegistry("myrepo"),
			repo:      "myrepo",
			dgst:      dgst,
			wantCount: 1,
		},
		{
			name:    "unknown repo",
			reg:     newRegistry("myrepo"),
			repo:    "unknown",
			dgst:    dgst,
			wantErr: errs.ErrUnknownRepository,
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

func TestStoreRegistry_SortedTags(t *testing.T) {
	newRegistry := func(repo string, tags []string) registry.Registry {
		st := storemocks.NopStore()
		st.SortedTagsFunc = func(_ context.Context, last string) ([]string, error) {
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
		return newStoreRegistry(t, map[string]store.Store{repo: st})
	}

	tests := []struct {
		name     string
		reg      registry.Registry
		repo     string
		last     string
		wantErr  error
		wantTags []string
	}{
		{
			name:     "all tags",
			reg:      newRegistry("myrepo", []string{"v1", "v2", "v3"}),
			repo:     "myrepo",
			wantTags: []string{"v1", "v2", "v3"},
		},
		{
			name:     "with last",
			reg:      newRegistry("myrepo", []string{"v1", "v2", "v3"}),
			repo:     "myrepo",
			last:     "v1",
			wantTags: []string{"v2", "v3"},
		},
		{
			name:    "unknown repo",
			reg:     newRegistry("myrepo", []string{"v1"}),
			repo:    "unknown",
			wantErr: errs.ErrUnknownRepository,
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
				return
			}
			for i, tag := range tags {
				if tag != tt.wantTags[i] {
					t.Errorf("tags[%d] = %q, want %q", i, tag, tt.wantTags[i])
				}
			}
		})
	}
}

func TestStoreRegistry_SortedRepos(t *testing.T) {
	tests := []struct {
		name      string
		sorted    []string
		wantRepos []string
	}{
		{
			name:      "returns repos from resolver",
			sorted:    []string{"a", "b", "c"},
			wantRepos: []string{"a", "b", "c"},
		},
		{
			name:      "empty repos",
			sorted:    []string{},
			wantRepos: []string{},
		},
		{
			name:      "nil repos",
			sorted:    nil,
			wantRepos: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoStore := make(map[string]store.Store, len(tt.sorted))
			for _, repo := range tt.sorted {
				repoStore[repo] = &storemocks.MockStore{}
			}
			reg := newStoreRegistry(t, repoStore)

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

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

// newRegistryForRepo returns a MockRegistry that owns only the given repo.
// Requests for any other repo return ErrUnknownRepository.
// The caller sets up only the method under test on the returned mock.
func newRegistryForRepo(owner string) *mocks.MockRegistry {
	r := mocks.NopRegistry()
	r.FetchFunc = func(_ context.Context, repo string, _ digest.Digest) (io.ReadCloser, error) {
		if repo != owner {
			return nil, errs.ErrUnknownRepository
		}
		panic("FetchFunc not configured for " + repo)
	}
	r.ExistsFunc = func(_ context.Context, repo string, _ digest.Digest) (bool, int64, error) {
		if repo != owner {
			return false, 0, errs.ErrUnknownRepository
		}
		panic("ExistsFunc not configured for " + repo)
	}
	r.ResolveFunc = func(_ context.Context, repo, _ string) (types.ShortDescriptor, io.ReadCloser, error) {
		if repo != owner {
			return types.ShortDescriptor{}, nil, errs.ErrUnknownRepository
		}
		panic("ResolveFunc not configured for " + repo)
	}
	r.PredecessorsFunc = func(_ context.Context, repo string, _ digest.Digest) ([]ocispec.Descriptor, error) {
		if repo != owner {
			return nil, errs.ErrUnknownRepository
		}
		panic("PredecessorsFunc not configured for " + repo)
	}
	r.SortedTagsFunc = func(_ context.Context, repo, _ string) ([]string, error) {
		if repo != owner {
			return nil, errs.ErrUnknownRepository
		}
		panic("SortedTagsFunc not configured for " + repo)
	}
	r.SortedReposFunc = func() []string { return nil }
	return r
}

func TestNewFallbackRegistry(t *testing.T) {
	t.Run("no registries", func(t *testing.T) {
		_, err := NewFallbackRegistry()
		if err == nil {
			t.Fatal("expected error for empty input")
		}
	})

	t.Run("nil registry in list", func(t *testing.T) {
		_, err := NewFallbackRegistry(mocks.NopRegistry(), nil, mocks.NopRegistry())
		if err == nil {
			t.Fatal("expected error for nil registry")
		}
	})

	t.Run("single registry returned directly", func(t *testing.T) {
		inner := mocks.NopRegistry()
		reg, err := NewFallbackRegistry(inner)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if reg != inner {
			t.Fatal("single-input case must return the input directly")
		}
	})

	t.Run("two registries ok", func(t *testing.T) {
		_, err := NewFallbackRegistry(mocks.NopRegistry(), mocks.NopRegistry())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestFallbackRegistry_Fetch(t *testing.T) {
	dgst := digest.FromString("x")

	newFetch := func(owner, body string) *mocks.MockRegistry {
		r := newRegistryForRepo(owner)
		r.FetchFunc = func(_ context.Context, repo string, _ digest.Digest) (io.ReadCloser, error) {
			if repo != owner {
				return nil, errs.ErrUnknownRepository
			}
			return io.NopCloser(strings.NewReader(body)), nil
		}
		return r
	}

	tests := []struct {
		name     string
		repo     string
		wantBody string
		wantErr  error
	}{
		{
			name:     "hits first registry",
			repo:     "r1",
			wantBody: "body1",
		},
		{
			name:     "skips first, hits second",
			repo:     "r2",
			wantBody: "body2",
		},
		{
			name:     "skips first and second, hits third",
			repo:     "r3",
			wantBody: "body3",
		},
		{
			name:    "all registries miss",
			repo:    "unknown",
			wantErr: errs.ErrUnknownRepository,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, err := NewFallbackRegistry(
				newFetch("r1", "body1"),
				newFetch("r2", "body2"),
				newFetch("r3", "body3"),
			)
			if err != nil {
				t.Fatalf("NewFallbackRegistry: %v", err)
			}

			rc, err := reg.Fetch(t.Context(), tt.repo, dgst)
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
			body, _ := io.ReadAll(rc)
			if string(body) != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestFallbackRegistry_Exists(t *testing.T) {
	dgst := digest.FromString("x")

	newExists := func(owner string, size int64) *mocks.MockRegistry {
		r := newRegistryForRepo(owner)
		r.ExistsFunc = func(_ context.Context, repo string, _ digest.Digest) (bool, int64, error) {
			if repo != owner {
				return false, 0, errs.ErrUnknownRepository
			}
			return true, size, nil
		}
		return r
	}

	tests := []struct {
		name      string
		repo      string
		wantExist bool
		wantSize  int64
		wantErr   error
	}{
		{
			name:      "hits first registry",
			repo:      "r1",
			wantExist: true,
			wantSize:  10,
		},
		{
			name:      "skips first, hits second",
			repo:      "r2",
			wantExist: true,
			wantSize:  20,
		},
		{
			name:      "skips first and second, hits third",
			repo:      "r3",
			wantExist: true,
			wantSize:  30,
		},
		{
			name:    "all registries miss",
			repo:    "unknown",
			wantErr: errs.ErrUnknownRepository,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, err := NewFallbackRegistry(
				newExists("r1", 10),
				newExists("r2", 20),
				newExists("r3", 30),
			)
			if err != nil {
				t.Fatalf("NewFallbackRegistry: %v", err)
			}

			exists, size, err := reg.Exists(t.Context(), tt.repo, dgst)
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

func TestFallbackRegistry_Resolve(t *testing.T) {
	dgst1 := digest.FromString("manifest1")
	dgst2 := digest.FromString("manifest2")
	dgst3 := digest.FromString("manifest3")

	newResolve := func(owner string, d digest.Digest) *mocks.MockRegistry {
		r := newRegistryForRepo(owner)
		r.ResolveFunc = func(_ context.Context, repo, _ string) (types.ShortDescriptor, io.ReadCloser, error) {
			if repo != owner {
				return types.ShortDescriptor{}, nil, errs.ErrUnknownRepository
			}
			return types.ShortDescriptor{Digest: d, Size: 1}, io.NopCloser(strings.NewReader("{}")), nil
		}
		return r
	}

	tests := []struct {
		name       string
		repo       string
		wantDigest digest.Digest
		wantErr    error
	}{
		{
			name:       "hits first registry",
			repo:       "r1",
			wantDigest: dgst1,
		},
		{
			name:       "skips first, hits second",
			repo:       "r2",
			wantDigest: dgst2,
		},
		{
			name:       "skips first and second, hits third",
			repo:       "r3",
			wantDigest: dgst3,
		},
		{
			name:    "all registries miss",
			repo:    "unknown",
			wantErr: errs.ErrUnknownRepository,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, err := NewFallbackRegistry(
				newResolve("r1", dgst1),
				newResolve("r2", dgst2),
				newResolve("r3", dgst3),
			)
			if err != nil {
				t.Fatalf("NewFallbackRegistry: %v", err)
			}

			desc, rc, err := reg.Resolve(t.Context(), tt.repo, "latest")
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
			if desc.Digest != tt.wantDigest {
				t.Errorf("digest = %v, want %v", desc.Digest, tt.wantDigest)
			}
		})
	}
}

func TestFallbackRegistry_Predecessors(t *testing.T) {
	dgst := digest.FromString("manifest")

	newPredecessors := func(owner string, ref ocispec.Descriptor) *mocks.MockRegistry {
		r := newRegistryForRepo(owner)
		r.PredecessorsFunc = func(_ context.Context, repo string, _ digest.Digest) ([]ocispec.Descriptor, error) {
			if repo != owner {
				return nil, errs.ErrUnknownRepository
			}
			return []ocispec.Descriptor{ref}, nil
		}
		return r
	}

	ref1 := ocispec.Descriptor{
		Digest: digest.FromString("ref1"),
		Size:   1,
	}
	ref2 := ocispec.Descriptor{
		Digest: digest.FromString("ref2"),
		Size:   2,
	}
	ref3 := ocispec.Descriptor{
		Digest: digest.FromString("ref3"),
		Size:   3,
	}

	tests := []struct {
		name       string
		repo       string
		wantDigest digest.Digest
		wantErr    error
	}{
		{
			name:       "hits first registry",
			repo:       "r1",
			wantDigest: ref1.Digest,
		},
		{
			name:       "skips first, hits second",
			repo:       "r2",
			wantDigest: ref2.Digest,
		},
		{
			name:       "skips first and second, hits third",
			repo:       "r3",
			wantDigest: ref3.Digest,
		},
		{
			name:    "all registries miss",
			repo:    "unknown",
			wantErr: errs.ErrUnknownRepository,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, err := NewFallbackRegistry(
				newPredecessors("r1", ref1),
				newPredecessors("r2", ref2),
				newPredecessors("r3", ref3),
			)
			if err != nil {
				t.Fatalf("NewFallbackRegistry: %v", err)
			}

			descs, err := reg.Predecessors(t.Context(), tt.repo, dgst)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Predecessors error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Predecessors unexpected error: %v", err)
			}
			if len(descs) != 1 || descs[0].Digest != tt.wantDigest {
				t.Errorf("predecessors[0].Digest = %v, want %v", descs[0].Digest, tt.wantDigest)
			}
		})
	}
}

func TestFallbackRegistry_SortedTags(t *testing.T) {
	newSortedTags := func(owner string, tags []string) *mocks.MockRegistry {
		r := newRegistryForRepo(owner)
		r.SortedTagsFunc = func(_ context.Context, repo, _ string) ([]string, error) {
			if repo != owner {
				return nil, errs.ErrUnknownRepository
			}
			return tags, nil
		}
		return r
	}

	tests := []struct {
		name     string
		repo     string
		wantTags []string
		wantErr  error
	}{
		{
			name:     "hits first registry",
			repo:     "r1",
			wantTags: []string{"v1"},
		},
		{
			name:     "skips first, hits second",
			repo:     "r2",
			wantTags: []string{"v2"},
		},
		{
			name:     "skips first and second, hits third",
			repo:     "r3",
			wantTags: []string{"v3"},
		},
		{
			name:    "all registries miss",
			repo:    "unknown",
			wantErr: errs.ErrUnknownRepository,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, err := NewFallbackRegistry(
				newSortedTags("r1", []string{"v1"}),
				newSortedTags("r2", []string{"v2"}),
				newSortedTags("r3", []string{"v3"}),
			)
			if err != nil {
				t.Fatalf("NewFallbackRegistry: %v", err)
			}

			tags, err := reg.SortedTags(t.Context(), tt.repo, "")
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("SortedTags error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("SortedTags unexpected error: %v", err)
			}
			if len(tags) != len(tt.wantTags) || tags[0] != tt.wantTags[0] {
				t.Errorf("tags = %v, want %v", tags, tt.wantTags)
			}
		})
	}
}

func TestFallbackRegistry_SortedRepos(t *testing.T) {
	newSortedRepos := func(repos []string) *mocks.MockRegistry {
		r := mocks.NopRegistry()
		r.SortedReposFunc = func() []string { return repos }
		return r
	}

	tests := []struct {
		name      string
		repos1    []string
		repos2    []string
		repos3    []string
		wantRepos []string
	}{
		{
			name:      "merges and sorts all repos",
			repos1:    []string{"c", "a"},
			repos2:    []string{"b"},
			repos3:    []string{"d"},
			wantRepos: []string{"a", "b", "c", "d"},
		},
		{
			name:      "deduplicates repos present in multiple registries",
			repos1:    []string{"a", "b"},
			repos2:    []string{"b", "c"},
			repos3:    []string{"c", "d"},
			wantRepos: []string{"a", "b", "c", "d"},
		},
		{
			name:      "handles empty registries",
			repos1:    []string{"a"},
			repos2:    nil,
			repos3:    nil,
			wantRepos: []string{"a"},
		},
		{
			name:      "all empty",
			repos1:    nil,
			repos2:    nil,
			repos3:    nil,
			wantRepos: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, err := NewFallbackRegistry(
				newSortedRepos(tt.repos1),
				newSortedRepos(tt.repos2),
				newSortedRepos(tt.repos3),
			)
			if err != nil {
				t.Fatalf("NewFallbackRegistry: %v", err)
			}

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

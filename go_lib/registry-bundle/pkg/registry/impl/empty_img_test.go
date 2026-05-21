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
	"errors"
	"io"
	"testing"

	"github.com/opencontainers/go-digest"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/errs"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/utils/set"
)

func newEmptyImgRegistry(t *testing.T, repoTags map[string]set.Set[string]) *EmptyImgRegistry {
	t.Helper()
	reg, err := NewEmptyImgRegistry(repoTags)
	if err != nil {
		t.Fatalf("NewEmptyImgRegistry: %v", err)
	}
	return reg.(*EmptyImgRegistry)
}

func TestNewEmptyImgRegistry(t *testing.T) {
	t.Run("empty repoTags", func(t *testing.T) {
		_, err := NewEmptyImgRegistry(map[string]set.Set[string]{})
		if err == nil {
			t.Fatal("expected error for empty repoTags")
		}
	})

	t.Run("empty tag set for a repo", func(t *testing.T) {
		_, err := NewEmptyImgRegistry(map[string]set.Set[string]{
			"myrepo": set.New[string](),
		})
		if err == nil {
			t.Fatal("expected error for empty tag set")
		}
	})

	t.Run("valid single repo", func(t *testing.T) {
		tags := set.New[string]()
		tags.Add("v1")
		_, err := NewEmptyImgRegistry(map[string]set.Set[string]{
			"myrepo": tags,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("valid multiple repos", func(t *testing.T) {
		tags1 := set.New[string]()
		tags1.Add("v1")
		tags2 := set.New[string]()
		tags2.Add("v2")
		_, err := NewEmptyImgRegistry(map[string]set.Set[string]{
			"repo1": tags1,
			"repo2": tags2,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestEmptyImgRegistry_Fetch(t *testing.T) {
	tags := set.New[string]()
	tags.Add("v1")
	reg := newEmptyImgRegistry(t, map[string]set.Set[string]{"myrepo": tags})

	t.Run("unknown repo", func(t *testing.T) {
		_, err := reg.Fetch(t.Context(), "unknown", reg.img.manifest.Digest)
		if !errors.Is(err, errs.ErrUnknownRepository) {
			t.Fatalf("Fetch error = %v, want %v", err, errs.ErrUnknownRepository)
		}
	})

	t.Run("manifest blob", func(t *testing.T) {
		rc, err := reg.Fetch(t.Context(), "myrepo", reg.img.manifest.Digest)
		if err != nil {
			t.Fatalf("Fetch unexpected error: %v", err)
		}
		defer rc.Close()
		body, _ := io.ReadAll(rc)
		if len(body) == 0 {
			t.Error("expected non-empty manifest blob")
		}
	})

	t.Run("unknown digest", func(t *testing.T) {
		_, err := reg.Fetch(t.Context(), "myrepo", digest.FromString("unknown"))
		if !errors.Is(err, errs.ErrBlobNotFound) {
			t.Fatalf("Fetch error = %v, want %v", err, errs.ErrBlobNotFound)
		}
	})
}

func TestEmptyImgRegistry_Exists(t *testing.T) {
	tags := set.New[string]()
	tags.Add("v1")
	reg := newEmptyImgRegistry(t, map[string]set.Set[string]{"myrepo": tags})

	t.Run("unknown repo", func(t *testing.T) {
		_, _, err := reg.Exists(t.Context(), "unknown", reg.img.manifest.Digest)
		if !errors.Is(err, errs.ErrUnknownRepository) {
			t.Fatalf("Exists error = %v, want %v", err, errs.ErrUnknownRepository)
		}
	})

	t.Run("manifest blob exists", func(t *testing.T) {
		exists, size, err := reg.Exists(t.Context(), "myrepo", reg.img.manifest.Digest)
		if err != nil {
			t.Fatalf("Exists unexpected error: %v", err)
		}
		if !exists {
			t.Error("expected exists = true")
		}
		if size != reg.img.manifest.Size {
			t.Errorf("size = %d, want %d", size, reg.img.manifest.Size)
		}
	})

	t.Run("unknown digest", func(t *testing.T) {
		_, _, err := reg.Exists(t.Context(), "myrepo", digest.FromString("unknown"))
		if !errors.Is(err, errs.ErrBlobNotFound) {
			t.Fatalf("Exists error = %v, want %v", err, errs.ErrBlobNotFound)
		}
	})
}

func TestEmptyImgRegistry_Resolve(t *testing.T) {
	tags := set.New[string]()
	tags.Add("v1")
	tags.Add("v2")
	reg := newEmptyImgRegistry(t, map[string]set.Set[string]{"myrepo": tags})

	t.Run("empty reference", func(t *testing.T) {
		_, _, err := reg.Resolve(t.Context(), "myrepo", "")
		if !errors.Is(err, errs.ErrMissingReference) {
			t.Fatalf("Resolve error = %v, want %v", err, errs.ErrMissingReference)
		}
	})

	t.Run("unknown repo", func(t *testing.T) {
		_, _, err := reg.Resolve(t.Context(), "unknown", "v1")
		if !errors.Is(err, errs.ErrUnknownRepository) {
			t.Fatalf("Resolve error = %v, want %v", err, errs.ErrUnknownRepository)
		}
	})

	t.Run("known tag", func(t *testing.T) {
		desc, rc, err := reg.Resolve(t.Context(), "myrepo", "v1")
		if err != nil {
			t.Fatalf("Resolve unexpected error: %v", err)
		}
		defer rc.Close()
		if desc.Digest != reg.img.manifest.Digest {
			t.Errorf("digest = %v, want %v", desc.Digest, reg.img.manifest.Digest)
		}
	})

	t.Run("resolve by digest", func(t *testing.T) {
		desc, rc, err := reg.Resolve(t.Context(), "myrepo", reg.img.manifest.Digest.String())
		if err != nil {
			t.Fatalf("Resolve unexpected error: %v", err)
		}
		defer rc.Close()
		if desc.Digest != reg.img.manifest.Digest {
			t.Errorf("digest = %v, want %v", desc.Digest, reg.img.manifest.Digest)
		}
	})

	t.Run("unknown reference", func(t *testing.T) {
		_, _, err := reg.Resolve(t.Context(), "myrepo", "nonexistent")
		if !errors.Is(err, errs.ErrManifestNotFound) {
			t.Fatalf("Resolve error = %v, want %v", err, errs.ErrManifestNotFound)
		}
	})
}

func TestEmptyImgRegistry_Predecessors(t *testing.T) {
	tags := set.New[string]()
	tags.Add("v1")
	reg := newEmptyImgRegistry(t, map[string]set.Set[string]{"myrepo": tags})

	t.Run("unknown repo", func(t *testing.T) {
		_, err := reg.Predecessors(t.Context(), "unknown", reg.img.manifest.Digest)
		if !errors.Is(err, errs.ErrUnknownRepository) {
			t.Fatalf("Predecessors error = %v, want %v", err, errs.ErrUnknownRepository)
		}
	})

	t.Run("manifest digest returns config", func(t *testing.T) {
		descs, err := reg.Predecessors(t.Context(), "myrepo", reg.img.manifest.Digest)
		if err != nil {
			t.Fatalf("Predecessors unexpected error: %v", err)
		}
		if len(descs) == 0 {
			t.Error("expected at least one successor descriptor")
		}
	})

	t.Run("unknown digest returns empty", func(t *testing.T) {
		descs, err := reg.Predecessors(t.Context(), "myrepo", digest.FromString("unknown"))
		if err != nil {
			t.Fatalf("Predecessors unexpected error: %v", err)
		}
		if len(descs) != 0 {
			t.Errorf("expected empty descs, got %v", descs)
		}
	})
}

func TestEmptyImgRegistry_SortedTags(t *testing.T) {
	tags := set.New[string]()
	tags.Add("v3")
	tags.Add("v1")
	tags.Add("v2")
	reg := newEmptyImgRegistry(t, map[string]set.Set[string]{"myrepo": tags})

	t.Run("unknown repo", func(t *testing.T) {
		_, err := reg.SortedTags(t.Context(), "unknown", "")
		if !errors.Is(err, errs.ErrUnknownRepository) {
			t.Fatalf("SortedTags error = %v, want %v", err, errs.ErrUnknownRepository)
		}
	})

	t.Run("all tags sorted", func(t *testing.T) {
		got, err := reg.SortedTags(t.Context(), "myrepo", "")
		if err != nil {
			t.Fatalf("SortedTags unexpected error: %v", err)
		}
		want := []string{"v1", "v2", "v3"}
		if len(got) != len(want) {
			t.Fatalf("tags = %v, want %v", got, want)
		}
		for i, tag := range got {
			if tag != want[i] {
				t.Errorf("tags[%d] = %q, want %q", i, tag, want[i])
			}
		}
	})

	t.Run("with last skips up to and including last", func(t *testing.T) {
		got, err := reg.SortedTags(t.Context(), "myrepo", "v1")
		if err != nil {
			t.Fatalf("SortedTags unexpected error: %v", err)
		}
		want := []string{"v2", "v3"}
		if len(got) != len(want) {
			t.Fatalf("tags = %v, want %v", got, want)
		}
		for i, tag := range got {
			if tag != want[i] {
				t.Errorf("tags[%d] = %q, want %q", i, tag, want[i])
			}
		}
	})

	t.Run("last beyond all tags returns empty", func(t *testing.T) {
		got, err := reg.SortedTags(t.Context(), "myrepo", "v3")
		if err != nil {
			t.Fatalf("SortedTags unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("expected empty tags, got %v", got)
		}
	})
}

func TestEmptyImgRegistry_SortedRepos(t *testing.T) {
	tags1 := set.New[string]()
	tags1.Add("v1")
	tags2 := set.New[string]()
	tags2.Add("v2")
	tags3 := set.New[string]()
	tags3.Add("v3")

	reg := newEmptyImgRegistry(t, map[string]set.Set[string]{
		"c-repo": tags1,
		"a-repo": tags2,
		"b-repo": tags3,
	})

	repos := reg.SortedRepos()
	want := []string{"a-repo", "b-repo", "c-repo"}
	if len(repos) != len(want) {
		t.Fatalf("repos = %v, want %v", repos, want)
	}
	for i, r := range repos {
		if r != want[i] {
			t.Errorf("repos[%d] = %q, want %q", i, r, want[i])
		}
	}
}

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

package dhregistry_test

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/registry/client"
	"github.com/deckhouse/deckhouse/pkg/registry/fake"
)

// multiArchIndex builds an index whose children differ only in the file they
// carry, so a test can tell which one a request resolved to.
func multiArchIndex(t *testing.T, arches ...string) v1.ImageIndex {
	t.Helper()

	idx := v1.ImageIndex(empty.Index)

	for _, arch := range arches {
		img := fake.NewImageBuilder().
			WithPlatform("linux", arch).
			WithFile("arch", arch).
			MustBuild()

		idx = mutate.AppendManifests(idx, mutate.IndexAddendum{
			Add:        img,
			Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: arch}},
		})
	}

	return idx
}

// TestGetIndex covers reading a multi-arch index whole. GetImage resolves an
// index down to one child, so a caller that needs every platform — a mirror,
// or anything re-pushing the index — has to ask for the index itself.
func TestGetIndex(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddIndex("deckhouse/fe/modules/stronghold", "v1.0.1", multiArchIndex(t, "amd64", "arm64"))

	module := newFakeRegistry(t, reg).Modules().Module("stronghold")

	idx, err := module.GetIndex(t.Context(), "v1.0.1")
	require.NoError(t, err)

	manifest, err := idx.IndexManifest()
	require.NoError(t, err)
	require.Len(t, manifest.Manifests, 2)

	var arches []string
	for _, child := range manifest.Manifests {
		arches = append(arches, child.Platform.Architecture)
	}

	assert.ElementsMatch(t, []string{"amd64", "arm64"}, arches)
}

// TestGetIndexOnPlainImage pins that asking for an index where there is only an
// image is an error, rather than a synthetic one-entry index.
func TestGetIndexOnPlainImage(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage("deckhouse/fe/modules/stronghold", "v1.0.1", fake.NewImageBuilder().MustBuild())

	_, err := newFakeRegistry(t, reg).Modules().Module("stronghold").GetIndex(t.Context(), "v1.0.1")
	assert.Error(t, err)
}

// TestGetManifestPlatform covers resolving an index down to one child's
// manifest. Without the option the index itself comes back — unlike GetImage,
// which silently resolves to linux/amd64.
func TestGetManifestPlatform(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddIndex("deckhouse/fe/modules/stronghold", "v1.0.1", multiArchIndex(t, "amd64", "arm64"))

	module := newFakeRegistry(t, reg).Modules().Module("stronghold")

	t.Run("without platform the index comes back", func(t *testing.T) {
		result, err := module.GetManifest(t.Context(), "v1.0.1")
		require.NoError(t, err)

		assert.True(t, result.GetMediaType().IsIndex())

		index, err := result.GetIndexManifest()
		require.NoError(t, err)
		assert.Len(t, index.GetManifests(), 2)
	})

	t.Run("with platform one child comes back", func(t *testing.T) {
		result, err := module.GetManifest(t.Context(), "v1.0.1",
			client.WithPlatform{Platform: &v1.Platform{OS: "linux", Architecture: "arm64"}})
		require.NoError(t, err)

		assert.False(t, result.GetMediaType().IsIndex())

		// A manifest does not carry its own platform — that lives in the index
		// entry pointing at it — so the child is identified by its digest.
		idx, err := module.GetIndex(t.Context(), "v1.0.1")
		require.NoError(t, err)

		index, err := idx.IndexManifest()
		require.NoError(t, err)

		var want v1.Hash
		for _, child := range index.Manifests {
			if child.Platform.Architecture == "arm64" {
				want = child.Digest
			}
		}

		assert.Equal(t, want, result.GetDescriptor().GetDigest())
	})
}

// TestFetchPlatform covers the same choice on a bundle: Fetch resolves to
// linux/amd64 unless the caller says otherwise.
func TestFetchPlatform(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")

	idx := v1.ImageIndex(empty.Index)

	for _, arch := range []string{"amd64", "arm64"} {
		img := fake.NewImageBuilder().
			WithPlatform("linux", arch).
			WithFile("images_digests.json", `{"image":"sha256:`+arch+`"}`).
			MustBuild()

		idx = mutate.AppendManifests(idx, mutate.IndexAddendum{
			Add:        img,
			Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: arch}},
		})
	}

	reg.MustAddIndex("deckhouse/fe/modules/stronghold", "v1.0.1", idx)

	module := newFakeRegistry(t, reg).Modules().Module("stronghold")

	b, err := module.Fetch(t.Context(), "v1.0.1")
	require.NoError(t, err)
	assert.Equal(t, "sha256:amd64", b.Digests().Images["image"], "no platform must resolve to amd64")

	b, err = module.Fetch(t.Context(), "v1.0.1",
		client.WithPlatform{Platform: &v1.Platform{OS: "linux", Architecture: "arm64"}})
	require.NoError(t, err)
	assert.Equal(t, "sha256:arm64", b.Digests().Images["image"])
}

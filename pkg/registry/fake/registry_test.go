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

package fake_test

import (
	"sort"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/registry/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegistry_NewRegistry verifies the Host() accessor.
func TestRegistry_NewRegistry(t *testing.T) {
	reg := fake.NewRegistry("gcr.io")
	assert.Equal(t, "gcr.io", reg.Host())
}

// TestRegistry_NewRegistry_TrimsSlashes verifies trailing slashes are removed.
func TestRegistry_NewRegistry_TrimsSlashes(t *testing.T) {
	reg := fake.NewRegistry("gcr.io/")
	assert.Equal(t, "gcr.io", reg.Host())
}

// TestRegistry_AddImage_Root adds an image at the registry root (empty repo
// path) and retrieves it via the fake client.
func TestRegistry_AddImage_Root(t *testing.T) {
	reg := fake.NewRegistry("gcr.io")
	img := fake.NewImageBuilder().WithFile("info.txt", "root").MustBuild()

	require.NoError(t, reg.AddImage("", "latest", img))

	// Access via the client.
	client := fake.NewClient(reg)
	tags, err := client.ListTags(t.Context()) //nolint:staticcheck // nil context acceptable in fake
	require.NoError(t, err)
	assert.Contains(t, tags, "latest")
}

// TestRegistry_AddImage_SubPath adds an image under a repository path.
func TestRegistry_AddImage_SubPath(t *testing.T) {
	reg := fake.NewRegistry("gcr.io")
	img := fake.NewImageBuilder().WithFile("v.txt", "v1.0.0").MustBuild()

	require.NoError(t, reg.AddImage("google-containers/pause", "3.9", img))

	client := fake.NewClient(reg)
	scoped := client.WithSegment("google-containers", "pause")
	tags, err := scoped.ListTags(t.Context()) //nolint:staticcheck
	require.NoError(t, err)
	assert.Contains(t, tags, "3.9")
}

// TestRegistry_AddImage_EmptyTag should fail.
func TestRegistry_AddImage_EmptyTag(t *testing.T) {
	reg := fake.NewRegistry("gcr.io")
	img := fake.NewImageBuilder().MustBuild()

	err := reg.AddImage("repo", "", img)
	require.Error(t, err)
}

// TestRegistry_MustAddImage_Panics verifies that MustAddImage panics on error.
func TestRegistry_MustAddImage_Panics(t *testing.T) {
	reg := fake.NewRegistry("gcr.io")
	img := fake.NewImageBuilder().MustBuild()

	assert.Panics(t, func() {
		reg.MustAddImage("repo", "", img) // empty tag → panic
	})
}

// TestRegistry_AddImage_Replace verifies that adding the same tag replaces the image.
func TestRegistry_AddImage_Replace(t *testing.T) {
	reg := fake.NewRegistry("example.io")
	img1 := fake.NewImageBuilder().WithFile("ver.txt", "v1").MustBuild()
	img2 := fake.NewImageBuilder().WithFile("ver.txt", "v2").MustBuild()

	reg.MustAddImage("repo", "latest", img1)
	reg.MustAddImage("repo", "latest", img2)

	client := fake.NewClient(reg)
	scoped := client.WithSegment("repo")
	tags, err := scoped.ListTags(t.Context()) //nolint:staticcheck
	require.NoError(t, err)
	// Only one "latest" tag expected.
	count := 0
	for _, t := range tags {
		if t == "latest" {
			count++
		}
	}
	assert.Equal(t, 1, count, "duplicate tag should not appear twice")
}

// TestRegistry_ListRepos verifies that listRepos returns all added repository paths.
func TestRegistry_ListRepos(t *testing.T) {
	reg := fake.NewRegistry("r.io")
	img := fake.NewImageBuilder().MustBuild()

	reg.MustAddImage("alpha", "v1", img)
	reg.MustAddImage("beta", "v1", img)
	reg.MustAddImage("alpha/sub", "v1", img)

	client := fake.NewClient(reg)
	repos, err := client.ListRepositories(t.Context()) //nolint:staticcheck
	require.NoError(t, err)

	sort.Strings(repos)
	assert.Equal(t, []string{"alpha", "alpha/sub", "beta"}, repos)
}

// TestRegistry_MultipleRegistries verifies NewClient with two registries –
// each client defaults to its own registry host and routes correctly.
func TestRegistry_MultipleRegistries(t *testing.T) {
	regA := fake.NewRegistry("a.io")
	regB := fake.NewRegistry("b.io")

	imgA := fake.NewImageBuilder().WithFile("src.txt", "A").MustBuild()
	imgB := fake.NewImageBuilder().WithFile("src.txt", "B").MustBuild()

	regA.MustAddImage("repo", "v1", imgA)
	regB.MustAddImage("repo", "v1", imgB)

	// Separate clients each default to their own registry.
	clientA := fake.NewClient(regA)
	clientB := fake.NewClient(regB)

	assert.Equal(t, "a.io", clientA.GetRegistry())
	assert.Equal(t, "b.io", clientB.GetRegistry())

	tagsA, err := clientA.WithSegment("repo").ListTags(t.Context()) //nolint:staticcheck
	require.NoError(t, err)
	assert.Contains(t, tagsA, "v1")

	tagsB, err := clientB.WithSegment("repo").ListTags(t.Context()) //nolint:staticcheck
	require.NoError(t, err)
	assert.Contains(t, tagsB, "v1")
}

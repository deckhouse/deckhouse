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
	"errors"
	"strings"
	"testing"

	dkpclient "github.com/deckhouse/deckhouse/pkg/registry/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/registry/fake"
)

// ----- helpers -----

func newFilledRegistry(host string) *fake.Registry {
	reg := fake.NewRegistry(host)
	reg.MustAddImage("deckhouse/ee", "v1.65.0",
		fake.NewImageBuilder().
			WithFile("version.json", `{"version":"v1.65.0"}`).
			WithLabel("org.opencontainers.image.version", "v1.65.0").
			MustBuild(),
	)
	reg.MustAddImage("deckhouse/ee", "v1.64.0",
		fake.NewImageBuilder().
			WithFile("version.json", `{"version":"v1.64.0"}`).
			MustBuild(),
	)
	reg.MustAddImage("deckhouse/ee/release-channel", "stable",
		fake.NewImageBuilder().
			WithFile("version.json", `{"version":"v1.64.0"}`).
			MustBuild(),
	)
	return reg
}

// ----- WithSegment / GetRegistry -----

// GetRegistry returns the HOST portion of the current path (not host+repo).
// This matches the upstream contract where GetRegistry returns the registry host.

func TestClient_WithSegment_ChainedPaths(t *testing.T) {
	reg := fake.NewRegistry("gcr.io")
	client := fake.NewClient(reg)

	// WithSegment appends to the path; GetRegistry returns only the host part.
	scoped := client.WithSegment("org").WithSegment("repo")
	assert.Equal(t, "gcr.io", scoped.GetRegistry())
}

func TestClient_WithSegment_MultiSegments(t *testing.T) {
	reg := fake.NewRegistry("gcr.io")
	client := fake.NewClient(reg)

	scoped := client.WithSegment("org", "repo", "sub")
	assert.Equal(t, "gcr.io", scoped.GetRegistry())
}

func TestClient_GetRegistry_DefaultHost(t *testing.T) {
	reg := fake.NewRegistry("reg.example.com")
	client := fake.NewClient(reg)

	assert.Equal(t, "reg.example.com", client.GetRegistry())
}

func TestClient_WithSegment_ScopeListTags(t *testing.T) {
	reg := fake.NewRegistry("gcr.io")
	img := fake.NewImageBuilder().WithFile("f.txt", "x").MustBuild()
	reg.MustAddImage("org/repo", "v1", img)
	client := fake.NewClient(reg)

	scoped := client.WithSegment("org").WithSegment("repo")
	tags, err := scoped.ListTags(t.Context())
	require.NoError(t, err)
	assert.Contains(t, tags, "v1")
}

// ----- GetDigest -----

func TestClient_GetDigest_ExistingTag(t *testing.T) {
	reg := newFilledRegistry("gcr.io")
	client := fake.NewClient(reg).WithSegment("deckhouse", "ee")

	hash, err := client.GetDigest(t.Context(), "v1.65.0")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(hash.String(), "sha256:"))
}

func TestClient_GetDigest_MissingTag(t *testing.T) {
	reg := newFilledRegistry("gcr.io")
	client := fake.NewClient(reg).WithSegment("deckhouse", "ee")

	_, err := client.GetDigest(t.Context(), "does-not-exist")
	require.Error(t, err)
	assert.True(t, errors.Is(err, dkpclient.ErrImageNotFound))
}

// ----- GetManifest -----

func TestClient_GetManifest_ExistingTag(t *testing.T) {
	reg := newFilledRegistry("gcr.io")
	client := fake.NewClient(reg).WithSegment("deckhouse", "ee")

	manifest, err := client.GetManifest(t.Context(), "v1.65.0")
	require.NoError(t, err)
	require.NotNil(t, manifest)
}

func TestClient_GetManifest_MissingTag(t *testing.T) {
	reg := newFilledRegistry("gcr.io")
	client := fake.NewClient(reg).WithSegment("deckhouse", "ee")

	_, err := client.GetManifest(t.Context(), "missing")
	require.Error(t, err)
}

// ----- GetImageConfig -----

func TestClient_GetImageConfig_ExistingTag(t *testing.T) {
	reg := newFilledRegistry("gcr.io")
	client := fake.NewClient(reg).WithSegment("deckhouse", "ee")

	cfg, err := client.GetImageConfig(t.Context(), "v1.65.0")
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "v1.65.0", cfg.Config.Labels["org.opencontainers.image.version"])
}

// ----- CheckImageExists -----

func TestClient_CheckImageExists_Present(t *testing.T) {
	reg := newFilledRegistry("gcr.io")
	client := fake.NewClient(reg).WithSegment("deckhouse", "ee")

	err := client.CheckImageExists(t.Context(), "v1.65.0")
	assert.NoError(t, err)
}

func TestClient_CheckImageExists_Absent(t *testing.T) {
	reg := newFilledRegistry("gcr.io")
	client := fake.NewClient(reg).WithSegment("deckhouse", "ee")

	err := client.CheckImageExists(t.Context(), "v2.0.0")
	require.Error(t, err)
	assert.True(t, errors.Is(err, dkpclient.ErrImageNotFound))
}

// ----- GetImage -----

func TestClient_GetImage_ByTag(t *testing.T) {
	reg := newFilledRegistry("gcr.io")
	client := fake.NewClient(reg).WithSegment("deckhouse", "ee")

	img, err := client.GetImage(t.Context(), "v1.65.0")
	require.NoError(t, err)
	require.NotNil(t, img)
}

func TestClient_GetImage_ByDigest(t *testing.T) {
	reg := newFilledRegistry("gcr.io")
	client := fake.NewClient(reg).WithSegment("deckhouse", "ee")

	// Retrieve digest first.
	hash, err := client.GetDigest(t.Context(), "v1.65.0")
	require.NoError(t, err)

	// Look up by digest reference (@sha256:...).
	img, err := client.GetImage(t.Context(), "@"+hash.String())
	require.NoError(t, err)
	require.NotNil(t, img)
}

func TestClient_GetImage_MissingTag(t *testing.T) {
	reg := newFilledRegistry("gcr.io")
	client := fake.NewClient(reg).WithSegment("deckhouse", "ee")

	_, err := client.GetImage(t.Context(), "missing-tag")
	require.Error(t, err)
	assert.True(t, errors.Is(err, dkpclient.ErrImageNotFound))
}

// ----- ListTags -----

func TestClient_ListTags(t *testing.T) {
	reg := newFilledRegistry("gcr.io")
	client := fake.NewClient(reg).WithSegment("deckhouse", "ee")

	tags, err := client.ListTags(t.Context())
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"v1.65.0", "v1.64.0"}, tags)
}

func TestClient_ListTags_EmptyRepo(t *testing.T) {
	reg := fake.NewRegistry("gcr.io")
	client := fake.NewClient(reg).WithSegment("no-such-repo")

	tags, err := client.ListTags(t.Context())
	require.NoError(t, err)
	assert.Empty(t, tags)
}

// ----- ListRepositories -----

func TestClient_ListRepositories_AllUnderHost(t *testing.T) {
	reg := newFilledRegistry("gcr.io")
	client := fake.NewClient(reg)

	repos, err := client.ListRepositories(t.Context())
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{
		"deckhouse/ee",
		"deckhouse/ee/release-channel",
	}, repos)
}

func TestClient_ListRepositories_Scoped(t *testing.T) {
	reg := newFilledRegistry("gcr.io")
	client := fake.NewClient(reg).WithSegment("deckhouse")

	repos, err := client.ListRepositories(t.Context())
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{
		"deckhouse/ee",
		"deckhouse/ee/release-channel",
	}, repos)
}

// ----- DeleteTag -----

func TestClient_DeleteTag_Existing(t *testing.T) {
	reg := newFilledRegistry("gcr.io")
	client := fake.NewClient(reg).WithSegment("deckhouse", "ee")

	require.NoError(t, client.DeleteTag(t.Context(), "v1.65.0"))

	err := client.CheckImageExists(t.Context(), "v1.65.0")
	require.Error(t, err)
}

func TestClient_DeleteTag_Missing(t *testing.T) {
	reg := newFilledRegistry("gcr.io")
	client := fake.NewClient(reg).WithSegment("deckhouse", "ee")

	err := client.DeleteTag(t.Context(), "does-not-exist")
	require.Error(t, err)
	assert.True(t, errors.Is(err, dkpclient.ErrImageNotFound))
}

// ----- TagImage -----

func TestClient_TagImage(t *testing.T) {
	reg := newFilledRegistry("gcr.io")
	client := fake.NewClient(reg).WithSegment("deckhouse", "ee")

	require.NoError(t, client.TagImage(t.Context(), "v1.65.0", "stable"))

	// "stable" should now resolve to the same digest as "v1.65.0".
	origDigest, err := client.GetDigest(t.Context(), "v1.65.0")
	require.NoError(t, err)
	newDigest, err := client.GetDigest(t.Context(), "stable")
	require.NoError(t, err)

	assert.Equal(t, origDigest.String(), newDigest.String())
}

func TestClient_TagImage_SourceMissing(t *testing.T) {
	reg := newFilledRegistry("gcr.io")
	client := fake.NewClient(reg).WithSegment("deckhouse", "ee")

	err := client.TagImage(t.Context(), "no-such-tag", "dest")
	require.Error(t, err)
}

// ----- PushImage -----

func TestClient_PushImage_NewTag(t *testing.T) {
	reg := fake.NewRegistry("push.io")
	client := fake.NewClient(reg).WithSegment("org", "app")

	img := fake.NewImageBuilder().WithFile("app.txt", "app v1").MustBuild()

	require.NoError(t, client.PushImage(t.Context(), "v2", img))

	tags, err := client.ListTags(t.Context())
	require.NoError(t, err)
	assert.Contains(t, tags, "v2")
}

func TestClient_PushImage_AutoCreatesRegistry(t *testing.T) {
	// PushImage should auto-create a new registry entry if the host is unknown.
	reg := fake.NewRegistry("known.io")
	client := fake.NewClient(reg)

	img := fake.NewImageBuilder().MustBuild()

	scopedToUnknown := client.WithSegment("unknown.io", "repo")
	require.NoError(t, scopedToUnknown.PushImage(t.Context(), "v1", img))
}

// ----- Cross-registry routing -----

func TestClient_CrossRegistryRouting(t *testing.T) {
	regSrc := fake.NewRegistry("src.io")
	regDst := fake.NewRegistry("dst.io")

	imgSrc := fake.NewImageBuilder().WithFile("src.txt", "source").MustBuild()
	imgDst := fake.NewImageBuilder().WithFile("dst.txt", "dest").MustBuild()

	regSrc.MustAddImage("lib", "v1", imgSrc)
	regDst.MustAddImage("lib", "v1", imgDst)

	clientSrc := fake.NewClient(regSrc)
	clientDst := fake.NewClient(regDst)

	// The default registry path is reported correctly for each client.
	assert.Equal(t, "src.io", clientSrc.GetRegistry())
	assert.Equal(t, "dst.io", clientDst.GetRegistry())

	tagsFromSrc, err := clientSrc.WithSegment("lib").ListTags(t.Context())
	require.NoError(t, err)
	assert.Contains(t, tagsFromSrc, "v1")

	tagsFromDst, err := clientDst.WithSegment("lib").ListTags(t.Context())
	require.NoError(t, err)
	assert.Contains(t, tagsFromDst, "v1")
}

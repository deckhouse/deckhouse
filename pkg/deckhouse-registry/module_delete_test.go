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
	"fmt"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/registry/fake"

	dhregistry "github.com/deckhouse/deckhouse/pkg/deckhouse-registry"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/bundle"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/release"
)

// moduleRepo is where newFakeRegistry (deckhouse + fe) puts the stronghold
// module; its release images live under moduleRepo/release.
const moduleRepo = "deckhouse/fe/modules/stronghold"

// TestModuleDelete covers the whole flow: the referenced images go first, then
// the release tag, then the bundle tag — nothing of the version is left.
func TestModuleDelete(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")

	// Two images the bundle ships, stored in the module repo by digest.
	controller := fake.NewImageBuilder().WithFile("controller", "v1").MustBuild()
	webhook := fake.NewImageBuilder().WithFile("webhook", "v1").MustBuild()
	controllerDigest := mustDigest(t, controller)
	webhookDigest := mustDigest(t, webhook)
	reg.MustAddImage(moduleRepo, "controller", controller)
	reg.MustAddImage(moduleRepo, "webhook", webhook)

	// The module bundle, whose images_digests.json points at those two images.
	digestsJSON := fmt.Sprintf(`{"controller":%q,"webhook":%q}`, controllerDigest, webhookDigest)
	reg.MustAddImage(moduleRepo, "v1.0.1",
		fake.NewImageBuilder().WithFile(bundle.RootPath, digestsJSON).MustBuild())

	// The matching release image.
	reg.MustAddImage(moduleRepo+"/release", "v1.0.1",
		fake.NewImageBuilder().WithFile(release.VersionFile, `{"version":"v1.0.1"}`).MustBuild())

	module := newFakeRegistry(t, reg).Modules().Module("stronghold")

	require.NoError(t, module.Delete(t.Context(), "v1.0.1"))

	// The bundle tag is gone.
	_, err := module.Fetch(t.Context(), "v1.0.1")
	assert.True(t, dhregistry.IsNotFound(err), "bundle tag should be gone, got %v", err)

	// The matching release tag is gone.
	_, err = module.Releases().Fetch(t.Context(), "v1.0.1")
	assert.True(t, dhregistry.IsNotFound(err), "release tag should be gone, got %v", err)

	// Both referenced images are gone by digest.
	for _, digest := range []string{controllerDigest, webhookDigest} {
		_, err := module.GetImage(t.Context(), "@"+digest)
		assert.True(t, dhregistry.IsNotFound(err), "image %s should be gone, got %v", digest, err)
	}

	// Nothing of the version is left in the module repository.
	tags, err := module.ListTags(t.Context())
	require.NoError(t, err)
	assert.Empty(t, tags)
}

// TestModuleDeleteIdempotent pins that images and a release tag which are
// already gone do not fail the delete: only the bundle tag is present, and it
// still removes cleanly.
func TestModuleDeleteIdempotent(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")

	// A bundle that references an image which was never pushed, and no release
	// tag at this version — the state a half-finished earlier delete leaves.
	missing := "sha256:" + strings.Repeat("0", 64)
	digestsJSON := fmt.Sprintf(`{"controller":%q}`, missing)
	reg.MustAddImage(moduleRepo, "v1.0.1",
		fake.NewImageBuilder().WithFile(bundle.RootPath, digestsJSON).MustBuild())

	module := newFakeRegistry(t, reg).Modules().Module("stronghold")

	require.NoError(t, module.Delete(t.Context(), "v1.0.1"))

	_, err := module.Fetch(t.Context(), "v1.0.1")
	assert.True(t, dhregistry.IsNotFound(err), "bundle tag should be gone, got %v", err)
}

// TestModuleDeleteMissingBundle pins that deleting a version whose bundle tag
// does not exist reports a not-found error rather than silently succeeding.
func TestModuleDeleteMissingBundle(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage(moduleRepo, "v1.0.1",
		fake.NewImageBuilder().WithFile(bundle.RootPath, `{}`).MustBuild())

	module := newFakeRegistry(t, reg).Modules().Module("stronghold")

	err := module.Delete(t.Context(), "v9.9.9")
	require.Error(t, err)
	assert.True(t, dhregistry.IsNotFound(err), "expected a not-found error, got %v", err)
}

// TestModuleDeleteInvalidDigest pins that a malformed digest in
// images_digests.json fails the delete and, crucially, leaves the bundle tag in
// place so the failure can be retried once the data is fixed.
func TestModuleDeleteInvalidDigest(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage(moduleRepo, "v1.0.1",
		fake.NewImageBuilder().WithFile(bundle.RootPath, `{"controller":"not-a-digest"}`).MustBuild())

	module := newFakeRegistry(t, reg).Modules().Module("stronghold")

	require.Error(t, module.Delete(t.Context(), "v1.0.1"))

	// The anchor is intact: the bundle is still readable.
	_, err := module.Fetch(t.Context(), "v1.0.1")
	require.NoError(t, err)
}

// mustDigest returns the digest of img as a "sha256:" string.
func mustDigest(t *testing.T, img v1.Image) string {
	t.Helper()

	h, err := img.Digest()
	require.NoError(t, err)

	return h.String()
}

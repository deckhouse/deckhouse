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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/registry/fake"

	dhregistry "github.com/deckhouse/deckhouse/pkg/deckhouse-registry"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/bundle"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/release"
)

// packageRepo is where newFakeRegistry (deckhouse + fe) puts the elma package;
// its release images live under packageRepo/version.
const packageRepo = "deckhouse/fe/packages/elma"

// TestPackageDelete covers the whole flow: the referenced images go first, then
// the version tag, then the bundle tag — nothing of the version is left.
func TestPackageDelete(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")

	// Two images the bundle ships, stored in the package repo by digest.
	controller := fake.NewImageBuilder().WithFile("controller", "v1").MustBuild()
	webhook := fake.NewImageBuilder().WithFile("webhook", "v1").MustBuild()
	controllerDigest := mustDigest(t, controller)
	webhookDigest := mustDigest(t, webhook)
	reg.MustAddImage(packageRepo, "controller", controller)
	reg.MustAddImage(packageRepo, "webhook", webhook)

	// The package bundle, whose images_digests.json points at those two images.
	digestsJSON := fmt.Sprintf(`{"controller":%q,"webhook":%q}`, controllerDigest, webhookDigest)
	reg.MustAddImage(packageRepo, "v1.0.1",
		fake.NewImageBuilder().WithFile(bundle.RootPath, digestsJSON).MustBuild())

	// The matching release image, under version/ rather than release/.
	reg.MustAddImage(packageRepo+"/version", "v1.0.1",
		fake.NewImageBuilder().WithFile(release.VersionFile, `{"version":"v1.0.1"}`).MustBuild())

	pkg := newFakeRegistry(t, reg).Packages().Package("elma")

	require.NoError(t, pkg.Delete(t.Context(), "v1.0.1"))

	// The bundle tag is gone.
	_, err := pkg.Fetch(t.Context(), "v1.0.1")
	assert.True(t, dhregistry.IsNotFound(err), "bundle tag should be gone, got %v", err)

	// The matching version tag is gone.
	_, err = pkg.Versions().Fetch(t.Context(), "v1.0.1")
	assert.True(t, dhregistry.IsNotFound(err), "version tag should be gone, got %v", err)

	// Both referenced images are gone by digest.
	for _, digest := range []string{controllerDigest, webhookDigest} {
		_, err := pkg.GetImage(t.Context(), "@"+digest)
		assert.True(t, dhregistry.IsNotFound(err), "image %s should be gone, got %v", digest, err)
	}

	// Nothing of the version is left in the package repository.
	tags, err := pkg.ListTags(t.Context())
	require.NoError(t, err)
	assert.Empty(t, tags)
}

// TestPackageDeleteIdempotent pins that images and a version tag which are
// already gone do not fail the delete: only the bundle tag is present, and it
// still removes cleanly.
func TestPackageDeleteIdempotent(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")

	// A bundle that references an image which was never pushed, and no version
	// tag at this version — the state a half-finished earlier delete leaves.
	missing := "sha256:" + strings.Repeat("0", 64)
	digestsJSON := fmt.Sprintf(`{"controller":%q}`, missing)
	reg.MustAddImage(packageRepo, "v1.0.1",
		fake.NewImageBuilder().WithFile(bundle.RootPath, digestsJSON).MustBuild())

	pkg := newFakeRegistry(t, reg).Packages().Package("elma")

	require.NoError(t, pkg.Delete(t.Context(), "v1.0.1"))

	_, err := pkg.Fetch(t.Context(), "v1.0.1")
	assert.True(t, dhregistry.IsNotFound(err), "bundle tag should be gone, got %v", err)
}

// TestPackageDeleteMissingBundle pins that deleting a version whose bundle tag
// does not exist reports a not-found error rather than silently succeeding.
func TestPackageDeleteMissingBundle(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage(packageRepo, "v1.0.1",
		fake.NewImageBuilder().WithFile(bundle.RootPath, `{}`).MustBuild())

	pkg := newFakeRegistry(t, reg).Packages().Package("elma")

	err := pkg.Delete(t.Context(), "v9.9.9")
	require.Error(t, err)
	assert.True(t, dhregistry.IsNotFound(err), "expected a not-found error, got %v", err)
}

// TestPackageDeleteInvalidDigest pins that a malformed digest in
// images_digests.json fails the delete and, crucially, leaves the bundle tag in
// place so the failure can be retried once the data is fixed.
func TestPackageDeleteInvalidDigest(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage(packageRepo, "v1.0.1",
		fake.NewImageBuilder().WithFile(bundle.RootPath, `{"controller":"not-a-digest"}`).MustBuild())

	pkg := newFakeRegistry(t, reg).Packages().Package("elma")

	require.Error(t, pkg.Delete(t.Context(), "v1.0.1"))

	// The anchor is intact: the bundle is still readable.
	_, err := pkg.Fetch(t.Context(), "v1.0.1")
	require.NoError(t, err)
}

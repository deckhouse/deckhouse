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

package fill

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pushInstaller puts an installer image into the registry carrying the file a release
// declares its image set in — the same path `d8 mirror pull` reads.
func pushInstaller(t *testing.T, target Registry, reference string, files map[string]any) {
	t.Helper()

	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	for path, content := range files {
		encoded, err := json.Marshal(content)
		require.NoError(t, err)

		require.NoError(t, writer.WriteHeader(&tar.Header{
			Name: path, Mode: 0o644, Size: int64(len(encoded)),
		}))
		_, err = writer.Write(encoded)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	layer, err := tarball.LayerFromReader(bytes.NewReader(archive.Bytes()))
	require.NoError(t, err)

	image, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	tag, err := name.NewTag(target.Address+"/"+reference, name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(tag, image))
}

// pushByDigest puts an image in with no tag at all, which is how every image of an embedded
// module lives in the registry: the release names them by digest and nothing else does.
func pushByDigest(t *testing.T, target Registry, repository string) v1.Hash {
	t.Helper()

	image, err := random.Image(256, 1)
	require.NoError(t, err)
	digest, err := image.Digest()
	require.NoError(t, err)

	reference, err := name.NewDigest(
		fmt.Sprintf("%s/%s@%s", target.Address, repository, digest.String()), name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(reference, image))

	return digest
}

func puller(t *testing.T) *remote.Puller {
	t.Helper()

	p, err := remote.NewPuller()
	require.NoError(t, err)
	return p
}

// TestReleaseCopiesWhatTheReleaseDeclares is the whole point of this discoverer.
//
// The set comes out of the release itself, so nothing has to be inferred from what a registry
// happens to hold — and nothing has to be permitted beyond pulling the images the cluster
// already pulls. Listing a registry's catalogue is a privilege of its own, and credentials
// scoped to a repository are refused for it: what that looked like in a cluster was a fill
// retrying every thirty seconds, forever, against a registry that answered 401.
func TestReleaseCopiesWhatTheReleaseDeclares(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)
	source.Repository = "deckhouse/ee"
	destination.Repository = "system/deckhouse"

	// Two module images, addressed the way the platform addresses its own: by digest, under
	// the repository root, with no tag anywhere.
	controller := pushByDigest(t, source, "deckhouse/ee")
	agent := pushByDigest(t, source, "deckhouse/ee")

	// And the release's own images.
	pushImage(t, source, "deckhouse/ee:v1.70.1")
	pushInstaller(t, source, "deckhouse/ee/install:v1.70.1", map[string]any{
		imagesDigestsFile: map[string]map[string]string{
			"registry":    {"registryController": controller.String(), "registryAgent": agent.String()},
			"nodeManager": {},
		},
	})

	// Something the release says nothing about. A cache sized for one release must not end
	// up holding the registry's history.
	pushImage(t, source, "deckhouse/ee:v1.69.0")

	copier := &Copier{
		Source: source, Destination: destination,
		Discover: Release{Versions: []string{"v1.70.1"}},
	}

	report, err := copier.Run(context.Background())
	require.NoError(t, err)
	assert.Empty(t, report.Failed)

	// The two declared images plus the release's own. Not the installer: it is where the set was read
	// from, and a running cluster never needs it.
	assert.EqualValues(t, 3, report.Written)

	for _, digest := range []v1.Hash{controller, agent} {
		reference, err := name.NewDigest(fmt.Sprintf("%s/system/deckhouse@%s",
			destination.Address, digest.String()), name.Insecure)
		require.NoError(t, err)

		descriptor, err := remote.Get(reference)
		require.NoError(t, err, "a digest the release declares was not copied")
		assert.Equal(t, digest, descriptor.Digest)
	}

	// Copied by digest, not under some invented tag: the cluster refers to these images by
	// digest, and a tag the upstream never published would be a name only this cache knows.
	_, err = digestOf(t, destination, "system/deckhouse:v1.70.1")
	require.NoError(t, err, "the release's own tag belongs in the cache")

	// And nothing the release is silent about.
	_, err = digestOf(t, destination, "system/deckhouse:v1.69.0")
	require.Error(t, err, "a version the cluster does not run was copied")
}

// TestReleaseCoversARollback is why the previous release is included.
//
// Two consecutive versions overlap in almost every image, so the union is what has to be
// there — and it has to be counted once, or the total the fill reports would be inflated by
// the images the two versions share.
func TestReleaseCoversARollback(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)
	source.Repository = "deckhouse/ee"
	destination.Repository = "system/deckhouse"

	shared := pushByDigest(t, source, "deckhouse/ee")
	onlyNew := pushByDigest(t, source, "deckhouse/ee")

	pushImage(t, source, "deckhouse/ee:v1.70.1")
	pushImage(t, source, "deckhouse/ee:v1.69.5")
	pushInstaller(t, source, "deckhouse/ee/install:v1.70.1", map[string]any{
		imagesDigestsFile: map[string]map[string]string{
			"registry": {"a": shared.String(), "b": onlyNew.String()},
		},
	})
	pushInstaller(t, source, "deckhouse/ee/install:v1.69.5", map[string]any{
		imagesDigestsFile: map[string]map[string]string{
			"registry": {"a": shared.String()},
		},
	})

	copier := &Copier{
		Source: source, Destination: destination,
		Discover: Release{Versions: []string{"v1.70.1", "v1.69.5"}},
	}

	report, err := copier.Run(context.Background())
	require.NoError(t, err)
	assert.Empty(t, report.Failed)

	// Two digests and two release images: 2 + 2. The shared digest is counted once, and the installers
	// are read for their image lists without being kept — see TestTheInstallerIsReadButNotKept.
	assert.EqualValues(t, 4, report.Written)
}

// TestReleaseRefusesRatherThanReportingAnEmptySet is about which way this fails.
//
// Completeness is what authorizes cutting an air-gapped cluster off from its upstream, and it
// is derived from what the fill put in place. So a fill that could not work out what to copy
// has to say so: reporting an empty set would make an empty store look like a finished job.
func TestReleaseRefusesRatherThanReportingAnEmptySet(t *testing.T) {
	source := startRegistry(t)
	source.Repository = "deckhouse/ee"

	t.Run("no version to enumerate", func(t *testing.T) {
		_, err := Release{}.Discover(context.Background(), source, puller(t))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nothing to copy")
	})

	t.Run("no installer for the version", func(t *testing.T) {
		_, err := Release{Versions: []string{"v1.70.1"}}.
			Discover(context.Background(), source, puller(t))
		require.Error(t, err)
	})

	t.Run("an installer that declares nothing", func(t *testing.T) {
		pushInstaller(t, source, "deckhouse/ee/install:v1.70.2", map[string]any{
			"deckhouse/candi/something-else.json": map[string]string{},
		})

		_, err := Release{Versions: []string{"v1.70.2"}}.
			Discover(context.Background(), source, puller(t))
		require.Error(t, err)
		assert.Contains(t, err.Error(), imagesDigestsFile)
	})
}

// TestReleasePrefersTagsOverDigests follows `d8 mirror pull`: where a release records both,
// the tags are what it is published under.
func TestReleasePrefersTagsOverDigests(t *testing.T) {
	source := startRegistry(t)
	source.Repository = "deckhouse/ee"

	digest := pushByDigest(t, source, "deckhouse/ee")
	pushInstaller(t, source, "deckhouse/ee/install:v1.70.1", map[string]any{
		imagesTagsFile:    map[string]map[string]string{"registry": {"a": "some-tag"}},
		imagesDigestsFile: map[string]map[string]string{"registry": {"a": digest.String()}},
	})

	references, err := Release{Versions: []string{"v1.70.1"}}.
		Discover(context.Background(), source, puller(t))
	require.NoError(t, err)

	var identifiers []string
	for _, reference := range references {
		identifiers = append(identifiers, reference.Identifier())
	}
	assert.Contains(t, identifiers, "some-tag")
	assert.NotContains(t, identifiers, digest.String())
}

// TestCatalogueIsStillUsedBetweenOurOwnReplicas guards the distinction this split exists for.
//
// A follower copying from the leader is copying from one of our own registries, where listing
// is both permitted and exactly what it wants: whatever the leader holds. Replacing that with
// a release-driven set would make a follower unable to receive anything the leader was given
// out of band — which in an air-gapped cluster is everything, since `d8 mirror push` is the
// only way in.
func TestCatalogueIsStillUsedBetweenOurOwnReplicas(t *testing.T) {
	leader := startRegistry(t)
	follower := startRegistry(t)
	leader.Repository = "system/deckhouse"
	follower.Repository = "system/deckhouse"

	// As `d8 mirror push` leaves it: content under our own prefix, with no release
	// declaring it.
	pushImage(t, leader, "system/deckhouse/one:v1")
	pushImage(t, leader, "system/deckhouse/two:v1")

	report, err := (&Copier{
		Source: leader, Destination: follower, Discover: Catalogue{},
	}).Run(context.Background())
	require.NoError(t, err)

	assert.EqualValues(t, 2, report.Written)
	assert.Empty(t, report.Failed)
}

// TestReleaseReadsTheSetFromThePlatformImage is what removes a permission from the critical path.
//
// The set is declared in two places, and they are the same file: measured on a cluster,
// `/deckhouse/modules/images_digests.json` in the running deckhouse image is byte for byte the
// installer's `deckhouse/candi/images_digests.json` of that version — 38277 bytes in both. The
// difference is what it costs to read. Installers live in a repository of their own, and a
// credential scoped to the platform's repository is refused for it: on a live cluster the fill of
// the running version failed with `401 Unauthorized` on `sys/deckhouse-oss/install`. The platform
// image needs no permission the cluster does not already have — it is the image the cluster runs.
func TestReleaseReadsTheSetFromThePlatformImage(t *testing.T) {
	source := startRegistry(t)
	source.Repository = "deckhouse/ee"

	declared := pushByDigest(t, source, "deckhouse/ee")

	// The platform image carries the set — and no installer exists at all, standing in for one
	// this cluster's credentials cannot read.
	pushInstaller(t, source, "deckhouse/ee:v1.70.1", map[string]any{
		platformDigestsFile: map[string]map[string]string{
			"registry": {"registryController": declared.String()},
		},
	})

	puller, err := remote.NewPuller(source.Options...)
	require.NoError(t, err)

	references, err := Release{Versions: []string{"v1.70.1"}}.Discover(context.Background(), source, puller)
	require.NoError(t, err, "the running image declares the set, so no installer is needed")

	var listed []string
	for _, reference := range references {
		listed = append(listed, reference.String())
	}

	assert.Contains(t, listed, source.Address+"/deckhouse/ee@"+declared.String(),
		"the declared image is enumerated")
	assert.Contains(t, listed, source.Address+"/deckhouse/ee:v1.70.1",
		"and the release's own image with it")

	// The whole point: an installer that cannot be read is not listed, so it cannot become a
	// reference that never copies. Listed unconditionally, as it was, that one refusal made the
	// fill permanently incomplete — hence no air-gap, and no replication to any follower.
	for _, reference := range listed {
		assert.NotContains(t, reference, "/install:",
			"an unreadable installer must not be enumerated")
	}
}

// TestTheInstallerIsReadButNotKept covers the other half: a readable installer is still the fallback
// SOURCE of the set, and still not part of the set.
//
// By the owner's rule — the installer is used by the installation and by nothing in a running cluster,
// so the cluster works without it. Holding it conditionally was the worst of the options: on clusters
// where the credential covers the installer repository the set contained one more image than on
// clusters where it does not, so two clusters of the same version disagreed about what "complete" meant.
func TestTheInstallerIsReadButNotKept(t *testing.T) {
	source := startRegistry(t)
	source.Repository = "deckhouse/ee"

	declared := pushByDigest(t, source, "deckhouse/ee")

	// The platform image carries nothing, so the set has to come from the installer.
	pushImage(t, source, "deckhouse/ee:v1.70.1")
	pushInstaller(t, source, "deckhouse/ee/install:v1.70.1", map[string]any{
		imagesDigestsFile: map[string]map[string]string{
			"registry": {"registryController": declared.String()},
		},
	})

	puller, err := remote.NewPuller(source.Options...)
	require.NoError(t, err)

	references, err := Release{Versions: []string{"v1.70.1"}}.Discover(context.Background(), source, puller)
	require.NoError(t, err)

	var listed []string
	for _, reference := range references {
		listed = append(listed, reference.String())
	}

	assert.Contains(t, listed, source.Address+"/deckhouse/ee@"+declared.String(),
		"the set it declares is what gets copied")
	for _, reference := range listed {
		assert.NotContains(t, reference, "/install:",
			"the installer is where the set was read from, not part of it")
	}
}

// TestReleaseNamesBothSourcesWhenNeitherAnswers: the two failures mean different things — the
// platform image is expected to be present, the installer is expected to be permitted — and an
// operator has to see which one to act on.
func TestReleaseNamesBothSourcesWhenNeitherAnswers(t *testing.T) {
	source := startRegistry(t)
	source.Repository = "deckhouse/ee"
	pushImage(t, source, "deckhouse/ee:v1.70.1")

	puller, err := remote.NewPuller(source.Options...)
	require.NoError(t, err)

	_, err = Release{Versions: []string{"v1.70.1"}}.Discover(context.Background(), source, puller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deckhouse/ee:v1.70.1")
	assert.Contains(t, err.Error(), "deckhouse/ee/install:v1.70.1")
}

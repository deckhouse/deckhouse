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

package gc

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pushHeld puts an image in by digest and records the manifest revision on disk, which is how the
// platform's own images live in a store: written by digest, with no tag anywhere.
func pushHeld(t *testing.T, target store, repository string) string {
	t.Helper()

	image, err := random.Image(64, 1)
	require.NoError(t, err)
	digest, err := image.Digest()
	require.NoError(t, err)

	reference, err := name.NewDigest(
		fmt.Sprintf("%s/%s@%s", target.Address, repository, digest.String()), name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(reference, image))

	revisionLink(t, target.dataDir, repository, digest.String())
	return digest.String()
}

func revisionLink(t *testing.T, root, repository, digest string) {
	t.Helper()

	algorithm, hex, found := strings.Cut(digest, ":")
	require.True(t, found)

	dir := filepath.Join(root, "docker", "registry", "v2", "repositories",
		filepath.FromSlash(repository), "_manifests", "revisions", algorithm, hex)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "link"), []byte(digest), 0o644))
}

// pushInstaller puts in the image a release declares its own image set in, which is where the
// collector reads what a kept release needs — the same file `d8 mirror pull` reads.
func pushInstaller(t *testing.T, target store, version string, digests []string) {
	t.Helper()

	byImage := make(map[string]string, len(digests))
	for i, digest := range digests {
		byImage[fmt.Sprintf("image%d", i)] = digest
	}
	encoded, err := json.Marshal(map[string]map[string]string{"registry": byImage})
	require.NoError(t, err)

	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	require.NoError(t, writer.WriteHeader(&tar.Header{
		Name: "deckhouse/candi/images_digests.json", Mode: 0o644, Size: int64(len(encoded)),
	}))
	_, err = writer.Write(encoded)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	layer, err := tarball.LayerFromReader(bytes.NewReader(archive.Bytes()))
	require.NoError(t, err)
	image, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	push(t, target, "system/deckhouse/install:"+version)
	tag, err := name.NewTag(target.Address+"/system/deckhouse/install:"+version, name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(tag, image))
}

func holds(t *testing.T, target store, repository, digest string) bool {
	t.Helper()

	reference, err := name.NewDigest(
		fmt.Sprintf("%s/%s@%s", target.Address, repository, digest), name.Insecure)
	require.NoError(t, err)

	_, err = remote.Head(reference)
	return err == nil
}

// TestRunReclaimsManifestsNoKeptReleaseDeclares is the pass that reclaims the store's actual weight.
//
// Every rule above it judges tags, and the platform's own images have none: the fill writes them by
// digest. Judged by tags alone they are never even considered — and distribution's own sweep cannot
// help, because a manifest keeps its blobs reachable. The store then grows by a release set per
// update, forever, which became the only unbounded path once the pass-through cache stopped expiring
// what it caches.
func TestRunReclaimsManifestsNoKeptReleaseDeclares(t *testing.T) {
	target := startRegistry(t)

	// What the deployed release declares, and what an older one left behind.
	current := pushHeld(t, target, "system/deckhouse")
	alsoCurrent := pushHeld(t, target, "system/deckhouse/module")
	superseded := pushHeld(t, target, "system/deckhouse/module")

	push(t, target, "system/deckhouse:v1.76.6")
	pushInstaller(t, target, "v1.76.6", []string{current, alsoCurrent})

	collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, &countingSweeper{})

	report, err := collector.Run(context.Background())
	require.NoError(t, err)

	assert.True(t, holds(t, target, "system/deckhouse", current),
		"an image the deployed release declares must survive")
	assert.True(t, holds(t, target, "system/deckhouse/module", alsoCurrent))
	assert.False(t, holds(t, target, "system/deckhouse/module", superseded),
		"an image no kept release declares is what this pass exists to remove")

	assert.Equal(t, 2, report.Kept[ReasonDeclared],
		"both declared manifests are counted as kept, and by that reason")
	assert.Len(t, report.Deleted, 1, "exactly the undeclared one")
}

// TestRunKeepsWhatAnUpdateWouldNeed is the second half of the operator's rule: the current set, and
// the set the cluster might switch to.
//
// A release newer than the deployed one is an update in progress — its images were fetched so that
// the switch can happen — and deleting them would mean fetching them again, or, in an air-gapped
// cluster, not switching at all.
func TestRunKeepsWhatAnUpdateWouldNeed(t *testing.T) {
	target := startRegistry(t)

	current := pushHeld(t, target, "system/deckhouse")
	next := pushHeld(t, target, "system/deckhouse")
	old := pushHeld(t, target, "system/deckhouse")

	push(t, target, "system/deckhouse:v1.76.6")
	push(t, target, "system/deckhouse:v1.77.0")
	pushInstaller(t, target, "v1.76.6", []string{current})
	pushInstaller(t, target, "v1.77.0", []string{next})

	collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, &countingSweeper{})

	report, err := collector.Run(context.Background())
	require.NoError(t, err)

	assert.True(t, holds(t, target, "system/deckhouse", current))
	assert.True(t, holds(t, target, "system/deckhouse", next),
		"the set of an update the cluster may switch to has to survive")
	assert.False(t, holds(t, target, "system/deckhouse", old))
	assert.Contains(t, report.Kept, ReasonDeclared)
}

// TestRunDeletesNoManifestWhenTheKeepSetCannotBeRead is the refusal, and it is the most important
// test here: this pass deletes the images the cluster runs on.
//
// An unreadable keep-set and an empty one are the same thing to a loop that deletes what is not in
// it. So the run fails and touches nothing, exactly as the tag pass refuses when the deployed
// version is unknown.
func TestRunDeletesNoManifestWhenTheKeepSetCannotBeRead(t *testing.T) {
	target := startRegistry(t)

	held := pushHeld(t, target, "system/deckhouse")
	// The release tag exists, so the tag pass keeps it — but no installer was ever pushed, so
	// what the release consists of cannot be read.
	push(t, target, "system/deckhouse:v1.76.6")

	collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, &countingSweeper{})

	report, err := collector.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot establish which manifests are needed")
	assert.Empty(t, report.Deleted, "nothing may be deleted on a guess")
	assert.True(t, holds(t, target, "system/deckhouse", held))
}

// TestRunDryRunTouchesNoManifest keeps the dry run honest for the pass that can empty a store.
func TestRunDryRunTouchesNoManifest(t *testing.T) {
	target := startRegistry(t)

	current := pushHeld(t, target, "system/deckhouse")
	superseded := pushHeld(t, target, "system/deckhouse")
	push(t, target, "system/deckhouse:v1.76.6")
	pushInstaller(t, target, "v1.76.6", []string{current})

	collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, &countingSweeper{})
	collector.DryRun = true

	report, err := collector.Run(context.Background())
	require.NoError(t, err)

	assert.NotEmpty(t, report.Deleted, "a dry run still says what it would do")
	assert.True(t, holds(t, target, "system/deckhouse", superseded),
		"and does none of it")
}

// TestRunSurvivesAManifestThatWillNotGo: one refusal is not a reason to abandon the rest, and the
// blobs of whatever did go are still reclaimable.
func TestRunSurvivesAManifestThatWillNotGo(t *testing.T) {
	target := startRegistry(t)

	current := pushHeld(t, target, "system/deckhouse")
	push(t, target, "system/deckhouse:v1.76.6")
	pushInstaller(t, target, "v1.76.6", []string{current})

	// A revision recorded on disk that the registry does not actually have: deleting it fails.
	revisionLink(t, target.dataDir, "system/deckhouse",
		"sha256:9999999999999999999999999999999999999999999999999999999999999999")

	sweeper := &countingSweeper{}
	collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, sweeper)

	report, err := collector.Run(context.Background())
	require.NoError(t, err, "a manifest that will not go is not a failed run")
	assert.NotEmpty(t, report.Failed)
	assert.True(t, holds(t, target, "system/deckhouse", current))
}

// TestRunCollectsOnAClusterRunningATag is the case a live cluster stopped on: `the deployed version
// "pr21788" is not a version, so no tag can be judged older than it`, and with that the whole run
// refused — nothing was ever reclaimed.
//
// The refusal belonged to one pass, not to both. Ordering is what the TAG pass needs, and a tag
// cannot be ordered; the manifest pass needs only the set that version declares, which is readable
// for a tag exactly as for a version. So the pass that cannot work stands down, every tag is kept,
// and the digests are still judged — which is where the store's weight is anyway.
func TestRunCollectsOnAClusterRunningATag(t *testing.T) {
	target := startRegistry(t)

	current := pushHeld(t, target, "system/deckhouse")
	superseded := pushHeld(t, target, "system/deckhouse")

	push(t, target, "system/deckhouse:pr21788")
	push(t, target, "system/deckhouse:v1.70.0")
	pushInstaller(t, target, "pr21788", []string{current})

	collector := newCollector(t, target, Releases{Deployed: "pr21788"}, &countingSweeper{})

	report, err := collector.Run(context.Background())
	require.NoError(t, err, "an unorderable version disables a rule, it does not stop the run")

	assert.True(t, holds(t, target, "system/deckhouse", current),
		"what the running tag declares must survive")
	assert.False(t, holds(t, target, "system/deckhouse", superseded),
		"and what nothing declares is still reclaimed")

	// Every tag survives, including one that looks older: without an orderable deployed version
	// there is no basis for calling it old.
	assert.True(t, exists(t, target, "system/deckhouse:v1.70.0"))
	assert.True(t, exists(t, target, "system/deckhouse:pr21788"))
	assert.Positive(t, report.Kept[ReasonUnorderable])
}

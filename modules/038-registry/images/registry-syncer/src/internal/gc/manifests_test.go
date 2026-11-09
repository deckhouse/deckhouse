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
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/registry-syncer/internal/fill"
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

// TestRunCollectsOnAClusterRunningATag is the case a run used to stop on: no tag can be judged older
// than a deployed version that is not a version at all, and with that the whole run refused, so
// nothing was ever reclaimed.
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

// pushModulePackage puts in a module release the way the store keeps it: a tag under modules/<name>
// whose image carries the flat account of the module's own images.
func pushModulePackage(t *testing.T, target store, module, version string, digests []string) {
	t.Helper()

	pushModulePackageAt(t, target, "system/deckhouse/modules/"+module, version, digests)
}

// pushModulePackageAt is pushModulePackage under a repository of the caller's choosing — the path the
// store keeps the modules of another ModuleSource under.
func pushModulePackageAt(t *testing.T, target store, repository, version string, digests []string) {
	t.Helper()

	byImage := make(map[string]string, len(digests))
	for i, digest := range digests {
		byImage[fmt.Sprintf("image%d", i)] = digest
	}
	encoded, err := json.Marshal(byImage)
	require.NoError(t, err)

	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	require.NoError(t, writer.WriteHeader(&tar.Header{Name: "images_digests.json", Mode: 0o644, Size: int64(len(encoded))}))
	_, err = writer.Write(encoded)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	layer, err := tarball.LayerFromReader(bytes.NewReader(archive.Bytes()))
	require.NoError(t, err)
	image, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	push(t, target, repository+":"+version)
	tag, err := name.NewTag(target.Address+"/"+repository+":"+version, name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(tag, image))
}

// TestRunKeepsWhatAPendingModuleUpdateWouldNeed is the module half of the rule above. The fill copies a
// pending module release ahead — or `d8 mirror push` brings it — so that the update finds it in the
// store; collected as undeclared, it was gone before the update could start, and in an air-gapped
// cluster nothing could bring it back.
func TestRunKeepsWhatAPendingModuleUpdateWouldNeed(t *testing.T) {
	target := startRegistry(t)

	current := pushHeld(t, target, "system/deckhouse")
	running := pushHeld(t, target, "system/deckhouse/modules/console")
	pending := pushHeld(t, target, "system/deckhouse/modules/console")
	dropped := pushHeld(t, target, "system/deckhouse/modules/console")

	push(t, target, "system/deckhouse:v1.76.6")
	pushInstaller(t, target, "v1.76.6", []string{current})
	pushModulePackage(t, target, "console", "v1.65.2", []string{running})
	pushModulePackage(t, target, "console", "v1.66.2", []string{pending})

	collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, &countingSweeper{})
	collector.Modules = []fill.ModuleRef{
		{Name: "console", Version: "v1.65.2"},
		{Name: "console", Version: "v1.66.2", Pending: true},
	}

	_, err := collector.Run(context.Background())
	require.NoError(t, err)

	assert.True(t, holds(t, target, "system/deckhouse/modules/console", running))
	assert.True(t, holds(t, target, "system/deckhouse/modules/console", pending),
		"an image of a pending module release has to survive until the update uses it")
	assert.False(t, holds(t, target, "system/deckhouse/modules/console", dropped))
}

// TestAPendingModuleUpdateThatCannotBeReadStopsTheCollection: one pending module whose set is in the
// store and cannot be read used to take every pending module's protection with it. Read one at a time
// now: one that is not there claims nothing and stops nothing; one that is there and unreadable stops
// the collection, since its images cannot be told apart from anything else.
func TestAPendingModuleUpdateThatCannotBeReadStopsTheCollection(t *testing.T) {
	target := startRegistry(t)

	current := pushHeld(t, target, "system/deckhouse")
	running := pushHeld(t, target, "system/deckhouse/modules/console")
	pending := pushHeld(t, target, "system/deckhouse/modules/console")
	dropped := pushHeld(t, target, "system/deckhouse/modules/console")

	push(t, target, "system/deckhouse:v1.76.6")
	pushInstaller(t, target, "v1.76.6", []string{current})
	pushModulePackage(t, target, "console", "v1.65.2", []string{running})
	pushModulePackage(t, target, "console", "v1.66.2", []string{pending})

	collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, &countingSweeper{})
	collector.Modules = []fill.ModuleRef{
		{Name: "console", Version: "v1.65.2"},
		{Name: "console", Version: "v1.66.2", Pending: true},
		// Not in the store: nothing to protect, nothing to stop for.
		{Name: "absent", Version: "v0.1.0", Pending: true},
	}
	_, err := collector.Run(context.Background())
	require.NoError(t, err)
	assert.True(t, holds(t, target, "system/deckhouse/modules/console", pending))
	assert.False(t, holds(t, target, "system/deckhouse/modules/console", dropped))

	// And one that is there and cannot be read.
	broken := pushHeld(t, target, "system/deckhouse/modules/console")
	pushUnreadablePackage(t, target, "broken", "v1.0.0")
	collector.Modules = append(collector.Modules, fill.ModuleRef{Name: "broken", Version: "v1.0.0", Pending: true})
	_, err = collector.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken")
	assert.True(t, holds(t, target, "system/deckhouse/modules/console", broken), "nothing is deleted")
}

// pushUnreadablePackage puts in a module package whose image set is not the map a package declares.
func pushUnreadablePackage(t *testing.T, target store, module, version string) {
	t.Helper()

	encoded, err := json.Marshal([]string{"not", "a", "map"})
	require.NoError(t, err)
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	require.NoError(t, writer.WriteHeader(&tar.Header{Name: "images_digests.json", Mode: 0o644, Size: int64(len(encoded))}))
	_, err = writer.Write(encoded)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	layer, err := tarball.LayerFromReader(bytes.NewReader(archive.Bytes()))
	require.NoError(t, err)
	image, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)
	tag, err := name.NewTag(target.Address+"/system/deckhouse/modules/"+module+":"+version, name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(tag, image))
}

// TestRunGivesBackWhatIsAheadWhenShortOfRoom: with ReleaseAhead — a store short of room, an upstream that
// holds it all again — a newer release and the images of a pending module update are collected like
// anything else the cluster has moved past. Without it they stay; see the tests above.
func TestRunGivesBackWhatIsAheadWhenShortOfRoom(t *testing.T) {
	target := startRegistry(t)

	current := pushHeld(t, target, "system/deckhouse")
	newer := pushHeld(t, target, "system/deckhouse")
	running := pushHeld(t, target, "system/deckhouse/modules/console")
	pending := pushHeld(t, target, "system/deckhouse/modules/console")

	push(t, target, "system/deckhouse:v1.76.6")
	pushInstaller(t, target, "v1.76.6", []string{current})
	push(t, target, "system/deckhouse:v1.77.0")
	pushInstaller(t, target, "v1.77.0", []string{newer})
	pushModulePackage(t, target, "console", "v1.65.2", []string{running})
	pushModulePackage(t, target, "console", "v1.66.2", []string{pending})

	collector := newCollector(t, target, Releases{Deployed: "v1.76.6", Pending: []string{"v1.77.0"}}, &countingSweeper{})
	collector.Modules = []fill.ModuleRef{
		{Name: "console", Version: "v1.65.2"},
		{Name: "console", Version: "v1.66.2", Pending: true},
	}
	collector.ReleaseAhead = true

	report, err := collector.Run(context.Background())
	require.NoError(t, err)

	assert.True(t, holds(t, target, "system/deckhouse", current), "the set the cluster runs stays")
	assert.True(t, holds(t, target, "system/deckhouse/modules/console", running))
	assert.False(t, holds(t, target, "system/deckhouse", newer), "the newer release is given back")
	assert.False(t, holds(t, target, "system/deckhouse/modules/console", pending), "and so is the module update")
	assert.Positive(t, report.Kept[ReasonCurrentRelease])
}

// TestRunReadsTheSetFromTheUpstreamWhenTheStoreCannot is the store that most needs collecting: it
// refuses writes, so it never took in the installer of the release the cluster moved to, so the set
// could not be read here, so nothing was judged and nothing was ever freed. The set is the fill's —
// read from the upstream the fill reads it from — and what it does not declare goes.
func TestRunReadsTheSetFromTheUpstreamWhenTheStoreCannot(t *testing.T) {
	target := startRegistry(t)
	upstream := startRegistry(t)

	current := pushHeld(t, target, "system/deckhouse")
	superseded := pushHeld(t, target, "system/deckhouse")

	// The release the cluster runs is on the upstream, installer and all; the store has neither.
	push(t, upstream, "system/deckhouse:pr880")
	pushInstaller(t, upstream, "pr880", []string{current})

	collector := newCollector(t, target, Releases{Deployed: "pr880"}, &countingSweeper{})
	collector.Upstream = &fill.Registry{Address: upstream.Address, Repository: "system/deckhouse", Insecure: true}

	report, err := collector.Run(context.Background())
	require.NoError(t, err)

	assert.True(t, holds(t, target, "system/deckhouse", current), "what the running release declares must survive")
	assert.False(t, holds(t, target, "system/deckhouse", superseded), "and what it does not declare is reclaimed")
	assert.Len(t, report.Deleted, 1)
}

// TestRunDeletesNothingWhenNeitherStoreNorUpstreamAnswers: the fallback widens where the set is read,
// not what is deleted without one.
func TestRunDeletesNothingWhenNeitherStoreNorUpstreamAnswers(t *testing.T) {
	target := startRegistry(t)
	upstream := startRegistry(t)

	held := pushHeld(t, target, "system/deckhouse")

	collector := newCollector(t, target, Releases{Deployed: "pr880"}, &countingSweeper{})
	collector.Upstream = &fill.Registry{Address: upstream.Address, Repository: "system/deckhouse", Insecure: true}

	report, err := collector.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot establish which manifests are needed")
	assert.Contains(t, err.Error(), "from the upstream")
	assert.Empty(t, report.Deleted)
	assert.True(t, holds(t, target, "system/deckhouse", held))
}

// TestACancellationStopsTheDeletions: a canceled run went on deleting every manifest it had judged,
// with the store's writes paused all the while, and the documentation promised it would stop.
func TestACancellationStopsTheDeletions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var deletes atomic.Int32
	real := registry.New(registry.Logger(log.New(io.Discard, "", 0)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		real.ServeHTTP(w, r)
		if r.Method == http.MethodDelete {
			// Canceled as the first deletion goes through.
			deletes.Add(1)
			cancel()
		}
	}))
	t.Cleanup(server.Close)
	parsed, err := url.Parse(server.URL)
	require.NoError(t, err)
	target := store{
		Registry: Registry{Address: parsed.Host, Insecure: true, Scope: "system/deckhouse"},
		dataDir:  t.TempDir(),
	}

	current := pushHeld(t, target, "system/deckhouse")
	for range 5 {
		pushHeld(t, target, "system/deckhouse")
	}
	push(t, target, "system/deckhouse:v1.76.6")
	pushInstaller(t, target, "v1.76.6", []string{current})

	sweeper := &countingSweeper{}
	collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, sweeper)
	report, err := collector.Run(ctx)

	require.ErrorIs(t, err, context.Canceled)
	assert.EqualValues(t, 1, deletes.Load(), "the one in flight reached the registry, and no other")
	assert.LessOrEqual(t, len(report.Deleted)+len(report.Failed), 1)
	assert.Zero(t, sweeper.runs)
}

// TestAModuleWhosePackageNeverArrivedStopsNothing: a module enabled just before the store ran out of
// room is declared, and its package never arrived. Nothing in the store is reached through its tag, so
// there is nothing of it to protect, and the store that only a collection can make room in is
// collected.
func TestAModuleWhosePackageNeverArrivedStopsNothing(t *testing.T) {
	target := startRegistry(t)

	current := pushHeld(t, target, "system/deckhouse")
	superseded := pushHeld(t, target, "system/deckhouse")
	push(t, target, "system/deckhouse:v1.76.6")
	pushInstaller(t, target, "v1.76.6", []string{current})

	collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, &countingSweeper{})
	collector.Modules = []fill.ModuleRef{{Name: "enabled-late", Version: "v0.3.0"}}

	report, err := collector.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, report.RemovedManifests)
	assert.True(t, holds(t, target, "system/deckhouse", current))
	assert.False(t, holds(t, target, "system/deckhouse", superseded))
}

// TestAModuleOfAMissingSourceStopsTheCollection: the previous round's high finding. A module of
// another ModuleSource is kept under module-sources/<source>/<name>; a module left without its source
// was looked for under modules/<name>, its package was not there, and its images went as protected by
// nothing — in air-gap, for good. Its release names a source, so the absent package is no package that
// never arrived, and nothing is deleted. Nor is anything when a package of the same name and version is
// on the platform's path, or when the module is only pending. The same module with its source found
// keeps its images.
func TestAModuleOfAMissingSourceStopsTheCollection(t *testing.T) {
	setUp := func(t *testing.T) (store, string, string) {
		target := startRegistry(t)

		current := pushHeld(t, target, "system/deckhouse")
		superseded := pushHeld(t, target, "system/deckhouse")
		push(t, target, "system/deckhouse:v1.76.6")
		pushInstaller(t, target, "v1.76.6", []string{current})

		image := pushHeld(t, target, "system/deckhouse/module-sources/partner/console")
		pushModulePackageAt(t, target, "system/deckhouse/module-sources/partner/console", "v1.2.0",
			[]string{image})
		return target, image, superseded
	}

	t.Run("unplaced", func(t *testing.T) {
		target, image, superseded := setUp(t)

		collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, &countingSweeper{})
		collector.Modules = []fill.ModuleRef{{Name: "console", Version: "v1.2.0", MissingSource: "partner"}}

		report, err := collector.Run(context.Background())
		require.Error(t, err)
		assert.Zero(t, report.RemovedManifests)
		assert.True(t, holds(t, target, "system/deckhouse/module-sources/partner/console", image))
		assert.True(t, holds(t, target, "system/deckhouse", superseded), "nothing is deleted at all")
	})

	t.Run("unplaced, with the platform's module of the same version", func(t *testing.T) {
		target, image, superseded := setUp(t)
		pushModulePackage(t, target, "console", "v1.2.0", nil)

		collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, &countingSweeper{})
		collector.Modules = []fill.ModuleRef{{Name: "console", Version: "v1.2.0", MissingSource: "partner"}}

		report, err := collector.Run(context.Background())
		require.Error(t, err)
		assert.Zero(t, report.RemovedManifests)
		assert.True(t, holds(t, target, "system/deckhouse/module-sources/partner/console", image))
		assert.True(t, holds(t, target, "system/deckhouse", superseded), "nothing is deleted at all")
	})

	t.Run("unplaced and pending", func(t *testing.T) {
		target, image, superseded := setUp(t)

		collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, &countingSweeper{})
		collector.Modules = []fill.ModuleRef{{
			Name: "console", Version: "v1.2.0", MissingSource: "partner", Pending: true,
		}}

		report, err := collector.Run(context.Background())
		require.Error(t, err)
		assert.Zero(t, report.RemovedManifests)
		assert.True(t, holds(t, target, "system/deckhouse/module-sources/partner/console", image))
		assert.True(t, holds(t, target, "system/deckhouse", superseded), "nothing is deleted at all")
	})

	t.Run("placed", func(t *testing.T) {
		target, image, superseded := setUp(t)

		collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, &countingSweeper{})
		collector.Modules = []fill.ModuleRef{{
			Name: "console", Version: "v1.2.0", Source: fill.ModuleSource{Name: "partner"},
		}}

		report, err := collector.Run(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, report.RemovedManifests)
		assert.True(t, holds(t, target, "system/deckhouse/module-sources/partner/console", image))
		assert.False(t, holds(t, target, "system/deckhouse", superseded))
	})
}

// TestTheChildrenOfAKeptIndexAreKept: a release declares the digest a pull asks for, and for an image
// built for more than one platform that is the index alone. Its platform manifests are revisions of
// their own in the store; deleted as declared by nobody, they left an index every pull failed on.
func TestTheChildrenOfAKeptIndexAreKept(t *testing.T) {
	target := startRegistry(t)

	index, err := random.Index(64, 1, 2)
	require.NoError(t, err)
	indexDigest, err := index.Digest()
	require.NoError(t, err)
	reference, err := name.NewDigest(
		fmt.Sprintf("%s/system/deckhouse@%s", target.Address, indexDigest), name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.WriteIndex(reference, index))

	// The store's files, as the fill leaves them: the index and each child a revision, and the
	// index's own content among the blobs.
	raw, err := index.RawManifest()
	require.NoError(t, err)
	require.Equal(t, indexDigest.String(), writeBlob(t, target.dataDir, raw))
	revisionLink(t, target.dataDir, "system/deckhouse", indexDigest.String())
	manifest, err := index.IndexManifest()
	require.NoError(t, err)
	require.Len(t, manifest.Manifests, 2)
	for _, child := range manifest.Manifests {
		revisionLink(t, target.dataDir, "system/deckhouse", child.Digest.String())
	}

	superseded := pushHeld(t, target, "system/deckhouse")
	push(t, target, "system/deckhouse:v1.76.6")
	pushInstaller(t, target, "v1.76.6", []string{indexDigest.String()})

	collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, &countingSweeper{})
	report, err := collector.Run(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, report.RemovedManifests, "only the image the release moved past")
	assert.False(t, holds(t, target, "system/deckhouse", superseded))
	assert.True(t, holds(t, target, "system/deckhouse", indexDigest.String()))
	for _, child := range manifest.Manifests {
		assert.True(t, holds(t, target, "system/deckhouse", child.Digest.String()),
			"a platform manifest of the kept index")
	}
}

// TestAPushThatStartsDuringTheCollectionIsNotSweptFromUnder: the manifest pass can take minutes, and a
// push that starts in them has its layers on disk and no manifest yet.
func TestAPushThatStartsDuringTheCollectionIsNotSweptFromUnder(t *testing.T) {
	target := startRegistry(t)

	current := pushHeld(t, target, "system/deckhouse")
	pushHeld(t, target, "system/deckhouse")
	push(t, target, "system/deckhouse:v1.76.6")
	pushInstaller(t, target, "v1.76.6", []string{current})

	sweeper := &countingSweeper{}
	collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, sweeper)
	collector.Pushing = func() bool { return true }

	report, err := collector.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "being pushed")
	assert.Equal(t, 1, report.RemovedManifests, "the manifests go; their blobs wait for the next run")
	assert.Zero(t, sweeper.runs)

	// With nothing deleted, there is nothing to report as failed: what lies unreferenced waits.
	collector.SweepAnyway = true
	report, err = collector.Run(context.Background())
	require.NoError(t, err)
	assert.Zero(t, report.RemovedManifests)
	assert.Zero(t, sweeper.runs)
}

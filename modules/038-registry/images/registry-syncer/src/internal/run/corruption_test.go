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

package run

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryimagev1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/registry.deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-syncer/internal/fill"
	"github.com/deckhouse/registry-syncer/internal/storetest"
)

// TestABrokenBlobOfTheSetIsShortAtOnce: a store that serves an image of the set broken does not hold the
// set, however clean a sample of it reads, and a leader hands the lease to a replica that holds it whole.
func TestABrokenBlobOfTheSetIsShortAtOnce(t *testing.T) {
	clean := fill.SampleResult{ReleaseHeld: true, Checked: 32}
	loop := samplingLoop(t, &sampler{results: []fill.SampleResult{clean}})
	image, blobs := storetest.Image(t, loop.DataDir, "system/deckhouse", "layer")
	_, pulledBlobs := storetest.Image(t, loop.DataDir, "system/deckhouse/pulled", "other")

	loop.declaredMu.Lock()
	loop.declared, loop.declaredFor = map[string]struct{}{image: {}}, "v1.76.6"
	loop.declaredMu.Unlock()

	bad := map[string]bool{}
	loop.Corrupted = func() map[string]bool { return bad }

	_, found := loop.ShortOfSet(context.Background(), "v1.76.6")
	assert.False(t, found, "nothing is broken")

	// A broken blob of an image nobody declares is no reason to hand anything over.
	bad[pulledBlobs[1]] = true
	loop.CorruptionChanged()
	_, found = loop.ShortOfSet(context.Background(), "v1.76.6")
	assert.False(t, found)
	assert.False(t, loop.RepairOwed(), "no image of the set is broken, so a fill can still finish the store")

	bad[blobs[1]] = true
	loop.CorruptionChanged()
	short, found := loop.ShortOfSet(context.Background(), "v1.76.6")
	assert.True(t, found)
	assert.Equal(t, Shortfall{Release: "v1.76.6", ReleaseHeld: true, Corrupted: 1}, short)
	// And only a Repair can finish it, so a collection does not wait for the fill — see gc.Plan.
	assert.True(t, loop.RepairOwed())

	// Read for another release, the set says nothing about this one.
	_, found = loop.ShortOfSet(context.Background(), "v1.77.0")
	assert.False(t, found)
}

// TestACorruptionChangeMakesTheNextIdlePassReadTheStore: the survey and the recheck are what take Full
// back, and what give it back after a repair.
func TestACorruptionChangeMakesTheNextIdlePassReadTheStore(t *testing.T) {
	loop := samplingLoop(t, &sampler{results: []fill.SampleResult{{ReleaseHeld: true}}})
	loop.lastStoreCheck.Store(time.Now().UnixNano())
	loop.sampleCleanAt = time.Now()

	loop.CorruptionChanged()

	assert.True(t, loop.surveyStale.Load())
	assert.Zero(t, loop.lastStoreCheck.Load())
	assert.True(t, loop.sampleCleanAt.IsZero())
}

// TestTheListingReadsTheSetTheLoopAlreadyRead: reading the set means reading the platform's image to its
// end, and the listing is rebuilt every few minutes — it must not make an idle replica do that.
func TestTheListingReadsTheSetTheLoopAlreadyRead(t *testing.T) {
	loop, _, _ := newLoop(t, true, nil, deployedRelease("v1.76.6"))
	// No local registry to read the set through: a fresh reading would fail.
	loop.LocalAddress = "127.0.0.1:1"

	digest := "sha256:aaaa111111111111111111111111111111111111111111111111111111111111"
	loop.declaredMu.Lock()
	loop.declared, loop.declaredFor = map[string]struct{}{digest: {}}, "v1.76.6"
	loop.declaredAt = time.Now().Add(-time.Hour)
	loop.declaredOwners = map[string][]registryimagev1alpha1.ImageOwner{digest: {{Kind: registryimagev1alpha1.OwnerPlatform}}}
	loop.declaredMu.Unlock()

	declared, owners, err := loop.DeclaredOwners(context.Background())
	assert.NoError(t, err, "an hour-old reading for the deployed release is used as it is")
	assert.Contains(t, declared, digest)
	assert.Len(t, owners[digest], 1)

	// Another release deployed: the old reading says nothing about it.
	loop.declaredMu.Lock()
	loop.declaredFor = "v1.75.0"
	loop.declaredMu.Unlock()
	_, _, err = loop.DeclaredOwners(context.Background())
	assert.Error(t, err)
}

// TestACollectionWaitsForAFillThatIsStillOwed: a store short of the set by a broken blob only is not
// finished by a fill, and a collection does not wait for one; a store short of other images as well is,
// and it does.
func TestACollectionWaitsForAFillThatIsStillOwed(t *testing.T) {
	loop := samplingLoop(t, &sampler{})
	broken, blobs := storetest.Image(t, loop.DataDir, "system/deckhouse", "layer")
	tag := filepath.Join(loop.DataDir, "docker", "registry", "v2", "repositories", "system", "deckhouse",
		"_manifests", "tags", "v1.76.6", "current", "link")
	require.NoError(t, os.MkdirAll(filepath.Dir(tag), 0o755))
	require.NoError(t, os.WriteFile(tag, []byte(broken), 0o644))

	scratch := storetest.New(t)
	notYet, _ := storetest.Image(t, scratch, "system/deckhouse", "not copied yet")

	loop.declaredMu.Lock()
	loop.declared, loop.declaredFor = map[string]struct{}{broken: {}, notYet: {}}, "v1.76.6"
	loop.declaredMu.Unlock()
	loop.Corrupted = func() map[string]bool { return map[string]bool{blobs[1]: true} }

	assert.True(t, loop.RepairOwed())
	assert.False(t, loop.OwesOnlyRepair(), "an image of the set is not there at all, so a fill is owed too")

	storetest.Image(t, loop.DataDir, "system/deckhouse", "not copied yet")
	assert.True(t, loop.OwesOnlyRepair(), "the broken blob is all the store lacks")
}

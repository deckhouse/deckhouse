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
	"context"
	"errors"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pushRelease puts a platform release into source: its image and the installer declaring one image.
func pushRelease(t *testing.T, source Registry, version string) {
	t.Helper()
	image := pushByDigest(t, source, "deckhouse/ee")
	pushImage(t, source, "deckhouse/ee:"+version)
	pushInstaller(t, source, "deckhouse/ee/install:"+version, map[string]any{
		imagesDigestsFile: map[string]map[string]string{"registry": {"registryController": image.String()}},
	})
}

func names(references []name.Reference) []string {
	var out []string
	for _, reference := range references {
		out = append(out, reference.String())
	}
	return out
}

// TestOnlyTheNextUpdateIsCopiedAhead is Manual mode with three minor releases pending. Updates are
// applied one at a time, so only the next is of use; copying all of them filled the store until the
// set itself was refused for the room.
func TestOnlyTheNextUpdateIsCopiedAhead(t *testing.T) {
	source := startRegistry(t)
	source.Repository = "deckhouse/ee"
	pushRelease(t, source, "v1.71.0")
	pushRelease(t, source, "v1.72.0")
	pushRelease(t, source, "v1.73.0")

	puller, err := remote.NewPuller(source.Options...)
	require.NoError(t, err)
	references, err := Release{Pending: []string{"v1.71.0", "v1.72.0", "v1.73.0"}}.Optional(t.Context(), source, puller)
	require.NoError(t, err)

	listed := names(references)
	assert.Contains(t, listed, source.Address+"/deckhouse/ee:v1.71.0")
	assert.NotContains(t, listed, source.Address+"/deckhouse/ee:v1.72.0")
	assert.NotContains(t, listed, source.Address+"/deckhouse/ee:v1.73.0")
}

// TestAModuleUpdateThatCannotBeReadLeavesTheRestAhead: one pending module whose image set cannot be
// read is named in the error, and the platform release beside it is still listed — it used to take
// the whole copy ahead with it.
func TestAModuleUpdateThatCannotBeReadLeavesTheRestAhead(t *testing.T) {
	source := startRegistry(t)
	source.Repository = "deckhouse/ee"
	pushRelease(t, source, "v1.71.0")
	pushInstaller(t, source, "deckhouse/ee/modules/broken:v1.0.0", map[string]any{
		"images_digests.json": []string{"not", "a", "map"},
	})

	puller, err := remote.NewPuller(source.Options...)
	require.NoError(t, err)
	references, err := Release{
		Pending: []string{"v1.71.0"},
		Modules: []ModuleRef{{Name: "broken", Version: "v1.0.0", Pending: true}},
	}.Optional(t.Context(), source, puller)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken", "the miss names the module")
	assert.Contains(t, names(references), source.Address+"/deckhouse/ee:v1.71.0", "and the release is still ahead")
}

// TestAnUpdateThatCannotBeReadIsNamed: a pending version whose set cannot be read is a miss, not a
// silence — a release that never arrives must say so somewhere.
func TestAnUpdateThatCannotBeReadIsNamed(t *testing.T) {
	source := startRegistry(t)
	source.Repository = "deckhouse/ee"

	puller, err := remote.NewPuller(source.Options...)
	require.NoError(t, err)
	references, err := Release{Pending: []string{"v1.99.0"}}.Optional(t.Context(), source, puller)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "v1.99.0")
	assert.Empty(t, references)
}

// partlyEnumerated lists one reference ahead of the set and says it could not list the rest.
type partlyEnumerated struct{ ahead name.Reference }

func (partlyEnumerated) Discover(context.Context, Registry, *remote.Puller) ([]name.Reference, error) {
	return nil, nil
}

func (d partlyEnumerated) Optional(context.Context, Registry, *remote.Puller) ([]name.Reference, error) {
	return []name.Reference{d.ahead}, errors.New("module broken: unreadable")
}

// TestTheCopyAheadCopiesWhatItCouldList: what could be enumerated is copied, and what could not is a
// miss — as is what the caller could not even set up a read of.
func TestTheCopyAheadCopiesWhatItCouldList(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)
	pushImage(t, source, "pending:v2")
	ahead, err := name.ParseReference(source.Address+"/pending:v2", name.Insecure)
	require.NoError(t, err)

	copier := &Copier{
		Source: source, Destination: destination,
		Discover:    partlyEnumerated{ahead: ahead},
		AheadUnread: []string{"vendor-module@v1.0.0: the source cannot be read"},
	}
	report := copier.CopyAhead(t.Context())

	assert.EqualValues(t, 1, report.AheadCopied, "the listed update is copied")
	require.Len(t, report.AheadMissed, 2)
	assert.Contains(t, report.AheadMissed[0], "vendor-module")
	assert.Contains(t, report.AheadMissed[1], "broken")

	tag, err := name.NewTag(destination.Address+"/pending:v2", name.Insecure)
	require.NoError(t, err)
	_, err = remote.Head(tag)
	assert.NoError(t, err)
}

// TestACopyAheadHoldsWhatItLeftWhole: the release is copied whole and held; the module update beside it
// that cannot be read is missed alone, and does not take the release with it.
func TestACopyAheadHoldsWhatItLeftWhole(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)
	source.Repository = "deckhouse/ee"
	pushRelease(t, source, "v1.71.0")
	pushInstaller(t, source, "deckhouse/ee/modules/broken:v1.0.0", map[string]any{
		"images_digests.json": []string{"not", "a", "map"},
	})

	copier := &Copier{
		Source: source, Destination: destination,
		Discover: Release{
			Pending: []string{"v1.71.0"},
			Modules: []ModuleRef{{Name: "broken", Version: "v1.0.0", Pending: true}},
		},
	}
	report := copier.CopyAhead(t.Context())

	assert.Equal(t, []string{PlatformPart("v1.71.0")}, report.AheadHeld)
	require.Len(t, report.AheadMissed, 1)
	assert.Contains(t, report.AheadMissed[0], "broken")
}

// TestACopyAheadReadsOnlyThePartsItIsFor: a part not wanted is not read at all — for the release that
// is its image, which a follower already holding it has no reason to read again.
func TestACopyAheadReadsOnlyThePartsItIsFor(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)
	source.Repository = "deckhouse/ee"
	pushRelease(t, source, "v1.71.0")

	recorder := &recordingTransport{}
	source.Options = []remote.Option{remote.WithTransport(recorder)}
	copier := &Copier{
		Source: source, Destination: destination,
		Discover:  Release{Pending: []string{"v1.71.0"}},
		AheadWant: func(string) bool { return false },
	}
	report := copier.CopyAhead(t.Context())

	assert.Empty(t, report.AheadHeld)
	assert.Zero(t, report.AheadCopied)
	assert.False(t, recorder.read("deckhouse/ee"), "the release was read although nobody wanted it")
}

// TestACopyAheadStopsWhenTheRoomRunsOut: asked as it goes, the room limit stops the copy, and what it
// stopped in is not held.
func TestACopyAheadStopsWhenTheRoomRunsOut(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)
	source.Repository = "deckhouse/ee"
	pushRelease(t, source, "v1.71.0")

	copier := &Copier{
		Source: source, Destination: destination,
		Discover:  Release{Pending: []string{"v1.71.0"}},
		AheadRoom: func() bool { return false },
	}
	report := copier.CopyAhead(t.Context())

	assert.Empty(t, report.AheadHeld)
	assert.Zero(t, report.AheadCopied)
	require.Len(t, report.AheadMissed, 1)
	assert.Contains(t, report.AheadMissed[0], "too little room")
}

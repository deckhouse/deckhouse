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

package integrity

import (
	"context"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-syncer/internal/fill"
	"github.com/deckhouse/registry-syncer/internal/storetest"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestTheLedgerTellsOnlyAboutChanges(t *testing.T) {
	ledger := &Ledger{}
	changes := 0
	ledger.OnChange(func() { changes++ })

	assert.Nil(t, ledger.Snapshot(), "an empty ledger is nil, so that asking costs nothing")
	assert.True(t, ledger.Add("sha256:b", "sha256:a"))
	assert.False(t, ledger.Add("sha256:a"), "a blob already recorded is no change")
	assert.Equal(t, []string{"sha256:a", "sha256:b"}, ledger.List())
	assert.True(t, ledger.Has("sha256:a"))

	assert.True(t, ledger.Retain(func(digest string) bool { return digest == "sha256:a" }))
	assert.False(t, ledger.Remove("sha256:b"))
	assert.True(t, ledger.Remove("sha256:a"))
	assert.Equal(t, 0, ledger.Len())
	assert.Equal(t, 3, changes)
}

func TestTheLedgerListsAtMostWhatTheStatusCarries(t *testing.T) {
	ledger := &Ledger{}
	for i := range registryv1alpha1.MaxCorruptedBlobs + 5 {
		ledger.Add(storetest.Digest([]byte{byte(i), byte(i >> 8)}))
	}
	assert.Len(t, ledger.List(), registryv1alpha1.MaxCorruptedBlobs)
	assert.Equal(t, registryv1alpha1.MaxCorruptedBlobs+5, ledger.Len())
}

func TestCheckComparesTheContentWithItsDigest(t *testing.T) {
	root := storetest.New(t)
	whole := storetest.Blob(t, root, []byte("whole"))
	broken := storetest.Blob(t, root, []byte("broken"))
	storetest.Corrupt(t, root, broken)

	outcome, read, err := Check(t.Context(), root, whole, Limiter(DefaultVerifyRate))
	require.NoError(t, err)
	assert.Equal(t, Intact, outcome)
	assert.EqualValues(t, len("whole"), read)

	outcome, _, err = Check(t.Context(), root, broken, nil)
	require.NoError(t, err)
	assert.Equal(t, Corrupted, outcome)

	outcome, _, err = Check(t.Context(), root, storetest.Digest([]byte("absent")), nil)
	require.NoError(t, err)
	assert.Equal(t, NotHeld, outcome)

	outcome, _, err = Check(t.Context(), root, "sha512:abcd", nil)
	require.NoError(t, err)
	assert.Equal(t, Unsupported, outcome, "only sha256 is checked; nothing else is in a store of the platform")
}

// The pace refuses a wait that would cross the deadline while the context is still alive: that is the
// window closing, not the disk failing, and the blob is not broken for it.
func TestAWindowClosingIsNotABrokenBlob(t *testing.T) {
	root := storetest.New(t)
	whole := storetest.Blob(t, root, make([]byte, 4*chunk))

	ctx, cancel := context.WithTimeout(t.Context(), time.Hour)
	defer cancel()
	slow := rate.NewLimiter(rate.Limit(1), chunk) // a chunk a second would take days
	outcome, _, err := Check(ctx, root, whole, slow)
	require.Error(t, err)
	assert.NoError(t, ctx.Err(), "the context has not ended")
	assert.Equal(t, Intact, outcome, "not read to its end is not broken")
}

func TestThePaceReadsTheStoreOnceACycleWithinItsBounds(t *testing.T) {
	const gib = int64(1) << 30
	assert.EqualValues(t, 15*gib/3600, Pace(15*gib, time.Hour, DefaultScrubMinRate, DefaultScrubMaxRate),
		"15 GB in an hour is about 4 MiB/s")
	assert.EqualValues(t, DefaultScrubMinRate, Pace(10<<20, time.Hour, DefaultScrubMinRate, DefaultScrubMaxRate))
	assert.EqualValues(t, DefaultScrubMaxRate, Pace(500*gib, time.Hour, DefaultScrubMinRate, DefaultScrubMaxRate),
		"a store too large for the cycle is read in longer, not faster")
}

func TestTheScrubRecordsWhatIsBrokenAndForgetsWhatIsGone(t *testing.T) {
	root := storetest.New(t)
	whole := storetest.Blob(t, root, []byte("whole"))
	broken := storetest.Blob(t, root, []byte("broken"))
	storetest.Corrupt(t, root, broken)

	ledger := &Ledger{}
	// Recorded earlier and since removed by a collection; and recorded earlier and whole now.
	gone := storetest.Digest([]byte("gone"))
	ledger.Add(gone, whole)

	var finished int
	scrub := &Scrub{
		Log: quiet(), Root: root, Ledger: ledger, Cycle: time.Hour,
		Sleep:    func(ctx context.Context, _ time.Duration) bool { return ctx.Err() == nil },
		Finished: func(context.Context, time.Time, int, int64) { finished++ },
	}
	require.True(t, scrub.pass(t.Context(), time.Hour))

	assert.Equal(t, []string{broken}, ledger.List())
	assert.Equal(t, 1, finished)
}

func TestTheScrubDoesNotRecordABlobRepairedUnderIt(t *testing.T) {
	root := storetest.New(t)
	content := []byte("repaired meanwhile")
	digest := storetest.Blob(t, root, content)
	storetest.Corrupt(t, root, digest)

	ledger := &Ledger{}
	scrub := &Scrub{
		Log: quiet(), Root: root, Ledger: ledger,
		// The second reading comes after the pause, by which time a repair has put a whole copy back.
		Sleep: func(context.Context, time.Duration) bool {
			require.NoError(t, os.WriteFile(storetest.BlobPath(root, digest), content, 0o644))
			return true
		},
	}
	require.True(t, scrub.pass(t.Context(), time.Hour))
	assert.Empty(t, ledger.List())
}

func TestAVerifyOfTheWholeStoreAndOfNamedImages(t *testing.T) {
	root := storetest.New(t)
	image, blobs := storetest.Image(t, root, "system/deckhouse/one", "layer-a", "layer-b")
	other, _ := storetest.Image(t, root, "system/deckhouse/two", "layer-c")
	storetest.Corrupt(t, root, blobs[2])

	ledger := &Ledger{}
	verifier := &Verifier{Log: quiet(), Root: root, Ledger: ledger}

	result, err := verifier.Verify(t.Context(), nil)
	require.NoError(t, err)
	assert.EqualValues(t, 7, result.CheckedBlobs, "two manifests, two configs and three layers")
	assert.Equal(t, []string{blobs[2]}, result.CorruptedBlobs)
	assert.EqualValues(t, 1, result.CorruptedCount)
	assert.Equal(t, []string{blobs[2]}, ledger.List())

	absent := storetest.Digest([]byte("nothing"))
	result, err = verifier.Verify(t.Context(), []string{other, absent})
	require.NoError(t, err)
	assert.EqualValues(t, 3, result.CheckedBlobs, "the manifest, its config and its layer")
	assert.Empty(t, result.CorruptedBlobs)
	assert.Equal(t, []string{absent}, result.MissingDigests)
	assert.Equal(t, []string{blobs[2]}, ledger.List(), "what was not read stays recorded")

	result, err = verifier.Verify(t.Context(), []string{image})
	require.NoError(t, err)
	assert.Equal(t, []string{blobs[2]}, result.CorruptedBlobs)
}

func TestResolveFindsImagesBlobsAndWhatIsNotThere(t *testing.T) {
	root := storetest.New(t)
	childA, blobsA := storetest.Image(t, root, "system/deckhouse/multi", "amd64")
	childB, blobsB := storetest.Image(t, root, "system/deckhouse/multi", "arm64")
	index := storetest.Index(t, root, "system/deckhouse/multi", childA, childB)

	targets, err := Resolve(root, []string{index, blobsB[1], storetest.Digest([]byte("nothing"))})
	require.NoError(t, err)
	require.Len(t, targets, 3)

	assert.Equal(t, "system/deckhouse/multi", targets[0].Repository)
	assert.ElementsMatch(t, append(append([]string{index, childA, childB}, blobsA...), blobsB...), targets[0].Blobs,
		"an index is everything its images consist of")
	assert.True(t, targets[0].Manifests[childA])

	assert.Equal(t, []string{blobsB[1]}, targets[1].Blobs)
	assert.Equal(t, "system/deckhouse/multi", targets[1].Repository, "a bare blob is asked for where an image names it")

	assert.Empty(t, targets[2].Blobs)
}

// source is a registry holding a whole copy of an image, and the same image laid out in a store.
func source(t *testing.T) (fill.Registry, string, string, []string) {
	t.Helper()

	server := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
	t.Cleanup(server.Close)
	parsed, err := url.Parse(server.URL)
	require.NoError(t, err)
	upstream := fill.Registry{Address: parsed.Host, Insecure: true}

	const repository = "system/deckhouse/app"
	image, err := random.Image(512, 2)
	require.NoError(t, err)
	reference, err := name.NewTag(upstream.Address+"/"+repository+":v1", name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(reference, image))

	// The same image in the store, as a fill leaves it.
	root := storetest.New(t)
	writeImage(t, root, repository, image)
	digest, err := image.Digest()
	require.NoError(t, err)
	blobs, _, err := Closure(root, digest.String())
	require.NoError(t, err)
	return upstream, root, digest.String(), blobs
}

func writeImage(t *testing.T, root, repository string, image v1.Image) {
	t.Helper()
	manifest, err := image.RawManifest()
	require.NoError(t, err)
	storetest.Revision(t, root, repository, storetest.Blob(t, root, manifest))
	config, err := image.RawConfigFile()
	require.NoError(t, err)
	storetest.Layer(t, root, repository, storetest.Blob(t, root, config))
	layers, err := image.Layers()
	require.NoError(t, err)
	for _, layer := range layers {
		content, err := layer.Compressed()
		require.NoError(t, err)
		raw, err := io.ReadAll(content)
		require.NoError(t, err)
		storetest.Layer(t, root, repository, storetest.Blob(t, root, raw))
	}
}

func TestARepairReplacesBrokenAndMissingBlobsWithWholeCopies(t *testing.T) {
	upstream, root, image, blobs := source(t)
	var layers []string
	for _, blob := range blobs {
		if blob != image {
			layers = append(layers, blob)
		}
	}
	require.GreaterOrEqual(t, len(layers), 3, "a config and two layers")
	storetest.Corrupt(t, root, layers[0])
	storetest.Remove(t, root, layers[1])

	ledger := &Ledger{}
	ledger.Add(layers[0])
	repairer := &Repairer{
		Log: quiet(), Root: root, Ledger: ledger,
		Sources: func(context.Context) ([]Source, error) {
			return []Source{{Name: "replica master-1", Registry: upstream}}, nil
		},
	}

	result, err := repairer.Repair(t.Context(), []string{image, storetest.Digest([]byte("nothing"))})
	require.NoError(t, err)
	require.Len(t, result.Digests, 2)
	assert.Equal(t, registryv1alpha1.DigestRepaired, result.Digests[0].Outcome, result.Digests[0].Message)
	assert.EqualValues(t, 2, result.Digests[0].RepairedBlobs)
	assert.Contains(t, result.Digests[0].Message, "replica master-1")
	assert.Equal(t, registryv1alpha1.DigestNotHeld, result.Digests[1].Outcome)
	assert.Empty(t, ledger.List(), "a repaired blob is not broken any more")

	for _, blob := range blobs {
		outcome, _, err := Check(t.Context(), root, blob, nil)
		require.NoError(t, err)
		assert.Equal(t, Intact, outcome, blob)
	}

	// And again: nothing to do.
	result, err = repairer.Repair(t.Context(), []string{image})
	require.NoError(t, err)
	assert.Equal(t, registryv1alpha1.DigestIntact, result.Digests[0].Outcome)
}

func TestARepairWithoutAWholeCopyLeavesTheBlobAsItWasAndSaysSo(t *testing.T) {
	_, root, image, blobs := source(t)
	broken := blobs[len(blobs)-1]
	if broken == image {
		broken = blobs[0]
	}
	storetest.Corrupt(t, root, broken)
	before, err := os.ReadFile(storetest.BlobPath(root, broken))
	require.NoError(t, err)

	ledger := &Ledger{}
	repairer := &Repairer{
		Log: quiet(), Root: root, Ledger: ledger,
		Sources: func(context.Context) ([]Source, error) { return nil, nil },
	}
	result, err := repairer.Repair(t.Context(), []string{image})
	require.NoError(t, err)
	assert.Equal(t, registryv1alpha1.DigestFailed, result.Digests[0].Outcome)
	assert.Contains(t, result.Digests[0].Message, "no other replica and no upstream")
	assert.Equal(t, []string{broken}, ledger.List(), "still broken, and recorded so")

	after, err := os.ReadFile(storetest.BlobPath(root, broken))
	require.NoError(t, err)
	assert.Equal(t, before, after, "nothing is removed when nothing can replace it")
}

func TestARepairGoesOnToTheNextSourceWhenOneHasNoCopy(t *testing.T) {
	upstream, root, image, blobs := source(t)
	layer := blobs[0]
	if layer == image {
		layer = blobs[1]
	}
	storetest.Corrupt(t, root, layer)

	// The first source holds nothing; the second the whole image.
	emptyPeer := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
	t.Cleanup(emptyPeer.Close)
	parsed, err := url.Parse(emptyPeer.URL)
	require.NoError(t, err)

	repairer := &Repairer{
		Log: quiet(), Root: root, Ledger: &Ledger{},
		Sources: func(context.Context) ([]Source, error) {
			return []Source{
				{Name: "replica master-0", Registry: fill.Registry{Address: parsed.Host, Insecure: true}},
				{Name: "replica master-2", Registry: upstream},
			}, nil
		},
	}
	result, err := repairer.Repair(t.Context(), []string{image})
	require.NoError(t, err)
	assert.Equal(t, registryv1alpha1.DigestRepaired, result.Digests[0].Outcome, result.Digests[0].Message)
	assert.Contains(t, result.Digests[0].Message, "replica master-2")
}

// A source that needs the broken copy moved aside, and then has no whole one: the broken copy goes back,
// so that the blob stays recorded, alerted on and asked for again rather than gone.
func TestAQuarantinedCopyComesBackWhenNoWholeOneCame(t *testing.T) {
	_, root, image, blobs := source(t)
	broken := blobs[0]
	if broken == image {
		broken = blobs[1]
	}
	storetest.Corrupt(t, root, broken)
	before, err := os.ReadFile(storetest.BlobPath(root, broken))
	require.NoError(t, err)

	emptyPeer := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
	t.Cleanup(emptyPeer.Close)
	parsed, err := url.Parse(emptyPeer.URL)
	require.NoError(t, err)

	ledger := &Ledger{}
	repairer := &Repairer{
		Log: quiet(), Root: root, Ledger: ledger,
		Sources: func(context.Context) ([]Source, error) {
			return []Source{{Name: "the upstream", Quarantine: true,
				Registry: fill.Registry{Address: parsed.Host, Insecure: true}}}, nil
		},
	}
	result, err := repairer.Repair(t.Context(), []string{image})
	require.NoError(t, err)
	assert.Equal(t, registryv1alpha1.DigestFailed, result.Digests[0].Outcome)
	assert.Equal(t, []string{broken}, ledger.List())

	after, err := os.ReadFile(storetest.BlobPath(root, broken))
	require.NoError(t, err, "the broken copy is back where it was")
	assert.Equal(t, before, after)
}

// A manifest the disk cannot read is a blob like any other: checked, found broken, repaired. Failing the
// closure left it unrepairable and the image's every Verify failing.
func TestAnUnreadableManifestIsABlobToCheck(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root reads a file whatever its mode")
	}
	root := storetest.New(t)
	image, _ := storetest.Image(t, root, "system/deckhouse", "layer")
	path := storetest.BlobPath(root, image)
	require.NoError(t, os.Chmod(path, 0))
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	blobs, manifests, err := Closure(root, image)
	require.NoError(t, err)
	assert.Contains(t, blobs, image)
	assert.True(t, manifests[image], "checked as the blob it is, by Check, rather than failing the image")
}

// A restart between moving a broken copy aside and replacing it left the store without the blob and
// the repair directory with files no one removed. The copy goes back unless a whole one took its place,
// and a download that never finished goes away.
func TestARestartPutsBackWhatAnUnfinishedRepairKeptAside(t *testing.T) {
	root := storetest.New(t)
	image, _ := storetest.Image(t, root, "system/deckhouse", "layer")
	blobs, _, err := Closure(root, image)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(blobs), 2)
	gone, replaced := blobs[0], blobs[1]

	repairer := &Repairer{Log: quiet(), Root: root, Ledger: &Ledger{}}
	_, _, err = repairer.quarantine(gone)
	require.NoError(t, err)
	_, _, err = repairer.quarantine(replaced)
	require.NoError(t, err)
	whole := []byte("a whole copy that came meanwhile")
	require.NoError(t, os.WriteFile(storetest.BlobPath(root, replaced), whole, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repairDir(root), "sha256-abc-123"), []byte("half"), 0o644))
	_, err = os.Stat(storetest.BlobPath(root, gone))
	require.ErrorIs(t, err, fs.ErrNotExist)

	restored, err := Recover(root)
	require.NoError(t, err)
	assert.Equal(t, 1, restored)

	_, err = os.Stat(storetest.BlobPath(root, gone))
	require.NoError(t, err, "the broken copy is back")
	content, err := os.ReadFile(storetest.BlobPath(root, replaced))
	require.NoError(t, err)
	assert.Equal(t, whole, content, "a whole copy is not replaced")
	left, err := os.ReadDir(repairDir(root))
	require.NoError(t, err)
	assert.Empty(t, left)
}

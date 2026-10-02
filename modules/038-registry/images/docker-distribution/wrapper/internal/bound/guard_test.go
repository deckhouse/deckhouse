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

package bound

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	blobPath   = "/docker/registry/v2/blobs/sha256/ab/abcdef/data"
	uploadPath = "/docker/registry/v2/repositories/system/deckhouse/_uploads/0f3a/data"
	linkPath   = "/docker/registry/v2/repositories/system/deckhouse/_layers/sha256/abcdef/link"
)

// disk stands in for a filesystem of the given capacity whose only content is the directory tree
// under root, so that what the guard writes through a real driver is what it later measures.
type disk struct {
	root     string
	capacity int64
	calls    int
}

func (d *disk) statfs(string) (int64, int64, error) {
	d.calls++
	var used int64
	err := filepath.Walk(d.root, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			used += info.Size()
		}
		return nil
	})
	return d.capacity - used, d.capacity, err
}

// fixed is a filesystem whose free space never changes.
func fixed(free int64) func(string) (int64, int64, error) {
	return func(string) (int64, int64, error) { return free, free, nil }
}

func refusal(t *testing.T, err error) *ErrStoreFull {
	t.Helper()
	var full *ErrStoreFull
	require.True(t, errors.As(err, &full), "expected a refusal, got %v", err)
	return full
}

func TestTheBudgetCountsBlobsAndUploadsButNotLinks(t *testing.T) {
	guard := New(t.TempDir(), 1000, 0)
	guard.statfs = fixed(1 << 40)

	require.NoError(t, guard.take(blobPath, 600, true))
	require.NoError(t, guard.take(uploadPath, 400, true))

	full := refusal(t, guard.take(blobPath, 1, true))
	assert.Equal(t, BudgetExhausted, full.Reason)
	assert.Equal(t, int64(1000), full.Used)
	assert.Equal(t, int64(1000), full.Budget)
	assert.Contains(t, full.Error(), "storage.size")

	// A link or a tag is not what the budget is about, and a push that could not write the link to
	// a blob it already stored would leave the store worse off than refusing the blob.
	assert.NoError(t, guard.take(linkPath, 600, true))
}

func TestAZeroBudgetLeavesOnlyTheReserve(t *testing.T) {
	guard := New(t.TempDir(), 0, 100)
	guard.statfs = fixed(1 << 50)

	for range 3 {
		require.NoError(t, guard.take(blobPath, 1<<48, true))
	}

	guard.statfs = fixed(150)
	full := refusal(t, guard.take(blobPath, 60, true))
	assert.Equal(t, ReserveExhausted, full.Reason)
}

func TestTheReserveAppliesToEveryWrite(t *testing.T) {
	guard := New(t.TempDir(), 0, 10<<20)
	guard.statfs = fixed(11 << 20)

	for _, path := range []string{blobPath, uploadPath, linkPath} {
		full := refusal(t, guard.take(path, 2<<20, true))
		assert.Equal(t, ReserveExhausted, full.Reason, path)
		assert.Equal(t, int64(11<<20), full.Free, path)
		assert.Contains(t, full.Error(), "dedicated disk", path)
	}
}

// TestAStreamStopsAtTheReserveWithoutMeasuringEveryChunk is what makes measuring only every few
// megabytes safe: between two measurements the guard subtracts what it admitted, so the stream
// stops at the reserve on the estimate, and the disk is measured a handful of times, not per chunk.
func TestAStreamStopsAtTheReserveWithoutMeasuringEveryChunk(t *testing.T) {
	root := t.TempDir()
	measured := &disk{root: root, capacity: 100 << 20}
	guard := New(root, 0, 40<<20)
	guard.statfs = measured.statfs

	file, err := os.Create(filepath.Join(root, "data"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })

	require.NoError(t, guard.take(uploadPath, 0, true))

	chunk := make([]byte, 1<<20)
	var refused error
	for range 100 {
		if refused = guard.take(uploadPath, int64(len(chunk)), false); refused != nil {
			break
		}
		_, err := file.Write(chunk)
		require.NoError(t, err)
	}

	assert.Equal(t, ReserveExhausted, refusal(t, refused).Reason)
	free, _, err := measured.statfs(root)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, free, int64(40<<20), "the stream went past the reserve")
	assert.Less(t, free, int64(41<<20), "the stream stopped short of the reserve")
	assert.LessOrEqual(t, measured.calls, 5)
}

func TestTheFilesystemIsMeasuredEveryFewMegabytesWithinAStream(t *testing.T) {
	guard := New(t.TempDir(), 0, 0)
	counting := &disk{root: t.TempDir(), capacity: 1 << 40}
	guard.statfs = counting.statfs

	require.NoError(t, guard.take(uploadPath, 0, true))
	for range 100 {
		require.NoError(t, guard.take(uploadPath, 1<<20, false))
	}

	assert.Equal(t, 1+100*(1<<20)/statfsEvery, counting.calls)
}

func TestTheFreeSpaceIsMeasuredAgainBeforeRefusing(t *testing.T) {
	guard := New(t.TempDir(), 0, 10<<20)

	free := int64(11 << 20)
	guard.statfs = func(string) (int64, int64, error) { return free, 1 << 40, nil }
	require.NoError(t, guard.take(uploadPath, 0, true))

	// Something else on the node freed space since the last measurement.
	free = 1 << 30
	assert.NoError(t, guard.take(uploadPath, 2<<20, false))
}

// TestANewFileIsChargedInBlocks: the store writes many small files, each taking whole blocks, and
// counting their length would let them spend the reserve unnoticed.
func TestANewFileIsChargedInBlocks(t *testing.T) {
	guard := New(t.TempDir(), 0, 1<<20)
	guard.statfs = fixed(1<<20 + 2*blockSize - 1)

	// 71 bytes fit by length and do not by blocks: one for the data, one for the file itself.
	full := refusal(t, guard.take(linkPath, 71, true))
	assert.Equal(t, ReserveExhausted, full.Reason)

	guard.statfs = fixed(1<<20 + 2*blockSize)
	assert.NoError(t, guard.take(linkPath, 71, true))
}

func TestAGuardThatCannotSeeTheDiskRefuses(t *testing.T) {
	guard := New(t.TempDir(), 0, 0)
	guard.statfs = func(string) (int64, int64, error) { return 0, 0, errors.New("no such device") }

	err := guard.take(linkPath, 1, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no such device")
}

func TestAMeasurementReplacesTheCountWithWhatIsOnDisk(t *testing.T) {
	root := t.TempDir()
	guard := New(root, 1000, 0)
	guard.statfs = fixed(1 << 40)

	write := func(path string, size int) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(path))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, make([]byte, size), 0o644))
	}

	write(blobPath, 300)
	write(uploadPath, 200)
	write(linkPath, 5000)
	require.NoError(t, guard.Measure())
	assert.Equal(t, int64(500), guard.base+guard.delta, "links are not counted")

	// Admitted and then abandoned — an upload the client never finished. Counted until the next
	// walk, which finds nothing and lets the budget go.
	require.NoError(t, guard.take(uploadPath, 400, true))
	assert.Equal(t, int64(900), guard.base+guard.delta)

	require.NoError(t, os.RemoveAll(filepath.Join(root, "docker", "registry", "v2", "repositories")))
	require.NoError(t, guard.Measure())
	assert.Equal(t, int64(300), guard.base+guard.delta)
}

func TestAMeasurementOfAnEmptyStoreIsZero(t *testing.T) {
	guard := New(t.TempDir(), 1000, 0)
	require.NoError(t, guard.Measure())
	assert.Zero(t, guard.base+guard.delta)
}

func TestSizesReadTheWayAnOperatorReadsThem(t *testing.T) {
	assert.Equal(t, "512 B", human(512))
	assert.Equal(t, "1.5 KiB", human(1536))
	assert.Equal(t, "100.0 GiB", human(100<<30))
}

// TestAPushIsRefusedUpFrontWithinAMeasurementOfALimit: the store stops a few blocks short of a
// limit, so "reached" would never be true up front, and every push would fail one block in instead.
func TestAPushIsRefusedUpFrontWithinAMeasurementOfALimit(t *testing.T) {
	guard := New(t.TempDir(), 100<<20, 10<<20)
	guard.statfs = fixed(1 << 40)
	require.NoError(t, guard.take(blobPath, 80<<20, true))

	full := guard.Refusal()
	require.NotNil(t, full, "20 MiB left of the budget")
	assert.Equal(t, BudgetExhausted, full.Reason)

	guard = New(t.TempDir(), 0, 10<<20)
	guard.statfs = fixed(20 << 20)
	full = guard.Refusal()
	require.NotNil(t, full, "10 MiB left above the reserve")
	assert.Equal(t, ReserveExhausted, full.Reason)

	// Named after the nearer limit.
	guard = New(t.TempDir(), 1<<30, 10<<20)
	guard.statfs = fixed(20 << 20)
	assert.Equal(t, ReserveExhausted, guard.Refusal().Reason)

	guard = New(t.TempDir(), 0, 10<<20)
	guard.statfs = fixed(100 << 20)
	assert.Nil(t, guard.Refusal())
}

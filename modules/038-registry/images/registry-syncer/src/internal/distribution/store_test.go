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

package distribution

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// TestTheReserveStaysAboveTheKubeletsThreshold pins the formula to the figures it was agreed on.
func TestTheReserveStaysAboveTheKubeletsThreshold(t *testing.T) {
	cases := []struct {
		capacity, reserve int64
	}{
		// Both parts at 10 %: the store stops at 80 % of a small master's disk.
		{50 * gib, 10 * gib},
		{200 * gib, 40 * gib},
		// The margin reaches its 20G ceiling first...
		{300 * gib, 30*gib + 20*gib},
		// ...and the kubelet's part its 40G one.
		{400 * gib, 60 * gib},
		{1024 * gib, 60 * gib},
	}
	for _, c := range cases {
		assert.Equal(t, c.reserve, Reserve(c.capacity), "capacity %d GiB", c.capacity/gib)

		// The kubelet's soft threshold is 10 %, capped at 40G decimal.
		kubelet := min(c.capacity/10, 40_000_000_000)
		assert.Greater(t, Reserve(c.capacity), kubelet, "capacity %d GiB", c.capacity/gib)
	}
}

func TestBudgetIsStorageSizeInBytes(t *testing.T) {
	budget, err := Budget("100Gi")
	require.NoError(t, err)
	assert.Equal(t, 100*gib, budget)

	budget, err = Budget("")
	require.NoError(t, err)
	assert.Zero(t, budget, "no storage.size: no budget")

	_, err = Budget("lots")
	assert.Error(t, err)
	_, err = Budget("0")
	assert.Error(t, err)
}

func TestTheWrapperCarriesTheBudgetAndTheReserve(t *testing.T) {
	spec := &registryv1alpha1.RegistryStorageSpec{Store: registryv1alpha1.StorageStore{Size: "100Gi"}}
	rendered, err := RenderWrapper(spec, testOptions())
	require.NoError(t, err)

	var parsed struct {
		Store struct {
			Budget  int64 `json:"budget"`
			Reserve int64 `json:"reserve"`
		} `json:"store"`
	}
	require.NoError(t, yaml.Unmarshal(rendered, &parsed))
	assert.Equal(t, 100*gib, parsed.Store.Budget)
	assert.Equal(t, 10*gib, parsed.Store.Reserve, "the test filesystem is 50 GiB")
}

// TestTheWrapperIsNeverRenderedWithoutAReserve: the registry refuses such a file, and it would be
// right to — so the refusal happens here, where the reason is known.
func TestTheWrapperIsNeverRenderedWithoutAReserve(t *testing.T) {
	options := testOptions()
	options.StoreCapacity = 0
	_, err := RenderWrapper(&registryv1alpha1.RegistryStorageSpec{}, options)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no reserve")
}

func TestCapacityMeasuresARealFilesystem(t *testing.T) {
	capacity, err := Capacity(t.TempDir())
	require.NoError(t, err)
	assert.Positive(t, capacity)

	_, err = Capacity("/does/not/exist")
	assert.Error(t, err)
}

func TestMeasureStoreCountsWhatTheRegistryCounts(t *testing.T) {
	dir := t.TempDir()
	put := func(path string, size int) {
		t.Helper()
		full := filepath.Join(dir, filepath.FromSlash(path))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, make([]byte, size), 0o644))
	}
	put("docker/registry/v2/blobs/sha256/ab/abcdef/data", 20<<20)
	put("docker/registry/v2/repositories/system/deckhouse/_uploads/0f3a/data", 5<<20)
	put("docker/registry/v2/repositories/system/deckhouse/_layers/sha256/abcdef/link", 30<<20)

	spec := &registryv1alpha1.RegistryStorageSpec{Store: registryv1alpha1.StorageStore{Size: "1Gi"}}
	usage, err := MeasureStore(dir, spec, 0)
	require.NoError(t, err)

	assert.Equal(t, int64(32<<20), usage.UsedBytes, "25 MiB of blobs and uploads, rounded up; links not counted")
	assert.Equal(t, int64(1<<30), usage.BudgetBytes)
	assert.Positive(t, usage.FilesystemCapacityBytes)
	assert.Equal(t, Reserve(usage.FilesystemCapacityBytes), usage.ReserveBytes)
	assert.Zero(t, usage.FilesystemFreeBytes%usageQuantum)
}

func TestMeasureStoreSaysWhenTheRegistryRefuses(t *testing.T) {
	dir := t.TempDir()
	blob := filepath.Join(dir, "docker", "registry", "v2", "blobs", "sha256", "ab", "abcdef", "data")
	require.NoError(t, os.MkdirAll(filepath.Dir(blob), 0o755))
	require.NoError(t, os.WriteFile(blob, make([]byte, 90<<20), 0o644))

	// A 50 GiB disk with plenty above its 10 GiB reserve: the machine's own disk would make the
	// outcome depend on whatever else is writing to it while the tests run.
	measureFilesystem = func(string) (int64, int64, error) { return 30 * gib, 50 * gib, nil }
	t.Cleanup(func() { measureFilesystem = statfs })

	within := &registryv1alpha1.RegistryStorageSpec{Store: registryv1alpha1.StorageStore{Size: "100Mi"}}
	usage, err := MeasureStore(dir, within, 50*gib)
	require.NoError(t, err)
	assert.False(t, usage.Writable, "10 MiB left of the budget: the registry refuses pushes up front")
	assert.Equal(t, registryv1alpha1.StoreBudgetExhausted, usage.Reason)

	roomy := &registryv1alpha1.RegistryStorageSpec{Store: registryv1alpha1.StorageStore{Size: "1Gi"}}
	usage, err = MeasureStore(dir, roomy, 50*gib)
	require.NoError(t, err)
	assert.True(t, usage.Writable)

	// The same disk with 20 MiB above its reserve.
	measureFilesystem = func(string) (int64, int64, error) { return 10*gib + 20<<20, 50 * gib, nil }
	usage, err = MeasureStore(dir, roomy, 50*gib)
	require.NoError(t, err)
	assert.False(t, usage.Writable)
	assert.Equal(t, registryv1alpha1.StoreReserveExhausted, usage.Reason)

	measureFilesystem = func(string) (int64, int64, error) { return 20 * gib, 50 * gib, nil }
	usage, err = MeasureStore(dir, roomy, 50*gib)
	require.NoError(t, err)
	assert.True(t, usage.Writable)
	assert.Empty(t, usage.Reason)
}

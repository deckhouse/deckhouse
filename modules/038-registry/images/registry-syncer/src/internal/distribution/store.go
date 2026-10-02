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
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"syscall"

	"k8s.io/apimachinery/pkg/api/resource"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

const gib = int64(1) << 30

// Reserve is how much of the filesystem under the store must stay free, for a filesystem of the
// given size.
//
// Two parts. The first is the kubelet's soft eviction threshold for its node filesystem, as the
// module's kubelet step configures it — 10 %, at most 40G — because the point of the reserve is
// that the store stops before the kubelet starts evicting, and on a typical master the store shares
// that filesystem. The second is room for everyone else who writes there: containerd unpacking an
// image, logs, etcd on a static master. The store cannot see or stop those writes, so the margin is
// sized for them — 10 % again, at most 20G.
//
// GiB rather than the kubelet's decimal G for both ceilings: the reserve must never come out below
// the kubelet's threshold, and the binary unit only errs above it.
//
// A function of the size alone, never of what is free: the result is rendered into the registry's
// configuration, and a figure that changed with every write would restart the registry on every
// pass of the loop.
func Reserve(capacity int64) int64 {
	return min(capacity/10, 40*gib) + min(capacity/10, 20*gib)
}

// Budget is `storage.size` in bytes: how much the store may hold. Zero when none is set.
func Budget(size string) (int64, error) {
	if size == "" {
		return 0, nil
	}
	quantity, err := resource.ParseQuantity(size)
	if err != nil {
		return 0, fmt.Errorf("storage size %q: %w", size, err)
	}
	if quantity.Sign() <= 0 {
		return 0, fmt.Errorf("storage size %q is not positive", size)
	}
	return quantity.Value(), nil
}

// Capacity is the size of the filesystem holding path.
func Capacity(path string) (int64, error) {
	_, capacity, err := statfs(path)
	return capacity, err
}

// refuseWithin is how close to a limit the registry stops accepting new writes: it refuses a push up
// front once less than this is left, rather than let it fail a block in. The registry's own figure —
// its guard's measurement interval — repeated here so that the status says what the registry does.
const refuseWithin = 32 << 20

// usageQuantum is what the reported sizes are rounded to, so that a filesystem shared with the rest
// of the node, whose free space moves by kilobytes all the time, does not rewrite the status on
// every pass.
const usageQuantum = 16 << 20

// MeasureStore is the replica's store against its two limits, measured the way the registry
// measures it.
func MeasureStore(dataDir string, spec *registryv1alpha1.RegistryStorageSpec, capacity int64) (*registryv1alpha1.StoreUsage, error) {
	budget, err := Budget(spec.Store.Size)
	if err != nil {
		return nil, err
	}
	used, err := storeUsage(dataDir)
	if err != nil {
		return nil, err
	}
	free, measured, err := measureFilesystem(dataDir)
	if err != nil {
		return nil, err
	}
	if capacity <= 0 {
		capacity = measured
	}
	reserve := Reserve(capacity)

	usage := &registryv1alpha1.StoreUsage{
		// Rounded toward the alarming side: used up, free down.
		UsedBytes:               (used + usageQuantum - 1) / usageQuantum * usageQuantum,
		BudgetBytes:             budget,
		ReserveBytes:            reserve,
		FilesystemFreeBytes:     free / usageQuantum * usageQuantum,
		FilesystemCapacityBytes: capacity,
		Writable:                true,
	}

	// The same decision as the registry's, from the exact figures rather than the rounded ones.
	reserveRoom := free - reserve
	budgetRoom := reserveRoom
	if budget > 0 {
		budgetRoom = budget - used
	}
	switch {
	case min(budgetRoom, reserveRoom) >= refuseWithin:
	case budget > 0 && budgetRoom < reserveRoom:
		usage.Writable, usage.Reason = false, registryv1alpha1.StoreBudgetExhausted
	default:
		usage.Writable, usage.Reason = false, registryv1alpha1.StoreReserveExhausted
	}
	return usage, nil
}

// storeUsage is what the budget counts: blobs, and uploads in progress. The links under each
// repository are skipped — most of the files in a store and none of the bytes.
func storeUsage(dataDir string) (int64, error) {
	var total int64
	walk := func(dir string, keep func(path string) bool) error {
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
			if entry.IsDir() && (entry.Name() == "_layers" || entry.Name() == "_manifests") {
				return filepath.SkipDir
			}
			if !entry.Type().IsRegular() || !keep(path) {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
			total += info.Size()
			return nil
		})
		if err != nil {
			return fmt.Errorf("measuring %s: %w", dir, err)
		}
		return nil
	}

	v2 := filepath.Join(dataDir, "docker", "registry", "v2")
	if err := walk(filepath.Join(v2, "blobs"), func(string) bool { return true }); err != nil {
		return 0, err
	}
	uploads := func(path string) bool { return strings.Contains(filepath.ToSlash(path), "/_uploads/") }
	if err := walk(filepath.Join(v2, "repositories"), uploads); err != nil {
		return 0, err
	}
	return total, nil
}

// measureFilesystem is statfs, and a variable so that the tests can stand in for a disk.
var measureFilesystem = statfs

// statfs reports the bytes available to an unprivileged writer, and the size, of the filesystem
// holding path.
func statfs(path string) (int64, int64, error) {
	for {
		var stat syscall.Statfs_t
		err := syscall.Statfs(path, &stat)
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return 0, 0, fmt.Errorf("measuring the filesystem under %s: %w", path, err)
		}
		size := int64(stat.Bsize) //nolint:unconvert // int64 on Linux, uint32 on darwin, where the tests also run
		return int64(stat.Bavail) * size, int64(stat.Blocks) * size, nil
	}
}

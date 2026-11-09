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

// ReservePart is one term of the reserve: a share of the filesystem's size, up to a ceiling. The
// reserve is the sum of its parts, taken of the filesystem's size as it is at the time — see
// ReserveParts and Reserve.
type ReservePart struct {
	// Percent is the share of the filesystem's size.
	Percent int64 `json:"percent"`

	// AtMost is the ceiling of this part, in bytes.
	AtMost int64 `json:"atMost"`
}

// ReserveParts is how much of the filesystem under the store must stay free, as a rule over its size.
//
// On a filesystem the store shares with the node — which is most masters: nothing requires a disk of
// its own — two parts. The first is the kubelet's soft eviction threshold for its node filesystem, as
// the module's kubelet step configures it — 10 %, at most 40G — because the point of the reserve is
// that the store stops before the kubelet starts evicting. The second is room for everyone else who
// writes there: containerd unpacking an image, logs, etcd on a static master. The store cannot see or
// stop those writes, so the margin is sized for them — 10 % again, at most 20G.
//
// On a filesystem of its own the kubelet's part protects nothing: the kubelet measures its own
// filesystem, and the store filling this one does not bring it a byte closer to evicting. So only the
// margin stays, kept for what the store cannot bound itself — the links and tags the budget does not
// count, the blocks a write takes beyond its length, a filesystem's own metadata. Whether the
// filesystem is the store's own is DetectFilesystem's call, and every doubt there is a no.
//
// GiB rather than the kubelet's decimal G for both ceilings: the reserve must never come out below
// the kubelet's threshold, and the binary unit only errs above it.
//
// A rule and not a figure, because the registry takes it of the size of the filesystem at every
// measurement: a disk grown under a running store moves the reserve with it, with no new
// configuration and no restart of the registry.
func ReserveParts(dedicated bool) []ReservePart {
	if dedicated {
		return []ReservePart{{Percent: 10, AtMost: 20 * gib}}
	}
	return []ReservePart{{Percent: 10, AtMost: 40 * gib}, {Percent: 10, AtMost: 20 * gib}}
}

// reserveQuantum is what the size of the filesystem is rounded up to before the reserve is taken of
// it. Up, so that the reserve only ever errs above what the parts say; and to a whole GiB, so that a
// filesystem whose reported size moves a little with its own bookkeeping does not move the reserve
// on every measurement. The registry rounds the same way, so the status says what it enforces.
const reserveQuantum = gib

// Reserve is how much of a filesystem of the given size the store leaves free: ReserveParts taken of
// the size, rounded up to a whole GiB. The registry's wrapper computes the same figure from the same
// parts.
func Reserve(capacity int64, dedicated bool) int64 {
	if capacity <= 0 {
		return 0
	}
	rounded := (capacity + reserveQuantum - 1) / reserveQuantum * reserveQuantum
	var reserve int64
	for _, part := range ReserveParts(dedicated) {
		// The share without the overflow capacity*percent would risk on a large enough disk.
		share := rounded/100*part.Percent + rounded%100*part.Percent/100
		reserve += min(share, part.AtMost)
	}
	return reserve
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

// refuseWithin is how close to a limit the registry stops accepting new writes: it refuses a push up
// front once less than this is left, rather than let it fail a block in. The registry's own figure —
// its guard's measurement interval — repeated here so that the status says what the registry does.
const refuseWithin = 32 << 20

// usageQuantum is what the reported sizes are rounded to, so that a filesystem shared with the rest
// of the node, whose free space moves by kilobytes all the time, does not rewrite the status on
// every pass.
const usageQuantum = 16 << 20

// MeasureStore is the replica's store against its two limits, measured the way the registry
// measures it: the reserve taken of the filesystem's size as it is now, which is what the registry
// takes it of too. dedicated says whether the filesystem is the store's own — see ReserveParts.
func MeasureStore(dataDir string, spec *registryv1alpha1.RegistryStorageSpec, dedicated bool) (*registryv1alpha1.StoreUsage, error) {
	budget, err := Budget(spec.Store.Size)
	if err != nil {
		return nil, err
	}
	used, err := storeUsage(dataDir)
	if err != nil {
		return nil, err
	}
	free, capacity, err := measureFilesystem(dataDir)
	if err != nil {
		return nil, err
	}
	reserve := Reserve(capacity, dedicated)

	usage := &registryv1alpha1.StoreUsage{
		// Rounded toward the alarming side: used up, free down.
		UsedBytes:           (used + usageQuantum - 1) / usageQuantum * usageQuantum,
		BudgetBytes:         budget,
		ReserveBytes:        reserve,
		FilesystemFreeBytes: free / usageQuantum * usageQuantum,
		// Down, like free: a filesystem whose reported size moves with its own bookkeeping would
		// otherwise rewrite the status on every pass.
		FilesystemCapacityBytes: capacity / usageQuantum * usageQuantum,
		DedicatedFilesystem:     dedicated,
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

// StoreBytes is the exact size of the store the way the budget counts it, unrounded: for telling how
// much a collection gave back, where MeasureStore's rounding would hide a small one entirely.
func StoreBytes(dataDir string) (int64, error) {
	return storeUsage(dataDir)
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

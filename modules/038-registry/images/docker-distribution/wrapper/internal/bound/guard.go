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

// Package bound keeps the store from filling the disk of the node it runs on.
//
// Every byte the store receives — a fill from the upstream, a follower replicating from the leader,
// a `d8 mirror push`, a cache miss kept for next time — ends in one process and one storage driver,
// so that is where the limit sits: synchronously, on every write, and below the point where the
// node itself starts to suffer. An alert cannot do this job. Between "fine" and "the disk is full"
// there are minutes, and the master that fills is usually every master at once, because all
// replicas receive the same image set at nearly the same time.
//
// Two limits, checked independently:
//
//   - the budget, `storage.size`: how much the store may hold. Applies to blobs and uploads in
//     progress, which is where the bytes are. Zero means none was configured.
//   - the reserve: how much of the filesystem must stay free. Applies to every write, links and
//     tags included, because on a filesystem the store shares with the kubelet, containerd and
//     etcd, nothing may take the last free bytes. The syncer derives it from the eviction
//     thresholds of the kubelet, so the store stops well before the kubelet starts evicting.
//
// Nothing here deletes anything. A full store refuses writes and keeps serving reads; what frees
// space is the garbage collection, on its own rules.
package bound

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Reason names which limit refused a write.
type Reason string

const (
	// BudgetExhausted: the store holds as much as `storage.size` allows.
	BudgetExhausted Reason = "BudgetExhausted"

	// ReserveExhausted: the filesystem under the store is down to the space kept for the node.
	ReserveExhausted Reason = "ReserveExhausted"
)

// ErrStoreFull is the refusal. Typed, so that the write endpoint can answer it with its own status
// and the syncer can tell it from a failure worth retrying.
type ErrStoreFull struct {
	Reason Reason

	// Used and Budget are the store's own bytes and its limit; Budget is zero when none is set.
	Used, Budget int64

	// Free and Reserve describe the filesystem the store is on.
	Free, Reserve int64
}

func (e *ErrStoreFull) Error() string {
	if e.Reason == BudgetExhausted {
		return fmt.Sprintf("the store is full: it holds %s of the %s budget; raise storage.size "+
			"or wait for the garbage collection to reclaim space", human(e.Used), human(e.Budget))
	}
	return fmt.Sprintf("the store refuses writes: the filesystem it is on has %s free, and %s is "+
		"kept for the node; free space on the node or move the store to a dedicated disk",
		human(e.Free), human(e.Reserve))
}

// statfsEvery is how many bytes may be written on a free-space figure before it is measured again.
//
// Between two measurements the guard subtracts what it wrote itself, so what escapes it is what
// OTHER processes wrote to a shared filesystem in the meantime — which the reserve is sized for.
const statfsEvery = 32 << 20

// blockSize is the allocation unit a new file is charged in against the reserve. 4 KiB is what ext4,
// xfs and tmpfs allocate by default; a filesystem with larger blocks is under-charged by the
// difference, which the reserve's margin is sized to absorb.
const blockSize = 4096

// Guard is the accounting for one store, shared by every driver instance in the process.
//
// One per process and not one per driver, because the process builds two: the serving half and the
// write endpoint each open the storage, and a guard in each would let both spend the same budget.
type Guard struct {
	root    string
	budget  int64
	reserve int64

	// statfs reports the free and total bytes of the filesystem holding a path. A field so that the
	// tests can stand in for a disk.
	statfs func(path string) (free, capacity int64, err error)

	mu sync.Mutex

	// base is what the last walk of the store found; delta is what the drivers wrote since.
	//
	// A walk plus a running count rather than a count alone. A count kept from commits and
	// cancellations has to follow every path distribution takes — a blob that already exists is
	// dropped instead of moved, a proxied upload is abandoned on a client's disconnect, a
	// collection deletes from another process — and each one it misses stays missed. The walk
	// corrects all of them, and between walks the count errs only upward: a write the walk already
	// saw is counted twice until the next one, which refuses a little early and never late.
	base  int64
	delta int64

	// free is the last measurement of the filesystem, and since what was written after it.
	free  int64
	since int64

	// refusing is the limit writes are refused on, empty while they are accepted. Kept only to log
	// the transitions: one line when the store stops accepting writes and one when it resumes,
	// rather than one per refused chunk.
	refusing Reason
	log      *slog.Logger
}

// New returns a guard over the store at root. A zero budget means none: only the reserve applies.
//
// The guard knows nothing about the store until Measure has run once; the caller does that before
// any listener opens.
func New(root string, budget, reserve int64) *Guard {
	return &Guard{root: root, budget: budget, reserve: reserve, statfs: statfs, log: slog.New(slog.DiscardHandler)}
}

// WithLogger makes the guard report when the store stops and resumes accepting writes.
func (g *Guard) WithLogger(log *slog.Logger) *Guard {
	g.log = log
	return g
}

// Used is what the store holds by the guard's count: the last walk plus what was written since.
func (g *Guard) Used() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.base + g.delta
}

// Refusal is why a new write would be refused, or nil when it would be accepted.
//
// For whoever answers a request before the storage sees it: the write endpoint refuses a push up
// front with a status the client can read, instead of letting it fail inside distribution as an
// unknown error. A budgeted write is assumed, which is what every upload and manifest is.
//
// Refused once less than statfsEvery is left before either limit, not only once a limit is
// reached. The store stops a few blocks short of a limit — whatever the last refused chunk did not
// take — and a check for "reached" would wave every later push through to fail one block in, as a
// 500. What those last megabytes could still hold is not worth that.
func (g *Guard) Refusal() *ErrStoreFull {
	g.mu.Lock()
	defer g.mu.Unlock()

	used := g.base + g.delta
	if err := g.refresh(); err != nil {
		return &ErrStoreFull{Reason: ReserveExhausted, Used: used, Budget: g.budget, Reserve: g.reserve}
	}

	reserveRoom := g.free - g.reserve
	budgetRoom := reserveRoom
	if g.budget > 0 {
		budgetRoom = g.budget - used
	}

	var full *ErrStoreFull
	switch {
	case min(budgetRoom, reserveRoom) >= statfsEvery:
		return nil
	case g.budget > 0 && budgetRoom < reserveRoom:
		// Named after the limit that is nearer: that is the one whose fix frees the store.
		full = &ErrStoreFull{Reason: BudgetExhausted, Used: used, Budget: g.budget, Reserve: g.reserve}
	default:
		full = &ErrStoreFull{
			Reason: ReserveExhausted, Used: used, Budget: g.budget, Free: g.free, Reserve: g.reserve,
		}
	}
	// Once the store is this close, pushes stop here and never reach the storage — so this is where
	// the moment it stopped gets logged, or it would not be logged at all.
	g.note(false, full)
	return full
}

// Measure walks the store and replaces the running count with what is on disk.
func (g *Guard) Measure() error {
	g.mu.Lock()
	counted := g.delta
	g.mu.Unlock()

	used, err := usage(g.root)
	if err != nil {
		return err
	}

	g.mu.Lock()
	g.base = used
	// Only what was counted before the walk began: what was written during it may or may not be
	// in the figure, and keeping it in the count is the side that errs toward refusing.
	g.delta -= counted
	g.mu.Unlock()

	return nil
}

// Run measures the store on every tick until the context ends.
func (g *Guard) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := g.Measure(); err != nil {
				// The count keeps growing with every write in the meantime, so a failed walk
				// leaves the guard refusing early rather than late.
				g.log.Error("measuring the store failed", "error", err.Error())
			}
		}
	}
}

// take admits size bytes about to be written at path, or refuses them.
//
// Admitting and counting are one step under one lock: two writers checking first and counting
// afterwards could both fit into the last free megabyte. fresh asks for the filesystem to be
// measured now rather than on the running estimate — once per opened writer and per small file,
// which is cheap next to the write itself.
func (g *Guard) take(path string, size int64, fresh bool) error {
	budgeted := budgeted(path)

	g.mu.Lock()
	defer g.mu.Unlock()

	err := g.admit(budgeted, size, fresh)
	g.note(budgeted, err)
	return err
}

// admit is take's decision. Called with the lock held.
func (g *Guard) admit(budgeted bool, size int64, fresh bool) error {
	used := g.base + g.delta
	if budgeted && g.budget > 0 && used+size > g.budget {
		return &ErrStoreFull{Reason: BudgetExhausted, Used: used, Budget: g.budget, Reserve: g.reserve}
	}

	// What the write costs the filesystem, which is not what it adds to the store. A new file takes
	// whole blocks, and the store writes many small ones — a link is 71 bytes and occupies 4 KiB —
	// so counting their length would let a push of many images spend the reserve a block at a time.
	// A chunk of a stream extends a file that is already open and is counted as it is.
	cost := size
	if fresh {
		cost = (size+blockSize-1)/blockSize*blockSize + blockSize
	}

	if fresh || g.since >= statfsEvery {
		if err := g.refresh(); err != nil {
			return err
		}
	}
	if g.free-g.since-cost < g.reserve {
		// The estimate only ever goes down between measurements, and space may have been freed
		// since the last one. Measure before refusing on it.
		if !fresh {
			if err := g.refresh(); err != nil {
				return err
			}
		}
		if g.free-g.since-cost < g.reserve {
			return &ErrStoreFull{
				Reason: ReserveExhausted, Used: used, Budget: g.budget,
				Free: g.free - g.since, Reserve: g.reserve,
			}
		}
	}

	if budgeted {
		g.delta += size
	}
	g.since += cost
	return nil
}

// note logs the moments the store stops and resumes accepting writes. Called with the lock held.
//
// Resuming is only ever concluded from a budgeted write, which answers to both limits: a link
// accepted while the budget is exhausted says nothing about the budget. And only once there is room
// again worth the name: at the edge, small writes keep fitting between refused chunks, and a store
// that flapped in the log on every one of them would bury the moment it actually recovered.
func (g *Guard) note(budgeted bool, err error) {
	var full *ErrStoreFull
	switch {
	case errors.As(err, &full):
		if g.refusing != full.Reason {
			g.refusing = full.Reason
			g.log.Warn("the store refuses writes", "reason", string(full.Reason), "detail", full.Error())
		}
	case err == nil && budgeted && g.refusing != "" && g.headroom() >= statfsEvery:
		g.log.Info("the store accepts writes again", "after", string(g.refusing),
			"used", g.base+g.delta, "budget", g.budget, "free", g.free-g.since, "reserve", g.reserve)
		g.refusing = ""
	}
}

// headroom is how much more the store could write before either limit refuses it. Called with the
// lock held.
func (g *Guard) headroom() int64 {
	room := g.free - g.since - g.reserve
	if g.budget > 0 {
		room = min(room, g.budget-(g.base+g.delta))
	}
	return room
}

// refresh measures the filesystem. Called with the lock held.
func (g *Guard) refresh() error {
	free, _, err := g.statfs(g.root)
	if err != nil {
		// Refused rather than admitted: a guard that cannot see the disk cannot promise anything
		// about it.
		return fmt.Errorf("measuring the free space under %s: %w", g.root, err)
	}
	g.free = free
	g.since = 0
	return nil
}

// budgeted tells whether a write at this driver path counts against the budget.
//
// Blobs and uploads are where the bytes are; manifests are blobs too. Links and tags are small and
// many, and counting them would put the budget at the mercy of the repository layout. They still
// answer to the reserve. A repository name component cannot begin with an underscore, so
// `_uploads` cannot be part of a name.
func budgeted(path string) bool {
	return strings.HasPrefix(path, "/docker/registry/v2/blobs/") || strings.Contains(path, "/_uploads/")
}

// usage is the sum of the blobs and the uploads under a store's root: what the budget counts.
func usage(root string) (int64, error) {
	var total int64

	sum := func(dir string, skip func(name string) bool, keep func(string) bool) error {
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				// A directory removed while walking — an upload finished, a collection ran — is
				// simply not there any more.
				if errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
			if entry.IsDir() && skip(entry.Name()) {
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
			return fmt.Errorf("walking %s: %w", dir, err)
		}
		return nil
	}

	v2 := filepath.Join(root, "docker", "registry", "v2")
	none := func(string) bool { return false }
	all := func(string) bool { return true }
	// The links: one small file per layer and per manifest of every repository, which is most of
	// the files in a store and none of the bytes. Walking them every 30 seconds is what once cost
	// the syncer most of a CPU; only the repository directories and the uploads are visited.
	links := func(name string) bool { return name == "_layers" || name == "_manifests" }
	uploads := func(path string) bool {
		return strings.Contains(filepath.ToSlash(path), "/_uploads/")
	}

	if err := sum(filepath.Join(v2, "blobs"), none, all); err != nil {
		return 0, err
	}
	if err := sum(filepath.Join(v2, "repositories"), links, uploads); err != nil {
		return 0, err
	}
	return total, nil
}

// statfs reports the bytes available to an unprivileged writer, which is also what the kubelet
// compares its eviction thresholds against.
// Returns the free bytes, then the capacity.
func statfs(path string) (int64, int64, error) {
	for {
		var stat syscall.Statfs_t
		err := syscall.Statfs(path, &stat)
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return 0, 0, err
		}
		size := int64(stat.Bsize) //nolint:unconvert // int64 on Linux, uint32 on darwin, where the tests also run
		return int64(stat.Bavail) * size, int64(stat.Blocks) * size, nil
	}
}

// human renders a size the way an operator reads it.
func human(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	value, suffix := float64(n), ""
	for _, s := range []string{"KiB", "MiB", "GiB", "TiB", "PiB"} {
		value /= unit
		suffix = s
		if value < unit {
			break
		}
	}
	return fmt.Sprintf("%.1f %s", value, suffix)
}

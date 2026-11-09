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
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/types"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-syncer/internal/distribution"
)

// storeMemoryFile is the file in the store's directory that keeps what the store was when a warning
// about its filesystem was first raised, for as long as the warning stands. See rememberStore.
//
// Beside the registry's tree and not in it: the registry, its garbage collection and the store's
// measure all start at docker/registry/v2, and none of them sees it.
const storeMemoryFile = ".filesystem-before"

// storeMemory is what storeMemoryFile holds.
type storeMemory struct {
	DedicatedFilesystem bool  `json:"dedicatedFilesystem"`
	UsedBytes           int64 `json:"usedBytes"`
}

// storeFilesystem is what the syncer found out about the filesystem under the store when it started.
// Read-only after that, so any goroutine may ask.
func (l *Loop) storeFilesystem() distribution.Filesystem {
	if l.Applier == nil {
		return distribution.Filesystem{}
	}
	return l.Applier.Options.StoreFilesystem
}

// recallStore reads what this replica's store was before this process started: compared with what
// the store is now, it is how a disk that was not mounted this time is told from one that never was.
//
// The store's own memory first — see rememberStore — and what the replica last reported when there is
// none. The report alone is not enough: the first pass replaces it with what the store is now, and a
// pod restarted with the disk still not mounted would find nothing left to compare with. Nothing when
// neither can be read, and then nothing is compared.
//
// What the replica last reported is also where lastMeasured starts — see publishedStore — so that a
// store a restarted syncer can neither measure nor write to is still published as refusing writes. Not
// a report of NotWritable, which a syncer before this one put over its measurement: it says nothing
// about the store's room, and the status already says what it would.
func (l *Loop) recallStore(ctx context.Context) {
	if l.DataDir != "" {
		if content, err := os.ReadFile(filepath.Join(l.DataDir, storeMemoryFile)); err == nil {
			var memory storeMemory
			if err := json.Unmarshal(content, &memory); err == nil {
				l.lastRun = &registryv1alpha1.StoreUsage{
					DedicatedFilesystem: memory.DedicatedFilesystem,
					UsedBytes:           memory.UsedBytes,
				}
				l.remembered = true
			}
		}
	}

	if l.Client == nil {
		return
	}
	storage := &registryv1alpha1.RegistryStorage{}
	if err := l.Client.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage); err != nil {
		return
	}
	for i := range storage.Status.Replicas {
		reported := storage.Status.Replicas[i].Store
		if storage.Status.Replicas[i].Node != l.Node || reported == nil {
			continue
		}
		if !l.remembered {
			l.lastRun = reported.DeepCopy()
		}
		if reported.Reason != registryv1alpha1.StoreNotWritable {
			l.lastMeasured.CompareAndSwap(nil, reported.DeepCopy())
		}
	}
}

// rememberStore writes lastRun into the store's directory, once, when it has raised a warning.
//
// Into the directory that is there now, which is the one the warning is about: the node's own
// directory while the disk meant for it is not mounted, and the disk while it hides what the node's
// directory held. So the memory is found on every restart for as long as the store stays as it is
// warned about, and is out of sight — under the disk, or taken away with it — once it is put right.
func (l *Loop) rememberStore() {
	if l.remembered || l.lastRun == nil {
		return
	}
	content, err := json.Marshal(storeMemory{
		DedicatedFilesystem: l.lastRun.DedicatedFilesystem,
		UsedBytes:           l.lastRun.UsedBytes,
	})
	if err == nil {
		err = os.WriteFile(filepath.Join(l.DataDir, storeMemoryFile), content, 0o644)
	}
	if err != nil {
		l.Log.Warn("the store's filesystem cannot be remembered, so a restart forgets the warning about it",
			"error", err.Error())
	}
	l.remembered = true
}

// forgetStore removes the store's memory of what it was: it holds something, and is the store the
// replica works with now.
func (l *Loop) forgetStore() {
	err := os.Remove(filepath.Join(l.DataDir, storeMemoryFile))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		l.Log.Warn("the store's memory of its filesystem cannot be removed", "error", err.Error())
	}
}

// inspectStore looks for what storeProblems looks for once at startup, so that the log says it before
// the first pass has run — which may be minutes later, behind a registry that is still walking its
// store.
func (l *Loop) inspectStore() {
	if l.DataDir == "" {
		return
	}
	var store *registryv1alpha1.StoreUsage
	if used, err := distribution.StoreBytes(l.DataDir); err == nil {
		store = &registryv1alpha1.StoreUsage{UsedBytes: used}
	}
	l.storeProblems(store)
}

// storeProblems is what is wrong with the store that the copy would not say: a directory the registry
// cannot write to — see distribution.CheckWritable — and a store on another filesystem than the last
// run's — see filesystemWarning. Logged when it appears, changes or goes away; returned for the
// replica's error. store is the store as measured now, nil when it could not be.
//
// The replica's error, because it is the one place an operator and the controller already look: a
// store the registry cannot write to is a replica that cannot be filled, and the copy's own error says
// only that every blob failed with a 500. A store that is not writable is also published as refusing
// writes — see publishedStore — for the condition and the alert, which read the store and not the error.
func (l *Loop) storeProblems(store *registryv1alpha1.StoreUsage) string {
	return l.storeProblemsFor(store, store != nil && store.UsedBytes > 0)
}

// storeProblemsWhileCopying is storeProblems for a pass under way that has copied or found a reference:
// the store holds something, whatever it measured last, so it is not walked again to be told so.
//
// For the progress reports, which replace the replica's error — see report.PublishProgress. Without it
// the first of them took the store's problems off the status until the pass ended, an hour later over
// a slow link: a store the registry cannot write to counts every reference it already holds as done,
// and read as filling, with nothing wrong with it, for the whole pass.
func (l *Loop) storeProblemsWhileCopying() string {
	return l.storeProblemsFor(nil, true)
}

// storeProblemsFor is storeProblems, told whether the store holds anything: from what it measured, or
// from a copy that landed in it. Only the loop's goroutine calls it — see lastRun.
func (l *Loop) storeProblemsFor(store *registryv1alpha1.StoreUsage, holdsData bool) string {
	if l.DataDir == "" {
		return ""
	}

	var problems []string
	if err := l.probeWritable(); err != nil {
		problems = append(problems, err.Error())
	}

	// Only from a store that was measured: one whose walk failed is not known to be empty, and the
	// warning says it is. Until it is measured again, what was said stands.
	switch {
	case holdsData:
		// A store off the disk meant for it stays so however much it holds: filled from the upstream
		// or the leader, it fills the node's filesystem, and the warning stands — and is remembered,
		// for a restart — until the disk is back. Otherwise this is the store the replica works with
		// now, and what it was before no longer tells anything about it.
		l.filesystemProblem = filesystemWarning(l.lastRun, l.storeFilesystem(), l.StoreHostPath, true)
		if l.filesystemProblem != "" {
			l.rememberStore()
			break
		}
		if l.lastRun != nil {
			l.lastRun = nil
			l.forgetStore()
		}
	case store != nil:
		l.filesystemProblem = filesystemWarning(l.lastRun, l.storeFilesystem(), l.StoreHostPath, false)
		if l.filesystemProblem != "" {
			l.rememberStore()
		}
	}
	if l.filesystemProblem != "" {
		problems = append(problems, l.filesystemProblem)
	}

	problem := strings.Join(problems, "; ")
	switch {
	case problem != "" && problem != l.storeProblem:
		l.Log.Warn("the store needs attention", "problem", problem)
	case problem == "" && l.storeProblem != "":
		l.Log.Info("the store no longer needs attention", "was", l.storeProblem)
	}
	l.storeProblem = problem
	return problem
}

// probeWritable runs the write probe — see distribution.CheckWritable — and keeps whether it failed for
// publishedStore. The probe alone, which touches nothing only the loop's goroutine may touch: watchStore
// asks it on its own goroutine, and storeProblems, which also keeps lastRun and the warnings, must not
// be asked from there.
func (l *Loop) probeWritable() error {
	checkWritable := l.checkWritable
	if checkWritable == nil {
		checkWritable = distribution.CheckWritable
	}
	err := checkWritable(l.DataDir, l.StoreHostPath)
	l.storeNotWritable.Store(err != nil)
	return err
}

// publishedStore is the store as the replica reports it: the measurement, and writes refused as
// NotWritable while the last probe found the registry's tree in the store refusing them. NotWritable
// wins over a limit the store is also at, because the operator's fix is the only one that helps: a
// collection or a larger budget gives room to a store the registry still cannot write to.
//
// Without it a store nobody could write to read writable — the measurement knows only space — and the
// condition, the metric and D8RegistryStoreWritesRefused said nothing; the one alert that did fire,
// two hours later, said the replica was still filling.
//
// Only what is published. RoomAhead, StoreRefusesWrites and the copy ahead keep to the measurement,
// and the collection on this replica with them: it releases what is ahead of the set when the store
// has no room for it, sweeps without waiting when the store is at a limit and does not wait for a fill
// that cannot land — each right for a store out of room and wrong for one whose room is fine and whose
// directory refuses.
//
// A store that cannot be measured now is published from lastMeasured while it refuses writes. Most of
// the time a failed measurement leaves the status as it is, but a tree the registry cannot write to is
// often one it cannot read either — the syncer is held to the same mode bits — and its walk fails with
// the probe: left as it is, the status kept saying writable for exactly the store it is about. Nil when
// nothing was measured and nothing has to be said, and when the store has never been measured.
func (l *Loop) publishedStore(measured *registryv1alpha1.StoreUsage) *registryv1alpha1.StoreUsage {
	if !l.storeNotWritable.Load() {
		return measured
	}
	if measured == nil {
		measured = l.lastMeasured.Load()
	}
	if measured == nil {
		return nil
	}
	published := measured.DeepCopy()
	published.Writable, published.Reason = false, registryv1alpha1.StoreNotWritable
	return published
}

// filesystemWarning is what a store on another filesystem than the last run's probably means, or ""
// when it is not that. before is what the store was before this process started, nil once the store
// holds something and is where it was meant to be; holdsData is whether it holds anything now.
//
// A disk for the store that is not mounted when the pod starts — mounted with nofail and late, failed,
// or added after the cluster was installed — does not stop anything: the kubelet creates the store's
// directory on the node's own filesystem and the store starts there, empty. On a cluster with an
// upstream it fills again, onto a filesystem it was meant to stay off; on an air-gapped one with a
// single master it has nothing to fill from, and every node is left without its images. The other way
// round, a disk mounted over a store that was filling the node's filesystem hides what it held.
//
// The first is said for as long as the disk is not there, filled or not: the store's fill is what
// takes the node's filesystem, and the operator who keeps the store there on purpose removes the
// store's memory — see rememberStore — and this pod. The second only while the store is empty, because
// that is when it is likely and costs the most; once the store fills on the disk, the disk is where it
// is meant to be. And only against what this replica reported, or remembered once it warned, because a
// marker left in the store on every run would not say it — the directory the last run wrote into is
// the one that is not mounted now.
func filesystemWarning(before *registryv1alpha1.StoreUsage, now distribution.Filesystem, hostPath string, holdsData bool) string {
	if before == nil {
		return ""
	}
	where := hostPath
	if where == "" {
		where = "the store's directory on the node"
	}
	switch {
	case before.DedicatedFilesystem && !now.Dedicated:
		state := "the store is empty and on the node's own filesystem"
		if holdsData {
			state = "the store is on the node's own filesystem"
		}
		return fmt.Sprintf("%s, while it was on a filesystem of its own when this replica last ran: the "+
			"disk meant for %s is probably not mounted. Mount it — before the kubelet starts, so in "+
			"/etc/fstab without nofail — and delete this pod; until then the store fills the node's "+
			"filesystem. To keep the store there, delete %s in that directory and this pod",
			state, where, storeMemoryFile)
	case !before.DedicatedFilesystem && before.UsedBytes > 0 && now.Dedicated && !holdsData:
		return fmt.Sprintf("the store is empty and on a filesystem of its own, while it held %d bytes "+
			"on the node's own filesystem when this replica last ran: what it held is probably still "+
			"there, hidden under the disk mounted at %s or above it", before.UsedBytes, where)
	}
	return ""
}

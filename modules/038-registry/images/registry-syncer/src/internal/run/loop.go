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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/registry-syncer/internal/distribution"
	"github.com/deckhouse/registry-syncer/internal/fill"
	"github.com/deckhouse/registry-syncer/internal/gc"
	"github.com/deckhouse/registry-syncer/internal/metrics"
	"github.com/deckhouse/registry-syncer/internal/report"
)

// Leadership tells the loop whether this replica currently holds the lease.
type Leadership interface {
	IsLeader() bool
}

// Loop keeps one replica in step with the desired storage state.
type Loop struct {
	Log    *slog.Logger
	Client client.Client

	// Namespace holds the Secret the storage spec references its credentials through.
	Namespace string

	// Node this replica runs on, and the key of its status entry.
	Node string

	// Applier keeps the serving process configured.
	Applier *distribution.Applier

	// Publisher writes this replica's status entry.
	Publisher *report.Publisher

	// Leadership decides which work this replica does.
	Leadership Leadership

	// LocalAddress is where this replica's own registry answers, used for the
	// catalogue read and as the destination of a fill.
	LocalAddress string

	// WriteAddress is where the NON-PROXYING listener of this replica's registry answers.
	//
	// Its own field rather than the serving address with the port swapped, so that a test can point a
	// loop at one registry and a deployment can point it at the other listener of its own.
	// Empty falls back to the serving address with the write endpoint's port, which is what the
	// storage pod always looks like.
	WriteAddress string

	// DataDir is where this replica's store keeps its blobs, mounted into the syncer.
	//
	// A field rather than the constant it defaults to, because the one number that can
	// authorize cutting a cluster off from its upstream is counted from this directory, and a
	// count that can only be exercised against a path baked into the binary is a count nobody
	// can test. Empty means the constant.
	DataDir string

	// ReportedAddress is where OTHER replicas reach this one, which is not the same
	// as LocalAddress: this replica talks to itself over the loopback interface,
	// while its neighbours come in over the node address.
	ReportedAddress string

	// LocalCA and LocalAuth are how to reach it.
	LocalCA                  string
	LocalUser, LocalPassword string

	// Metrics is what this replica reports about its own work. Optional: nothing about
	// filling a cache should depend on whether anyone is watching.
	Metrics *metrics.Metrics

	// filling records that a fill or a replication is in flight, so that the garbage collection
	// can wait rather than fail it: collecting makes the registry refuse writes, and a fill is
	// writes. Filling the cache is what the cache is for; reclaiming is housekeeping.
	filling atomic.Bool

	// The copy ahead, run in the background — see considerAhead. aheadMu guards the fields after it;
	// aheadRunning is read by Filling without it.
	aheadMu sync.Mutex
	// The copy in progress: how to stop it, when it has, and what it was started under — see aheadScope.
	aheadCancel context.CancelFunc
	aheadDone   chan struct{}
	aheadScope  string
	// aheadHeld is the parts of what is ahead of the set a copy here left whole — see AheadKey;
	// aheadMissedFor the parts the last copy was for and missed something of, and when — see aheadRetry.
	// aheadForgotten: the collection took what is ahead of the set back, so what this replica reported
	// before no longer stands — see ForgetAhead.
	aheadHeld      map[string]bool
	aheadMissedFor string
	aheadMissedAt  time.Time
	aheadForgotten bool
	aheadRunning   atomic.Bool

	// writes serialises this replica's own writes to the cache against the garbage collection.
	//
	// Inside the storage pod the syncer is the only writer, so "the store must not be written to
	// while it is being collected" needs no cooperation from the registry: it is this process
	// refraining. Which is why nothing here touches the registry's configuration any more — a
	// collection used to make it refuse writes by restarting it, and the kubelet counts a restart as
	// a crash. Serving images is not something a collection may interrupt.
	writes sync.Mutex

	// The last survey of the store and when it was taken. See surveyStore.
	surveyMu    sync.Mutex
	survey      fill.Survey
	surveyedAt  time.Time
	surveyedFor int

	// The last declared set and when it was read. See declaredSet.
	declaredMu  sync.Mutex
	declared    map[string]struct{}
	declaredFor string
	declaredAt  time.Time

	// lastTotalDigests is the last measured size of the store, carried between passes.
	//
	// Kept here because measuring it walks the store and only some passes do that: without a carry-over
	// the status would alternate between a real number and zero, and zero in that field reads as an
	// emptied store.
	lastTotalDigests atomic.Int32

	// lastDeclaredDigests is the last measured SIZE OF THE SET, carried the same way and for the same
	// reason: with the numerator carried over and the denominator carried nowhere, a restarted process
	// publishes progress against an absent total, which reads as no progress at all.
	lastDeclaredDigests atomic.Int32

	// complete records whether this replica holds the whole expected set, as of its last pass.
	//
	// Kept here rather than re-read from the API because the garbage collection asks it on its own
	// schedule, and an answer from the API would be one round trip behind this replica's own
	// knowledge — which on a store that is filling is the difference between "wait" and "go ahead".
	complete atomic.Bool

	// copiesNothing is whether the last pass found nothing for this replica to copy from — see OwesCopy.
	// The zero value owes, so a process that has not decided yet holds a collection back.
	copiesNothing atomic.Bool

	// desired is the last desired state that was read, so that a component on another
	// schedule — the garbage collection — can act on the same instruction without racing
	// this loop for it.
	desired atomic.Pointer[registryv1alpha1.RegistryStorageSpec]

	// Interval is how often the desired state is re-read. A poll rather than a
	// watch: the loop has to re-apply the configuration and re-report anyway, and a
	// missed event would otherwise leave the replica silently stale.
	Interval time.Duration

	// StartupGrace is how long after the process starts a registry of its own that does not answer
	// yet is waited for rather than reported. Two minutes when unset. See storeStarting.
	StartupGrace time.Duration

	// started is when Run began, and the zero time — which is long ago — outside it.
	started time.Time

	// RegistryWait is how long a pass waits for this replica's registry to listen before it leaves
	// the copy to the next pass. Zero means localRegistryWait.
	RegistryWait time.Duration

	// CopyPacer bounds how fast the copy ahead of the set reads from its source. Nil copies as fast as
	// the source answers. Copying the set itself — the fill and the replication — is not paced: see
	// fill.Copier.Ahead and fill.Pacer.
	CopyPacer *fill.Pacer

	// StoreRecheck is how often an idle replica reads its own store again. Zero means storeRecheck.
	// See recheckStore.
	StoreRecheck time.Duration

	// lastStoreCheck is when recheckStore last read the store, in Unix nanoseconds.
	lastStoreCheck atomic.Int64

	// registryDownSince is when this replica's registry was first found not listening, in Unix
	// nanoseconds, and zero while it listens. See registryDownReported.
	registryDownSince atomic.Int64
}

// surveyStore reads the store, at most once a minute.
//
// Cached because reading it walks the tree and the answer cannot change unless something wrote to the
// store — which, on the path this is called from, means a `d8 mirror push` that has just happened or is
// still happening. An interval short enough to catch a finished push and long enough to stop the loop
// living on the disk.
func (l *Loop) surveyStore(declared map[string]struct{}, deployed string) (fill.Survey, error) {
	const freshFor = time.Minute

	l.surveyMu.Lock()
	defer l.surveyMu.Unlock()

	// A request with no declared set asks only for the total, and any fresh survey answers it — the
	// total does not depend on what was declared. Asking for a SET, on the other hand, is only answered
	// by a survey taken for that same set.
	if time.Since(l.surveyedAt) < freshFor && (len(declared) == 0 || l.surveyedFor == len(declared)) {
		return l.survey, nil
	}

	survey, err := fill.Take(l.dataDir(), strings.Trim(constant.Path, "/"), declared,
		[]string{deployed})
	if err != nil {
		return fill.Survey{}, err
	}

	// Keyed on the size of the declared set as well as on time: a cluster that has just been updated
	// declares a different set, and answering that from a cache would judge the new set by the old
	// walk.
	l.survey, l.surveyedAt, l.surveyedFor = survey, time.Now(), len(declared)
	return survey, nil
}

// declaredSet is what the CLUSTER needs, as digests, cached for the same minute as the survey.
//
// Read here rather than in each branch because every branch has to answer with the same set, and the
// obvious source — whatever the branch's own copier enumerated — is NOT the same set. A follower
// enumerates through the leader's store, and that store also serves the nodes as a pass-through
// cache, so everything a node ever pulled through it joins the count. Replicas of one cluster would
// then report different sizes for the same set at the same moment, all of them "full", and a
// numerator can exceed its own denominator — which is not a number anybody can act on.
//
// Cached because reading it is a handful of registry requests — the release image, the installer it
// declares, and a HEAD per tag — and the loop runs every thirty seconds. Keyed on the deployed
// version as well as on time: an update changes the set, and answering that from the cache would
// judge a new set by an old reading.
func (l *Loop) declaredSet(ctx context.Context) (map[string]struct{}, string, error) {
	const freshFor = time.Minute

	releases, err := gc.FromCluster(ctx, l.Client)
	if err != nil {
		return nil, "", err
	}

	l.declaredMu.Lock()
	defer l.declaredMu.Unlock()

	if l.declared != nil && l.declaredFor == releases.Deployed && time.Since(l.declaredAt) < freshFor {
		return l.declared, releases.Deployed, nil
	}

	local, err := l.localRegistry()
	if err != nil {
		return nil, "", err
	}

	modules, err := gc.ModulesFromCluster(ctx, l.Client)
	if err != nil {
		return nil, "", err
	}

	declared, err := fill.DeclaredDigests(ctx, local, versionsOf(releases), modules)
	if err != nil {
		return nil, "", err
	}

	l.declared, l.declaredFor, l.declaredAt = declared, releases.Deployed, time.Now()
	return declared, releases.Deployed, nil
}

// announceStart says that a fill or a replication has begun, so the storage does not read `Idle`
// while it is under way.
//
// The report itself is published at the end of a pass, which is right — it reports what happened —
// but it leaves the first fill of a store with no report at all, and a controller with no reports
// calls the storage `Idle`: exactly when an operator most needs to see that something started.
// Publisher.Announce is create-only, so this cannot overwrite real numbers on a later pass.
//
// The denominator comes from the cheapest source that knows it, in order: what the spec declares,
// what the previous pass counted, and only then the set itself, which costs an API read.
func (l *Loop) announceStart(ctx context.Context, state report.State, stated int32) {
	if l.Publisher == nil {
		return
	}

	state.Full = false
	state.VerifiedDigests = 0
	state.TotalDigests = 0

	state.DeclaredDigests = stated
	if state.DeclaredDigests == 0 {
		state.DeclaredDigests = l.lastDeclaredDigests.Load()
	}
	if state.DeclaredDigests == 0 {
		if declared, _, err := l.declaredSet(ctx); err == nil {
			state.DeclaredDigests = int32(len(declared))
		}
	}

	if err := l.Publisher.Announce(ctx, state); err != nil {
		// Warned, never fatal: an unannounced fill is a fill nobody can see, not a fill that is
		// wrong. The pass carries on and publishes its real numbers at the end.
		l.Log.Warn("cannot announce that the fill started; the storage will read Idle until this pass ends",
			"error", err.Error())
	}
}

// reportHeld replaces a branch's own accounting with what this replica holds of the declared set.
//
// The branches count what their copier moved, which answers "what did this pass do" — worth logging
// and wrong to publish. The status is read for how much of what the cluster needs is here, and that
// has to mean the same in every role, or the replicas of one cluster cannot be compared at all.
//
// A failure leaves the branch's numbers alone rather than zeroing them: a count a pass out of date
// beats no count, and zero in these fields reads as an emptied store.
func (l *Loop) reportHeld(ctx context.Context, state *report.State) bool {
	declared, deployed, err := l.declaredSet(ctx)
	if err != nil && l.releaseAbsent(ctx) {
		// The set is read out of the release image, and this store does not hold it: the most
		// incomplete a store can be, not a reason to keep the last report. Kept, it stood: a replica
		// that lost the release's tag went on reporting itself full, needSync stayed down, and nothing
		// ever copied the release back — in an air-gapped cluster, the one place it can come from is
		// the leader, and only through a replication that this report never asked for.
		l.Log.Warn("the store does not hold the release the set is read out of, so this replica is not complete",
			"error", err.Error())
		state.Full = false
		return true
	}
	if err != nil {
		l.Log.Warn("cannot tell what the cluster needs, so the status keeps what this pass copied",
			"error", err.Error())
		return false
	}

	survey, err := l.surveyStore(declared, deployed)
	if err != nil {
		l.Log.Warn("cannot read the store, so the status keeps what this pass copied",
			"error", err.Error())
		return false
	}

	state.VerifiedDigests = survey.Declared
	state.TotalDigests = survey.Total
	// The denominator, from the same reading as the numerator. Taken here rather than computed by the
	// controller because this is where the set is known at all, and remembered for the passes that do
	// not read the store — the same carry-over the size of the store gets, for the same reason.
	state.DeclaredDigests = int32(len(declared))
	if state.DeclaredDigests > 0 {
		l.lastDeclaredDigests.Store(state.DeclaredDigests)
	}

	// And completeness from the same reading, which is the half that actually guards the cluster.
	//
	// The branches decide it from their own copier's report — "everything I set out to copy is
	// accounted for" — and that answer cannot see what the copier never looked at. A pull-through
	// cache writes a manifest's revision link the moment it serves it, so a copier finds the manifest
	// already present and counts it as done while not one of its layers is on disk. Every replica can
	// then report `full` and `safeToDropUpstream: true` over a store from which nothing can be pulled
	// once the upstream is gone.
	//
	// The rule is the one applyCatalogue already used, now applied wherever the numbers come from:
	// every declared digest servable from this store, and the deployed release resolvable by tag.
	// An empty set reads as incomplete, which withholds the transition rather than authorising it on
	// no evidence.
	full := len(declared) > 0 &&
		survey.Declared >= int32(len(declared)) &&
		len(survey.MissingTags) == 0
	if !full && state.Full {
		l.Log.Info("the store does not hold everything the cluster needs, so this replica is not complete",
			"held", survey.Declared, "declared", len(declared), "missing_tags", survey.MissingTags)
	}
	state.Full = full
	return true
}

// Filling reports whether this replica is filling its store right now.
//
// Read by the garbage collection before it fires. A method rather than a channel or a lock held
// across the work: the question is asked once, at one instant, and a stale "yes" merely postpones
// housekeeping by a minute while a stale "no" would only be possible if the fill had already
// finished.
func (l *Loop) Filling() bool { return l.filling.Load() || l.aheadRunning.Load() }

// Complete reports whether this replica held the whole expected set as of its last pass.
func (l *Loop) Complete() bool { return l.complete.Load() }

// OwesCopy reports whether this replica's last pass was one that copies: a fill or a replication.
//
// needSync is one switch for every replica, and it can stay up for a reason this replica can do
// nothing about. An air-gapped leader has no source to fill from — it counts what was pushed — and
// while it is short of the set, needSync stays up for the followers' sake. Read as "a fill is owed
// here", that held its collection back with no end: a store over its budget could not be reclaimed,
// could not take the rest of the set, and could not become full. A collection waits for the copy this
// replica makes, not for one nobody will.
func (l *Loop) OwesCopy() bool { return !l.copiesNothing.Load() }

// PauseWrites keeps this replica from writing to its cache until the returned function is called.
//
// Held by the garbage collection for as long as it runs. The lock is on the writer, not on the
// registry: the registry keeps serving throughout, which is the whole difference between a collection
// and an outage.
func (l *Loop) PauseWrites() func() {
	l.writes.Lock()
	return l.writes.Unlock
}

// Desired is the last desired state this loop read, or nil before the first pass.
//
// Exposed rather than re-read because the two readers must not disagree: a collection that
// rendered a configuration from a newer spec than the loop is applying would be undone by the
// loop's next pass, and the replica would flip between them.
func (l *Loop) Desired() *registryv1alpha1.RegistryStorageSpec { return l.desired.Load() }

// LocalOptions is how to talk to this replica's own registry.
func (l *Loop) LocalOptions() []remote.Option {
	registry, err := l.localRegistry()
	if err != nil {
		return nil
	}
	return registry.Options
}

// PublishCollection records the outcome of a garbage collection in this replica's status.
//
// Written whatever the outcome, because "it has not collected since Tuesday" is the useful
// fact and it is invisible if only successes are recorded.
func (l *Loop) PublishCollection(ctx context.Context, finished time.Time, failure error) error {
	if l.Publisher == nil {
		return nil
	}

	state := report.State{
		Node:            l.Node,
		Role:            Role(l.Leadership != nil && l.Leadership.IsLeader()),
		Address:         l.ReportedAddress,
		CollectedAt:     &finished,
		CollectionError: errorText(failure),
	}
	return l.Publisher.PublishCollection(ctx, state)
}

// progressEvery is how often a pass under way reports how far it has got, after the first report.
const progressEvery = 30 * time.Second

// progress is the callback a copier reports through: logged at every reference, and published — see
// report.PublishProgress — the first time and then at most every progressEvery.
//
// The first report is the one that matters most. It comes with the first reference copied or found —
// never before, since the copier calls this after every reference, failed ones included — which means
// the set could be read and the destination written: whatever failed the previous pass is behind this
// one, and the error it left in the status stops holding the storage at Failed.
func (l *Loop) progress(ctx context.Context, state report.State, what string) func(done, total int32) {
	var published time.Time
	return func(done, total int32) {
		l.Log.Debug(what+" progress", "done", done, "total", total)

		// Nothing copied or found yet, and publishing takes the previous pass's error back — on a
		// leader whose writes all fail, for the whole of every pass, which kept the storage out of
		// Failed and the leader looking healthy to the election. The error goes once something lands.
		if done == 0 {
			return
		}
		if l.Publisher == nil || (!published.IsZero() && time.Since(published) < progressEvery) {
			return
		}
		published = time.Now()

		update := report.State{
			Node: state.Node, Role: state.Role, Address: state.Address,
			VerifiedDigests: done, DeclaredDigests: total,
		}
		if err := l.Publisher.PublishProgress(ctx, update); err != nil && ctx.Err() == nil {
			l.Log.Warn("reporting the "+what+" progress failed", "error", err.Error())
		}
	}
}

// storeEvery is how often the store is measured and reported apart from the passes. A pass can last
// an hour; a store filling at the speed of the node's network needs to be seen sooner than that.
const storeEvery = time.Minute

// watchStore reports the store's measurement on its own schedule, whatever the pass is doing.
//
// Only the store field is written — see report.PublishStore — so a fill in progress, a collection
// and this never erase each other. The registry enforces the limits either way: this is what the
// alerts and the StoreWritable condition see.
func (l *Loop) watchStore(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		spec := l.desired.Load()
		if spec == nil || l.Publisher == nil {
			continue
		}
		usage := l.measureStore(spec)
		if usage == nil {
			continue
		}
		state := report.State{
			Node:    l.Node,
			Role:    Role(l.Leadership != nil && l.Leadership.IsLeader()),
			Address: l.ReportedAddress,
			Store:   usage,
		}
		if err := l.Publisher.PublishStore(ctx, state); err != nil && ctx.Err() == nil {
			l.Log.Warn("reporting the store failed", "error", err.Error())
		}
	}
}

// measureStore reads how much of the node's disk the store takes. Nil when it cannot be read, and
// the last measurement stays in the status: the registry enforces the limits whether or not this
// succeeds, so a failed measurement is worth a warning and nothing more.
func (l *Loop) measureStore(spec *registryv1alpha1.RegistryStorageSpec) *registryv1alpha1.StoreUsage {
	if l.DataDir == "" {
		return nil
	}
	var capacity int64
	if l.Applier != nil {
		capacity = l.Applier.Options.StoreCapacity
	}
	usage, err := distribution.MeasureStore(l.DataDir, spec, capacity)
	if err != nil {
		l.Log.Warn("measuring the store failed", "error", err.Error())
		return nil
	}
	return usage
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// observeLeadership records whether this replica holds the lease.
//
// Per replica rather than as one cluster-wide gauge, because the interesting failures are
// disagreements — two replicas both believing they lead, or none of them doing so — and a
// single gauge could express neither.
func (l *Loop) observeLeadership(isLeader bool) {
	if l.Metrics == nil {
		return
	}

	if isLeader {
		l.Metrics.Leader.Set(1)
		return
	}
	l.Metrics.Leader.Set(0)
}

// observePass records what a pass did, from the same state that is reported to the API.
//
// Taken from the report rather than counted separately, so the metric and the status cannot
// disagree about the same pass — which is the failure mode of every parallel accounting.
func (l *Loop) observePass(action string, started time.Time, state *report.State) {
	if l.Metrics == nil {
		return
	}

	result := metrics.ResultSuccess
	if state.Error != "" {
		result = metrics.ResultFailure
	}
	l.Metrics.Passes.WithLabelValues(action, result).Inc()
	l.Metrics.FillDuration.Observe(time.Since(started).Seconds())

	if state.VerifiedDigests > 0 {
		l.Metrics.References.WithLabelValues(metrics.OutcomeWritten).Add(float64(state.VerifiedDigests))
	}
}

// Run drives the loop until the context is cancelled.
func (l *Loop) Run(ctx context.Context) error {
	interval := l.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}

	go l.watchStore(ctx, storeEvery)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	l.started = time.Now()
	for {
		if err := l.once(ctx); err != nil {
			// Never fatal. A replica that exits on an error stops serving images, which
			// is worse than a replica that keeps serving stale content and says so.
			l.Log.Error("the pass failed", "error", err.Error())
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (l *Loop) once(ctx context.Context) error {
	spec, err := l.desiredState(ctx)
	if err != nil {
		return err
	}

	// Remembered for the garbage collection, which runs on its own schedule and must act on
	// the same instruction this loop is applying rather than read its own.
	l.desired.Store(spec)
	if spec == nil {
		// No storage object: the cache is off, or the controller has not created it
		// yet. Nothing to configure and nothing to report.
		return nil
	}

	// The configuration is applied before anything else, and regardless of role.
	// This is the path a credential change takes while the Deckhouse operator is
	// down, and it must not wait behind a fill.
	if changed, err := l.Applier.Apply(spec); err != nil {
		return fmt.Errorf("applying the registry configuration: %w", err)
	} else if changed {
		l.Log.Info("the registry configuration changed and the process was restarted")
	}

	if l.storeStarting(ctx) {
		return nil
	}

	isLeader := l.Leadership.IsLeader()
	// The address is reported by the replica itself, so a follower can replicate from
	// the leader without resolving a node name and without reading Node objects.
	state := report.State{Node: l.Node, Role: Role(isLeader), Address: l.ReportedAddress}
	stated := ExpectedDigests(spec)

	leader, err := l.currentLeader(ctx)
	if err != nil {
		return err
	}

	l.observeLeadership(isLeader)

	action := Decide(spec, isLeader, leader)

	// Which set a full report would be about. Every replica says, so that whoever decides who may
	// lead compares reports about the same set — see MayLead. Only the leader with an upstream acts
	// on a change of it — see Reverify.
	var setKey, aheadKey string
	keysKnown := false
	if set, ahead, err := l.keys(ctx); err != nil {
		l.Log.Warn("cannot tell which set the cluster needs, so this pass reports no set with its count",
			"error", err.Error())
	} else {
		setKey, aheadKey, keysKnown = set, ahead, true
	}
	reported, reportedSource, reportedAhead := l.reportedSets(ctx)

	// A copy ahead started under another arrangement reads from where this replica no longer should: an
	// upstream the cluster has dropped or whose account has changed, a leader that is not the leader
	// any more, or the upstream of a replica that now follows.
	l.stopAheadOutside(aheadScope(spec, isLeader, leader))
	reverified, withdraw := Reverify(action, isLeader, spec, reported, setKey)
	if withdraw {
		l.Log.Info("the set the cluster needs changed since the store was found full; " +
			"withdrawing that and verifying the store against the new one")
		if err := l.Publisher.Withdraw(ctx, state); err != nil {
			// Not verifying on a Full that could not be withdrawn would leave it standing; verifying
			// anyway at least replaces it at the end of the pass.
			l.Log.Warn("withdrawing the full report failed", "error", err.Error())
		}
		// Withdrawn, so not to be put back: a pass that ends up doing nothing — its registry not listening
		// — would otherwise have its store check restamp the very set this pass took away.
		reported = ""
	}
	action = reverified

	// needSync is one switch for every replica, and it stays up until the last of them holds the set.
	// A replica that already does — found full against this very set on its last pass — has nothing
	// to copy while it waits for the others, and a pass would only confirm that, at the price of
	// enumerating the set (the platform image read to its end, per version) and asking for every
	// manifest of it.
	if AlreadyHeld(action, setKey, reported, reportedSource, leader) {
		action = ActionNone
	}
	// Decided here, before a registry that is not listening turns the pass into nothing: that copy is
	// still owed, only postponed.
	copies := action == ActionFill || action == ActionReplicate
	l.copiesNothing.Store(!copies)
	if !copies {
		// A silence is timed across the passes that wanted the registry and could not have it. One that
		// wanted nothing ends the series: left standing, a silence days old made the next ordinary
		// restart of the registry an error at once, without the grace it is given.
		l.registryDownSince.Store(0)
	}

	// The fill and the replication go through this replica's own registry, which is not listening
	// for a while after it restarts: the configuration applied above may have just restarted it, and
	// the wrapper walks the whole store before it opens a port. A pass started in that window failed
	// every reference on "connection refused" and published it as the replica's error — on a fresh
	// cluster, "432 of 432 references could not be copied" at the very start of the fill. So the pass
	// waits for the registry, which covers a restart. One that stays down past that is reported as
	// what it is, once, and the copy is left to the next pass; the rest of the pass still runs.
	if action == ActionFill || action == ActionReplicate {
		// The set first: a copy ahead in progress gives way to a pass that has to copy it, and is
		// resumed once the store is complete again.
		l.stopAhead()
		if err := l.awaitLocalRegistry(ctx); err != nil {
			l.Log.Warn("this replica's registry is not listening, so the copy waits for the next pass",
				"error", err.Error())
			action = ActionNone
			// Reported only once it has been down for a while. The wrapper walks the whole store before it
			// opens a port, after every restart the configuration causes, and on a large store that takes
			// longer than one wait: reported at once, every such restart turned the storage Failed.
			since := l.registryDownSince.Load()
			if since == 0 {
				since = time.Now().UnixNano()
				l.registryDownSince.Store(since)
			}
			if time.Since(time.Unix(0, since)) >= registryDownReported {
				state.Error = fmt.Sprintf("this replica's registry is not listening: %v", err)
			}
		} else {
			l.registryDownSince.Store(0)
		}
	}

	// What is ahead of the set, copied once this pass has said what it found — see copyAhead.
	var ahead *fill.Copier

	switch action {
	case ActionFill:
		started := time.Now()
		l.filling.Store(true)
		l.announceStart(ctx, state, stated)
		func() {
			// Waits out a collection rather than racing it. The other direction is guarded too:
			// a collection does not start while a fill is owed — see gc.Plan.FillPending.
			done := l.PauseWrites()
			defer done()
			ahead = l.applyFill(ctx, spec, stated, &state)
		}()
		l.filling.Store(false)
		// What the fill moved is logged; what the replica HOLDS of the declared set is published.
		l.reportHeld(ctx, &state)
		l.observePass(string(action), started, &state)
	case ActionReplicate:
		started := time.Now()
		l.filling.Store(true)
		l.announceStart(ctx, state, stated)
		func() {
			done := l.PauseWrites()
			defer done()
			ahead = l.applyReplicate(ctx, leader, &state)
		}()
		l.filling.Store(false)
		// The same correction as above, and the branch it matters most on: a follower enumerates
		// through the leader's store, which holds more than the set.
		l.reportHeld(ctx, &state)
		l.observePass(string(action), started, &state)
	case ActionCountCatalogue:
		l.applyCatalogue(ctx, &state)
	case ActionNone:
		// Keep whatever was reported last: this pass learned nothing new, and
		// overwriting the count with a zero would look like the storage emptied.
		if err := l.carryOverCount(ctx, &state); err != nil {
			return err
		}
		l.recheckStore(ctx, &state, reported)
	}

	// While the write endpoint is open, the store has the last word on how much it
	// holds — whatever the pass above managed to do.
	if StoreIsAuthority(spec, isLeader) {
		l.recountFromStore(ctx, &state)
	}

	// Full, and about which set: the one read before the pass. Had it moved during the pass, the next
	// pass sees a different fingerprint. A pass that did nothing carried the last report over, set
	// included, and says nothing new about either.
	if action != ActionNone {
		state.VerifiedSet = ""
		if state.Full && state.Error == "" {
			state.VerifiedSet = setKey
		}
	}

	// TotalDigests is NOT recomputed here, and that is the correction.
	//
	// It used to be, on every pass, so that a field filled in by one code path would not read as "the
	// store emptied" on the others. The intent was right and the cost was not: counting walks the whole
	// store, the loop runs every 30 seconds, and the process ends up spending most of a CPU on it.
	//
	// So it is answered where the store is already being read — see applyCatalogue — and carried over
	// otherwise. Carrying over is what keeps the original concern addressed: the status keeps the last
	// number that was actually measured instead of dropping to zero on a pass that did not look.
	if state.TotalDigests == 0 {
		// Nothing measured it on this pass — the fill and replication paths do not read the store — so
		// ask, at most once a minute, and fall back to the last number if even that is refused.
		//
		// Needed because moving the count into applyCatalogue alone empties the field everywhere else,
		// leaving a verified count beside an absent total — precisely the "reads as an emptied store"
		// the field was introduced to prevent. The survey is shared with the catalogue path and cached
		// the same way, so this costs one walk a minute at worst, not one per pass.
		if survey, err := l.surveyStore(nil, ""); err == nil && survey.Total > 0 {
			state.TotalDigests = survey.Total
		} else {
			state.TotalDigests = l.lastTotalDigests.Load()
		}
	}
	if state.TotalDigests > 0 {
		l.lastTotalDigests.Store(state.TotalDigests)
	}

	// And the denominator, which has the same shape of problem and one wrinkle of its own: memory is
	// empty in a freshly started process, and on a cluster with an upstream and no air-gap declaration
	// NOTHING on a steady-state pass computes the set — not the fill path, which knows only what it
	// wrote, and not the survey above, which is asked without a set on purpose. Without this,
	// `totalDigests` climbs pass after pass while `declaredDigests` stays absent, and the status still
	// shows no progress at all.
	//
	// So it is asked for, and only while it is unknown: one API read on the passes before the first
	// answer, and nothing afterwards. The store is not walked again for it — the size of the set comes
	// from what the cluster declares, not from the disk.
	if state.DeclaredDigests == 0 {
		if declared, _, err := l.declaredSet(ctx); err == nil && len(declared) > 0 {
			state.DeclaredDigests = int32(len(declared))
		} else {
			state.DeclaredDigests = l.lastDeclaredDigests.Load()
		}
	}
	if state.DeclaredDigests > 0 {
		l.lastDeclaredDigests.Store(state.DeclaredDigests)
	}

	// Remembered for the garbage collection, which asks on its own schedule: an incomplete store
	// must not be collected, and the loop is the one that knows.
	l.complete.Store(state.Full)

	// Measured last, after whatever the pass wrote: a figure taken at the start of an hour-long
	// fill would, published at its end, put back what the store watcher had long since replaced.
	state.Store = l.measureStore(spec)

	// What is ahead of the set, held as of the last copy that went through — or, unknown this pass, as
	// reported last.
	state.AheadSet = reportedAhead
	if keysKnown {
		state.AheadSet = l.heldAhead(aheadKey, reportedAhead)
	}

	err = l.Publisher.Publish(ctx, state)
	if keysKnown {
		l.considerAhead(ctx, spec, isLeader, leader, aheadKey, state, ahead)
	}
	return err
}

// aheadRetry is how soon a copy ahead that missed something is tried again for the same updates.
const aheadRetry = 10 * time.Minute

// aheadRoom is the share of the store a copy ahead leaves free: none starts into a store fuller than
// that.
const aheadRoom = 5 // one fifth

// considerAhead starts a copy of what is ahead of the set — the next platform release, the pending
// module versions — in the background, if this replica does not hold it yet and can get it.
//
// In the background because it is background work: run inside the pass, at the background pace and
// through an upstream that may be slow to answer, it held the next pass back — on a harbor stand two
// minutes once and four and a half once more, chasing one reference the cold mirror did not have, while
// the set had grown and waited to be copied. The cluster converges on the set, not on what is ahead of
// it, and converged a quarter of an hour later than the same run on main.
//
// Only from a store complete for the set: until it is, the next pass copies the set, and that is what
// the cluster is waiting for. Held is decided by the copy itself — see AheadKey — so a copy stopped by
// a pass that copies the set, cut short by a restart, or missing what its source does not have yet
// leaves it unheld, and an idle pass starts it again; one that missed something waits aheadRetry
// first, so as not to read a whole release every half a minute for an image nobody has published.
//
// Where it is copied from is the replica's own business. The leader with an upstream copies from it.
// An air-gapped leader has nothing to copy from — what is ahead was pushed onto it, or was not — so it
// checks its own store, and holds what it finds. A follower copies from a leader that holds the same
// updates, and only them: copying a part the leader is short of would only miss what the leader misses.
//
// Part by part — see AheadKey. A replica copies the parts it lacks, a follower those of them its leader
// holds, and a part missed stays missed alone: one module whose source cannot be read no longer keeps
// the platform release beside it off every follower.
func (l *Loop) considerAhead(
	ctx context.Context, spec *registryv1alpha1.RegistryStorageSpec, isLeader bool, leader *Leader,
	current string, state report.State, passCopier *fill.Copier,
) {
	if current == "" || !state.Full || state.Error != "" {
		return
	}
	held := splitParts(state.AheadSet)
	var copyable map[string]bool
	if !isLeader {
		if leader == nil {
			return
		}
		copyable = splitParts(leader.AheadSet)
	}
	var want []string
	for part := range splitParts(current) {
		if !held[part] && (copyable == nil || copyable[part]) {
			want = append(want, part)
		}
	}
	if len(want) == 0 {
		return
	}
	key := joinParts(want)
	wanted := splitParts(key)

	l.aheadMu.Lock()
	running := l.aheadCancel != nil
	missedLately := l.aheadMissedFor == key && time.Since(l.aheadMissedAt) < aheadRetry
	l.aheadMu.Unlock()
	if running || missedLately {
		return
	}

	checking := isLeader && spec.Upstream == nil
	if !checking && !roomAhead(state.Store) {
		l.Log.Info("the store is too full to copy ahead of the set, so the updates are left for when they "+
			"are deployed", "retry_in", aheadRetry.String())
		l.missedAhead(key)
		return
	}

	copier, run := passCopier, (*fill.Copier).RunAhead
	if copier == nil {
		built, err := l.aheadCopier(ctx, spec, isLeader, leader)
		if err != nil {
			l.Log.Warn("cannot copy ahead of the set", "error", err.Error(), "retry_in", aheadRetry.String())
			l.missedAhead(key)
			return
		}
		copier, run = built, (*fill.Copier).CopyAhead
	}
	copier.AheadWant = func(part string) bool { return wanted[part] }
	if !checking {
		copier.AheadRoom = func() bool { return roomAhead(l.measureStore(spec)) }
	}
	l.startAhead(ctx, func(ctx context.Context) fill.Report { return run(copier, ctx) }, key,
		aheadScope(spec, isLeader, leader))
}

// roomAhead reports whether a store has room for what is ahead of the set: writable, and with a fifth
// of its budget — or of its filesystem, without one — still free. Unmeasured is room: the store refuses
// writes itself when it has none, and the copy ahead stops at the first refusal.
//
// The set comes first, and the room it may need is the room a copy ahead would take. A store filled
// ahead to its last byte refused the next change of the set — a patch, a module enabled — and the
// storage went Failed with an upstream that could not be dropped, over updates nothing had deployed.
func roomAhead(store *registryv1alpha1.StoreUsage) bool {
	if store == nil {
		return true
	}
	if !store.Writable {
		return false
	}
	room := store.FilesystemFreeBytes - store.ReserveBytes
	whole := store.FilesystemCapacityBytes
	if store.BudgetBytes > 0 {
		room = min(room, store.BudgetBytes-store.UsedBytes)
		whole = store.BudgetBytes
	}
	return whole <= 0 || room >= whole/aheadRoom
}

// missedAhead records a copy ahead for key that did not go through, so that the next waits aheadRetry.
func (l *Loop) missedAhead(key string) {
	l.aheadMu.Lock()
	defer l.aheadMu.Unlock()
	l.aheadMissedFor, l.aheadMissedAt = key, time.Now()
}

// heldAhead is the parts of what is ahead of the set this replica holds, of the current ones: those a
// copy here left whole, and those it reported before a restart. Parts no longer pending fall away.
func (l *Loop) heldAhead(current, reported string) string {
	parts := splitParts(current)
	l.aheadMu.Lock()
	defer l.aheadMu.Unlock()
	for part := range l.aheadHeld {
		if !parts[part] {
			delete(l.aheadHeld, part)
		}
	}
	if l.aheadForgotten {
		reported = ""
	}
	before := splitParts(reported)
	var held []string
	for part := range parts {
		if l.aheadHeld[part] || before[part] {
			held = append(held, part)
		}
	}
	return joinParts(held)
}

// ForgetAhead drops what this replica holds ahead of the set: the collection has taken it back — see
// gc.Collector.ReleaseAhead — and saying otherwise would send its followers to copy what it no longer
// has. Copied again, part by part, once the store has room.
func (l *Loop) ForgetAhead() {
	l.aheadMu.Lock()
	defer l.aheadMu.Unlock()
	l.aheadHeld = nil
	l.aheadForgotten = true
}

// RoomAhead reports whether this replica's store has room for what is ahead of the set — see roomAhead.
func (l *Loop) RoomAhead() bool {
	spec := l.Desired()
	if spec == nil {
		return true
	}
	return roomAhead(l.measureStore(spec))
}

// aheadScope is what a copy ahead is started under: the role, and where it reads from — the leader it
// follows, or the upstream with its account and authority.
func aheadScope(spec *registryv1alpha1.RegistryStorageSpec, isLeader bool, leader *Leader) string {
	if !isLeader {
		if leader == nil {
			return "follower"
		}
		return "follower of " + leader.Node + " at " + leader.Address
	}
	if spec == nil || spec.Upstream == nil {
		return "leader"
	}
	encoded, _ := json.Marshal(spec.Upstream)
	sum := sha256.Sum256(encoded)
	return "leader of " + hex.EncodeToString(sum[:8])
}

// aheadCopier builds a copy of what is ahead of the set for a replica that copies nothing else this
// pass. See considerAhead for where each role copies it from.
func (l *Loop) aheadCopier(
	ctx context.Context, spec *registryv1alpha1.RegistryStorageSpec, isLeader bool, leader *Leader,
) (*fill.Copier, error) {
	releases, err := gc.FromCluster(ctx, l.Client)
	if err != nil {
		return nil, err
	}
	modules, err := gc.ModulesFromCluster(ctx, l.Client)
	if err != nil {
		return nil, err
	}
	if isLeader && spec.Upstream != nil {
		return l.copier(spec, releases, modules)
	}

	destination, err := l.writeRegistry()
	if err != nil {
		return nil, err
	}
	discover := fill.Release{
		Versions: versionsOf(releases), Pending: releases.Pending, Modules: modules,
		CatalogueUnavailable: func(err error) {
			l.Log.Warn("the module catalogue cannot be listed, so an air-gapped cluster will not be "+
				"able to enumerate modules", "error", err.Error())
		},
	}
	if isLeader {
		// Read from the store it is checked into: what is there is skipped as present, and what is not
		// is a miss, absent at a source that is the store itself.
		return &fill.Copier{Source: destination, Destination: destination, Discover: discover, StoreDir: l.dataDir()}, nil
	}

	local, err := l.localRegistry()
	if err != nil {
		return nil, err
	}
	ahead := local
	ahead.Address = leader.Address
	if ahead.Options, err = fill.PacedRegistryOptions(l.LocalCA, l.LocalUser, l.LocalPassword, l.CopyPacer); err != nil {
		return nil, err
	}
	return &fill.Copier{Source: ahead, Ahead: &ahead, Destination: destination, Discover: discover, StoreDir: l.dataDir()}, nil
}

// startAhead runs a copy ahead for key in the background.
func (l *Loop) startAhead(ctx context.Context, run func(context.Context) fill.Report, key, scope string) {
	l.stopAhead()

	l.aheadMu.Lock()
	defer l.aheadMu.Unlock()
	aheadCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	l.aheadCancel, l.aheadDone, l.aheadScope = cancel, done, scope
	l.aheadRunning.Store(true)

	go func() {
		defer close(done)
		defer l.aheadRunning.Store(false)

		release := l.PauseWrites()
		report := run(aheadCtx)
		release()

		stopped := aheadCtx.Err() != nil
		switch {
		case len(report.AheadMissed) > 0 && !stopped:
			// Said with what was missed: a release that never arrives looks, from outside, like one
			// that was never asked for.
			l.Log.Warn("copied ahead of the set, with some of it missing", "copied", report.AheadCopied,
				"missed", len(report.AheadMissed), "first_missed", report.AheadMissed[0],
				"retry_in", aheadRetry.String())
		case report.AheadCopied > 0 || stopped:
			l.Log.Info("copied ahead of the set", "copied", report.AheadCopied, "stopped", stopped)
		}

		l.aheadMu.Lock()
		defer l.aheadMu.Unlock()
		if l.aheadDone == done {
			l.aheadCancel, l.aheadDone, l.aheadScope = nil, nil, ""
		}
		// Whole is whole, stopped or not; what was missed waits aheadRetry, unless it was only stopped.
		if l.aheadHeld == nil {
			l.aheadHeld = map[string]bool{}
		}
		for _, part := range report.AheadHeld {
			l.aheadHeld[part] = true
		}
		if !stopped && len(report.AheadMissed) > 0 {
			l.aheadMissedFor, l.aheadMissedAt = key, time.Now()
		}
		cancel()
	}()
}

// stopAhead stops a copy ahead in progress and waits until it has.
func (l *Loop) stopAhead() {
	l.aheadMu.Lock()
	cancel, done := l.aheadCancel, l.aheadDone
	l.aheadMu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	<-done
}

// stopAheadOutside stops a copy ahead started under a scope other than this one.
func (l *Loop) stopAheadOutside(scope string) {
	l.aheadMu.Lock()
	other := l.aheadCancel != nil && l.aheadScope != scope
	l.aheadMu.Unlock()
	if other {
		l.stopAhead()
	}
}

// waitAhead waits for a copy ahead in progress to finish on its own.
func (l *Loop) waitAhead() {
	l.aheadMu.Lock()
	done := l.aheadDone
	l.aheadMu.Unlock()
	if done != nil {
		<-done
	}
}

// recountFromStore replaces this pass's accounting with what the storage actually
// holds.
//
// The fill's complaint is logged rather than reported, and that is the point of
// the exercise: it travels in the same field the transition is vetoed by — see
// LeaderFull — so a fill that could not run at all would block a transition whose
// evidence no longer depends on it. A cluster installed from a tag rather than a
// release channel has no DeckhouseRelease to enumerate, and reported exactly
// that: "no release is deployed, so there is nothing to judge the store against",
// every thirty seconds, while the pushed bundle sat in the store unaccounted for.
//
// Failing to read the store is a different matter and does reach the status:
// then there is no evidence, and no evidence is not permission.
func (l *Loop) recountFromStore(ctx context.Context, state *report.State) {
	if state.Error != "" {
		l.Log.Warn("the fill did not finish and the write endpoint is open, "+
			"so completeness is judged by what the store holds",
			"error", state.Error)
		state.Error = ""
	}

	l.applyCatalogue(ctx, state)
}

func (l *Loop) applyFill(
	ctx context.Context, spec *registryv1alpha1.RegistryStorageSpec, stated int32, state *report.State,
) *fill.Copier {
	// What the cluster runs, and what it ran before it.
	//
	// The same pair the collector keeps, so that the fill puts in what the collector would
	// not throw away — and read here rather than passed in, because it changes underneath a
	// long-lived loop every time the cluster is updated.
	releases, err := gc.FromCluster(ctx, l.Client)
	if err != nil {
		// Said in the log as well as the status, because this is the reason a fill does
		// nothing at all, and a status field is not where anybody looks first.
		l.Log.Error("cannot tell what the cluster runs, so there is nothing to fill towards",
			"error", err.Error())
		state.Error = err.Error()
		return nil
	}

	modules, err := gc.ModulesFromCluster(ctx, l.Client)
	if err != nil {
		state.Error = err.Error()
		return nil
	}

	copier, err := l.copier(spec, releases, modules)
	if err != nil {
		state.Error = err.Error()
		return nil
	}
	copier.OnProgress = l.progress(ctx, *state, "fill")
	copier.SeparateAhead = true

	// `stated` is logged and nothing more. It is what an operator counted in a bundle, and by the
	// owner's rule it may not enter the decision — the fill is judged against the set the run
	// enumerates, which is what the cluster needs. Kept in the log because the two numbers differing is
	// worth seeing when a store looks smaller than somebody expected.
	l.Log.Info("filling the storage from the upstream",
		"upstream", spec.Upstream.Host, "deployed", releases.Deployed,
		"previous", releases.Previous, "stated", stated)

	result, err := copier.Run(ctx)
	if err != nil {
		state.Error = err.Error()
		state.VerifiedDigests = result.Written
		return nil
	}

	state.VerifiedDigests = result.Written
	state.Full = result.Complete()
	if len(result.Failed) > 0 {
		// A partial fill is reported as a failure with the count it did reach: the
		// cache is still useful, and the nodes fall back to the upstream for the rest.
		state.Error = fmt.Sprintf("%d of %d references could not be copied, the first being %s",
			len(result.Failed), result.Total, result.Failed[0])
	}

	l.Log.Info("the fill finished",
		"written", result.Written, "skipped", result.Skipped, "failed", len(result.Failed), "full", state.Full)
	return copier
}

// applyReplicate copies the leader's content into this replica.
//
// The read credentials are the same on every replica, so a follower reaches the
// leader with what it already has: no separate replication identity to distribute,
// and nothing that grants more than a pull.
func (l *Loop) applyReplicate(
	ctx context.Context, leader *Leader, state *report.State,
) *fill.Copier {
	local, err := l.localRegistry()
	if err != nil {
		state.Error = err.Error()
		return nil
	}

	source := local
	source.Address = leader.Address
	// Not paced, like the leader's own fill of the set, and for the same reason one step further on: a
	// follower without the set is a replica the cluster cannot fall back on, and in an air-gapped
	// cluster there is nothing else to fall back on. Paced, two followers of a fresh dhctl-airgap
	// cluster took half an hour to copy 13 GB from their leader, where the scenario — and main, which
	// never paced it — expects minutes. What is ahead of the set is paced, as on the leader.
	if source.Options, err = fill.RegistryOptions(l.LocalCA, l.LocalUser, l.LocalPassword); err != nil {
		state.Error = err.Error()
		return nil
	}
	ahead := source
	if ahead.Options, err = fill.PacedRegistryOptions(l.LocalCA, l.LocalUser, l.LocalPassword, l.CopyPacer); err != nil {
		state.Error = err.Error()
		return nil
	}

	// Written through the instance that does not proxy — see writeRegistry. A follower filling
	// through the serving one skips every layer for the same reason a leader does.
	destination, err := l.writeRegistry()
	if err != nil {
		state.Error = err.Error()
		return nil
	}

	// What to copy is read out of the releases, exactly as the leader's own fill reads it, and not
	// by asking the leader what it holds.
	//
	// Asking looks right — the leader is one of our own replicas, and listing it is permitted — and
	// it was what this did. It is unsound for a reason that has nothing to do with permission: while
	// an upstream is configured the leader serves as a pass-through cache, so a listing is answered
	// with the UPSTREAM's contents. A follower would then set out to copy every tag of the upstream
	// through the leader — thousands of development builds — instead of the expected set, and it
	// would do it at the leader's expense.
	//
	// The same versions the leader fills towards give the same set by construction, which is what a
	// takeover needs: the installer image that declares the set is itself part of the set, so it is
	// in the leader's store and is read from there.
	releases, err := gc.FromCluster(ctx, l.Client)
	if err != nil {
		l.Log.Error("cannot tell what the cluster runs, so there is nothing to replicate towards",
			"error", err.Error())
		state.Error = err.Error()
		return nil
	}

	versions := versionsOf(releases)

	// The same modules the count and the fill enumerate: a follower that copied less than the leader
	// holds would report itself incomplete forever, and one that copied more would look for images
	// nobody published.
	// Origins are not resolved here: a follower reads every module from the leader's store, where the
	// leader keeps them all under the same path whatever their source.
	modules, err := gc.ModulesFromCluster(ctx, l.Client)
	if err != nil {
		state.Error = err.Error()
		return nil
	}

	l.Log.Info("replicating from the leader",
		"leader", leader.Node, "address", leader.Address,
		"deployed", releases.Deployed, "previous", releases.Previous)

	copier := &fill.Copier{
		Source:      source,
		Destination: destination,
		Discover: fill.Release{
			Versions: versions, Pending: releases.Pending, Modules: modules,
			CatalogueUnavailable: func(err error) {
				l.Log.Warn("the module catalogue cannot be listed, so an air-gapped cluster will not be "+
					"able to enumerate modules", "error", err.Error())
			},
		},
		// So that an image the store holds without its layers is copied rather than skipped: the
		// registry answers "present" for those, and a follower would otherwise never repair itself.
		StoreDir: l.dataDir(),
		// No source in the progress. It used to name the leader from the first reference on, and an
		// entry already full for the set with the new leader as its source reads, to the controller,
		// as a replication that finished: a pass cut short — the StatefulSet rolling — left a follower
		// that never copied from the leader it names. The source goes in with the final report alone.
		OnProgress:    l.progress(ctx, *state, "replication"),
		SeparateAhead: true,
		Ahead:         &ahead,
	}

	result, err := copier.Run(ctx)
	state.Source = leader.Node
	state.VerifiedDigests = result.Written

	if err != nil {
		state.Error = err.Error()
		return nil
	}

	state.Full = result.Complete()

	// What the leader does not hold yet is not this replica's failure, and is deliberately not
	// reported as one: a follower now copies from a leader that is still filling, so a pass in that
	// window legitimately ends with part of the set still to come. Reported as an error it would make
	// a working replica look broken — and, through the eligibility rules, disqualify it from leading.
	if len(result.Failed) > 0 {
		state.Error = fmt.Sprintf("%d of %d references could not be replicated, the first being %s",
			len(result.Failed), result.Total, result.Failed[0])
	}

	l.Log.Info("the replication finished",
		"written", result.Written, "skipped", result.Skipped,
		"pending_on_the_leader", len(result.Pending),
		"failed", len(result.Failed), "full", state.Full)
	return copier
}

// currentLeader reads who to replicate from out of the reports.
//
// Taken from the replica entries rather than from the aggregate `leader` field: the
// entries are what each replica wrote about itself, so a stale or hand-edited
// summary cannot send a follower to copy from a replica that is not actually
// complete.
func (l *Loop) currentLeader(ctx context.Context) (*Leader, error) {
	storage := &registryv1alpha1.RegistryStorage{}
	if err := l.Client.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading the storage state: %w", err)
	}

	for i := range storage.Status.Replicas {
		replica := &storage.Status.Replicas[i]
		if replica.Role != registryv1alpha1.ReplicaRoleLeader || replica.Node == l.Node {
			continue
		}
		return &Leader{
			Node:    replica.Node,
			Address: replica.Address,
			// A replica that both claims completeness and reports a failure is not
			// trustworthy enough to copy a whole set from.
			Full:     replica.Full && replica.Error == "",
			AheadSet: replica.AheadSet,
		}, nil
	}
	return nil, nil
}

// dataDir is where this replica's blobs are, defaulting to where the container mounts them.
func (l *Loop) dataDir() string {
	if l.DataDir != "" {
		return l.DataDir
	}
	return distribution.DataDir
}

// applyCatalogue accounts for how much of the expected set this replica holds, off its own filesystem.
//
// Two things it deliberately does not do. It does not ask the registry — a pass-through cache answers
// "what do you have" with its upstream's contents, and this number decides whether the cluster may be
// cut off from that upstream. And it does not count everything on disk: a store legitimately holds more
// than any release declares, because the cache settles whatever the cluster pulls through it.
//
// Both halves matter because this number has to mean the same thing as the one a copy reports. When the
// two disagree, a replica answers one way after a copying pass and another after a counting one; `full`
// follows whichever ran last and flaps; eligibility follows `full`, so the lease moves; and a fill
// restarts on every move — the lease travels between replicas and no fill ever finishes.
func (l *Loop) applyCatalogue(ctx context.Context, state *report.State) {
	// The same set, from the same cache, as the fill and replication branches publish against — see
	// declaredSet. Read separately here once, which is how the leader and its followers came to
	// answer with different sets while both called themselves full.
	declared, deployed, err := l.declaredSet(ctx)
	if err != nil {
		state.Error = err.Error()
		return
	}

	// One walk of the store for all three answers — see fill.Take — and not on every pass.
	//
	// Three separate readers, every thirty seconds, cost most of a CPU on a master. Merging them into
	// one walk divides that by three; not repeating it while nothing can have changed removes what is
	// left. The store only changes here through a push, so a survey at most once a minute is as current
	// as anything downstream needs: what it gates is the air-gap transition, and delaying that by up to
	// a minute costs nothing that matters.
	//
	// The previous survey is reused rather than the numbers being guessed at: a stale count is a count
	// that was true a minute ago, while a guess is never true.
	survey, err := l.surveyStore(declared, deployed)
	if err != nil {
		state.Error = err.Error()
		return
	}
	held := survey.Declared
	state.TotalDigests = survey.Total
	// The size of the set, from the same survey as the part of it that is present.
	state.DeclaredDigests = int32(len(declared))

	// The set, and nothing but the set: how full the store is of what the cluster needs. What an
	// operator stated in `storage.source.expectedDigests` is not consulted, not even when the set comes
	// out empty — see Report.Complete for the reasoning. An empty set reads as
	// incomplete, which is the safe direction: it withholds the air-gap transition rather than
	// authorising it on no evidence.
	state.VerifiedDigests = held

	// Counting is not enough, and this is the check that was missing.
	//
	// A count says how many manifests are on the disk; it does not say that the release can be RESOLVED
	// from this store. Those come apart, and expensively: a leader can report full while its followers
	// get `MANIFEST_UNKNOWN` for the release tag from it and the agent gets `NAME_UNKNOWN: repository
	// name not known to registry` — a store that authorizes dropping the upstream and then cannot hand
	// the release to anybody. Replication enumerates the set by reading
	// the release BY TAG, so a set without its tag does not propagate, and neither does an update.
	//
	// The deployed version only. The previous one is for a rollback, which is worth having and not worth
	// blocking a transition over, and the installer is not in the set at all — a running cluster never
	// needs it.
	missing := survey.MissingTags

	state.Full = len(declared) > 0 && held >= int32(len(declared)) && len(missing) == 0
	if len(missing) > 0 {
		// Said out loud, because the alternative is a store that counts as complete-but-not-full with
		// nothing to explain why. This is not an error of the pass: the count is honest, the store is
		// simply not usable as a source yet.
		l.Log.Warn("the store holds the set but cannot resolve the release, so it is not complete",
			"missing", strings.Join(missing, ","), "held", held, "declared", len(declared))
	}
}

// versionsOf is the pair the store is kept for: what the cluster runs, and what it would roll back to.
// versionsOf is the platform versions the set is made of. Not the pending ones: those are copied ahead
// of time, outside the set — see fill.Release.Optional.
func versionsOf(releases gc.Releases) []string {
	versions := make([]string, 0, 2)
	for _, version := range []string{releases.Deployed, releases.Previous} {
		if version != "" {
			versions = append(versions, version)
		}
	}
	return versions
}

// carryOverCount keeps this replica's last accounting when the pass did none, so an idle pass does not
// look like the storage emptied.
//
// Carried verbatim — the count AND the verdict — because a pass that learned nothing must not
// contradict the one that did. Recomputing fullness from `expectedDigests`, which most clusters do not
// state, could only answer "not full", so every idle pass erased a completeness a fill had just
// established.
//
// The cycle that produces is worth knowing, because the symptom sits nowhere near the cause: the leader
// reports full, the controller clears `needSync`, the next pass goes idle and erases the fullness, the
// controller asks for a fill again — and since eligibility to lead depends on being full, the lease
// travels with it and the whole thing reads as flapping leader election.
// setKey is the fingerprint of the set the cluster needs now — see SetKey.
func (l *Loop) keys(ctx context.Context) (string, string, error) {
	return CurrentKeys(ctx, l.Client)
}

// CurrentSetKey is the fingerprint of the set the cluster needs now, read from the API server: what
// a replica's Full is compared against, both by the replica reporting it and by every replica
// deciding who may lead.
func CurrentSetKey(ctx context.Context, c client.Client) (string, error) {
	set, _, err := CurrentKeys(ctx, c)
	return set, err
}

// CurrentKeys is CurrentSetKey and, beside it, the AheadKey of what is pending now.
func CurrentKeys(ctx context.Context, c client.Client) (string, string, error) {
	releases, err := gc.FromCluster(ctx, c)
	if err != nil {
		return "", "", err
	}
	modules, err := gc.ModulesFromCluster(ctx, c)
	if err != nil {
		return "", "", err
	}
	catalogue, err := gc.CatalogueFromCluster(ctx, c)
	if err != nil {
		return "", "", err
	}
	return SetKey(releases, modules, catalogue), AheadKey(releases, modules), nil
}

// reportedSets is the fingerprint this replica last reported Full against, read back from the status
// so that it survives a restart, with the replica it was replicated from — and the AheadSet it last
// reported. Empty when it cannot be read, which Reverify and AlreadyHeld treat as "not verified" — the
// one direction that costs a pass rather than a wrong answer.
func (l *Loop) reportedSets(ctx context.Context) (string, string, string) {
	storage := &registryv1alpha1.RegistryStorage{}
	if err := l.Client.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage); err != nil {
		return "", "", ""
	}
	for i := range storage.Status.Replicas {
		replica := &storage.Status.Replicas[i]
		if replica.Node != l.Node {
			continue
		}
		if !replica.Full {
			return "", "", replica.AheadSet
		}
		return replica.VerifiedSet, replica.Source, replica.AheadSet
	}
	return "", "", ""
}

func (l *Loop) carryOverCount(ctx context.Context, state *report.State) error {
	storage := &registryv1alpha1.RegistryStorage{}
	if err := l.Client.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage); err != nil {
		return client.IgnoreNotFound(err)
	}

	for i := range storage.Status.Replicas {
		if storage.Status.Replicas[i].Node != l.Node {
			continue
		}
		state.VerifiedDigests = storage.Status.Replicas[i].VerifiedDigests
		state.DeclaredDigests = storage.Status.Replicas[i].DeclaredDigests
		state.Full = storage.Status.Replicas[i].Full
		state.Source = storage.Status.Replicas[i].Source
		state.VerifiedSet = storage.Status.Replicas[i].VerifiedSet
		return nil
	}
	return nil
}

func (l *Loop) desiredState(ctx context.Context) (*registryv1alpha1.RegistryStorageSpec, error) {
	storage := &registryv1alpha1.RegistryStorage{}
	if err := l.Client.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading the desired storage state: %w", err)
	}

	// The spec names its credentials rather than carrying them, because it is a
	// cluster-scoped object. Resolved here, at the single place the spec is read, so that
	// everything downstream — the registry configuration and the fill copier alike — works
	// with credentials it can actually use.
	if err := l.resolveAuth(ctx, &storage.Spec); err != nil {
		return nil, fmt.Errorf("resolving the credentials the storage spec references: %w", err)
	}

	return &storage.Spec, nil
}

// resolveAuth turns the references in a storage spec into the credentials they name.
func (l *Loop) resolveAuth(
	ctx context.Context, spec *registryv1alpha1.RegistryStorageSpec,
) error {
	auths := spec.ReferencedAuths()
	if len(auths) == 0 {
		return nil
	}

	contents := map[string]map[string][]byte{}
	for _, name := range registryv1alpha1.SecretNames(auths) {
		secret := &corev1.Secret{}
		key := types.NamespacedName{Namespace: l.Namespace, Name: name}
		if err := l.Client.Get(ctx, key, secret); err != nil {
			return fmt.Errorf("reading %s: %w", key, err)
		}
		contents[name] = secret.Data
	}

	return registryv1alpha1.ResolveAuths(auths, contents)
}

func (l *Loop) copier(
	spec *registryv1alpha1.RegistryStorageSpec, releases gc.Releases, modules []fill.ModuleRef,
) (*fill.Copier, error) {
	// Through the canonical reader, which understands both forms the credential arrives in. Reading
	// only the pair is how every fill went out anonymous while the cache, which decodes the combined
	// form, kept serving images: see Auth.BasicCredentials.
	username, password := spec.Upstream.Auth.BasicCredentials()

	// The set is read unpaced and only what is ahead of it through the pacer — see Copier.Ahead. Paced,
	// the set itself kept the nodes on the upstream for as long as the pacer took to copy it: some
	// 12 minutes for 11.8 GB at the default rate, during which a cold proxy-cache upstream failed
	// the pulls and the platform stood still. The harbor unit converged in 36 minutes of its 45 where
	// the same build without the pacer took 27, and on the next run did not converge at all.
	upstreamOptions, err := fill.RegistryOptions(spec.Upstream.CA, username, password)
	if err != nil {
		return nil, fmt.Errorf("upstream: %w", err)
	}
	aheadOptions, err := fill.PacedRegistryOptions(spec.Upstream.CA, username, password, l.CopyPacer)
	if err != nil {
		return nil, fmt.Errorf("upstream: %w", err)
	}

	// The destination is the non-proxying instance — see writeRegistry. Through the serving one a
	// fill writes manifests and no layers at all, because the proxy answers "already have that blob"
	// by fetching it from the upstream.
	local, err := l.writeRegistry()
	if err != nil {
		return nil, err
	}

	versions := versionsOf(releases)

	upstream := fill.Registry{
		Address:    spec.Upstream.Host,
		Repository: trimSlashes(spec.Upstream.Path),
		Options:    upstreamOptions,
	}

	ahead := upstream
	ahead.Options = aheadOptions

	// Where each module is read from: the upstream, its own ModuleSource's registry, or — for a source
	// already pointed at this store — the store itself, through the listener that does not proxy, so
	// that what is checked is what the store holds.
	//
	// The modules the cluster runs strictly: one whose source cannot be read is a set that cannot be
	// copied. The updates it is about to apply one at a time and paced, as everything ahead of the set
	// is: a source that cannot be read for an update leaves that update out of the copy ahead — which
	// then misses it, and is tried again — instead of failing the copy of the set the cluster runs.
	var kept, pending []fill.ModuleRef
	for _, module := range modules {
		if module.Pending {
			pending = append(pending, module)
		} else {
			kept = append(kept, module)
		}
	}
	if kept, err = fill.ResolveOrigins(kept, upstream, local); err != nil {
		return nil, err
	}
	var updates []fill.ModuleRef
	var unread []string
	for _, module := range pending {
		resolved, err := fill.ResolvePacedOrigins([]fill.ModuleRef{module}, ahead, local, l.CopyPacer)
		if err != nil {
			l.Log.Warn("the source of a pending module update cannot be read, so it is not copied ahead",
				"module", module.Name, "version", module.Version, "error", err.Error())
			unread = append(unread, module.Name+"@"+module.Version+": "+err.Error())
			continue
		}
		updates = append(updates, resolved...)
	}
	modules = append(append([]fill.ModuleRef{}, kept...), updates...)

	return &fill.Copier{
		Source:      upstream,
		Ahead:       &ahead,
		Routes:      fill.Routes(kept, local),
		AheadRoutes: fill.Routes(updates, local),
		AheadUnread: unread,
		Destination: local,
		// Read out of the releases themselves rather than by listing the upstream. Listing
		// somebody else's registry is a privilege of its own, which credentials scoped to
		// pulling — all a license grants — are refused for; and "everything the upstream
		// holds" is not a set a cache can be complete with respect to anyway.
		Discover: fill.Release{
			Versions: versions, Pending: releases.Pending, Modules: modules,
			CatalogueUnavailable: func(err error) {
				l.Log.Warn("the module catalogue cannot be listed, so an air-gapped cluster will not be "+
					"able to enumerate modules", "error", err.Error())
			},
		},
		// The same repair the replication path needs: a store filled by proxying holds manifests
		// without layers, and the registry reports those as present.
		StoreDir: l.dataDir(),
	}, nil
}

// storeStarting reports that this replica's own registry does not answer yet while the process is
// still young enough for that to be its start, so the pass is skipped instead of reporting it.
//
// The syncer and the registry start together, and the registry needs a moment — longer when the
// syncer has just written its configuration — before it listens. A pass in that moment reads the
// declared set through this very store, gets "connection refused", and publishes it: the storage
// goes Failed and safeToDropUpstream false on every start of the pod, until the next clean pass
// clears it. Measured on an Engine cluster on 2026-10-02, at the first start and again after an
// update replaced the replica.
//
// Skipping keeps the previous report, which is the truer one during a restart: the store's data is
// on the node and outlives the pod. Bounded, because a registry that never comes up is a failure,
// and past the grace it is reported as one exactly as before.
func (l *Loop) storeStarting(ctx context.Context) bool {
	if l.LocalAddress == "" {
		return false
	}
	grace := l.StartupGrace
	if grace <= 0 {
		grace = 2 * time.Minute
	}
	if time.Since(l.started) >= grace {
		return false
	}

	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", l.LocalAddress)
	if err == nil {
		_ = conn.Close()
		return false
	}
	l.Log.Info("this replica's registry does not answer yet; the pass waits for it instead of reporting its start as a failure",
		"address", l.LocalAddress, "error", err.Error())
	return true
}

// releaseAbsent reports whether this replica's own store answers that it does not hold the deployed
// release — the image the set is read out of.
func (l *Loop) releaseAbsent(ctx context.Context) bool {
	releases, err := gc.FromCluster(ctx, l.Client)
	if err != nil || releases.Deployed == "" {
		return false
	}
	local, err := l.localRegistry()
	if err != nil {
		return false
	}
	return fill.ReleaseAbsent(ctx, local, releases.Deployed)
}

// storeRecheck is how often an idle replica reads its own store again, unless Loop.StoreRecheck says
// otherwise.
const storeRecheck = 6 * time.Hour

// recheckStore re-reads what the store holds on an idle pass, once in a while.
//
// Nothing is copied while every replica holds the set, so nothing else would notice a store that
// lost part of it — a disk replaced or cleaned by hand, a directory restored from an old copy: the
// replica would go on reporting the last pass that did copy. Read again here, from this replica's
// own disk and registry — no upstream, no leader — and reported as found; a replica that is no
// longer complete makes the controller raise needSync, and the copy that follows repairs it.
//
// Not on every pass, because reading the set means reading the platform image to its end, and an
// idle cluster has no reason to spend that every half a minute. The first idle pass of a process
// does it, which covers a restart; after that, every StoreRecheck. A reading that fails changes
// nothing and is tried again on the next pass.
//
// It can take a fingerprint away and never gives one: a reading that finds the store complete says
// what is on the disk now, not that a pass copied the set the fingerprint names — a follower reads its
// own disk, and only a replication says it holds what its leader holds. The fingerprint stays whatever
// the last copying pass made it, or goes.
func (l *Loop) recheckStore(ctx context.Context, state *report.State, reported string) {
	every := l.StoreRecheck
	if every <= 0 {
		every = storeRecheck
	}
	if last := l.lastStoreCheck.Load(); last != 0 && time.Since(time.Unix(0, last)) < every {
		return
	}
	// Counted as a check whether or not it could read the store, so that a reading that fails is tried
	// again in storeRecheckRetry rather than on every pass of the loop.
	l.lastStoreCheck.Store(time.Now().Add(min(storeRecheckRetry, every) - every).UnixNano())
	if !l.reportHeld(ctx, state) {
		return
	}
	l.lastStoreCheck.Store(time.Now().UnixNano())

	state.VerifiedSet = ""
	if state.Full && state.Error == "" {
		state.VerifiedSet = reported
	}
}

// registryDownReported is how long this replica's registry may stay silent before the status says so.
const registryDownReported = 5 * time.Minute

// storeRecheckRetry is how soon a store check that could not read the store is tried again.
const storeRecheckRetry = 10 * time.Minute

// localRegistryWait is how long a pass waits for this replica's registry to open its ports, unless
// Loop.RegistryWait says otherwise.
const localRegistryWait = 20 * time.Second

// awaitLocalRegistry waits until both of this replica's listeners accept a connection: the serving
// one, which a follower and the set enumeration read through, and the write endpoint the copy lands
// on. A connection is the whole question — a registry that answers at all answers the copy's own
// errors properly, while one that is not listening fails every reference the same way.
func (l *Loop) awaitLocalRegistry(ctx context.Context) error {
	if l.LocalAddress == "" {
		return nil
	}
	write, err := l.writeRegistry()
	if err != nil {
		return err
	}

	wait := l.RegistryWait
	if wait <= 0 {
		wait = localRegistryWait
	}
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()

	var dialer net.Dialer
	for _, address := range []string{l.LocalAddress, write.Address} {
		for {
			connection, err := dialer.DialContext(ctx, "tcp", address)
			if err == nil {
				_ = connection.Close()
				break
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("%s: %w", address, err)
			case <-time.After(time.Second):
			}
		}
	}
	return nil
}

func (l *Loop) localRegistry() (fill.Registry, error) {
	options, err := fill.RegistryOptions(l.LocalCA, l.LocalUser, l.LocalPassword)
	if err != nil {
		return fill.Registry{}, fmt.Errorf("local storage: %w", err)
	}

	return fill.Registry{
		Address:    l.LocalAddress,
		Repository: trimSlashes(constant.Path),
		Options:    options,
	}, nil
}

// writeRegistry is the same store, addressed through the listener that does NOT proxy.
//
// Filling through the serving instance does not fill anything, and fails silently: before uploading a
// layer the client asks whether the destination already holds that blob, and a pull-through cache
// fetches it from the upstream to answer and says yes. The upload is skipped, the manifest is
// written, and the store holds a complete set of manifests naming blobs it does not have — servable
// only while the upstream is reachable, which is exactly what an air-gapped cluster lacks. Every
// manifest reports as written while the store gains nothing. `d8 mirror push` is unaffected, writing
// to this endpoint from outside, which is why a bundle installation always had a real store while
// self-filling did not.
//
// The registry serves that listener itself, on WriteEndpointPort, from the same process and storage
// with no proxy in front of it. Its client certificate is about trusting the address a request claims
// to come from, not about admission: it answers an unauthenticated request with 401 like the serving
// listener, so the credentials this replica already holds are enough.
func (l *Loop) writeRegistry() (fill.Registry, error) {
	registry, err := l.localRegistry()
	if err != nil {
		return fill.Registry{}, err
	}

	if l.WriteAddress != "" {
		registry.Address = l.WriteAddress
		return registry, nil
	}

	host := registry.Address
	if index := strings.LastIndex(host, ":"); index > 0 && !strings.Contains(host[index:], "]") {
		host = host[:index]
	}
	registry.Address = fmt.Sprintf("%s:%d", host, distribution.WriteEndpointPort)

	return registry, nil
}

func trimSlashes(path string) string {
	for len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	for len(path) > 0 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}
	return path
}

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
	"fmt"
	"log/slog"
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

	// WriteAddress is where the NON-PROXYING instance of this replica's registry answers.
	//
	// Its own field rather than the serving address with the port swapped, so that a test can point a
	// loop at one registry and a deployment can point it at the instance beside the serving one.
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

	// complete records whether this replica holds the whole expected set, as of its last pass.
	//
	// Kept here rather than re-read from the API because the garbage collection asks it on its own
	// schedule, and an answer from the API would be one round trip behind this replica's own
	// knowledge — which on a store that is filling is the difference between "wait" and "go ahead".
	complete atomic.Bool

	// desired is the last desired state that was read, so that a component on another
	// schedule — the garbage collection — can act on the same instruction without racing
	// this loop for it.
	desired atomic.Pointer[registryv1alpha1.RegistryStorageSpec]

	// Interval is how often the desired state is re-read. A poll rather than a
	// watch: the loop has to re-apply the configuration and re-report anyway, and a
	// missed event would otherwise leave the replica silently stale.
	Interval time.Duration
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
// cache, so everything a node ever pulled through it joins the count. Measured on `ly-mmc`: the
// leader reported 332 while its followers reported 400, of the same cluster, at the same moment, both
// "full"; one sample read 398 verified out of 384 held, which is not a number anybody can act on.
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

// reportHeld replaces a branch's own accounting with what this replica holds of the declared set.
//
// The branches count what their copier moved, which answers "what did this pass do" — a useful thing
// to log and the wrong thing to publish. What the status is read for is how much of what the cluster
// needs is here, and that has to mean the same in every role, or the three replicas of one cluster
// cannot be compared with each other at all.
//
// A failure leaves the branch's numbers alone rather than zeroing them: a count that is a pass out of
// date beats no count, and zero in these fields reads as an emptied store.
func (l *Loop) reportHeld(ctx context.Context, state *report.State) {
	declared, deployed, err := l.declaredSet(ctx)
	if err != nil {
		l.Log.Warn("cannot tell what the cluster needs, so the status keeps what this pass copied",
			"error", err.Error())
		return
	}

	survey, err := l.surveyStore(declared, deployed)
	if err != nil {
		l.Log.Warn("cannot read the store, so the status keeps what this pass copied",
			"error", err.Error())
		return
	}

	state.VerifiedDigests = survey.Declared
	state.TotalDigests = survey.Total

	// And completeness from the same reading, which is the half that actually guards the cluster.
	//
	// The branches decide it from their own copier's report — "everything I set out to copy is
	// accounted for" — and that answer cannot see what the copier never looked at. A pull-through
	// cache writes a manifest's revision link the moment it serves it, so a copier finds the manifest
	// already present and counts it as done while not one of its layers is on disk. Measured on
	// `ly-mmc`: 333 MB of store, twenty manifests sampled and twenty missing layers, three replicas
	// reporting `full` and `safeToDropUpstream: true` — and an air-gap in which nothing could be
	// pulled at all.
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
}

// Filling reports whether this replica is filling its store right now.
//
// Read by the garbage collection before it fires. A method rather than a channel or a lock held
// across the work: the question is asked once, at one instant, and a stale "yes" merely postpones
// housekeeping by a minute while a stale "no" would only be possible if the fill had already
// finished.
func (l *Loop) Filling() bool { return l.filling.Load() }

// Complete reports whether this replica held the whole expected set as of its last pass.
func (l *Loop) Complete() bool { return l.complete.Load() }

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

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

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

	switch action := Decide(spec, isLeader, leader); action {
	case ActionFill:
		started := time.Now()
		l.filling.Store(true)
		func() {
			// Waits out a collection rather than racing it. The other direction is guarded too:
			// a collection does not start while a fill is owed — see gc.Plan.FillPending.
			done := l.PauseWrites()
			defer done()
			l.applyFill(ctx, spec, stated, &state)
		}()
		l.filling.Store(false)
		// What the fill moved is logged; what the replica HOLDS of the declared set is published.
		l.reportHeld(ctx, &state)
		l.observePass(string(action), started, &state)
	case ActionReplicate:
		started := time.Now()
		l.filling.Store(true)
		func() {
			done := l.PauseWrites()
			defer done()
			l.applyReplicate(ctx, leader, &state)
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
	}

	// While the write endpoint is open, the store has the last word on how much it
	// holds — whatever the pass above managed to do.
	if StoreIsAuthority(spec, isLeader) {
		l.recountFromStore(ctx, &state)
	}

	// TotalDigests is NOT recomputed here, and that is the correction.
	//
	// It used to be, on every pass, so that a field filled in by one code path would not read as "the
	// store emptied" on the others. The intent was right and the cost was never measured: counting it
	// walks the whole store, the loop runs every 30 seconds, and on an air-gapped master that showed up
	// as `registry-syncer` at 95% CPU with 102 minutes of processor time on a node up for 154.
	//
	// So it is answered where the store is already being read — see applyCatalogue — and carried over
	// otherwise. Carrying over is what keeps the original concern addressed: the status keeps the last
	// number that was actually measured instead of dropping to zero on a pass that did not look.
	if state.TotalDigests == 0 {
		// Nothing measured it on this pass — the fill and replication paths do not read the store — so
		// ask, at most once a minute, and fall back to the last number if even that is refused.
		//
		// Needed because moving the count into applyCatalogue alone emptied the field everywhere else:
		// measured on a caching cluster right after the fix, `verified=396` beside `total: null`, which
		// is precisely the "reads as an emptied store" the field was introduced to prevent. The survey
		// is shared with the catalogue path and cached the same way, so this costs one walk a minute at
		// worst, not one per pass.
		if survey, err := l.surveyStore(nil, ""); err == nil && survey.Total > 0 {
			state.TotalDigests = survey.Total
		} else {
			state.TotalDigests = l.lastTotalDigests.Load()
		}
	}
	if state.TotalDigests > 0 {
		l.lastTotalDigests.Store(state.TotalDigests)
	}

	// Remembered for the garbage collection, which asks on its own schedule: an incomplete store
	// must not be collected, and the loop is the one that knows.
	l.complete.Store(state.Full)

	return l.Publisher.Publish(ctx, state)
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
) {
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
		return
	}

	modules, err := gc.ModulesFromCluster(ctx, l.Client)
	if err != nil {
		state.Error = err.Error()
		return
	}

	copier, err := l.copier(spec, releases, modules)
	if err != nil {
		state.Error = err.Error()
		return
	}

	// `stated` is logged and nothing more. It is what an operator measured in a bundle, and by the
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
		return
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
}

// applyReplicate copies the leader's content into this replica.
//
// The read credentials are the same on every replica, so a follower reaches the
// leader with what it already has: no separate replication identity to distribute,
// and nothing that grants more than a pull.
func (l *Loop) applyReplicate(
	ctx context.Context, leader *Leader, state *report.State,
) {
	local, err := l.localRegistry()
	if err != nil {
		state.Error = err.Error()
		return
	}

	source := local
	source.Address = leader.Address

	// Written through the instance that does not proxy — see writeRegistry. A follower filling
	// through the serving one skips every layer for the same reason a leader does.
	destination, err := l.writeRegistry()
	if err != nil {
		state.Error = err.Error()
		return
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
		return
	}

	versions := make([]string, 0, 2)
	for _, version := range []string{releases.Deployed, releases.Previous} {
		if version != "" {
			versions = append(versions, version)
		}
	}

	// The same modules the count and the fill enumerate: a follower that copied less than the leader
	// holds would report itself incomplete forever, and one that copied more would look for images
	// nobody published.
	modules, err := gc.ModulesFromCluster(ctx, l.Client)
	if err != nil {
		state.Error = err.Error()
		return
	}

	l.Log.Info("replicating from the leader",
		"leader", leader.Node, "address", leader.Address,
		"deployed", releases.Deployed, "previous", releases.Previous)

	copier := &fill.Copier{
		Source:      source,
		Destination: destination,
		Discover:    fill.Release{Versions: versions, Modules: modules},
		// So that an image the store holds without its layers is copied rather than skipped: the
		// registry answers "present" for those, and a follower would otherwise never repair itself.
		StoreDir: l.dataDir(),
		OnProgress: func(done, total int32) {
			l.Log.Debug("replication progress", "done", done, "total", total)
		},
	}

	result, err := copier.Run(ctx)
	state.Source = leader.Node
	state.VerifiedDigests = result.Written

	if err != nil {
		state.Error = err.Error()
		return
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
			Full: replica.Full && replica.Error == "",
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
// Both halves matter because this number has to mean the same thing as the one a copy reports. When it
// did not, a replica said 333 after a copying pass and 348 after a counting one; `full` followed
// whichever ran last and flapped; eligibility follows `full`, so the lease moved; and a fill restarts on
// every move. Measured on a cluster: the lease travelling between three replicas every twenty seconds,
// none of them ever finishing.
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
	// Three separate readers, every thirty seconds, cost 95% of a CPU on an air-gapped master. Merging
	// them into one walk divides that by three; not repeating it while nothing can have changed removes
	// what is left. The store only changes here through a push, so a survey at most once a minute is as
	// current as anything downstream needs: what it gates is the air-gap transition, and delaying that
	// by up to a minute is not a cost anybody can measure.
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

	// The set, and nothing but the set: how full the store is of what the cluster needs. What an
	// operator stated in `storage.source.expectedDigests` is not consulted, not even when the set comes
	// out empty — see Report.Complete for the measurement that settled it. An empty set reads as
	// incomplete, which is the safe direction: it withholds the air-gap transition rather than
	// authorising it on no evidence.
	state.VerifiedDigests = held

	// Counting is not enough, and this is the check that was missing.
	//
	// A count says how many manifests are on the disk; it does not say that the release can be RESOLVED
	// from this store. Those come apart, and expensively: on a three-master cluster the leader reported
	// full while its followers got `MANIFEST_UNKNOWN` for `:pr21788` from it, and the agent got
	// `NAME_UNKNOWN: repository name not known to registry` — a store that authorized dropping the
	// upstream and then could not hand the release to anybody. Replication enumerates the set by reading
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
// Carried verbatim — the count AND the verdict — because an idle pass learned nothing, and a pass that
// learned nothing must not contradict the one that did. It used to recompute fullness from
// `expectedDigests`, and where that is not stated the recomputation could only answer "not full": every
// idle pass therefore erased a completeness that a fill had just established.
//
// What that cost is worth writing down, because the symptom was nowhere near the cause. A leader filled
// the store and reported full; the controller saw the storage converged and cleared `needSync`; with
// nothing left to do the next pass went idle and erased the fullness; the controller saw a store that
// was not converged and asked for a fill again. Every few seconds, for as long as the cluster ran. And
// since a replica's eligibility to lead depends on being full, the lease travelled with it — which is
// what made this look like flapping leader election, and what three earlier fixes to leadership could
// not have cured.
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
		state.Full = storage.Status.Replicas[i].Full
		state.Source = storage.Status.Replicas[i].Source
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

	upstreamOptions, err := fill.RegistryOptions(spec.Upstream.CA, username, password)
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

	versions := make([]string, 0, 2)
	for _, version := range []string{releases.Deployed, releases.Previous} {
		if version != "" {
			versions = append(versions, version)
		}
	}

	return &fill.Copier{
		Source: fill.Registry{
			Address:    spec.Upstream.Host,
			Repository: trimSlashes(spec.Upstream.Path),
			Options:    upstreamOptions,
		},
		Destination: local,
		// Read out of the releases themselves rather than by listing the upstream. Listing
		// somebody else's registry is a privilege of its own, which credentials scoped to
		// pulling — all a license grants — are refused for; and "everything the upstream
		// holds" is not a set a cache can be complete with respect to anyway.
		Discover: fill.Release{Versions: versions, Modules: modules},
		// The same repair the replication path needs: a store filled by proxying holds manifests
		// without layers, and the registry reports those as present.
		StoreDir: l.dataDir(),
		OnProgress: func(done, total int32) {
			l.Log.Debug("fill progress", "done", done, "total", total)
		},
	}, nil
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

// writeRegistry is the same store, addressed through the instance that does NOT proxy.
//
// Filling through the serving instance does not fill anything, and the way it fails is silent. Before
// uploading a layer the client asks the destination whether it already holds that blob; the serving
// instance is a pull-through cache, so it fetches the blob from the upstream to answer and says yes.
// The upload is skipped, the manifest is written, and the store ends up holding a complete set of
// manifests naming blobs it does not have — servable only for as long as the upstream is reachable,
// which is precisely the condition an air-gapped cluster does not have.
//
// Measured on `ly-mmc`: a fill of the whole set reporting `written=400, skipped=0` that left the store
// at the same 333 MB and the same 450 blobs; every layer of every sampled manifest absent from disk
// while the registry answered 200 for it. `d8 mirror push` was never affected — it writes to this
// endpoint from outside — which is why installations from a bundle always had a real store and
// self-filling never did.
//
// The second instance exists already, for publication, over the same data directory: it never proxies
// and it answers on WriteEndpointPort. Its client certificate requirement is about trusting the
// address a request claims to come from, not about admission — it answers an unauthenticated request
// with 401, the same as the serving one — so the credentials this replica already holds are enough.
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

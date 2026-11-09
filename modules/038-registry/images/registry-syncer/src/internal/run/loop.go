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
	"net/http"
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

	// Present is the election's view of which reports still stand: those of replicas whose pod and node
	// are there. Nil keeps every report. See currentLeader.
	Present func(ctx context.Context, replicas []registryv1alpha1.StorageReplicaStatus) []registryv1alpha1.StorageReplicaStatus

	// ReadLease is what the storage lease says about who leads. Nil means unknown, and the reported
	// roles decide who a follower copies from — see LeaderOf.
	ReadLease func(ctx context.Context) Lease

	// ArrangementEvery is how often a copy under way checks that the arrangement it started under still
	// holds. Zero means arrangementEvery. See whileArranged.
	ArrangementEvery time.Duration

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

	// StoreHostPath is DataDir's directory on the node, for the advice when the store cannot be
	// written to. Empty when it is not known, and the advice then names no path.
	StoreHostPath string

	// ReportedAddress is where OTHER replicas reach this one, which is not the same
	// as LocalAddress: this replica talks to itself over the loopback interface,
	// while its neighbours come in over the node address.
	ReportedAddress string

	// LocalCA and LocalAuth are how to reach it.
	LocalCA                  string
	LocalUser, LocalPassword string

	// LocalTransport builds the transport to this replica's own registry. Nil means
	// fill.ClientTransport with LocalCA and LocalClientCertificate. A field so that a test can give the registry a listener that asks more of a client
	// than a CA; see localTransport.
	LocalTransport func() (*http.Transport, error)

	// LocalClientCertificate is what this replica presents to its own write endpoint, which refuses a
	// handshake without a client certificate from the module's authority. In the storage pod it is
	// the registry's serving pair: see distribution.CertificateFile. Nil presents nothing, which is a
	// loop pointed at a registry that does not ask.
	//
	// Carried by localTransport, and so by every client of this replica's own registry: writeRegistry
	// and LocalOptions, and with them the fill, the replication and the garbage collection, and the
	// health watch's probe of the listeners.
	LocalClientCertificate *fill.ClientCertificate

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
	// aheadForgotten: what is ahead of the set was taken back, by the collection or with a store found
	// short, so what this replica reported before no longer stands — see ForgetAhead; aheadForgets counts
	// the times it was, so that a copy started before does not put back what it held.
	aheadHeld      map[string]bool
	aheadMissedFor string
	aheadMissedAt  time.Time
	aheadForgotten bool
	aheadForgets   uint64
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

	// surveyStale is whether the last survey is not to be answered from, however fresh: a step-down or a
	// take-back found the store short of what it said — see StepDown and TakeBackFull — and the pass that
	// ends either must say what the store holds now. A flag rather than surveyMu, which a walk holds for
	// as long as the walk takes, and which the review, under reportMu, must not wait on.
	//
	// Set as well where the store has changed, or is about to be judged, since the last walk: after a
	// fill or a replication copied into it, so that reportHeld counts what the copy left rather than a
	// walk from before it, and so that a pass whose reportHeld failed — its numbers then the branch's own
	// — has its total taken fresh too, and an idle pass does not refute those numbers with a walk from
	// before the copy (see refutedByCount); and on the first pass that sees air-gap requested — see
	// airGapSeen.
	surveyStale atomic.Bool

	// airGapSeen is whether the last pass saw spec.airGapRequested. The pass that first sees it sets
	// surveyStale, so the transition's recount — which runs on every pass from then on, and on whose
	// Full the controller drops the upstream — walks the disk rather than answer from a survey up to a
	// minute old, taken perhaps before the store was lost. Narrows the window and does not close it: the
	// controller can drop the upstream in the very reconcile that requests the transition, on the report
	// the leader made before it.
	airGapSeen atomic.Bool

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

	// currentSet is the fingerprint of the set the cluster needed at the last pass that could tell,
	// for whoever outside the loop has to notice the set changing — see CurrentSet.
	currentSet atomic.Value

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

	// storeRead is whether this process has published a report it read off its own store: a copy into
	// it, a count of it, or a recheck that read it. From then on its reports carry started as
	// SyncerStartedAt — the passes after it carry that reading over, and dropping the stamp on them
	// would make the status flap — and before then they carry none. The controller drops the upstream
	// only on a lease holder's entry stamped no earlier than its syncer container started, so a Full
	// left by the previous process — over a store wiped with the pod, say — cannot do it. Not set by a
	// publish alone: a first recheck that failed, its upstream not answering, publishes the Full the
	// previous process left.
	storeRead atomic.Bool

	// addressPublished is set once this process has written ReportedAddress into its entry. See
	// publishAddress.
	addressPublished bool

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

	// registryDownSince is when this replica's registry was first found not answering, in Unix
	// nanoseconds, and zero while it answers. See registryDownReported.
	//
	// Started by whichever looks first — a pass waiting to copy, or the health watch — and ended only
	// by the health watch, when the registry answers HTTP again: a port that accepts a connection is
	// not yet a registry that answers one. See watchHealth.
	registryDownSince atomic.Int64

	// lastRun is what this replica's store was before this process started — from the store's own
	// memory of it when there is one, from what the replica last reported otherwise — read before
	// anything here reports again, and dropped once the store holds something where it is meant to
	// be. See recallStore and filesystemWarning. Touched only by the loop's own goroutine, as are the
	// three below.
	lastRun *registryv1alpha1.StoreUsage

	// remembered is whether the store's directory already holds lastRun, so that it is written once.
	// See rememberStore.
	remembered bool

	// filesystemProblem is what filesystemWarning said when the store was last measured: kept as it is
	// while the store cannot be measured, because whether it is empty is then not known.
	filesystemProblem string

	// storeProblem is what storeProblems found last, so that it is logged when it appears, changes
	// or goes away rather than on every pass.
	storeProblem string

	// checkWritable is distribution.CheckWritable, or what a test puts in its place: tests may run as
	// root with every capability, which no mode bit stops. Nil means distribution.CheckWritable.
	checkWritable func(dataDir, hostPath string) error

	// measure is distribution.MeasureStore, or what a test puts in its place: the space left on the
	// disk a test runs on is no figure to assert on. Nil means distribution.MeasureStore.
	measure func(dataDir string, spec *registryv1alpha1.RegistryStorageSpec, dedicated bool) (*registryv1alpha1.StoreUsage, error)

	// lastMeasured is the store as last measured, or, before this process measured it, as the replica
	// last reported it — see recallStore. For publishedStore, when the store cannot be measured now:
	// set and read from the loop's goroutine and watchStore's.
	lastMeasured atomic.Pointer[registryv1alpha1.StoreUsage]

	// storeNotWritable is whether the last write probe found the store refusing writes — see
	// probeWritable. Like lastMeasured, set from two goroutines, the loop's and watchStore's, and read
	// by whichever publishes the store — see publishedStore.
	storeNotWritable atomic.Bool

	// configured is whether a pass has applied the registry configuration since this process started.
	// The wrapper does not start the registry before it has one, so until then a registry that does
	// not answer is one that has not been asked to, and the health watch does not time its silence.
	configured atomic.Bool

	// HealthEvery is how often the health watch probes this replica's registry and store. Zero means
	// healthEvery. See watchHealth.
	HealthEvery time.Duration

	// StoreProbe writes to the store and takes the write back, returning what refused it. Nil means
	// probeStore. A field so that a test can make a disk fail, refuse, or hang.
	StoreProbe func(dir string) error

	// StoreProbeTimeout is how long one write probe may take before it counts as a failure. Zero means
	// storeProbeTimeout.
	StoreProbeTimeout time.Duration

	// What the health watch found, read by the pass and by the election; see HealthError.
	healthMu  sync.Mutex
	healthErr string

	// The health watch's own state, touched by nothing else: when the store first refused the probe,
	// and the probe still running, if one is.
	storeFailingSince time.Time
	storeProbing      chan error
	storeProbeStarted time.Time

	// StoreSample reads a sample of the set off this replica's disk. Nil means fill.Sample. A field so
	// that a test can make the disk slow, failing or short of the set — see ShortOfSet.
	StoreSample func(root, scope, release string, digests []string) (fill.SampleResult, error)

	// StoreSampleTimeout is how long one sample may take before it counts as no finding. Zero means
	// stepDownCheckTimeout.
	StoreSampleTimeout time.Duration

	// lost is whether this replica handed the lease over because its store did not hold the set another
	// replica held whole — see StepDown — or, holding the lease with nobody else full, took its report
	// back for the same reason — see TakeBackFull — and no pass has copied into the store or counted it
	// since. Until one has, what the status carried over about this replica is not to be believed, by
	// the passes or by the election.
	lost atomic.Bool

	// tookBack is whether lost came from TakeBackFull: the report taken back and the lease kept. A
	// step-down still owes its role, the lease and the line that says why — see StepDown. Set and cleared
	// with lost, under reportMu.
	tookBack atomic.Bool

	// losses counts the step-downs and take-backs of this process, so that a pass can tell one came in
	// while it ran. A reading taken before the loss was found and published after it — a count from a
	// walk of the store made before the wipe, in air-gap or during the transition, where the leader
	// counts on every pass — says what the store held then, not now, and must not end the loss: published,
	// it would put Full back on the leader the controller is about to drop the upstream on. Changed under
	// reportMu.
	losses atomic.Uint64

	// reportMu orders a pass's report against a step-down's: a pass that carried Full over before the
	// step-down must not publish it after the demotion, and one that read the store must not have its
	// reading taken back by a step-down that came in between.
	reportMu sync.Mutex

	// ShortOfSet's own state: how many checks in a row found most of the sample missing, when the last
	// check found nothing, and the sample still running, if one is.
	sampleMu      sync.Mutex
	sampleShort   int
	sampleCleanAt time.Time
	sampling      chan sampleOutcome
	sampleStarted time.Time
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
	// by a survey taken for that same set. Neither is answered from a survey a step-down or a take-back
	// has found wrong since — see surveyStale.
	stale := l.surveyStale.Swap(false)
	if !stale && time.Since(l.surveyedAt) < freshFor && (len(declared) == 0 || l.surveyedFor == len(declared)) {
		return l.survey, nil
	}

	survey, err := fill.Take(l.dataDir(), strings.Trim(constant.Path, "/"), declared,
		[]string{deployed})
	if err != nil {
		if stale {
			// Not walked, so the survey it was not to be answered from still is not.
			l.surveyStale.Store(true)
		}
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

// TryPauseWrites is PauseWrites if nothing holds the cache now, and says whether it did.
//
// For the garbage collection, which takes its turn on a lease the other replicas wait for: a fill
// holds the pause for its whole pass, and waiting it out with the lease in hand would hold them out
// too.
func (l *Loop) TryPauseWrites() (func(), bool) {
	if !l.writes.TryLock() {
		return nil, false
	}
	return l.writes.Unlock, true
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

		// What is wrong with the store goes with it: a store the registry cannot write to reaches here
		// too, on the references it already holds, and an update without an error would take it off
		// the status for the rest of the pass. Here, on the loop's goroutine — the copier calls this
		// from its own loop over the set — so the probe creates and removes a file in up to three
		// directories every progressEvery: nothing on a working disk, and on one that hangs it holds
		// the pass here as the next copy would. And so does what the health watch found: it stands for
		// as long as the watch finds the registry or the store failing, copy or no copy. The error a
		// progress report clears is the previous pass's, never these.
		problems := l.storeProblemsWhileCopying()
		if health := l.HealthError(); health != "" {
			if problems != "" {
				problems += "; "
			}
			problems += health
		}
		update := report.State{
			Node: state.Node, Role: state.Role, Address: state.Address,
			VerifiedDigests: done, DeclaredDigests: total,
			Error: problems,
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
//
// The write probe runs here too, before each measurement, so that a store whose directory stops
// taking writes is published as NotWritable within a tick, and not when a pass that may take an hour
// ends — see publishedStore. The probe alone and not storeProblems, which only the loop's goroutine
// may call: the error text follows with the next progress report or the end of the pass.
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
		if l.DataDir != "" {
			_ = l.probeWritable()
		}
		usage := l.publishedStore(l.measureStore(spec))
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
// succeeds, so a failed measurement is worth a warning and nothing more. Kept as lastMeasured for a
// store that cannot be measured and refuses writes — see publishedStore.
func (l *Loop) measureStore(spec *registryv1alpha1.RegistryStorageSpec) *registryv1alpha1.StoreUsage {
	if l.DataDir == "" {
		return nil
	}
	measure := l.measure
	if measure == nil {
		measure = distribution.MeasureStore
	}
	usage, err := measure(l.DataDir, spec, l.storeFilesystem().Dedicated)
	if err != nil {
		l.Log.Warn("measuring the store failed", "error", err.Error())
		return nil
	}
	l.lastMeasured.Store(usage.DeepCopy())
	return usage
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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

// arrangementEvery is how often a copy under way checks that the arrangement it started under still
// holds, unless Loop.ArrangementEvery says otherwise.
//
// For the leader's fill that is one atomic read. For a follower it is also whether the replica it
// copies from still leads: a read of the storage and of the lease — and, only while the lease cannot be
// read, a list of the storage pods and a read of up to three nodes — paid only while a follower copies.
// Small beside the minute a lease lasts and a node takes to be counted lost, so a copy outlives its
// arrangement by seconds rather than by the rest of the pass.
const arrangementEvery = 10 * time.Second

// whileArranged gives a copy a context that ends when the arrangement it started under does: this
// replica's role changes, or — for a follower — the leader it copies from no longer leads. The function
// it returns stops the watch, and reports whether the watch ended the copy.
//
// A pass read its role once, at its start, and a copy lasts as long as the set takes to move. A
// follower that took the lease kept copying from the leader it replaced: from a leader whose node lost
// power that is a connection nothing answers, and on a DVP stand it hung for sixteen minutes, and the
// next request waited on a dial, while the new leader filled nothing — fifty-six minutes, until the
// container was restarted by hand. Its neighbour, by the roles, kept copying from the dead leader for an hour.
// A leader that lost the lease kept filling from the upstream with the account it shares with the new
// one, and ended the pass by reporting itself leader, which takes the role from the replica that holds
// the lease — see report.Merge.
//
// The pass only: what is ahead of the set is copied under its own scope, which the next pass compares
// — see stopAheadOutside.
func (l *Loop) whileArranged(ctx context.Context, isLeader bool, leader *Leader) (context.Context, func() bool) {
	every := l.ArrangementEvery
	if every <= 0 {
		every = arrangementEvery
	}

	copyCtx, cancel := context.WithCancel(ctx)
	var ended atomic.Bool
	done := make(chan struct{})
	finished := make(chan struct{})

	go func() {
		defer close(finished)
		ticker := time.NewTicker(every)
		defer ticker.Stop()

		for {
			select {
			case <-copyCtx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
			}
			if !l.rearranged(copyCtx, isLeader, leader) {
				continue
			}
			select {
			case <-done:
				// The copy finished on its own while this looked: it went through under the arrangement
				// it started under, and the pass reports it.
			default:
				ended.Store(true)
				cancel()
			}
			return
		}
	}()

	return copyCtx, func() bool {
		close(done)
		cancel()
		<-finished
		return ended.Load()
	}
}

// rearranged reports whether the arrangement a copy was started under has ended — see whileArranged.
//
// A leader that cannot be read is no evidence that it has: the answer comes from the API server, and an
// unreachable one says nothing about the replica the copy reads from — the rule the election and the
// lost-node filter keep too. Its copy goes on, and fails on its own if the leader is gone.
//
// A lease that cannot be read is the same, though the storage could be: the roles currentLeader falls
// back to are a guess, and right after a failover a wrong one — the replica that took the lease still
// reports Follower while its first fill runs, and the dead leader's Leader stands until then. Judged by
// them, one failed read of the lease cut a copy from the live holder and lost the pass it was in.
func (l *Loop) rearranged(ctx context.Context, isLeader bool, leader *Leader) bool {
	if l.Leadership.IsLeader() != isLeader {
		return true
	}
	if isLeader {
		return false
	}
	current, guessed, err := l.currentLeader(ctx)
	if err != nil || guessed {
		return false
	}
	return current == nil || current.Node != leader.Node || current.Address != leader.Address
}

// leaveRearranged ends a pass whose copy the arrangement ended under it — see whileArranged.
//
// Nothing is reported, and that is the point. What the pass holds is about the role it started with,
// and its error is the cancellation: a Leader from a replica that no longer holds the lease would take
// the role from the one that does — see report.Merge — and an error would turn the storage Failed over
// a copy nothing failed. The progress the copy published stands; it never touches the role. The next
// pass starts under the new arrangement at the next tick — at once after a copy that ran past one,
// since the ticker keeps it.
func (l *Loop) leaveRearranged(isLeader bool) error {
	l.Log.Info("the copy stopped: this replica's role, or the leader it copies from, changed; "+
		"the next pass starts under the new one", "leader", isLeader)
	return nil
}

// Run drives the loop until the context is cancelled.
func (l *Loop) Run(ctx context.Context) error {
	interval := l.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}

	// Before anything reports: the first report replaces what the last run said.
	l.recallStore(ctx)
	l.inspectStore()

	go l.watchStore(ctx, storeEvery)
	go l.watchHealth(ctx, l.HealthEvery)

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
	// Before anything this pass reads: a loss found after this has its reading come too late — see losses.
	losses := l.losses.Load()

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

	// The first pass under a request for air-gap reads the store afresh — see airGapSeen. Cleared when
	// the request goes, so that the next request does the same.
	if spec.AirGapRequested {
		if !l.airGapSeen.Swap(true) {
			l.surveyStale.Store(true)
		}
	} else {
		l.airGapSeen.Store(false)
	}

	// The configuration is applied before anything else, and regardless of role.
	// This is the path a credential change takes while the Deckhouse operator is
	// down, and it must not wait behind a fill.
	if changed, err := l.Applier.Apply(spec); err != nil {
		return fmt.Errorf("applying the registry configuration: %w", err)
	} else if changed {
		l.Log.Info("the registry configuration changed and the process was restarted")
	}
	l.configured.Store(true)

	if l.storeStarting(ctx) {
		return nil
	}
	l.publishAddress(ctx)

	isLeader := l.Leadership.IsLeader()
	// The address is reported by the replica itself, so a follower can replicate from
	// the leader without resolving a node name into an address.
	state := report.State{Node: l.Node, Role: Role(isLeader), Address: l.ReportedAddress}
	stated := ExpectedDigests(spec)

	leader, _, err := l.currentLeader(ctx)
	if err != nil {
		return err
	}

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
		l.currentSet.Store(set)
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
	// A pass that wants nothing from the registry no longer ends a silence, as it once did so that a
	// silence days old would not make the next ordinary restart an error at once. That made the silence
	// of a leader with nothing to copy invisible: it reported itself full and kept the lease while its
	// registry answered nobody. The health watch ends a silence now, when the registry answers again —
	// see watchHealth.

	// Whether what this pass publishes comes from a copy or a count of the store, rather than carried
	// over from the status; only such a report takes back a step-down — see StepDown. Not a recheck: on
	// a store that lost the release it says Full is false and carries the old count over.
	read := false
	// Whether an idle pass's recheck read the store: not a reading that ends a step-down, but one this
	// process made, which is what SyncerStartedAt asks — see storeRead.
	rechecked := false

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
		}
	}

	// What is ahead of the set, copied once this pass has said what it found — see copyAhead.
	var ahead *fill.Copier

	switch action {
	case ActionFill:
		started := time.Now()
		l.filling.Store(true)
		l.announceStart(ctx, state, stated)
		copyCtx, stopWatching := l.whileArranged(ctx, isLeader, leader)
		func() {
			// Waits out a collection rather than racing it. The other direction is guarded too:
			// a collection does not start while a fill is owed — see gc.Plan.FillPending.
			done := l.PauseWrites()
			defer done()
			ahead = l.applyFill(copyCtx, spec, stated, &state)
		}()
		rearranged := stopWatching()
		l.filling.Store(false)
		if rearranged {
			return l.leaveRearranged(isLeader)
		}
		// The copy changed the store: what it holds is read off the disk, not off a walk from before the
		// copy — see surveyStale.
		l.surveyStale.Store(true)
		// What the fill moved is logged; what the replica HOLDS of the declared set is published.
		l.reportHeld(ctx, &state)
		l.observePass(string(action), started, &state)
		read = true
	case ActionReplicate:
		started := time.Now()
		l.filling.Store(true)
		l.announceStart(ctx, state, stated)
		copyCtx, stopWatching := l.whileArranged(ctx, isLeader, leader)
		func() {
			done := l.PauseWrites()
			defer done()
			ahead = l.applyReplicate(copyCtx, leader, &state)
		}()
		rearranged := stopWatching()
		l.filling.Store(false)
		if rearranged {
			return l.leaveRearranged(isLeader)
		}
		l.surveyStale.Store(true)
		// The same correction as above, and the branch it matters most on: a follower enumerates
		// through the leader's store, which holds more than the set.
		l.reportHeld(ctx, &state)
		l.observePass(string(action), started, &state)
		read = true
	case ActionCountCatalogue:
		l.applyCatalogue(ctx, &state)
		if !isLeader && setKey != "" && reported == setKey {
			// A follower counts while it has nobody to copy from — see LeaderOf: above all while the lease
			// is free, between a leader that let it go and the next one to take it. One found full against
			// this very set holds what it copied from the replica it names, and saying so is what lets that
			// replica, leading again, find this one already holding it — see AlreadyHeld, and
			// FollowersBehind in the controller. Dropped, every follower copied the set again once the
			// lease was back where it had been.
			//
			// Only then: a follower whose copy had not finished, or that was full against another set,
			// never copied this set from that replica, and a count that finds it full here must not read
			// as a replication that finished — see applyReplicate.
			state.Source = reportedSource
		}
		read = true
	case ActionNone:
		// Keep whatever was reported last: this pass learned nothing new, and
		// overwriting the count with a zero would look like the storage emptied.
		if err := l.carryOverCount(ctx, &state); err != nil {
			return err
		}
		rechecked = l.recheckStore(ctx, &state, reported)
	}

	// While the write endpoint is open, the store has the last word on how much it
	// holds — whatever the pass above managed to do.
	if StoreIsAuthority(spec, isLeader) {
		l.recountFromStore(ctx, &state)
		read = read || state.Error == ""
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
	//
	// measured is whether this pass's total comes from a walk of the store, rather than from the last
	// number: only a measured total is evidence against a Full carried over — see refuteCarriedFull.
	measured := false
	if state.TotalDigests == 0 {
		// Nothing measured it on this pass — an idle pass reads nothing, and a copy or a count whose
		// reading failed has no total — so ask, at most once a minute, and fall back to the last number
		// only if even that is refused.
		//
		// Needed because moving the count into applyCatalogue alone empties the field everywhere else,
		// leaving a verified count beside an absent total — precisely the "reads as an emptied store"
		// the field was introduced to prevent. The survey is shared with the catalogue path and cached
		// the same way, so this costs one walk a minute at worst, not one per pass.
		//
		// A zero from a walk is a reading, and is published: a store that has emptied reads as one. Zero
		// from no walk is not, and the last number stands. It used to be the other way round for any
		// zero, and on a stand a store wiped in air-gap went on reporting the 446 manifests it had held
		// for five minutes, until something was pushed into it again.
		if survey, err := l.surveyStore(nil, ""); err == nil {
			state.TotalDigests = survey.Total
			measured = true
		} else {
			state.TotalDigests = l.lastTotalDigests.Load()
		}
	}
	if state.TotalDigests > 0 || measured {
		l.lastTotalDigests.Store(state.TotalDigests)
	}

	// The one thing an idle pass can learn about its store without reading more of it: whether the
	// total it has just measured can hold the Full it carried over — see refuteCarriedFull. Not after a
	// pass that read the store or rechecked it, whose Full is its own reading.
	if action == ActionNone && !read && !rechecked && measured {
		l.refuteCarriedFull(ctx, &state, isLeader, setKey)
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

	// Measured last, after whatever the pass wrote: a figure taken at the start of an hour-long
	// fill would, published at its end, put back what the store watcher had long since replaced.
	state.Store = l.measureStore(spec)

	// A replica that handed the lease over for a store that did not hold the set, or kept it and took its
	// report back, says so until a pass copies into the store or counts it — see StepDown and
	// TakeBackFull. Whatever this pass carried over was read from the status, and the status may be from
	// before the demotion: a pass that began before it, with Full in hand, would otherwise put Full back.
	// Under reportMu from here to the report, so that a step-down cannot come between this and the write;
	// nothing in between touches the disk.
	l.reportMu.Lock()
	// A reading counts only when no loss was found while the pass ran — see losses.
	masked := l.lost.Load() && (!read || l.losses.Load() != losses)
	if masked {
		state.Full, state.VerifiedSet, state.VerifiedDigests = false, "", 0
	}

	// Remembered for the garbage collection, which asks on its own schedule: an incomplete store
	// must not be collected, and the loop is the one that knows.
	l.complete.Store(state.Full)

	// What is ahead of the set, held as of the last copy that went through — or, unknown this pass, as
	// reported last.
	state.AheadSet = reportedAhead
	if keysKnown {
		state.AheadSet = l.heldAhead(aheadKey, reportedAhead)
	}
	if masked {
		state.AheadSet = ""
	}

	// What the health watch found stands until the watch takes it back: a pass that went well — an idle
	// one above all, which looks at nothing — does not know the registry answers or the disk takes
	// writes, and publishing it without the error would hand a hung leader back its lease every half a
	// minute. Its own error, if it has one, says more about what this pass did and stays. After
	// VerifiedSet, so that the fingerprint survives: once the registry answers again the replica is
	// full again without a copy.
	if state.Error == "" {
		state.Error = l.HealthError()
	}

	// The role this report states is the one the pass began with, and the lease may have moved since:
	// a count, a wait for a collection or a withdrawal is not stopped by it the way a copy is — see
	// whileArranged. Published, a Leader from a replica that no longer holds the lease takes the role
	// from the one that does — see report.Merge — and the followers read the roles whenever the lease
	// cannot be read. So nothing is said, and the next pass says it under the new role.
	if l.Leadership.IsLeader() != isLeader {
		l.reportMu.Unlock()
		l.Log.Info("this replica's role changed during the pass, so it reports nothing; " +
			"the next pass reports under the new role")
		return nil
	}

	// After Full and the set it was found against: a store that cannot take a write still holds what
	// it holds, and a store that was verified is not verified again for it on every pass. The error
	// is what keeps such a replica from being copied from or elected — see isFull.
	if problem := l.storeProblems(state.Store); problem != "" {
		if state.Error != "" {
			problem += "; " + state.Error
		}
		state.Error = problem
	}

	// Which process this report speaks for, once that process has read its store — see storeRead.
	// Outside Run there is no start to speak for.
	if (read || rechecked || l.storeRead.Load()) && !l.started.IsZero() {
		started := l.started
		state.SyncerStartedAt = &started
	}

	// Published with the store as the probe just found it — see publishedStore; what is ahead of the
	// set is still decided on the measurement alone.
	published := state
	published.Store = l.publishedStore(state.Store)
	err = l.Publisher.Publish(ctx, published)
	if err == nil && read && !masked {
		// A reading published supersedes the step-down: the status now says what the store holds.
		l.lost.Store(false)
		l.tookBack.Store(false)
	}
	if err == nil && (read || rechecked) {
		l.storeRead.Store(true)
	}
	l.reportMu.Unlock()
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
// gc.Collector.ReleaseAhead — or the store was found short of the set — see StepDown — and saying
// otherwise would send its followers to copy what it no longer has. Copied again, part by part, once the
// store has room.
//
// A copy ahead already running is left to finish or be stopped, but what it reports held is not taken:
// it was copied into the store as it was before, and part of it may be gone with the rest.
func (l *Loop) ForgetAhead() {
	l.aheadMu.Lock()
	defer l.aheadMu.Unlock()
	l.aheadHeld = nil
	l.aheadForgotten = true
	l.aheadForgets++
}

// RoomAhead reports whether this replica's store has room for what is ahead of the set — see roomAhead.
// From the measurement alone, not from what is published: the collection releases what is ahead of the
// set when this says no, and a store whose directory refuses writes gets no room back from that.
func (l *Loop) RoomAhead() bool {
	spec := l.Desired()
	if spec == nil {
		return true
	}
	return roomAhead(l.measureStore(spec))
}

// CurrentSet is the fingerprint of the set the cluster needs, as of the last pass that could tell —
// see SetKey. Empty until one could.
func (l *Loop) CurrentSet() string {
	set, _ := l.currentSet.Load().(string)
	return set
}

// StoreRefusesWrites reports whether this replica's store refuses writes now: over its budget, or down
// to the space kept free on its filesystem. Unmeasured is not a refusal, and neither is a directory the
// registry cannot write to: that one is published as NotWritable — see publishedStore — and is not a
// store a collection makes room in.
//
// Asked by the garbage collection before it waits for a fill. A fill into a store that refuses writes
// fails at its first blob and is tried again on the next pass, for as long as nothing frees room — and
// the one thing that frees room is the collection that would be waiting for it.
func (l *Loop) StoreRefusesWrites() bool {
	spec := l.Desired()
	if spec == nil {
		return false
	}
	store := l.measureStore(spec)
	return store != nil && !store.Writable
}

// UpstreamRegistry is the upstream the set is filled from, with the credentials to read it, or nil in
// air-gap, where there is none.
//
// For the garbage collection, which reads the set out of this store and cannot when the store never
// got to hold the installer of the release the cluster runs — a store that refuses writes cannot take
// it in. The fill reads the same set from here.
func (l *Loop) UpstreamRegistry() (*fill.Registry, error) {
	spec := l.Desired()
	if spec == nil || spec.Upstream == nil {
		return nil, nil
	}
	username, password := spec.Upstream.Auth.BasicCredentials()
	options, err := fill.RegistryOptions(spec.Upstream.CA, username, password)
	if err != nil {
		return nil, fmt.Errorf("upstream: %w", err)
	}
	return &fill.Registry{
		Address:    spec.Upstream.Host,
		Repository: trimSlashes(spec.Upstream.Path),
		Options:    options,
	}, nil
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
	forgets := l.aheadForgets

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
		// Whole is whole, stopped or not, unless what is ahead was forgotten since the copy started;
		// what was missed waits aheadRetry, unless it was only stopped.
		if l.aheadHeld == nil {
			l.aheadHeld = map[string]bool{}
		}
		if l.aheadForgets == forgets {
			for _, part := range report.AheadHeld {
				l.aheadHeld[part] = true
			}
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
// The sync account is the same on every replica, so a follower reaches the leader with
// what it already fills with: no separate replication identity to distribute. It can
// write — it has to, to put what it copies into its own store — which is why it is a
// replica's own account and not the one the nodes pull with.
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
	// a working replica look broken — and, through the eligibility rules, keep it from counting as full.
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

// currentLeader reads who to replicate from: the replica holding the lease, by its own report — see
// LeaderOf.
//
// Taken from the replica entries rather than from the aggregate `leader` field: the
// entries are what each replica wrote about itself, so a stale or hand-edited
// summary cannot send a follower to copy from a replica that is not actually
// complete.
//
// When the lease cannot be read, the roles decide, and only among the entries the election counts —
// see Present — so that a dead leader's Leader, which stands until the replica that replaced it
// publishes, does not make its replica a source. Not while the lease can be read: a holder it shows
// live has renewed within the last minute, and only a replica that is there renews, while a node is
// counted lost only well after its replica stopped renewing — see nodeLostAfter in cmd. All the filter
// could do then is drop a holder that renews while its kubelet does not report, which the controller
// still routes every pull to.
//
// The second result says the roles decided because the lease could not be read — see rearranged.
func (l *Loop) currentLeader(ctx context.Context) (*Leader, bool, error) {
	storage := &registryv1alpha1.RegistryStorage{}
	if err := l.Client.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage); err != nil {
		if errors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("reading the storage state: %w", err)
	}

	lease := Lease{}
	if l.ReadLease != nil {
		lease = l.ReadLease(ctx)
	}
	replicas := storage.Status.Replicas
	if !lease.Known && l.Present != nil {
		replicas = l.Present(ctx, replicas)
	}
	return LeaderOf(l.Node, replicas, lease), l.ReadLease != nil && !lease.Known, nil
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

// keys is the fingerprint of the set the cluster needs now — see SetKey — and of what is ahead of it.
func (l *Loop) keys(ctx context.Context) (string, string, error) {
	return CurrentKeys(ctx, l.Client)
}

// CurrentSetKey is the fingerprint of the set the cluster needs now, read from the API server: what
// a replica's Full is compared against, both by the replica reporting it and by every replica
// deciding who may lead.
func CurrentSetKey(ctx context.Context, c client.Client) (string, error) {
	set, _, err := CurrentSet(ctx, c)
	return set, err
}

// CurrentSet is CurrentSetKey and, beside it, the release deployed now, from the same reading of the
// releases: a step-down judges a store by that release's tag only against the set it belongs to — see
// WhyStepAside. Read twice, an update coming between the two readings would have a full replica
// checked for a release it has no reason to hold yet.
func CurrentSet(ctx context.Context, c client.Client) (string, string, error) {
	releases, set, _, err := currentReading(ctx, c)
	return set, releases.Deployed, err
}

// CurrentKeys is CurrentSetKey and, beside it, the AheadKey of what is pending now.
func CurrentKeys(ctx context.Context, c client.Client) (string, string, error) {
	_, set, ahead, err := currentReading(ctx, c)
	return set, ahead, err
}

// currentReading is the releases the cluster has now, and the SetKey and AheadKey made of them.
func currentReading(ctx context.Context, c client.Client) (gc.Releases, string, string, error) {
	releases, err := gc.FromCluster(ctx, c)
	if err != nil {
		return gc.Releases{}, "", "", err
	}
	modules, err := gc.ModulesFromCluster(ctx, c)
	if err != nil {
		return gc.Releases{}, "", "", err
	}
	catalogue, err := gc.CatalogueFromCluster(ctx, c)
	if err != nil {
		return gc.Releases{}, "", "", err
	}
	return releases, SetKey(releases, modules, catalogue), AheadKey(releases, modules), nil
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
//
// Carried from the status, which a replica whose store was lost does not correct by itself: see
// StepDown and TakeBackFull for the leader's way out, and recheckStore for everyone's. Nor is it
// carried past the store's own count: a Full beside a total of the disk that cannot hold it — at least
// half of what it verified absent altogether — is taken back on the pass that measured the total, on the
// leader and on a follower alike — see refuteCarriedFull.
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
// on the node and outlives the pod — unless the node lost it, a wiped local_data say, and the kept
// report is then wrong until this replica reads its store. A leader beside a replica full for the
// current set finds that out from the release's tag and a sample of the set — see ShortOfSet — and,
// holding the lease when it starts, from the tag before the registry starts — see stepDownAtStart in
// the command; any replica at its next recheck — see recheckStore. Bounded, because a registry
// that never comes up is a failure, and past the grace it is reported as one exactly as before.
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

// publishAddress writes where this replica serves into its entry, once per process and as soon as
// its registry answers, instead of waiting for the end of the first pass.
//
// The entry outlives the pod, so a replaced replica's entry still carries the address its
// predecessor reported until something writes it. The registry controller builds the layout and the
// push endpoint from that address, and storageupdate waits for it before replacing the next replica.
// It matters during the update to the store's current ports, when the predecessor served on another
// port. Retried on the next pass when it fails.
func (l *Loop) publishAddress(ctx context.Context) {
	if l.addressPublished || l.Publisher == nil || l.Node == "" || l.ReportedAddress == "" {
		return
	}
	state := report.State{Node: l.Node, Address: l.ReportedAddress}
	if err := l.Publisher.PublishAddress(ctx, state); err != nil {
		l.Log.Warn("cannot report where this replica serves; the previous report stays until this pass ends",
			"address", l.ReportedAddress, "error", err.Error())
		return
	}
	l.addressPublished = true
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
// replica would go on reporting the last pass that did copy. Read again here — see reportHeld — and
// reported as found; a replica that is no longer complete makes the controller raise needSync, and
// the copy that follows repairs it.
//
// The count is taken from this replica's own disk (fill.Take), never from a leader. The set it is
// counted against is read through this replica's own serving listener, and with an upstream that
// listener is the pull-through: the reading can come from the upstream, and fails while the upstream
// does — and a store that lost the release's tag gets it back from the upstream that way.
//
// Not on every pass, because reading the set means reading the platform image to its end, and an
// idle cluster has no reason to spend that every half a minute. The first idle pass of a process
// that gets past storeStarting does it, which covers a restart; after that, every StoreRecheck. A
// reading that fails changes nothing and is tried again after storeRecheckRetry, not on the next
// pass. Rare on purpose, which leaves a store lost between two rechecks reported as it was. A leader
// whose report says it holds the set does not wait for this when its store has lost the set — the
// release, or most of a sample of it: its store is looked at about once a minute, and found short it
// hands the lease to a replica full for the current set, or, with nobody else full, keeps the lease and
// takes its report back — see ShortOfSet. Nor does any replica whose idle pass measures a total of the
// store that cannot hold the Full it carries over — see refuteCarriedFull. A smaller loss, or one while
// a pass copies the set in, waits for this on a leader too.
//
// It can take a fingerprint away and never gives one: a reading that finds the store complete says
// what is on the disk now, not that a pass copied the set the fingerprint names — a follower reads its
// own disk, and only a replication says it holds what its leader holds. The fingerprint stays whatever
// the last copying pass made it, or goes.
//
// Reports whether it read the store: what this pass publishes is then this process's own reading —
// see storeRead.
func (l *Loop) recheckStore(ctx context.Context, state *report.State, reported string) bool {
	every := l.StoreRecheck
	if every <= 0 {
		every = storeRecheck
	}
	if last := l.lastStoreCheck.Load(); last != 0 && time.Since(time.Unix(0, last)) < every {
		return false
	}
	// Counted as a check whether or not it could read the store, so that a reading that fails is tried
	// again in storeRecheckRetry rather than on every pass of the loop.
	l.lastStoreCheck.Store(time.Now().Add(min(storeRecheckRetry, every) - every).UnixNano())
	if !l.reportHeld(ctx, state) {
		return false
	}
	l.lastStoreCheck.Store(time.Now().UnixNano())

	state.VerifiedSet = ""
	if state.Full && state.Error == "" {
		state.VerifiedSet = reported
	}
	return true
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

// localRegistry is how to talk to this replica's own registry, through either of its listeners, over
// localTransport.
func (l *Loop) localRegistry() (fill.Registry, error) {
	transport, err := l.localTransport()
	if err != nil {
		return fill.Registry{}, fmt.Errorf("local storage: %w", err)
	}

	return fill.Registry{
		Address:    l.LocalAddress,
		Repository: trimSlashes(constant.Path),
		Options:    fill.TransportOptions(transport, l.LocalUser, l.LocalPassword),
	}, nil
}

// localTransport is the transport every client of this replica's own registry is built on: the
// registry protocol's, through localRegistry and writeRegistry, and the plain HTTP the health watch
// asks with, in probeRegistry.
//
// One place, so that whatever a listener comes to ask of a client — a client certificate on the write
// endpoint — is given here once and goes with every client. Given to the fill's clients alone, it would
// leave the probe refused where the fill is admitted: five minutes later every replica would report its
// registry as not answering, and the election would read that as a reason to move the lease.
func (l *Loop) localTransport() (*http.Transport, error) {
	if l.LocalTransport != nil {
		return l.LocalTransport()
	}
	return fill.ClientTransport(l.LocalCA, l.LocalClientCertificate)
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
// with no proxy in front of it. It admits two clients and refuses every other handshake: the ingress
// controller proxying `d8 mirror push`, and this syncer, with the client certificate localTransport
// carries. The credentials are checked after that, as on the serving listener — the certificate
// decides who may ask, the sync account what it may do.
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

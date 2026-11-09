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
	"strings"
	"time"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/registry-syncer/internal/fill"
	"github.com/deckhouse/registry-syncer/internal/report"
)

// A leader whose store was lost kept the lease, and the status said it was full.
//
// The store's data lives on the node, and a wiped local_data, a disk replaced or cleaned by hand, takes
// it away without the syncer noticing. A restarted syncer keeps the previous report while its registry
// starts (storeStarting), an idle pass carries Full over (carryOverCount), and only the recheck reads the
// store again — rarely, on purpose: reading the set means reading the platform image to its end, and the
// cluster is idle most of its life. Until then the leader's report stood. With an upstream the incumbent
// rule kept it leading beside a full follower, and every node's pull went past its empty cache to the
// upstream; in air-gap it answered 404. Worst, a leader whose pod was recreated takes its own lease back
// at once — the lease names the node — and the controller's LeaderFull read its record from the previous
// process: with the air-gap transition requested, the upstream could be dropped with an empty leader,
// which cannot be undone.
//
// The rechecks stay as rare as they are. What changes is that the replica holding the lease, or about to
// take it, looks at a sample of what it serves: the deployed release's tag and up to stepDownSample
// manifests of the set, read off its own disk. While another replica holds the whole set, a store found
// short takes its report back — Full, the count to zero, the fingerprints — hands the lease over and
// copies the set from the new leader like any follower.
//
// With nobody else full there is nothing to hand over to, and the holder keeps the lease: a replica that
// leads while empty is still the one that fills. Its report goes back all the same. That report is what
// the air-gap transition is gated on, and the reconcile that first sees the transition requested drops
// the upstream on the leader's Full at once. On one master, or while the followers are still copying, a
// store wiped under a running syncer left that Full standing until the recheck, up to six hours, and the
// upstream could be dropped on an empty leader. Taken back, the controller holds the upstream and asks
// for a fill, the leader makes it under the lease it kept — in air-gap it counts — and the pass that read
// the store says what it holds.
//
// An idle pass does not wait for the sample when the store's own count already says it: a Full carried
// over beside a fresh total of the disk that cannot hold it is taken back on that pass, on the leader,
// and withdrawn on a follower — see refutedByCount.

// stepDownSample is how many manifests of the set one check reads off the disk.
const stepDownSample = 32

// stepDownMinChecked is the fewest of them a check must have looked at for the share missing to count:
// a set read for another release, or one that is mostly members the store could not resolve, leaves too
// few to say anything.
const stepDownMinChecked = 4

// stepDownCheckTimeout is how long one check may take, unless Loop.StoreSampleTimeout says otherwise. A
// tag link and a few dozen small files, most of them in the page cache; a disk that needs longer than
// this is for the health watch to judge, and the check finds nothing.
const stepDownCheckTimeout = 5 * time.Second

// stepDownRecheck is how long a check that found nothing stands before the next one. While the leader
// reports the set this is the whole cost: one tag link, 32 manifests and the layers they name, once a
// minute, on the leader — beside full replicas or alone — and, in air-gap where every full replica
// stands, on those too.
const stepDownRecheck = time.Minute

// Shortfall is what a check of this replica's store found: whether the deployed release resolves by its
// tag, and how many of a sample of the set it held.
//
// Or, Counted, what the store's own count refutes: Total digests on the disk altogether, against the
// Checked its report says it verified of the set, Missing of them at the least — see refutedByCount.
// Release is not looked at then.
type Shortfall struct {
	Release     string
	ReleaseHeld bool
	Checked     int
	Missing     int
	Counted     bool
	Total       int
}

// refutedByCount is the third finding, beside ShortOfSet's two, and the one that costs nothing: a Full
// an idle pass carried over from the status, about the set the cluster needs now, beside a count of the
// store taken on that same pass that cannot hold it — at least half of the digests the report says it
// verified are not on the disk at all, the same bar as the sample's.
//
// Within one survey the two cannot disagree that way: the count of the set is a subset of the count of
// everything (fill.Take). An idle pass, though, takes its Full and the count of the set from the status
// and the total from a walk of the disk at most a minute old, and on a stand a store wiped under the
// leader published full: true with 445 verified and 2 in total for a minute, until the sample's second
// check took it back. The count alone says it at the first idle pass whose walk is after the loss.
//
// At least half, not any shortfall: what the set counts and what the walk counts are both read off the
// disk, but by two readings up to a minute apart, and a few manifests between them are a collection or
// a module that came and went, not a lost store. A loss smaller than that is left to the sample and the
// recheck, as before.
//
// The bar is wide for another reason too: the two numbers are distinct manifests only as reportHeld
// publishes them. A fill or a replication whose reportHeld failed publishes the copier's own count
// (fill.Report.Written), which goes up once per reference, a tag that aliases another's digest
// included, and can stand a little above the distinct manifests the walk counts. Half of it absent is
// not that; and a Full taken back wrongly costs a refill, not the cluster.
//
// Only a Full about the current set, by name: one about an older set is Reverify's to take away, and
// a set that cannot be told is no evidence either way.
func refutedByCount(state *report.State, setKey string) (Shortfall, bool) {
	if setKey == "" || !state.Full || state.VerifiedSet != setKey || state.VerifiedDigests <= 0 {
		return Shortfall{}, false
	}
	verified, total := int(state.VerifiedDigests), int(state.TotalDigests)
	if 2*total > verified {
		return Shortfall{}, false
	}
	return Shortfall{Counted: true, Total: total, Checked: verified, Missing: verified - total}, true
}

// refuteCarriedFull acts on refutedByCount, on an idle pass that took the count of the store fresh and
// read nothing else of it: a pass that copied, counted or rechecked published what it read, and a Full
// from a reading is not carried over.
//
// The lease holder takes its report back — see TakeBackFull — and the pass then publishes nothing of
// what it carried over (see once). It keeps the lease with nobody else full, and fills or counts; with
// another replica full for the set, the next review steps it down in that replica's favour — see
// stepDownOwed. A pass whose lease moved since it began publishes nothing at all — see once — and is left
// alone here, so that a replica that no longer leads is not marked as having taken back a lead.
//
// A follower has no lease to keep or give, and nothing reads its store on its behalf: a stale Full on one
// was left to the recheck, six hours, and the election could hand the lease to it. Its pass publishes
// itself not full, with the count and the fingerprints withdrawn and nothing held ahead, and its next
// idle pass reads the store again (recheckStore) to say what it holds. The controller, reading it not
// full, raises needSync, and a replication from the leader repairs it.
func (l *Loop) refuteCarriedFull(ctx context.Context, state *report.State, isLeader bool, setKey string) {
	short, refuted := refutedByCount(state, setKey)
	if !refuted || l.Lost() {
		return
	}
	if isLeader {
		if l.Leadership.IsLeader() {
			l.TakeBackFull(ctx, short)
		}
		return
	}

	l.Log.Warn("this replica's report says it holds the set, but its store holds fewer digests than that "+
		"report verified, so it reports itself not full and reads its store again on its next pass",
		"store_total", short.Total, "verified", short.Checked, "set", setKey)
	state.Full, state.VerifiedSet, state.VerifiedDigests = false, "", 0
	l.ForgetAhead()
	l.lastStoreCheck.Store(0)
}

// sampleOutcome is what one sample of the store came back with, and for which release.
type sampleOutcome struct {
	release string
	result  fill.SampleResult
	err     error
}

// WhyStepDown names another replica that holds the whole set the cluster needs now, by a report nobody
// should doubt, or nil when there is none: the only condition under which a leader short of the set
// hands the lease over — to that replica.
//
// Stricter than isFull — see fullForSet. A report that names no set, or a set that cannot be told, is
// not taken at its word here: a step-down that went to a replica full for the previous set, or to one
// whose registry or disk fails, would hand the cluster to a source no better than the one it left.
// Stricter only that way: a replica it names, isFull counts full too, so the rule does not hand the lease
// straight back; and it decides only where the lease goes, never whether this replica is short — that is
// ShortOfSet's evidence, whatever this replica's report says. And not this replica's own report: with
// nobody else full, leading while short is still how the set gets filled — the holder keeps the lease
// and only takes its report back, see TakeBackFull.
func WhyStepDown(
	self string, replicas []registryv1alpha1.StorageReplicaStatus, currentSet string,
) *registryv1alpha1.StorageReplicaStatus {
	for i := range replicas {
		replica := &replicas[i]
		if replica.Node != self && fullForSet(replica, currentSet) {
			return replica
		}
	}
	return nil
}

// fullForSet reads a report as WhyStepDown takes it: full for the set the cluster needs now, by name, with
// no error and no health error. Not when the set cannot be told.
func fullForSet(replica *registryv1alpha1.StorageReplicaStatus, currentSet string) bool {
	return currentSet != "" && replica.Full && replica.Error == "" && replica.HealthError == "" &&
		replica.VerifiedSet == currentSet
}

// WhyStepAside says why this replica, which the rule lets stand, should not after all: another replica
// holds the whole set — see WhyStepDown — and this one's store does not — see ShortOfSet. Empty when it
// should stand.
//
// currentSet and deployed come from one reading of the releases — see CurrentSet — and nothing here
// reads the API server: the store is judged by the tag of the release the set was keyed on, and while a
// clean check stands the review costs no more than it did.
//
// holds says whether this replica holds the lease, and is asked only when the store was found short, or,
// with nobody else full, before looking at all. A holder steps down — see StepDown — and the second
// result tells the caller to give the lease up where the elector will not: a replica that does not stand
// but whose name the lease still carries, the process that has just started above all. One that does not
// hold the lease publishes nothing: it does not stand, and is left to its recheck — the rechecks stay as
// rare as they were.
//
// With nobody else full the replica stands whatever its store holds, and a holder whose own report says
// it holds the set has that report checked the same way: found short, it takes the report back and keeps
// the lease — see takeBackWhenShort.
//
// Once stepped down, the replica stays aside without looking again while the other is full, until a
// pass copies into its store or counts it — see Lost. A holder that took its report back and kept the
// lease does the same as soon as another replica is full for the set: that replica is the better source,
// and the step-down it owed is made then — role and all, see stepDownOwed — without reading the disk
// again. Unless a pass has read the store since the review saw it lost: the take-back is over then, and
// the replica stands until the next review looks at its store afresh.
func (l *Loop) WhyStepAside(
	ctx context.Context, replicas []registryv1alpha1.StorageReplicaStatus, currentSet, deployed string,
	holds func() bool,
) (string, bool) {
	full := WhyStepDown(l.Node, replicas, currentSet)
	if full == nil {
		l.takeBackWhenShort(ctx, replicas, currentSet, deployed, holds)
		return "", false
	}
	why := fmt.Sprintf("%s holds the whole image set and this replica's store does not hold it", full.Node)
	if l.Lost() {
		holder := holds()
		if holder && !l.stepDownOwed(ctx, full) {
			// A pass read the store since Lost was asked and published what it holds: the step-down or
			// the take-back is over, and the next review judges the store afresh.
			return "", false
		}
		return why, holder
	}

	if deployed == "" {
		// No release, no tag to look for, and a sample of a set that cannot be told: no evidence.
		return "", false
	}
	short, found := l.ShortOfSet(ctx, deployed)
	if !found {
		return "", false
	}
	if !holds() {
		return why, false
	}
	l.StepDown(ctx, short, full)
	return why, true
}

// takeBackWhenShort is the check with nobody else full: the holder, while its own report says it holds
// the set the cluster needs now — see fullForSet — has its store looked at as ShortOfSet looks at it, and
// takes the report back when the store is found short — see TakeBackFull. It keeps the lease, and the
// caller's answer stays that it should stand.
//
// The same check, with its throttle and both of its findings. On a stand, a store wiped under a leader
// with an upstream had the release's tag back within 35 s, written by a pull through its own
// pull-through, and only the sample of the set told the loss.
//
// Nothing is read for a replica that does not hold the lease — a follower is left to its recheck, as it
// always was — nor for one whose report says nothing to take back: not full, full for another set or for
// none, with an error, or taken back already and not read since. Nor while a pass copies the set into
// the store: the store is changing under the sample, and a report that is being earned again is not the
// one in question. A copy of what is ahead of the set does not count as one: it adds to the store, takes
// nothing of the set away, and runs, paced, for as long as the next release takes to copy.
func (l *Loop) takeBackWhenShort(
	ctx context.Context, replicas []registryv1alpha1.StorageReplicaStatus, currentSet, deployed string,
	holds func() bool,
) {
	own := ownReport(l.Node, replicas)
	if own == nil || !fullForSet(own, currentSet) || deployed == "" || l.Lost() || l.filling.Load() {
		return
	}
	if !holds() {
		return
	}
	short, found := l.ShortOfSet(ctx, deployed)
	if !found {
		return
	}
	l.TakeBackFull(ctx, short)
}

// ShortOfSet checks this replica's store against the set it serves, and reports what it found and
// whether that is reason to hand the lease over — or, with nobody else full, to take the report back.
//
// Two findings are. The deployed release does not resolve by its tag on the disk: every replica reads
// the set out of that image, a store found full holds it by definition (see reportHeld), and a follower
// replicating from a leader without it gets MANIFEST_UNKNOWN — conclusive at once. Or at least half of a
// sample of the set is missing, on two checks in a row, with enough of it looked at to tell: a store
// that lost the set loses most of any sample, while a module enabled a moment ago, or an update half
// copied, misses a few manifests and has a set other than the full replica's anyway.
//
// The first is the one that fires on a store lost under a running leader, with an upstream or without:
// nothing on an idle leader reads the release again — the count is carried over, and so is the size of
// the set. It is also the only one before the registry has started. With an upstream a pass that reads
// the set — the first idle pass of a new process, whose recheck it is, the 6-hourly recheck, a fill or a
// replication — reads the release through this replica's own pull-through, which writes the tag back.
// The second is for a store that holds the release and not the set behind it: the tag written back that
// way, or a store that lost part of its data and kept the release — which is what the nodes see, their
// pulls of the set going past the cache.
//
// The sample is of the set the last pass read, and only when it was read for the deployed release; it
// is none otherwise, and only the tag is looked at. Read in a goroutine of its own, one at a time, as
// the health watch probes the disk: a check that does not come back in time, or a store that cannot be
// read, is no finding. A check that finds nothing stands for stepDownRecheck.
func (l *Loop) ShortOfSet(ctx context.Context, deployed string) (Shortfall, bool) {
	l.sampleMu.Lock()
	defer l.sampleMu.Unlock()

	if !l.sampleCleanAt.IsZero() && time.Since(l.sampleCleanAt) < stepDownRecheck {
		return Shortfall{}, false
	}

	result, err := l.sampleStore(ctx, deployed)
	if err != nil {
		l.sampleShort = 0
		if ctx.Err() == nil {
			l.Log.Debug("cannot read a sample of the set off the store, so it is taken as held", "error", err.Error())
		}
		return Shortfall{}, false
	}

	short := Shortfall{Release: deployed, ReleaseHeld: result.ReleaseHeld, Checked: result.Checked, Missing: result.Missing}
	switch {
	case !result.ReleaseHeld:
		l.sampleShort = 0
		return short, true
	case result.Checked >= stepDownMinChecked && 2*result.Missing >= result.Checked:
		l.sampleShort++
		if l.sampleShort < 2 {
			return short, false
		}
		l.sampleShort = 0
		return short, true
	default:
		l.sampleShort = 0
		l.sampleCleanAt = time.Now()
		return short, false
	}
}

// sampleStore reads a sample of the set off the disk — see ShortOfSet — waiting for it no longer than
// the check's timeout. A sample still running from an earlier check is waited on rather than another
// started, so that a disk stuck in the kernel does not gather goroutines.
func (l *Loop) sampleStore(ctx context.Context, deployed string) (fill.SampleResult, error) {
	timeout := l.StoreSampleTimeout
	if timeout <= 0 {
		timeout = stepDownCheckTimeout
	}

	if l.sampling == nil {
		sample := l.StoreSample
		if sample == nil {
			sample = fill.Sample
		}
		done := make(chan sampleOutcome, 1)
		root, scope := l.dataDir(), strings.Trim(constant.Path, "/")
		go func() {
			// The set is copied in here too: the pass holds declaredMu while it reads the set, which can
			// take as long as the registry takes to answer.
			result, err := sample(root, scope, deployed, l.sampleOfSet(deployed))
			done <- sampleOutcome{release: deployed, result: result, err: err}
		}()
		l.sampling, l.sampleStarted = done, time.Now()
	}

	finished := func(outcome sampleOutcome) (fill.SampleResult, error) {
		l.sampling = nil
		if outcome.err == nil && outcome.release != deployed {
			return fill.SampleResult{}, fmt.Errorf("the sample was taken for %s, and %s is deployed now",
				outcome.release, deployed)
		}
		return outcome.result, outcome.err
	}

	select {
	case outcome := <-l.sampling:
		return finished(outcome)
	default:
	}

	wait := time.NewTimer(max(timeout-time.Since(l.sampleStarted), 0))
	defer wait.Stop()

	select {
	case outcome := <-l.sampling:
		return finished(outcome)
	case <-wait.C:
		return fill.SampleResult{}, fmt.Errorf("reading a sample of the set off %s has not finished within %s",
			l.dataDir(), timeout)
	case <-ctx.Done():
		return fill.SampleResult{}, ctx.Err()
	}
}

// sampleOfSet is up to stepDownSample digests of the set the last pass read, when it read it for the
// deployed release; none otherwise. Taken in the order the map gives, which differs from one check to
// the next, so that two checks do not read the same manifests.
func (l *Loop) sampleOfSet(deployed string) []string {
	l.declaredMu.Lock()
	defer l.declaredMu.Unlock()

	if l.declared == nil || l.declaredFor != deployed {
		return nil
	}
	sample := make([]string, 0, stepDownSample)
	for key := range l.declared {
		if len(sample) == stepDownSample {
			break
		}
		if fill.Resolved(key) {
			sample = append(sample, key)
		}
	}
	return sample
}

// StepDown takes back what this replica's report says it holds, because the store was found short of
// the set full holds whole — see ShortOfSet — and leaves the lease to be given up: the caller does that,
// through the elector or past it.
//
// What is published, in one write: Full false, the count zero, no verifiedSet, aheadSet or source, role
// Follower, and safeToDropUpstream and allReplicasFull withdrawn — see report.Publisher.Demote. The
// count goes to zero rather than to a figure from the sample, which is not a count; the next pass that
// reads the store publishes the real one. What is ahead of the set goes too: a copy ahead held on a
// store that lost the set is not held either, and followers sent to copy it from here would find
// nothing.
//
// Until a pass copies into the store or counts it — a replication from the new leader, which the
// controller asks for as soon as this replica reports itself not full, or a count — the replica stays
// stepped down: the passes publish nothing of what the status carried over (see once), and the
// election judges it by the demotion (see WithOwnLoss), so that the incumbent rule does not hand it the
// lease straight back. A demotion that could not be written is written by the next pass the same way.
//
// Said once, at Warn, with the evidence: the operator's question afterwards is why the lease moved.
//
// A holder that took its report back with nobody else full — see TakeBackFull — steps down too, once
// another replica is full, but through stepDownOwed rather than here.
func (l *Loop) StepDown(ctx context.Context, short Shortfall, full *registryv1alpha1.StorageReplicaStatus) {
	l.reportMu.Lock()
	defer l.reportMu.Unlock()

	if l.lost.Swap(true) {
		return
	}
	l.losses.Add(1)
	l.ForgetAhead()
	l.complete.Store(false)
	l.surveyStale.Store(true)

	l.Log.Warn("this replica leads, but its store does not hold the set that another replica holds whole, "+
		"so it gives up the lease and copies the set from the new leader",
		"release", short.Release, "release_held", short.ReleaseHeld,
		"sampled", short.Checked, "missing", short.Missing,
		"full_replica", full.Node, "set", full.VerifiedSet)
	l.demote(ctx)
}

// stepDownOwed is the step-down a holder that took its report back — see TakeBackFull — owes once
// another replica is full for the set: what it took back stays taken back, and what it still owes is the
// role, the lease and the line saying why they moved. The evidence was logged when the report went back;
// this line names the replica the lease goes to.
//
// Decided under reportMu, not on what the caller read before: a pass that read the store and published
// what it holds in between has ended the take-back, and a step-down then would demote a reading just
// published, with no evidence to give. Asked by a lost holder on every review, owed or not — one lock,
// nothing read — and reports whether the replica is still lost, the step-down owed or not.
func (l *Loop) stepDownOwed(ctx context.Context, full *registryv1alpha1.StorageReplicaStatus) bool {
	l.reportMu.Lock()
	defer l.reportMu.Unlock()

	if !l.tookBack.Swap(false) {
		return l.lost.Load()
	}
	// lost is set with tookBack and cleared with it, under this lock: the replica stays lost.
	l.ForgetAhead()
	l.complete.Store(false)

	l.Log.Warn("this replica leads with a store it found short of the set, and another replica now holds "+
		"the set whole, so it gives up the lease and copies the set from the new leader",
		"full_replica", full.Node, "set", full.VerifiedSet)
	l.demote(ctx)
	return true
}

// demote publishes a step-down — see report.Publisher.Demote. Under reportMu.
func (l *Loop) demote(ctx context.Context) {
	if l.Publisher == nil {
		return
	}
	if err := l.Publisher.Demote(ctx, report.State{Node: l.Node}); err != nil {
		l.Log.Warn("reporting that this replica does not hold the set failed; its next pass reports it",
			"error", err.Error())
	}
}

// TakeBackFull takes back what this replica's report says it holds, as StepDown does, and keeps the
// lease: the store was found short of the set — see ShortOfSet — and no other replica holds the set
// whole, so there is nobody to hand over to, and this replica is still the one that fills — see
// takeBackWhenShort. Or an idle pass found its carried-over Full refuted by the store's own count — see
// refuteCarriedFull — without asking whether another replica is full: if one is, the next review steps
// this one down in its favour, as it does any holder that took its report back.
//
// What is published, in one write: Full false, the count zero, no verifiedSet, aheadSet or source, and
// safeToDropUpstream and allReplicasFull withdrawn — see report.Publisher.TakeBack. The role stays as
// reported: Leader, for a holder whose pass has run under the lease, and a holder whose first pass has
// not is given the role by that pass, not here — a take-back claims nothing, and setting Leader without
// taking it from the entry that still says so would leave two.
//
// The controller, reading the leader not full, holds the upstream and raises needSync, and the leader's
// next pass fills — the reported set gone, Reverify makes it fill even before the controller asks — or,
// in air-gap and during the transition, counts its store. The pass that read the store
// publishes what it holds and ends this, as a reading ends a step-down. That pass walks the store again
// rather than answer from the survey of the last minute — see surveyStale: a count on every pass, as in
// air-gap and in the transition, would otherwise publish the walk taken before the loss, Full and all,
// end the take-back, and have the next review take the report back again.
//
// Until then the replica is lost as a step-down leaves it — see Lost — so that a pass that carried Full
// over before this write does not put it back (see once) and the election does not read a Full it took
// back (see WithOwnLoss). With nobody full the incumbent rule keeps it leading by the lease, or, while
// the lease cannot be read, by the role it kept. Should another replica become full for the set in the
// meantime, WhyStepAside steps it down in that replica's favour.
//
// Said once, at Warn, with the evidence, as a step-down is: the operator's question afterwards is why the
// leader stopped reporting itself full and filled again.
func (l *Loop) TakeBackFull(ctx context.Context, short Shortfall) {
	l.reportMu.Lock()
	defer l.reportMu.Unlock()

	if l.lost.Swap(true) {
		return
	}
	l.tookBack.Store(true)
	l.losses.Add(1)
	l.ForgetAhead()
	l.complete.Store(false)
	l.surveyStale.Store(true)

	if short.Counted {
		// Reached from an idle pass, which does not know whether another replica is full: the next review
		// hands the lease over if one is, and the line does not claim otherwise.
		l.Log.Warn("this replica leads, but its store holds fewer digests than its report says it verified, "+
			"so it takes the report back and fills or counts the store again; it keeps the lease unless "+
			"another replica holds the set whole",
			"store_total", short.Total, "verified", short.Checked, "missing", short.Missing)
	} else {
		l.Log.Warn("this replica leads, but its store does not hold the set its report says it holds, and no other "+
			"replica holds the set whole, so it keeps the lease, takes the report back and fills or counts the store again",
			"release", short.Release, "release_held", short.ReleaseHeld,
			"sampled", short.Checked, "missing", short.Missing)
	}

	if l.Publisher == nil {
		return
	}
	if err := l.Publisher.TakeBack(ctx, report.State{Node: l.Node}); err != nil {
		l.Log.Warn("reporting that this replica does not hold the set failed; its next pass reports it",
			"error", err.Error())
	}
}

// Lost reports whether this replica stepped down, or took its report back, for a store that did not hold
// the set, and no pass has copied into the store or counted it since — see StepDown and TakeBackFull.
func (l *Loop) Lost() bool { return l.lost.Load() }

// TookBack reports whether this replica took its report back and kept the lease, nobody else being full,
// and no pass has copied into the store or counted it since — see TakeBackFull. It then owes a step-down
// once another replica holds the whole set — see stepDownOwed — even where the rule, rather than
// WhyStepAside, is the first to say it should not stand.
func (l *Loop) TookBack() bool { return l.tookBack.Load() }

// WithOwnLoss is the reports the election reads, with this replica's own entry as its step-down left
// it: not full, nothing verified, no fingerprints — see StepDown.
//
// The status catches up with the demotion in one write, or with the next pass if that write failed; the
// replica deciding about itself does not wait for either: with Full still standing in its own entry it
// would count itself full, and stand beside the replica it has just handed the lease to. The incumbent
// rule is left to the caller — see WhyStepAside — since a lease not yet given up still names it.
//
// Or as its take-back left it — see TakeBackFull. Its own Full cleared, nobody is full while no other
// replica is, and the incumbent rule keeps the holder leading; once another replica is full, the rule,
// or WhyStepAside, steps it aside.
func WithOwnLoss(
	self string, replicas []registryv1alpha1.StorageReplicaStatus, lost bool,
) []registryv1alpha1.StorageReplicaStatus {
	if !lost {
		return replicas
	}
	judged := make([]registryv1alpha1.StorageReplicaStatus, len(replicas))
	copy(judged, replicas)
	for i := range judged {
		if judged[i].Node != self {
			continue
		}
		judged[i].Full = false
		judged[i].VerifiedDigests = 0
		judged[i].VerifiedSet = ""
		judged[i].AheadSet = ""
	}
	return judged
}

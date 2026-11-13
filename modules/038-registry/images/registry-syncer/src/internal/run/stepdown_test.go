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
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-syncer/internal/fill"
	"github.com/deckhouse/registry-syncer/internal/report"
	"github.com/deckhouse/registry-syncer/internal/storetest"
)

// TestWhyStepDownWantsAReplicaFullForThisSet: a leader short of the set hands the lease over only to a
// replica whose report nobody should doubt — full for the set the cluster needs now, with no error and
// no health error — and never to itself.
func TestWhyStepDownWantsAReplicaFullForThisSet(t *testing.T) {
	const set = "set-now"
	full := registryv1alpha1.StorageReplicaStatus{Node: "master-1", Full: true, VerifiedSet: set}
	leader := registryv1alpha1.StorageReplicaStatus{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedSet: set,
	}

	got := WhyStepDown("master-0", []registryv1alpha1.StorageReplicaStatus{leader, full}, set)
	require.NotNil(t, got)
	assert.Equal(t, "master-1", got.Node)
	assert.True(t, isFull(got, set), "the rule counts it full too, and does not hand the lease straight back")

	for name, other := range map[string]registryv1alpha1.StorageReplicaStatus{
		"full for the previous set": {Node: "master-1", Full: true, VerifiedSet: "set-before"},
		"full for no set it names":  {Node: "master-1", Full: true},
		"not full":                  {Node: "master-1", VerifiedSet: set, VerifiedDigests: 450},
		"full with an error":        {Node: "master-1", Full: true, VerifiedSet: set, Error: "a copy failed"},
		"full and failing":          {Node: "master-1", Full: true, VerifiedSet: set, HealthError: "a disk failed"},
	} {
		assert.Nil(t, WhyStepDown("master-0", []registryv1alpha1.StorageReplicaStatus{leader, other}, set), name)
	}
	assert.Nil(t, WhyStepDown("master-0", []registryv1alpha1.StorageReplicaStatus{leader, full}, ""),
		"a set that cannot be told")
	assert.Nil(t, WhyStepDown("master-0", []registryv1alpha1.StorageReplicaStatus{leader}, set),
		"this replica is the only full one: nobody to hand over to")
}

// TestWithOwnLossChangesOnlyItsOwnEntry: the election judges this replica by its step-down before the
// status catches up, and the others as they report.
func TestWithOwnLossChangesOnlyItsOwnEntry(t *testing.T) {
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true,
			VerifiedDigests: 453, VerifiedSet: "abc", AheadSet: "def", Error: "kept"},
		{Node: "master-1", Full: true, VerifiedDigests: 453, VerifiedSet: "abc", AheadSet: "def"},
	}

	assert.Equal(t, replicas, WithOwnLoss("master-0", replicas, false), "nothing lost, nothing changed")

	judged := WithOwnLoss("master-0", replicas, true)
	assert.Equal(t, registryv1alpha1.StorageReplicaStatus{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Error: "kept",
	}, judged[0])
	assert.Equal(t, replicas[1], judged[1])
	assert.True(t, replicas[0].Full, "the caller's reports are not changed under it")
}

// sampler is a disk for ShortOfSet: what each sample finds, and how often one was taken.
type sampler struct {
	mu      sync.Mutex
	results []fill.SampleResult
	err     error
	hang    chan struct{}
	taken   atomic.Int32
	last    []string
}

func (s *sampler) sample(_, _, _ string, digests []string) (fill.SampleResult, error) {
	s.taken.Add(1)
	if s.hang != nil {
		<-s.hang
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.last = digests
	if s.err != nil {
		return fill.SampleResult{}, s.err
	}
	result := s.results[0]
	if len(s.results) > 1 {
		s.results = s.results[1:]
	}
	return result, nil
}

func (s *sampler) digests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last
}

func samplingLoop(t *testing.T, disk *sampler) *Loop {
	t.Helper()
	loop, _, _ := newLoop(t, true, nil)
	loop.DataDir = t.TempDir()
	loop.StoreSample = disk.sample
	return loop
}

// expire makes the last clean check old enough to be repeated.
func expire(loop *Loop) {
	loop.sampleMu.Lock()
	loop.sampleCleanAt = time.Now().Add(-stepDownRecheck)
	loop.sampleMu.Unlock()
}

// TestShortOfSetActsOnTheReleaseAtOnce: a store without the deployed release's tag is short of the set
// by definition, and the first check says so.
func TestShortOfSetActsOnTheReleaseAtOnce(t *testing.T) {
	loop := samplingLoop(t, &sampler{results: []fill.SampleResult{{ReleaseHeld: false}}})

	short, found := loop.ShortOfSet(context.Background(), "v1.76.6")
	assert.True(t, found)
	assert.Equal(t, Shortfall{Release: "v1.76.6"}, short)
}

// TestShortOfSetWantsMostOfTheSampleMissingTwiceInARow: a module enabled a moment ago, an update half
// copied, misses a few manifests; a store that lost the set misses most of any sample, on every check.
func TestShortOfSetWantsMostOfTheSampleMissingTwiceInARow(t *testing.T) {
	half := fill.SampleResult{ReleaseHeld: true, Checked: 32, Missing: 16}
	clean := fill.SampleResult{ReleaseHeld: true, Checked: 32}

	t.Run("two in a row", func(t *testing.T) {
		loop := samplingLoop(t, &sampler{results: []fill.SampleResult{half, half}})
		_, found := loop.ShortOfSet(context.Background(), "v1.76.6")
		assert.False(t, found, "one check is not enough")
		short, found := loop.ShortOfSet(context.Background(), "v1.76.6")
		assert.True(t, found, "the next review checks again at once, and two are")
		assert.Equal(t, Shortfall{Release: "v1.76.6", ReleaseHeld: true, Checked: 32, Missing: 16}, short)
	})

	t.Run("a clean check in between", func(t *testing.T) {
		loop := samplingLoop(t, &sampler{results: []fill.SampleResult{half, clean, half, half}})
		_, found := loop.ShortOfSet(context.Background(), "v1.76.6")
		assert.False(t, found)
		_, found = loop.ShortOfSet(context.Background(), "v1.76.6")
		assert.False(t, found)
		expire(loop)
		_, found = loop.ShortOfSet(context.Background(), "v1.76.6")
		assert.False(t, found, "the count starts again after a clean check")
		_, found = loop.ShortOfSet(context.Background(), "v1.76.6")
		assert.True(t, found)
	})

	t.Run("a few missing", func(t *testing.T) {
		disk := &sampler{results: []fill.SampleResult{{ReleaseHeld: true, Checked: 32, Missing: 1}}}
		loop := samplingLoop(t, disk)
		for range 5 {
			_, found := loop.ShortOfSet(context.Background(), "v1.76.6")
			assert.False(t, found)
			expire(loop)
		}
		assert.EqualValues(t, 5, disk.taken.Load())
	})

	t.Run("too little looked at to tell", func(t *testing.T) {
		loop := samplingLoop(t, &sampler{results: []fill.SampleResult{{ReleaseHeld: true, Checked: 3, Missing: 3}}})
		for range 3 {
			_, found := loop.ShortOfSet(context.Background(), "v1.76.6")
			assert.False(t, found)
		}
	})
}

// TestACleanCheckStandsForAMinute: while every replica is full this is the whole cost — one sample a
// minute.
func TestACleanCheckStandsForAMinute(t *testing.T) {
	disk := &sampler{results: []fill.SampleResult{{ReleaseHeld: true, Checked: 32}}}
	loop := samplingLoop(t, disk)

	for range 4 {
		_, found := loop.ShortOfSet(context.Background(), "v1.76.6")
		assert.False(t, found)
	}
	assert.EqualValues(t, 1, disk.taken.Load(), "the reviews within a minute read nothing")

	expire(loop)
	_, _ = loop.ShortOfSet(context.Background(), "v1.76.6")
	assert.EqualValues(t, 2, disk.taken.Load())
}

// TestASampleThatCannotBeHadFindsNothing: a disk that does not answer in time, or cannot be read, is
// no evidence that the content is gone — and a sample stuck in the kernel is not started again.
func TestASampleThatCannotBeHadFindsNothing(t *testing.T) {
	t.Run("hanging", func(t *testing.T) {
		disk := &sampler{results: []fill.SampleResult{{}}, hang: make(chan struct{})}
		loop := samplingLoop(t, disk)
		loop.StoreSampleTimeout = 20 * time.Millisecond

		_, found := loop.ShortOfSet(context.Background(), "v1.76.6")
		assert.False(t, found)
		_, found = loop.ShortOfSet(context.Background(), "v1.76.6")
		assert.False(t, found)
		assert.EqualValues(t, 1, disk.taken.Load(), "the stuck sample is waited on, not repeated")

		close(disk.hang)
		require.Eventually(t, func() bool {
			_, found := loop.ShortOfSet(context.Background(), "v1.76.6")
			return found
		}, time.Second, 5*time.Millisecond, "what it finds once it returns is its answer")
	})

	t.Run("unreadable", func(t *testing.T) {
		loop := samplingLoop(t, &sampler{err: errors.New("input/output error")})
		for range 3 {
			_, found := loop.ShortOfSet(context.Background(), "v1.76.6")
			assert.False(t, found)
		}
	})
}

// TestTheSampleIsOfTheSetTheLastPassRead: up to stepDownSample digests of the set, never a member the
// store could not resolve, and none of a set read for another release — only the tag is looked at then.
func TestTheSampleIsOfTheSetTheLastPassRead(t *testing.T) {
	disk := &sampler{results: []fill.SampleResult{{ReleaseHeld: true}}}
	loop := samplingLoop(t, disk)

	declared := map[string]struct{}{"missing:registry/system/deckhouse/modules/foo:v1": {}}
	for i := range 40 {
		declared[fmt.Sprintf("sha256:%064x", i)] = struct{}{}
	}
	loop.declaredMu.Lock()
	loop.declared, loop.declaredFor, loop.declaredAt = declared, "v1.76.6", time.Now()
	loop.declaredMu.Unlock()

	_, _ = loop.ShortOfSet(context.Background(), "v1.76.6")
	sample := disk.digests()
	assert.Len(t, sample, stepDownSample)
	for _, digest := range sample {
		assert.True(t, strings.HasPrefix(digest, "sha256:"), digest)
	}

	expire(loop)
	_, _ = loop.ShortOfSet(context.Background(), "v1.77.0")
	assert.Empty(t, disk.digests(), "a set read for another release says nothing about this one")
}

// lossCluster is a cluster where master-1 holds the whole set and this replica, master-0, holds the
// lease with a report that says the same; its store is t.TempDir(), empty unless the test fills it.
func lossCluster(t *testing.T, replicas ...registryv1alpha1.StorageReplicaStatus) (*Loop, client.Client, *bytes.Buffer, string) {
	t.Helper()
	storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
	storage.Status.Replicas = replicas
	storage.Status.SafeToDropUpstream = true
	storage.Status.AllReplicasFull = true

	c := releaseAwareClient(t, storage, deployedRelease("v1.76.6"))
	set, deployed, err := CurrentSet(context.Background(), c)
	require.NoError(t, err)
	require.Equal(t, "v1.76.6", deployed, "the release the set was keyed on")
	// The fingerprint is the cluster's, so reports written before it was known take it now.
	refreshed := &registryv1alpha1.RegistryStorage{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(storage), refreshed))
	for i := range refreshed.Status.Replicas {
		if refreshed.Status.Replicas[i].VerifiedSet == "now" {
			refreshed.Status.Replicas[i].VerifiedSet = set
		}
	}
	require.NoError(t, c.Status().Update(context.Background(), refreshed))

	loop, _, _ := newLoop(t, true, nil)
	logs := &bytes.Buffer{}
	loop.Log = slog.New(slog.NewTextHandler(logs, nil))
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.DataDir = t.TempDir()
	return loop, c, logs, set
}

func fullLeader() registryv1alpha1.StorageReplicaStatus {
	return registryv1alpha1.StorageReplicaStatus{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true,
		VerifiedDigests: 453, DeclaredDigests: 453, VerifiedSet: "now", AheadSet: "ahead",
	}
}

func fullFollower() registryv1alpha1.StorageReplicaStatus {
	return registryv1alpha1.StorageReplicaStatus{
		Node: "master-1", Role: registryv1alpha1.ReplicaRoleFollower, Full: true,
		VerifiedDigests: 453, DeclaredDigests: 453, VerifiedSet: "now", Source: "master-0",
	}
}

func statusOf(t *testing.T, c client.Client) *registryv1alpha1.RegistryStorage {
	t.Helper()
	storage := &registryv1alpha1.RegistryStorage{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: registryv1alpha1.SingletonName}, storage))
	return storage
}

func holding(holds bool) func() bool { return func() bool { return holds } }

// TestALeaderWhoseStoreWasWipedStepsDown is defect 52 on a stand: local_data wiped under the leader,
// whose report still said full beside a follower that held the whole set. It takes its report back —
// Full, the count, the fingerprints, the permissions — says why once, and stays aside without looking
// again until a pass reads its store.
func TestALeaderWhoseStoreWasWipedStepsDown(t *testing.T) {
	loop, c, logs, set := lossCluster(t, fullLeader(), fullFollower())
	loop.complete.Store(true)
	loop.aheadMu.Lock()
	loop.aheadHeld = map[string]bool{"ahead": true}
	loop.aheadMu.Unlock()
	replicas := statusOf(t, c).Status.Replicas

	why, handOver := loop.WhyStepAside(context.Background(), replicas, set, "v1.76.6", holding(true))
	assert.Equal(t, "master-1 holds the whole image set and this replica's store does not hold it", why)
	assert.True(t, handOver, "it holds the lease, and gives it up")
	assert.True(t, loop.Lost())
	assert.False(t, loop.Complete(), "an incomplete store is not collected")
	assert.Empty(t, loop.heldAhead("ahead", "ahead"), "nor does it hold what is ahead of the set any more")

	status := statusOf(t, c).Status
	assert.False(t, status.SafeToDropUpstream)
	assert.False(t, status.AllReplicasFull)
	own := status.Replicas[0]
	assert.False(t, own.Full)
	assert.Zero(t, own.VerifiedDigests)
	assert.Empty(t, own.VerifiedSet)
	assert.Empty(t, own.AheadSet)
	assert.Equal(t, registryv1alpha1.ReplicaRoleFollower, own.Role)
	assert.True(t, status.Replicas[1].Full, "the full replica is left as it was")

	warning := logs.String()
	assert.Contains(t, warning, "level=WARN")
	assert.Contains(t, warning, "gives up the lease")
	assert.Contains(t, warning, "full_replica=master-1")
	assert.Contains(t, warning, "release_held=false")

	loop.StoreSample = func(string, string, string, []string) (fill.SampleResult, error) {
		t.Fatal("stepped down, it does not look again")
		return fill.SampleResult{}, nil
	}
	again, handOver := loop.WhyStepAside(context.Background(), replicas, set, "v1.76.6", holding(true))
	assert.Equal(t, why, again, "it stays aside while the other is full")
	assert.True(t, handOver, "and gives up a lease that still names it")
	loop.StepDown(context.Background(), Shortfall{}, &replicas[1])
	assert.Equal(t, 1, strings.Count(logs.String(), "gives up the lease"), "said once")
}

// TestAStoreThatHoldsTheSetLeads: the release on the disk and the sample whole — the leader keeps the
// lease, as the incumbent rule says.
func TestAStoreThatHoldsTheSetLeads(t *testing.T) {
	loop, c, _, set := lossCluster(t, fullLeader(), fullFollower())
	release := "sha256:5e1ea5e000000000000000000000000000000000000000000000000000000000"
	heldOnDisk(t, loop.DataDir, release)
	tagOnDisk(t, loop.DataDir, "v1.76.6", release)

	why, handOver := loop.WhyStepAside(context.Background(), statusOf(t, c).Status.Replicas, set, "v1.76.6", holding(true))
	assert.Empty(t, why)
	assert.False(t, handOver)
	assert.False(t, loop.Lost())
	assert.True(t, statusOf(t, c).Status.Replicas[0].Full)
}

// TestAReleaseDeployedAfterTheSetWasReadIsNotLookedFor: the set key and the release whose tag is looked
// for come from one reading. An update deployed between the election's reading and the check — the
// copy ahead of it not finished — would otherwise have a healthy full leader checked for a tag it has
// no reason to hold yet, and step down. And the check reads nothing of the API server itself, so a
// review while every replica is full costs what it did.
func TestAReleaseDeployedAfterTheSetWasReadIsNotLookedFor(t *testing.T) {
	loop, c, _, set := lossCluster(t, fullLeader(), fullFollower())
	release := "sha256:5e1ea5e000000000000000000000000000000000000000000000000000000000"
	heldOnDisk(t, loop.DataDir, release)
	tagOnDisk(t, loop.DataDir, "v1.76.6", release)

	// The update lands after the set was read for v1.76.6.
	ctx := context.Background()
	previous := deployedRelease("v1.76.6")
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(previous), previous))
	previous.Object["status"] = map[string]any{"phase": "Superseded"}
	require.NoError(t, c.Update(ctx, previous))
	require.NoError(t, c.Create(ctx, deployedRelease("v1.77.0")))
	_, now, err := CurrentSet(ctx, c)
	require.NoError(t, err)
	require.Equal(t, "v1.77.0", now)

	lists := 0
	loop.Client = interceptor.NewClient(c.(client.WithWatch), interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			lists++
			return c.List(ctx, list, opts...)
		},
	})

	why, handOver := loop.WhyStepAside(ctx, statusOf(t, c).Status.Replicas, set, "v1.76.6", holding(true))
	assert.Empty(t, why, "the store holds the release the set was keyed on")
	assert.False(t, handOver)
	assert.False(t, loop.Lost())
	assert.True(t, statusOf(t, c).Status.Replicas[0].Full)
	assert.Zero(t, lists, "the check lists nothing: the release comes with the set")
}

// TestWithNobodyElseFullTheHolderTakesItsReportBack: a store wiped under a running leader, alone or
// beside a replica that holds only part of the set. There is nobody to hand the lease to, and an empty
// leader is still the one that fills, so it keeps the lease and its role; but its report — Full, the
// count, the fingerprints, and the permissions resting on it — goes back at once, rather than standing
// until the recheck six hours on, for the air-gap transition to drop the upstream on.
func TestWithNobodyElseFullTheHolderTakesItsReportBack(t *testing.T) {
	behind := fullFollower()
	behind.Full, behind.VerifiedSet, behind.VerifiedDigests = false, "", 300

	for name, replicas := range map[string][]registryv1alpha1.StorageReplicaStatus{
		"alone":                  {fullLeader()},
		"beside a replica short": {fullLeader(), behind},
	} {
		t.Run(name, func(t *testing.T) {
			loop, c, logs, set := lossCluster(t, replicas...)
			loop.complete.Store(true)
			loop.aheadMu.Lock()
			loop.aheadHeld = map[string]bool{"ahead": true}
			loop.aheadMu.Unlock()
			reported := statusOf(t, c).Status.Replicas

			why, handOver := loop.WhyStepAside(context.Background(), reported, set, "v1.76.6", holding(true))
			assert.Empty(t, why, "it still stands: nobody else holds the set")
			assert.False(t, handOver, "and keeps the lease")
			assert.True(t, loop.Lost())
			assert.False(t, loop.Complete(), "an incomplete store is not collected")
			assert.Empty(t, loop.heldAhead("ahead", "ahead"), "nor does it hold what is ahead of the set any more")

			status := statusOf(t, c).Status
			assert.False(t, status.SafeToDropUpstream, "nothing is left for the air-gap transition to stand on")
			assert.False(t, status.AllReplicasFull)
			own := status.Replicas[0]
			assert.False(t, own.Full)
			assert.Zero(t, own.VerifiedDigests)
			assert.Empty(t, own.VerifiedSet)
			assert.Empty(t, own.AheadSet)
			assert.Equal(t, registryv1alpha1.ReplicaRoleLeader, own.Role, "it is still the one that fills")
			assert.EqualValues(t, 453, own.DeclaredDigests, "the size of the set is no claim, and stays")
			if len(replicas) > 1 {
				assert.Equal(t, reported[1], status.Replicas[1], "the other replica is left as it was")
			}

			warning := logs.String()
			assert.Contains(t, warning, "level=WARN")
			assert.Contains(t, warning, "keeps the lease")
			assert.Contains(t, warning, "release=v1.76.6")
			assert.Contains(t, warning, "release_held=false")
			assert.NotContains(t, warning, "gives up the lease")

			// The reviews after it, with the reports as the status had them or as it has them now: nothing
			// is read again until a pass reads the store, and nothing is said again.
			disk := &sampler{results: []fill.SampleResult{{ReleaseHeld: false}}}
			loop.StoreSample = disk.sample
			for _, replicas := range [][]registryv1alpha1.StorageReplicaStatus{reported, status.Replicas} {
				why, handOver := loop.WhyStepAside(context.Background(), replicas, set, "v1.76.6", holding(true))
				assert.Empty(t, why)
				assert.False(t, handOver)
			}
			assert.Zero(t, disk.taken.Load())
			assert.Equal(t, 1, strings.Count(logs.String(), "keeps the lease"), "said once")
		})
	}
}

// TestACopyAheadRunningThroughALossDoesNotPutItBack is defect 53 on the paths of a loss: a copy ahead
// was running when the store was found short of the set, and on finishing, stopped or not, it wrote the
// parts it had copied back as held. The next pass published them, and followers went to copy from the
// leader what its store no longer had, to MANIFEST_UNKNOWN every aheadRetry.
func TestACopyAheadRunningThroughALossDoesNotPutItBack(t *testing.T) {
	for name, lose := range map[string]func(*Loop, []registryv1alpha1.StorageReplicaStatus){
		"taken back": func(loop *Loop, _ []registryv1alpha1.StorageReplicaStatus) {
			loop.TakeBackFull(context.Background(), Shortfall{Release: "v1.76.6"})
		},
		"stepped down": func(loop *Loop, replicas []registryv1alpha1.StorageReplicaStatus) {
			loop.StepDown(context.Background(), Shortfall{Release: "v1.76.6"}, &replicas[1])
		},
	} {
		for _, stopped := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s, stopped %t", name, stopped), func(t *testing.T) {
				loop, c, _, _ := lossCluster(t, fullLeader(), fullFollower())
				finish := make(chan struct{})
				loop.startAhead(context.Background(), func(ctx context.Context) fill.Report {
					select {
					case <-ctx.Done():
					case <-finish:
					}
					return fill.Report{AheadHeld: []string{"P"}, AheadCopied: 1}
				}, "P", "leader")

				lose(loop, statusOf(t, c).Status.Replicas)
				if stopped {
					loop.stopAhead()
				} else {
					close(finish)
					loop.waitAhead()
				}
				assert.Empty(t, loop.heldAhead("P", ""), "copied into the store as it was before the loss")

				loop.startAhead(context.Background(), func(context.Context) fill.Report {
					return fill.Report{AheadHeld: []string{"P"}, AheadCopied: 1}
				}, "P", "leader")
				loop.waitAhead()
				assert.Equal(t, "P", loop.heldAhead("P", ""), "a copy started after it holds what it copied")
			})
		}
	}
}

// TestALeaderAloneFindsAStoreWhoseTagCameBack: with an upstream, a pull of the release through the
// leader's own pull-through writes its tag back onto a wiped store — within 35 s on a stand — and only
// the sample of the set tells the loss, on two checks in a row. The same check as beside a full replica.
func TestALeaderAloneFindsAStoreWhoseTagCameBack(t *testing.T) {
	loop, c, logs, set := lossCluster(t, fullLeader())
	half := fill.SampleResult{ReleaseHeld: true, Checked: 32, Missing: 30}
	loop.StoreSample = (&sampler{results: []fill.SampleResult{half, half}}).sample

	why, handOver := loop.WhyStepAside(context.Background(), statusOf(t, c).Status.Replicas, set, "v1.76.6", holding(true))
	assert.Empty(t, why)
	assert.False(t, handOver)
	assert.False(t, loop.Lost(), "one check is not enough")
	assert.True(t, statusOf(t, c).Status.Replicas[0].Full)

	why, handOver = loop.WhyStepAside(context.Background(), statusOf(t, c).Status.Replicas, set, "v1.76.6", holding(true))
	assert.Empty(t, why)
	assert.False(t, handOver)
	assert.True(t, loop.Lost(), "two are")
	own := statusOf(t, c).Status.Replicas[0]
	assert.False(t, own.Full)
	assert.Equal(t, registryv1alpha1.ReplicaRoleLeader, own.Role)
	assert.Contains(t, logs.String(), "release_held=true")
	assert.Contains(t, logs.String(), "sampled=32")
	assert.Contains(t, logs.String(), "missing=30")
}

// TestWithNobodyElseFullAStoreThatHoldsTheSetIsLeftAlone: the release on the disk and the sample whole —
// nothing goes back, and a clean check stands for a minute, as beside a full replica.
func TestWithNobodyElseFullAStoreThatHoldsTheSetIsLeftAlone(t *testing.T) {
	loop, c, logs, set := lossCluster(t, fullLeader())
	release := "sha256:5e1ea5e000000000000000000000000000000000000000000000000000000000"
	heldOnDisk(t, loop.DataDir, release)
	tagOnDisk(t, loop.DataDir, "v1.76.6", release)

	for range 3 {
		why, handOver := loop.WhyStepAside(context.Background(), statusOf(t, c).Status.Replicas, set, "v1.76.6", holding(true))
		assert.Empty(t, why)
		assert.False(t, handOver)
	}
	assert.False(t, loop.Lost())
	status := statusOf(t, c).Status
	assert.True(t, status.Replicas[0].Full)
	assert.True(t, status.SafeToDropUpstream)
	assert.NotContains(t, logs.String(), "level=WARN")
}

// TestWithNobodyElseFullNothingIsReadWithoutAReportToTakeBack: the check is the holder's, and only for a
// report that says it holds the set the cluster needs now. A follower is left to its recheck; a report
// not full, full for another set or for none, or carrying an error, has nothing to take back; a pass
// copying the set is earning its report again; and a set or a release that cannot be told is no
// evidence.
func TestWithNobodyElseFullNothingIsReadWithoutAReportToTakeBack(t *testing.T) {
	cases := map[string]struct {
		leader    func(*registryv1alpha1.StorageReplicaStatus)
		holds     bool
		filling   bool
		set       func(string) string
		noRelease bool
	}{
		"a replica that does not hold the lease": {holds: false},
		"a report not full": {holds: true, leader: func(r *registryv1alpha1.StorageReplicaStatus) {
			r.Full = false
		}},
		"a report full for another set": {holds: true, leader: func(r *registryv1alpha1.StorageReplicaStatus) {
			r.VerifiedSet = "set-before"
		}},
		"a report full for no set it names": {holds: true, leader: func(r *registryv1alpha1.StorageReplicaStatus) {
			r.VerifiedSet = ""
		}},
		"a report with an error": {holds: true, leader: func(r *registryv1alpha1.StorageReplicaStatus) {
			r.Error = "a pass failed"
		}},
		"a store that cannot serve": {holds: true, leader: func(r *registryv1alpha1.StorageReplicaStatus) {
			r.HealthError = "a disk failed"
		}},
		"a pass copying the set": {holds: true, filling: true},
		// A report that names no set either: only the guard on the set tells the two apart.
		"a set that cannot be told": {holds: true, set: func(string) string { return "" },
			leader: func(r *registryv1alpha1.StorageReplicaStatus) { r.VerifiedSet = "" }},
		"no release deployed": {holds: true, noRelease: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			loop, c, logs, set := lossCluster(t, fullLeader())
			replicas := statusOf(t, c).Status.Replicas
			if tc.leader != nil {
				tc.leader(&replicas[0])
			}
			if tc.set != nil {
				set = tc.set(set)
			}
			deployed := "v1.76.6"
			if tc.noRelease {
				deployed = ""
			}
			loop.filling.Store(tc.filling)
			disk := &sampler{results: []fill.SampleResult{{ReleaseHeld: false}}}
			loop.StoreSample = disk.sample

			why, handOver := loop.WhyStepAside(context.Background(), replicas, set, deployed, holding(tc.holds))
			assert.Empty(t, why)
			assert.False(t, handOver)
			assert.Zero(t, disk.taken.Load(), "nothing is read")
			assert.False(t, loop.Lost())
			assert.True(t, statusOf(t, c).Status.Replicas[0].Full, "nothing published")
			assert.NotContains(t, logs.String(), "level=WARN")
		})
	}
}

// TestALeaderThatTookItsReportBackStepsDownOnceAnotherIsFull: the leader kept the lease with nobody to
// hand it to, and another replica then came to hold the whole set — a follower copying through the
// leader's pull-through, say. That replica is the better source: the leader steps down in its favour
// without reading its disk again, and makes the step-down it owed — role Follower, the lease given up,
// the line saying where the lease went — once.
func TestALeaderThatTookItsReportBackStepsDownOnceAnotherIsFull(t *testing.T) {
	loop, c, logs, set := lossCluster(t, fullLeader(), fullFollower())
	alone := statusOf(t, c).Status.Replicas
	follower := alone[1]
	alone[1].Full, alone[1].VerifiedSet = false, ""

	why, handOver := loop.WhyStepAside(context.Background(), alone, set, "v1.76.6", holding(true))
	require.Empty(t, why)
	require.False(t, handOver)
	require.True(t, loop.Lost())
	require.Equal(t, registryv1alpha1.ReplicaRoleLeader, statusOf(t, c).Status.Replicas[0].Role)

	disk := &sampler{results: []fill.SampleResult{{ReleaseHeld: false}}}
	loop.StoreSample = disk.sample
	beside := statusOf(t, c).Status.Replicas
	beside[1] = follower

	why, handOver = loop.WhyStepAside(context.Background(), beside, set, "v1.76.6", holding(true))
	assert.Equal(t, "master-1 holds the whole image set and this replica's store does not hold it", why)
	assert.True(t, handOver, "it holds the lease, and gives it up")
	assert.True(t, loop.Lost(), "and stays aside until a pass reads its store")
	assert.Zero(t, disk.taken.Load(), "without reading the disk again")
	own := statusOf(t, c).Status.Replicas[0]
	assert.Equal(t, registryv1alpha1.ReplicaRoleFollower, own.Role, "followers reading roles copy from it no more")
	assert.False(t, own.Full)
	assert.Contains(t, logs.String(), "now holds the set whole, so it gives up the lease")
	assert.Contains(t, logs.String(), "full_replica=master-1")

	again, handOver := loop.WhyStepAside(context.Background(), beside, set, "v1.76.6", holding(true))
	assert.Equal(t, why, again, "it stays aside while the other is full")
	assert.True(t, handOver, "and gives up a lease that still names it")
	_, handOver = loop.WhyStepAside(context.Background(), beside, set, "v1.76.6", holding(false))
	assert.False(t, handOver, "a lease it no longer holds is nothing to give up")
	assert.Equal(t, 1, strings.Count(logs.String(), "gives up the lease"), "said once")

	t.Run("a replica that no longer holds the lease", func(t *testing.T) {
		loop, c, logs, set := lossCluster(t, fullLeader(), fullFollower())
		alone := statusOf(t, c).Status.Replicas
		alone[1].Full = false
		_, _ = loop.WhyStepAside(context.Background(), alone, set, "v1.76.6", holding(true))
		require.True(t, loop.Lost())

		_, handOver := loop.WhyStepAside(context.Background(), statusOf(t, c).Status.Replicas, set, "v1.76.6", holding(false))
		assert.False(t, handOver)
		assert.Equal(t, registryv1alpha1.ReplicaRoleLeader, statusOf(t, c).Status.Replicas[0].Role,
			"its next pass reports the role it has; the step-down is the holder's")
		assert.NotContains(t, logs.String(), "gives up the lease")
	})
}

// TestAReadingBetweenTheReviewAndTheOwedStepDownEndsIt: the review saw the take-back stand, and a pass
// that read the store published what it holds before the step-down took reportMu — a fill finished, say,
// as the follower copying through the pull-through finished too. The take-back is over: nothing is
// demoted, nothing is said, and the replica stands until the next review looks at its store afresh.
func TestAReadingBetweenTheReviewAndTheOwedStepDownEndsIt(t *testing.T) {
	loop, c, logs, set := lossCluster(t, fullLeader(), fullFollower())
	alone := statusOf(t, c).Status.Replicas
	follower := alone[1]
	alone[1].Full, alone[1].VerifiedSet = false, ""
	_, _ = loop.WhyStepAside(context.Background(), alone, set, "v1.76.6", holding(true))
	require.True(t, loop.TookBack())

	beside := statusOf(t, c).Status.Replicas
	beside[1] = follower
	before := statusOf(t, c).Status.Replicas[0]
	readInBetween := func() bool {
		// What once does under reportMu once a reading is published.
		loop.lost.Store(false)
		loop.tookBack.Store(false)
		return true
	}

	why, handOver := loop.WhyStepAside(context.Background(), beside, set, "v1.76.6", readInBetween)
	assert.Empty(t, why, "the reading superseded the take-back")
	assert.False(t, handOver)
	assert.False(t, loop.Lost(), "and the replica is not lost again")
	assert.Equal(t, before, statusOf(t, c).Status.Replicas[0], "nothing published over the reading")
	assert.NotContains(t, logs.String(), "gives up the lease")
}

// TestATakeBackIsNotUndoneByASurveyFromBeforeTheLoss: an air-gapped leader counts its store on every
// pass, from a survey that stands for a minute. The store is wiped and the report taken back: the next
// count walks the disk again, rather than publish the walk taken before the wipe — Full and all — and
// have the next review take the report back again.
func TestATakeBackIsNotUndoneByASurveyFromBeforeTheLoss(t *testing.T) {
	store := startRegistry(t)
	running := pushDigest(t, store, "system/deckhouse")
	pushImage(t, store, "system/deckhouse:v1.76.6")
	pushInstaller(t, store, "system/deckhouse/install:v1.76.6", []string{running})

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{Node: "master-0"}}
	loop, _, _ := newLoop(t, true, nil)
	c := releaseAwareClient(t, storage, deployedRelease("v1.76.6"))
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.LocalAddress = store
	loop.WriteAddress = store
	loop.DataDir = t.TempDir()
	release := resolveTag(t, store, "system/deckhouse", "v1.76.6")
	heldOnDisk(t, loop.DataDir, running, release)
	tagOnDisk(t, loop.DataDir, "v1.76.6", release)

	require.NoError(t, loop.once(context.Background()))
	require.True(t, replicaOf(t, c, "master-0").Full, "the set on the disk, counted")

	// Wiped, within the minute the survey stands for.
	loop.DataDir = t.TempDir()
	loop.TakeBackFull(context.Background(), Shortfall{Release: "v1.76.6"})
	require.False(t, replicaOf(t, c, "master-0").Full)

	require.NoError(t, loop.once(context.Background()))
	assert.False(t, replicaOf(t, c, "master-0").Full, "the count says what the disk holds now")
	assert.False(t, loop.Lost(), "and, having read the store, ends the take-back")

	t.Run("a walk that fails leaves the survey stale", func(t *testing.T) {
		loop.surveyStale.Store(true)
		loop.DataDir = filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(loop.DataDir, nil, 0o600))
		_, err := loop.surveyStore(map[string]struct{}{running: {}}, "v1.76.6")
		require.Error(t, err, "a store under a file cannot be walked")
		assert.True(t, loop.surveyStale.Load(), "the next reading walks it, not the survey before the wipe")
	})
}

// lossDuringThePass leads, and takes the report back the first time the pass asks — after the pass has
// begun and before it reads the store — as a review running beside the pass would.
type lossDuringThePass struct {
	loop  *Loop
	asked atomic.Int32
}

func (l *lossDuringThePass) IsLeader() bool {
	if l.asked.Add(1) == 1 {
		l.loop.TakeBackFull(context.Background(), Shortfall{Release: "v1.76.6"})
	}
	return true
}

// TestAReadingTakenBeforeTheLossDoesNotEndIt: an air-gapped leader counts its store on every pass, and
// the review that finds the store short runs beside the pass. A count that began before the take-back
// says what the store held then — here the set still on the disk, as a walk made before a wipe would —
// and published, it would put Full back on the leader the controller drops the upstream on. It is
// masked, and the take-back stands; the next pass, which nothing came into, counts and ends it.
func TestAReadingTakenBeforeTheLossDoesNotEndIt(t *testing.T) {
	store := startRegistry(t)
	running := pushDigest(t, store, "system/deckhouse")
	pushImage(t, store, "system/deckhouse:v1.76.6")
	pushInstaller(t, store, "system/deckhouse/install:v1.76.6", []string{running})

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{Node: "master-0"}}
	loop, _, _ := newLoop(t, true, nil)
	c := releaseAwareClient(t, storage, deployedRelease("v1.76.6"))
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.LocalAddress = store
	loop.WriteAddress = store
	loop.DataDir = t.TempDir()
	release := resolveTag(t, store, "system/deckhouse", "v1.76.6")
	heldOnDisk(t, loop.DataDir, running, release)
	tagOnDisk(t, loop.DataDir, "v1.76.6", release)

	loop.Leadership = &lossDuringThePass{loop: loop}
	require.NoError(t, loop.once(context.Background()))
	replica := replicaOf(t, c, "master-0")
	assert.False(t, replica.Full, "the count from before the loss is not published")
	assert.Zero(t, replica.VerifiedDigests)
	assert.True(t, loop.Lost(), "and does not end the take-back")
	assert.True(t, loop.TookBack())

	loop.Leadership = fixedLeadership(true)
	require.NoError(t, loop.once(context.Background()))
	assert.True(t, replicaOf(t, c, "master-0").Full, "a count that nothing came into says what the disk holds")
	assert.False(t, loop.Lost(), "and ends the take-back")
}

// TestAReplicaThatDoesNotHoldTheLeaseOnlyStaysAside: a follower with a stale Full and an empty store,
// standing in air-gap beside the full leader, does not stand — and publishes nothing: it is left to its
// recheck, as before.
func TestAReplicaThatDoesNotHoldTheLeaseOnlyStaysAside(t *testing.T) {
	stale := fullFollower()
	stale.Node = "master-0"
	leader := fullLeader()
	leader.Node = "master-1"
	loop, c, logs, set := lossCluster(t, stale, leader)

	why, handOver := loop.WhyStepAside(context.Background(), statusOf(t, c).Status.Replicas, set, "v1.76.6", holding(false))
	assert.Equal(t, "master-1 holds the whole image set and this replica's store does not hold it", why)
	assert.False(t, handOver)
	assert.False(t, loop.Lost())
	assert.True(t, statusOf(t, c).Status.Replicas[0].Full, "nothing published")
	assert.True(t, statusOf(t, c).Status.SafeToDropUpstream)
	assert.NotContains(t, logs.String(), "gives up the lease")
}

// TestAnIdlePassAfterAStepDownPutsNothingBack: a pass that carried Full over — read before the
// demotion landed, or after a demotion that could not be written — publishes the demotion instead.
func TestAnIdlePassAfterAStepDownPutsNothingBack(t *testing.T) {
	local := startRegistry(t)
	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{Scheme: registryv1alpha1.SchemeHTTP, Host: local},
		},
		NeedSync: false,
	})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true,
		VerifiedDigests: 459, VerifiedSet: "abc", AheadSet: "def",
	}}
	loop, c, _ := newLoop(t, true, storage)
	loop.LocalAddress = local
	loop.WriteAddress = local
	loop.lost.Store(true)

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.False(t, replica.Full)
	assert.Zero(t, replica.VerifiedDigests)
	assert.Empty(t, replica.VerifiedSet)
	assert.Empty(t, replica.AheadSet)
	assert.False(t, loop.Complete())
	assert.True(t, loop.Lost(), "an idle pass read nothing, so the step-down stands")
}

// TestTheLeadersFillEndsTheTakeBack: a leader that kept the lease and took its report back fills under
// it. Its reported set gone, its next pass fills even before the controller raises needSync — see
// Reverify — and the fill, which read the store, publishes what it holds, Full again, as the leader, and
// ends the take-back.
func TestTheLeadersFillEndsTheTakeBack(t *testing.T) {
	upstream := startRegistry(t)
	local := startRegistry(t)
	one := pushDigest(t, upstream, "deckhouse/ee")
	two := pushDigest(t, upstream, "deckhouse/ee")
	pushImage(t, upstream, "deckhouse/ee:v1.70.1")
	pushInstaller(t, upstream, "deckhouse/ee/install:v1.70.1", []string{one, two})

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{Scheme: registryv1alpha1.SchemeHTTP, Host: upstream, Path: "/deckhouse/ee"},
		},
		Source: &registryv1alpha1.StorageSource{ExpectedDigests: 4},
		// The controller has not read the take-back yet.
		NeedSync: false,
	})
	// The entry as the take-back left it.
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, DeclaredDigests: 4,
	}}
	loop, _, _ := newLoop(t, true, nil)
	c := releaseAwareClient(t, storage, deployedRelease("v1.70.1"))
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.LocalAddress = local
	loop.WriteAddress = local
	loop.lost.Store(true)
	loop.tookBack.Store(true)

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.True(t, replica.Full, "what the fill put in is what it reports")
	assert.NotEmpty(t, replica.VerifiedSet)
	assert.Equal(t, registryv1alpha1.ReplicaRoleLeader, replica.Role)
	assert.False(t, loop.Lost(), "a reading published ends the take-back")
	assert.False(t, loop.tookBack.Load())
}

// TestAReplicationTakesTheStepDownBack: the ex-leader copies the set from the new one like any follower,
// and the pass that read the store says what it holds — full again — and ends the step-down.
func TestAReplicationTakesTheStepDownBack(t *testing.T) {
	leaderStorage := startRegistry(t)
	followerStorage := startRegistry(t)
	one := pushDigest(t, leaderStorage, "system/deckhouse")
	two := pushDigest(t, leaderStorage, "system/deckhouse")
	pushImage(t, leaderStorage, "system/deckhouse:v1.76.6")
	pushInstaller(t, leaderStorage, "system/deckhouse/install:v1.76.6", []string{one, two})

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Source:   &registryv1alpha1.StorageSource{ExpectedDigests: 4},
		NeedSync: true,
	})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 4,
			Address: leaderStorage},
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleFollower},
	}
	loop, c, _ := newLoop(t, false, storage, deployedRelease("v1.76.6"))
	loop.LocalAddress = followerStorage
	loop.WriteAddress = followerStorage
	loop.ReportedAddress = followerStorage
	loop.lost.Store(true)

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.True(t, replica.Full, "what the copy put in is what it reports")
	assert.Equal(t, "master-1", replica.Source)
	assert.False(t, loop.Lost(), "a reading published ends the step-down")
}

// TestACountTakesTheStepDownBack: in air-gap, between the release of the lease and the takeover, the
// ex-leader counts its own store — and says what it found, which ends the step-down too.
func TestACountTakesTheStepDownBack(t *testing.T) {
	store := startRegistry(t)
	running := pushDigest(t, store, "system/deckhouse")
	pushImage(t, store, "system/deckhouse:v1.76.6")
	pushInstaller(t, store, "system/deckhouse/install:v1.76.6", []string{running})

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{Node: "master-0"}}
	loop, _, _ := newLoop(t, true, nil)
	c := releaseAwareClient(t, storage, deployedRelease("v1.76.6"))
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.LocalAddress = store
	loop.WriteAddress = store
	loop.DataDir = t.TempDir()
	loop.lost.Store(true)
	// The same for an air-gapped leader that kept the lease and took its report back: it counts on every
	// pass.
	loop.tookBack.Store(true)

	// Nothing on the disk: the count is honest, and says so.
	require.NoError(t, loop.once(context.Background()))
	assert.False(t, replicaOf(t, c, "master-0").Full)
	assert.False(t, loop.Lost())
	assert.False(t, loop.tookBack.Load())

	// The set back on the disk, and the next count says that.
	release := resolveTag(t, store, "system/deckhouse", "v1.76.6")
	heldOnDisk(t, loop.DataDir, running, release)
	tagOnDisk(t, loop.DataDir, "v1.76.6", release)
	loop.surveyedAt = time.Time{}
	require.NoError(t, loop.once(context.Background()))
	assert.True(t, replicaOf(t, c, "master-0").Full)
}

// TestAnAirGapRecountTakesTheStepDownBack: during the air-gap transition the leader's pass, idle or not,
// ends with a recount of its store — see StoreIsAuthority — and that recount is what says what the
// store holds. One that read the store ends the step-down; one that failed does not, and the pass
// publishes the step-down again.
func TestAnAirGapRecountTakesTheStepDownBack(t *testing.T) {
	transition := func(t *testing.T, store string) (*Loop, client.Client) {
		t.Helper()
		storage := storageWith(registryv1alpha1.RegistryStorageSpec{
			Upstream: &registryv1alpha1.Upstream{
				Endpoint: registryv1alpha1.Endpoint{Scheme: registryv1alpha1.SchemeHTTP, Host: "held.example.invalid"},
			},
			Publish:         true,
			AirGapRequested: true,
			NeedSync:        false,
		})
		storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
			Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 459,
		}}
		loop, c, _ := newLoop(t, true, storage, deployedRelease("v1.76.6"))
		// Full for the set the cluster needs now, so that the pass stays idle rather than verifying the
		// store against a new set — see Reverify: only the recount reads the store.
		set, err := CurrentSetKey(context.Background(), c)
		require.NoError(t, err)
		stored := statusOf(t, c)
		stored.Status.Replicas[0].VerifiedSet = set
		require.NoError(t, c.Status().Update(context.Background(), stored))
		loop.LocalAddress = store
		loop.WriteAddress = store
		loop.DataDir = t.TempDir()
		loop.lost.Store(true)
		return loop, c
	}

	t.Run("a recount that read the store", func(t *testing.T) {
		store := startRegistry(t)
		running := pushDigest(t, store, "system/deckhouse")
		pushImage(t, store, "system/deckhouse:v1.76.6")
		pushInstaller(t, store, "system/deckhouse/install:v1.76.6", []string{running})
		loop, c := transition(t, store)
		release := resolveTag(t, store, "system/deckhouse", "v1.76.6")
		heldOnDisk(t, loop.DataDir, running, release)
		tagOnDisk(t, loop.DataDir, "v1.76.6", release)

		require.NoError(t, loop.once(context.Background()))

		assert.True(t, replicaOf(t, c, "master-0").Full, "what the store holds, counted")
		assert.False(t, loop.Lost(), "a recount published ends the step-down")
	})

	t.Run("a recount that failed", func(t *testing.T) {
		// No release on the store, so there is no set to count against.
		loop, c := transition(t, startRegistry(t))

		require.NoError(t, loop.once(context.Background()))

		replica := replicaOf(t, c, "master-0")
		assert.False(t, replica.Full)
		assert.Zero(t, replica.VerifiedDigests)
		assert.Empty(t, replica.VerifiedSet)
		assert.NotEmpty(t, replica.Error, "the recount failed and says so")
		assert.True(t, loop.Lost(), "a reading that failed is no reading")
	})
}

// idleCluster is lossCluster with an upstream and needSync down, so that the pass of the replica under
// test is idle: it carries its report over from the status, its store was checked a moment ago so that
// no recheck reads it, and only the total of the store is measured — off a store of held manifests.
func idleCluster(
	t *testing.T, isLeader bool, held int, replicas ...registryv1alpha1.StorageReplicaStatus,
) (*Loop, client.Client, *bytes.Buffer, string) {
	t.Helper()
	loop, c, logs, set := lossCluster(t, replicas...)
	storage := statusOf(t, c)
	storage.Spec.Upstream = &registryv1alpha1.Upstream{
		Endpoint: registryv1alpha1.Endpoint{Scheme: registryv1alpha1.SchemeHTTP, Host: closedAddress(t)},
	}
	require.NoError(t, c.Update(context.Background(), storage))
	loop.Leadership = fixedLeadership(isLeader)
	loop.lastStoreCheck.Store(time.Now().UnixNano())
	storeHolding(t, loop.DataDir, held)
	return loop, c, logs, set
}

// refuted is what no idle pass may publish once it has measured the store: Full beside a total of the
// disk that holds at most half of what the report says it verified — see refutedByCount.
func refuted(replica registryv1alpha1.StorageReplicaStatus) bool {
	return replica.Full && replica.VerifiedDigests > 0 && 2*replica.TotalDigests <= replica.VerifiedDigests
}

// TestAnIdleLeaderWhoseCountRefutesItsFullTakesItBack is the stand's minute: local_data wiped under the
// leader, the release's tag back through its pull-through, and the entry published full: true with 445
// verified and 2 in total until the sample's second check. The idle pass that measures the total takes
// the report back on its own, at once, with no review and no sample — keeping the lease and the role.
func TestAnIdleLeaderWhoseCountRefutesItsFullTakesItBack(t *testing.T) {
	for name, held := range map[string]int{
		"a store with one manifest left": 1,
		// No repositories directory at all: a walk that finds nothing is a reading of zero, not an
		// unmeasured store, and the last total from before the wipe is not published.
		"an empty store": 0,
	} {
		t.Run(name, func(t *testing.T) {
			loop, c, logs, _ := idleCluster(t, true, held, fullLeader())
			loop.lastTotalDigests.Store(446)
			loop.complete.Store(true)
			disk := &sampler{results: []fill.SampleResult{{ReleaseHeld: true}}}
			loop.StoreSample = disk.sample

			require.NoError(t, loop.once(context.Background()))

			status := statusOf(t, c).Status
			own := status.Replicas[0]
			assert.False(t, own.Full)
			assert.Zero(t, own.VerifiedDigests)
			assert.Empty(t, own.VerifiedSet)
			assert.Empty(t, own.AheadSet)
			assert.EqualValues(t, held, own.TotalDigests, "the total the walk found, zero included")
			assert.EqualValues(t, 453, own.DeclaredDigests, "the size of the set is no claim, and stays")
			assert.Equal(t, registryv1alpha1.ReplicaRoleLeader, own.Role, "it is still the one that fills")
			assert.False(t, status.SafeToDropUpstream, "nothing is left for the air-gap transition to stand on")
			assert.False(t, status.AllReplicasFull)
			assert.False(t, refuted(own))

			assert.True(t, loop.Lost(), "after one pass, with no review")
			assert.True(t, loop.TookBack())
			assert.False(t, loop.Complete())
			assert.Zero(t, disk.taken.Load(), "and with no sample: the count said it")
			assert.EqualValues(t, held, loop.lastTotalDigests.Load())

			warning := logs.String()
			assert.Contains(t, warning, "level=WARN")
			assert.Contains(t, warning, "holds fewer digests than its report says it verified")
			assert.Contains(t, warning, fmt.Sprintf("store_total=%d verified=453", held))
			assert.NotContains(t, warning, "no other replica holds the set whole",
				"an idle pass does not know that, and does not claim it")

			// The next idle pass is a fill now, its reported set gone; here nothing answers it, and it is
			// masked like any pass that did not read the store. Nothing is said again.
			require.NoError(t, loop.once(context.Background()))
			assert.False(t, statusOf(t, c).Status.Replicas[0].Full)
			assert.Equal(t, 1, strings.Count(logs.String(), "holds fewer digests"), "said once")
		})
	}
}

// TestAnIdlePassLeavesAFullItsCountCanHoldAlone: a store whose total holds what the report verified, or
// more than half of it, is not refuted — a few manifests between two readings a minute apart are a
// collection or a module that came and went — and an idle pass then changes nothing in the report. The
// bar is the sample's: at least half missing.
func TestAnIdlePassLeavesAFullItsCountCanHoldAlone(t *testing.T) {
	ten := fullLeader()
	ten.VerifiedDigests, ten.DeclaredDigests, ten.AheadSet = 10, 10, ""

	for name, tc := range map[string]struct {
		held    int
		refuted bool
	}{
		"the whole set and more": {held: 12},
		"one short":              {held: 9},
		"just over half":         {held: 6},
		"half":                   {held: 5, refuted: true},
	} {
		t.Run(name, func(t *testing.T) {
			loop, c, _, set := idleCluster(t, true, tc.held, ten)

			require.NoError(t, loop.once(context.Background()))
			own := statusOf(t, c).Status.Replicas[0]
			assert.EqualValues(t, tc.held, own.TotalDigests)
			assert.Equal(t, tc.refuted, loop.Lost())
			assert.Equal(t, !tc.refuted, own.Full)
			assert.False(t, refuted(own))
			if tc.refuted {
				return
			}
			assert.Equal(t, set, own.VerifiedSet)
			assert.EqualValues(t, 10, own.VerifiedDigests)

			// Compared without the store's usage, which follows the free space of the filesystem the test's
			// temporary directory is on: other tests writing to it can move that across a rounding step
			// between two passes, and the status is then rewritten for it, as it should be.
			before := own
			before.Store = nil
			require.NoError(t, loop.once(context.Background()))
			after := statusOf(t, c).Status.Replicas[0]
			after.Store = nil
			assert.Equal(t, before, after, "an idle pass over a sound store changes nothing in its report")
			assert.False(t, refuted(after))
		})
	}
}

// TestACountDoesNotRefuteWhatItCannotJudge: a Full about another set is Reverify's, one about no set, or
// with the set unknown, is no evidence either way, and a total that was not measured — the walk refused —
// is the last number, not a reading.
func TestACountDoesNotRefuteWhatItCannotJudge(t *testing.T) {
	t.Run("full for no set", func(t *testing.T) {
		state := report.State{Full: true, VerifiedDigests: 10}
		_, found := refutedByCount(&state, "set-now")
		assert.False(t, found)
		state.VerifiedSet = "set-before"
		_, found = refutedByCount(&state, "set-now")
		assert.False(t, found, "another set")
		state.VerifiedSet = "set-now"
		_, found = refutedByCount(&state, "")
		assert.False(t, found, "a set that cannot be told")
		state.Full = false
		_, found = refutedByCount(&state, "set-now")
		assert.False(t, found, "nothing to refute")
	})

	t.Run("a walk that fails", func(t *testing.T) {
		loop, c, logs, _ := idleCluster(t, true, 0, fullLeader())
		loop.DataDir = filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(loop.DataDir, nil, 0o600))
		loop.lastTotalDigests.Store(453)

		require.NoError(t, loop.once(context.Background()))
		own := statusOf(t, c).Status.Replicas[0]
		assert.True(t, own.Full, "a store that cannot be walked refutes nothing")
		assert.EqualValues(t, 453, own.TotalDigests, "and the last number stands")
		assert.False(t, loop.Lost())
		assert.NotContains(t, logs.String(), "holds fewer digests")
	})

	// A process that has not measured its store yet has no last number, and the total it falls back on
	// is zero: not a reading of an empty store, and no evidence against the Full it carried over.
	t.Run("a walk that fails in a fresh process", func(t *testing.T) {
		loop, c, logs, _ := idleCluster(t, true, 0, fullLeader())
		loop.DataDir = filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(loop.DataDir, nil, 0o600))
		require.Zero(t, loop.lastTotalDigests.Load())

		require.NoError(t, loop.once(context.Background()))
		own := statusOf(t, c).Status.Replicas[0]
		assert.True(t, own.Full, "a store that cannot be walked refutes nothing, with no last number either")
		assert.EqualValues(t, 453, own.VerifiedDigests)
		assert.False(t, loop.Lost())
		assert.False(t, loop.TookBack())
		assert.NotContains(t, logs.String(), "holds fewer digests")
	})
}

// TestAFollowerWhoseCountRefutesItsFullWithdrawsIt: a follower carrying a stale Full over a wiped store
// was left to its recheck, six hours, while the election could hand it the lease. It publishes itself
// not full on the pass that measured the total, and its next idle pass reads its store again. The lease
// is no business of a follower's: nothing is taken back and nothing owed.
func TestAFollowerWhoseCountRefutesItsFullWithdrawsIt(t *testing.T) {
	stale := fullFollower()
	stale.Node = "master-0"
	leader := fullLeader()
	leader.Node, leader.Address, leader.AheadSet = "master-1", closedAddress(t), ""
	loop, c, logs, set := idleCluster(t, false, 1, stale, leader)

	// An update pending, and this replica holding what is ahead of the set by its report and by a copy
	// of its own: an AheadSet the pass would publish but for the withdrawal.
	pending := deployedRelease("v1.77.0")
	pending.Object["status"] = map[string]any{"phase": "Pending"}
	require.NoError(t, c.Create(context.Background(), pending))
	gotSet, ahead, err := CurrentKeys(context.Background(), c)
	require.NoError(t, err)
	require.Equal(t, set, gotSet, "the pending release is not part of the set")
	require.NotEmpty(t, ahead)
	storage := statusOf(t, c)
	storage.Status.Replicas[0].AheadSet = ahead
	require.NoError(t, c.Status().Update(context.Background(), storage))
	loop.aheadMu.Lock()
	loop.aheadHeld = map[string]bool{ahead: true}
	loop.aheadMu.Unlock()

	require.NoError(t, loop.once(context.Background()))

	status := statusOf(t, c).Status
	own := status.Replicas[0]
	assert.False(t, own.Full)
	assert.Zero(t, own.VerifiedDigests)
	assert.Empty(t, own.VerifiedSet)
	assert.Empty(t, own.AheadSet, "what it held ahead was copied into the store it lost")
	assert.False(t, refuted(own))
	assert.EqualValues(t, 1, own.TotalDigests)
	assert.Equal(t, registryv1alpha1.ReplicaRoleFollower, own.Role)
	assert.Empty(t, own.Error, "a stale report is not a failure of this pass")
	assert.Equal(t, leader.Full, status.Replicas[1].Full, "the leader's entry is left alone")
	assert.False(t, loop.Lost(), "a follower has nothing to take back or step down from")
	assert.False(t, loop.TookBack())
	assert.False(t, loop.Complete())
	assert.Zero(t, loop.lastStoreCheck.Load(), "its next idle pass reads the store")
	assert.Contains(t, logs.String(), "reports itself not full and reads its store again")
	assert.Contains(t, logs.String(), "store_total=1 verified=453")
}

// TestALeaderRefutedByItsCountStepsDownOnceAnotherIsFull: the idle pass does not ask whether another
// replica is full, and takes the report back as a holder with nobody else full does. With a follower
// full for the set, the next review steps it down in that replica's favour — the step-down a take-back
// owes — without reading the disk.
func TestALeaderRefutedByItsCountStepsDownOnceAnotherIsFull(t *testing.T) {
	loop, c, logs, set := idleCluster(t, true, 1, fullLeader(), fullFollower())

	require.NoError(t, loop.once(context.Background()))
	require.True(t, loop.TookBack())
	require.False(t, statusOf(t, c).Status.Replicas[0].Full)
	assert.False(t, refuted(statusOf(t, c).Status.Replicas[0]))

	disk := &sampler{results: []fill.SampleResult{{ReleaseHeld: false}}}
	loop.StoreSample = disk.sample
	why, handOver := loop.WhyStepAside(context.Background(), statusOf(t, c).Status.Replicas, set, "v1.76.6", holding(true))
	assert.Equal(t, "master-1 holds the whole image set and this replica's store does not hold it", why)
	assert.True(t, handOver)
	assert.Zero(t, disk.taken.Load())
	assert.Equal(t, registryv1alpha1.ReplicaRoleFollower, statusOf(t, c).Status.Replicas[0].Role)
	assert.Contains(t, logs.String(), "now holds the set whole, so it gives up the lease")
}

// TestALeaseLostDuringThePassIsNotTakenBack: the pass began as the holder and the lease moved before
// the count refuted its Full. The pass publishes nothing — see once — and the replica, which no longer
// leads, is not marked as having taken back a lead it does not have.
func TestALeaseLostDuringThePassIsNotTakenBack(t *testing.T) {
	loop, c, logs, _ := idleCluster(t, true, 1, fullLeader())
	moving := &leaseMovesAfter{}
	moving.left.Store(1)
	loop.Leadership = moving
	before := statusOf(t, c).Status.Replicas[0]

	require.NoError(t, loop.once(context.Background()))

	assert.False(t, loop.Lost())
	assert.False(t, loop.TookBack())
	assert.Equal(t, before, statusOf(t, c).Status.Replicas[0], "nothing published")
	assert.NotContains(t, logs.String(), "holds fewer digests")
}

// leaseMovesAfter holds the lease for the first left questions and not after.
type leaseMovesAfter struct{ left atomic.Int32 }

func (l *leaseMovesAfter) IsLeader() bool { return l.left.Add(-1) >= 0 }

// TestACopiedStoreIsWalkedAfresh: a fill or a replication changes the store, and the survey from before
// it is not what the store holds after it. A fill whose reportHeld cannot read the set publishes the
// branch's own numbers, Full included; had the total come from a survey cached before the copy — an
// empty store, a minute ago — the next idle pass would have refuted that Full and refilled for nothing.
func TestACopiedStoreIsWalkedAfresh(t *testing.T) {
	upstream := startRegistry(t)
	local := startRegistry(t)
	one := pushDigest(t, upstream, "deckhouse/ee")
	two := pushDigest(t, upstream, "deckhouse/ee")
	pushImage(t, upstream, "deckhouse/ee:v1.70.1")
	pushInstaller(t, upstream, "deckhouse/ee/install:v1.70.1", []string{one, two})

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{Scheme: registryv1alpha1.SchemeHTTP, Host: upstream, Path: "/deckhouse/ee"},
		},
		Source:   &registryv1alpha1.StorageSource{ExpectedDigests: 4},
		NeedSync: true,
	})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, DeclaredDigests: 4,
	}}
	loop, _, _ := newLoop(t, true, nil)
	c := releaseAwareClient(t, storage, deployedRelease("v1.70.1"))
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.LocalAddress = local
	loop.WriteAddress = local
	loop.DataDir = t.TempDir()

	// Surveyed while the store was empty, and then the copy lands on the disk.
	_, err := loop.surveyStore(nil, "")
	require.NoError(t, err)
	storeHolding(t, loop.DataDir, 4)

	require.NoError(t, loop.once(context.Background()))
	replica := replicaOf(t, c, "master-0")
	require.True(t, replica.Full, "the branch's own numbers, the set unreadable through the store")
	assert.EqualValues(t, 4, replica.TotalDigests, "the total walked after the copy, not before it")

	// The controller lowers needSync; the next pass is idle and measures the total again, within the
	// minute the survey stands for.
	stored := statusOf(t, c)
	stored.Spec.NeedSync = false
	require.NoError(t, c.Update(context.Background(), stored))
	loop.lastStoreCheck.Store(time.Now().UnixNano())

	require.NoError(t, loop.once(context.Background()))
	replica = replicaOf(t, c, "master-0")
	assert.True(t, replica.Full, "the store holds what the fill said")
	assert.False(t, refuted(replica))
	assert.False(t, loop.Lost())
}

// TestAReplicatedStoreIsWalkedAfresh is the replication's half of TestACopiedStoreIsWalkedAfresh: a
// follower whose reportHeld cannot read the set publishes the branch's own numbers, Full included, and
// the total beside them is walked after the copy, not answered from the survey of the empty store.
func TestAReplicatedStoreIsWalkedAfresh(t *testing.T) {
	leaderStorage := startRegistry(t)
	followerStorage := startRegistry(t)
	one := pushDigest(t, leaderStorage, "system/deckhouse")
	two := pushDigest(t, leaderStorage, "system/deckhouse")
	pushImage(t, leaderStorage, "system/deckhouse:v1.76.6")
	pushInstaller(t, leaderStorage, "system/deckhouse/install:v1.76.6", []string{one, two})
	release := resolveTag(t, leaderStorage, "system/deckhouse", "v1.76.6")

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Source:   &registryv1alpha1.StorageSource{ExpectedDigests: 4},
		NeedSync: true,
	})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader,
		Full: true, VerifiedDigests: 3, Address: leaderStorage,
	}}
	loop, _, _ := newLoop(t, false, nil)
	c := releaseAwareClient(t, storage, deployedRelease("v1.76.6"))
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.LocalAddress = followerStorage
	loop.WriteAddress = followerStorage
	loop.ReportedAddress = followerStorage
	loop.DataDir = t.TempDir()

	// Surveyed while the store was empty, and then the copy lands on the disk: the two images and the
	// release.
	_, err := loop.surveyStore(nil, "")
	require.NoError(t, err)
	heldOnDisk(t, loop.DataDir, one, two, release)

	require.NoError(t, loop.once(context.Background()))
	replica := replicaOf(t, c, "master-0")
	require.True(t, replica.Full, "the branch's own numbers, the set unreadable through the store")
	assert.EqualValues(t, 3, replica.VerifiedDigests)
	assert.EqualValues(t, 3, replica.TotalDigests, "the total walked after the copy, not before it")

	// The controller lowers needSync; the next pass is idle and measures the total within the minute.
	stored := statusOf(t, c)
	stored.Spec.NeedSync = false
	require.NoError(t, c.Update(context.Background(), stored))
	loop.lastStoreCheck.Store(time.Now().UnixNano())

	require.NoError(t, loop.once(context.Background()))
	replica = replicaOf(t, c, "master-0")
	assert.True(t, replica.Full, "the store holds what the replication said")
	assert.False(t, refuted(replica))
	assert.Empty(t, replica.Error)
}

// TestTheFirstPassUnderAnAirGapRequestWalksTheStore: the transition's recount, on whose Full the
// controller drops the upstream, does not answer from a survey of the set taken before the request —
// one that a store lost since would still pass. The pass that first sees the request walks the disk.
func TestTheFirstPassUnderAnAirGapRequestWalksTheStore(t *testing.T) {
	store := startRegistry(t)
	running := pushDigest(t, store, "system/deckhouse")
	pushImage(t, store, "system/deckhouse:v1.76.6")
	pushInstaller(t, store, "system/deckhouse/install:v1.76.6", []string{running})

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{Node: "master-0"}}
	loop, _, _ := newLoop(t, true, nil)
	c := releaseAwareClient(t, storage, deployedRelease("v1.76.6"))
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.LocalAddress = store
	loop.WriteAddress = store
	loop.DataDir = t.TempDir()
	release := resolveTag(t, store, "system/deckhouse", "v1.76.6")
	heldOnDisk(t, loop.DataDir, running, release)
	tagOnDisk(t, loop.DataDir, "v1.76.6", release)

	require.NoError(t, loop.once(context.Background()))
	require.True(t, replicaOf(t, c, "master-0").Full, "the set on the disk, counted")
	require.False(t, loop.airGapSeen.Load())

	// Lost within the minute the survey stands for, with nothing to notice it; then air-gap is
	// requested, the upstream held through the transition.
	loop.DataDir = t.TempDir()
	stored := statusOf(t, c)
	stored.Spec.Upstream = &registryv1alpha1.Upstream{
		Endpoint: registryv1alpha1.Endpoint{Scheme: registryv1alpha1.SchemeHTTP, Host: "held.example.invalid"},
	}
	stored.Spec.AirGapRequested = true
	require.NoError(t, c.Update(context.Background(), stored))
	loop.lastStoreCheck.Store(time.Now().UnixNano())

	require.NoError(t, loop.once(context.Background()))
	replica := replicaOf(t, c, "master-0")
	assert.False(t, replica.Full, "the recount says what the disk holds now, not the survey from before")
	assert.Zero(t, replica.VerifiedDigests)
	assert.False(t, refuted(replica))
	assert.True(t, loop.airGapSeen.Load())

	// Seen once per request: the passes after it answer from the survey as before.
	assert.False(t, loop.surveyStale.Load())
	require.NoError(t, loop.once(context.Background()))
	assert.False(t, loop.surveyStale.Load())

	// And a request that goes and comes back walks again. The request goes, and the set is pushed back
	// and found whole: the report and the survey of the set a recount leaves, seeded here, since with
	// an upstream held and no request nothing recounts, and the pass that does run fills (Reverify),
	// which walks the store afresh by itself. Lost once more within the minute that survey stands for,
	// the first recount of a new request says so.
	stored = statusOf(t, c)
	stored.Spec.AirGapRequested = false
	require.NoError(t, c.Update(context.Background(), stored))
	require.NoError(t, loop.once(context.Background()))
	assert.False(t, loop.airGapSeen.Load())

	heldOnDisk(t, loop.DataDir, running, release)
	tagOnDisk(t, loop.DataDir, "v1.76.6", release)
	set, _, err := CurrentSet(context.Background(), c)
	require.NoError(t, err)
	stored = statusOf(t, c)
	own := &stored.Status.Replicas[0]
	own.Full, own.VerifiedSet, own.VerifiedDigests, own.Error = true, set, 2, ""
	require.NoError(t, c.Status().Update(context.Background(), stored))
	declared, deployed, err := loop.declaredSet(context.Background())
	require.NoError(t, err)
	loop.surveyStale.Store(true)
	survey, err := loop.surveyStore(declared, deployed)
	require.NoError(t, err)
	require.EqualValues(t, len(declared), survey.Declared, "the set back on the disk, surveyed whole")

	loop.DataDir = t.TempDir()
	stored = statusOf(t, c)
	stored.Spec.AirGapRequested = true
	require.NoError(t, c.Update(context.Background(), stored))
	require.NoError(t, loop.once(context.Background()))
	assert.False(t, replicaOf(t, c, "master-0").Full, "the store as it is now, not the reading from before the request")
	assert.True(t, loop.airGapSeen.Load())
}

// TestAPassDoesNotPutBackTheFullAFindingTookBack: a pass that read the store, or carried its Full over,
// before a blob of the set was found broken publishes that Full after the finding took it back — see
// report.Integrity.ShortOfSet — unless it sees the finding came in while it ran. A broken blob no image of
// the set references takes nothing back.
func TestAPassDoesNotPutBackTheFullAFindingTookBack(t *testing.T) {
	ten := fullLeader()
	ten.VerifiedDigests, ten.DeclaredDigests, ten.AheadSet = 10, 10, ""

	for name, ofTheSet := range map[string]bool{"a blob of the set": true, "a blob of no image of the set": false} {
		t.Run(name, func(t *testing.T) {
			loop, c, _, _ := idleCluster(t, true, 12, ten)
			image, blobs := storetest.Image(t, loop.DataDir, "system/deckhouse", "layer")
			_, other := storetest.Image(t, loop.DataDir, "system/deckhouse/pulled", "other")
			loop.declaredMu.Lock()
			loop.declared, loop.declaredFor = map[string]struct{}{image: {}}, "v1.76.6"
			loop.declaredMu.Unlock()

			broken := other[1]
			if ofTheSet {
				broken = blobs[1]
			}
			// Found while the pass runs: the first look at the recorded blobs is the pass's own.
			bad := map[string]bool{}
			loop.Corrupted = func() map[string]bool {
				if len(bad) == 0 {
					bad[broken] = true
					loop.CorruptionChanged()
					return nil
				}
				return bad
			}

			require.NoError(t, loop.once(context.Background()))
			assert.Equal(t, !ofTheSet, statusOf(t, c).Status.Replicas[0].Full)
			assert.Equal(t, !ofTheSet, loop.Complete())
		})
	}
}

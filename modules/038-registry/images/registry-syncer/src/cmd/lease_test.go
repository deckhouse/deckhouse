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

package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-syncer/internal/run"
)

// TestTheLeaseSaysWhoLeads: a holder renewing is live; one that stopped renewing a moment ago still
// leads, so that a minute without the API server does not restart an incomplete fill elsewhere; one
// gone for longer, or a released lease, names nobody, and the election is open.
func TestTheLeaseSaysWhoLeads(t *testing.T) {
	renewed := time.Date(2026, 9, 30, 18, 37, 0, 0, time.UTC)
	lease := func(holder string) *coordinationv1.Lease {
		return &coordinationv1.Lease{Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       ptr.To(holder),
			RenewTime:            &metav1.MicroTime{Time: renewed},
			LeaseDurationSeconds: ptr.To[int32](30),
		}}
	}

	assert.Equal(t, run.Lease{Known: true, Holder: "master-0", Live: true},
		leaseAt(lease("master-0"), renewed.Add(10*time.Second)))
	assert.Equal(t, run.Lease{Known: true, Holder: "master-0"},
		leaseAt(lease("master-0"), renewed.Add(31*time.Second)), "expired a moment ago: still the one leading")
	assert.Equal(t, run.Lease{Known: true},
		leaseAt(lease("master-0"), renewed.Add(91*time.Second)), "gone: nobody leads")
	assert.Equal(t, run.Lease{Known: true}, leaseAt(lease(""), renewed), "released on stepping aside")
	assert.Equal(t, run.Lease{Known: true}, leaseAt(&coordinationv1.Lease{}, renewed), "never held")
}

// TestAReplicaWithoutAPodIsNotInTheElection: a report outlives its pod. The leader's replica removed,
// its pod's replacement waiting unscheduled on a cordoned node, the others read its last report — the
// fullest, and role Leader — and waited for it.
func TestAReplicaWithoutAPodIsNotInTheElection(t *testing.T) {
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0"}, {Node: "master-1"}, {Node: "master-2"}, {Node: "master-3"},
	}
	pod := func(node string, phase corev1.PodPhase, deleting bool) corev1.Pod {
		p := corev1.Pod{Spec: corev1.PodSpec{NodeName: node}, Status: corev1.PodStatus{Phase: phase}}
		if deleting {
			p.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		}
		return p
	}
	pods := []corev1.Pod{
		pod("master-0", corev1.PodRunning, false),
		pod("", corev1.PodPending, false), // master-1's replacement, not scheduled anywhere
		pod("master-2", corev1.PodRunning, true),
		pod("master-3", corev1.PodPending, false), // scheduled, still starting
	}

	var nodes []string
	for _, replica := range replicasOnNodes("master-2", replicas, pods) {
		nodes = append(nodes, replica.Node)
	}
	assert.Equal(t, []string{"master-0", "master-2", "master-3"}, nodes,
		"master-1 has no pod; master-2 is the one asking, and always stands for itself")
}

// nodeIn is a node whose Ready condition has had status since a moment.
func nodeIn(status corev1.ConditionStatus, since time.Time) *corev1.Node {
	return &corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
		{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
		{Type: corev1.NodeReady, Status: status, LastTransitionTime: metav1.Time{Time: since}},
	}}}
}

// nodesOf answers a read of a node from a fixed set, and the error given for any node it does not hold.
func nodesOf(nodes map[string]*corev1.Node, missing error) func(string) (*corev1.Node, error) {
	return func(name string) (*corev1.Node, error) {
		if node, ok := nodes[name]; ok {
			return node, nil
		}
		return nil, missing
	}
}

func nodeNames(replicas []registryv1alpha1.StorageReplicaStatus) []string {
	var names []string
	for _, replica := range replicas {
		names = append(names, replica.Node)
	}
	return names
}

// TestAReplicaOnALostNodeIsNotInTheElection: a pod does not leave a node that dies — the storage
// tolerates not-ready and unreachable for good — so the election has to tell from the node. Unknown for
// longer than nodeLostAfter is a kubelet nobody has heard from, and its replica is out; anything else,
// and a node that cannot be read, keeps it in.
func TestAReplicaOnALostNodeIsNotInTheElection(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	notFound := apierrors.NewNotFound(schema.GroupResource{Resource: "nodes"}, "")

	for _, tc := range []struct {
		name   string
		node   *corev1.Node
		err    error
		counts bool
	}{
		{"unheard from for two minutes: lost", nodeIn(corev1.ConditionUnknown, now.Add(-2*time.Minute)), nil, false},
		{"unheard from for half a minute: a kubelet late with a report", nodeIn(corev1.ConditionUnknown, now.Add(-30*time.Second)), nil, true},
		{"not ready, the kubelet saying so: alive", nodeIn(corev1.ConditionFalse, now.Add(-10*time.Minute)), nil, true},
		// The pod's own readiness is not asked: its registry walks the store before it opens a port,
		// and that takes minutes on a large one.
		{"ready, its pod not ready for ten minutes: alive", nodeIn(corev1.ConditionTrue, now.Add(-10*time.Minute)), nil, true},
		{"no Ready condition at all: alive", &corev1.Node{}, nil, true},
		{"gone: lost", nil, notFound, false},
		{"cannot be read: alive, as when the pods cannot be listed", nil, errors.New("the server is currently unable to handle the request"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nodes := map[string]*corev1.Node{"master-0": nodeIn(corev1.ConditionTrue, now.Add(-time.Hour))}
			if tc.node != nil {
				nodes["master-1"] = tc.node
			}
			missing := tc.err
			if missing == nil {
				missing = notFound
			}
			replicas := []registryv1alpha1.StorageReplicaStatus{{Node: "master-0"}, {Node: "master-1"}}

			got := nodeNames(replicasOnLiveNodes("master-0", replicas, nodesOf(nodes, missing), now))
			if tc.counts {
				assert.Equal(t, []string{"master-0", "master-1"}, got)
			} else {
				assert.Equal(t, []string{"master-0"}, got)
			}
		})
	}

	t.Run("its own replica always stands for itself", func(t *testing.T) {
		nodes := map[string]*corev1.Node{"master-0": nodeIn(corev1.ConditionUnknown, now.Add(-time.Hour))}
		replicas := []registryv1alpha1.StorageReplicaStatus{{Node: "master-0"}}

		assert.Equal(t, []string{"master-0"},
			nodeNames(replicasOnLiveNodes("master-0", replicas, nodesOf(nodes, notFound), now)),
			"a replica that can ask is on a node that is there, whatever its object says")
	})
}

// TestASurvivorTakesOverFromADeadFullLeader is the dead end the lost-node rule exists for. The leader
// filled the set and died; its pod stays Running on a node the cluster no longer hears from, and its
// last report — full — kept `someone is full` true, so neither incomplete survivor would stand, and the
// cluster never filled again. Without the report, exactly one of them leads: the fuller one — also when
// its last copy from the dead leader failed, as it does when the leader dies under it.
func TestASurvivorTakesOverFromADeadFullLeader(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 459},
		{Node: "master-1", Role: registryv1alpha1.ReplicaRoleFollower, VerifiedDigests: 300},
		{Node: "master-2", Role: registryv1alpha1.ReplicaRoleFollower, VerifiedDigests: 120},
	}
	nodes := nodesOf(map[string]*corev1.Node{
		"master-0": nodeIn(corev1.ConditionUnknown, now.Add(-2*time.Minute)),
		"master-1": nodeIn(corev1.ConditionTrue, now.Add(-time.Hour)),
		"master-2": nodeIn(corev1.ConditionTrue, now.Add(-time.Hour)),
	}, apierrors.NewNotFound(schema.GroupResource{Resource: "nodes"}, ""))

	for _, lease := range []run.Lease{
		{Known: true, CanFill: true},                     // expired past its grace: nobody leads
		{Known: true, CanFill: true, Holder: "master-0"}, // expired, still in its grace
	} {
		var leading []string
		for _, self := range []string{"master-1", "master-2"} {
			if run.MayLead(self, replicasOnLiveNodes(self, replicas, nodes, now), "", lease) {
				leading = append(leading, self)
			}
		}
		assert.Equal(t, []string{"master-1"}, leading, "lease %+v", lease)

		for _, self := range []string{"master-1", "master-2"} {
			assert.False(t, run.MayLead(self, replicas, "", lease),
				"counted, the dead leader's report keeps %s out: the dead end this closes", self)
		}

		cutOff := append([]registryv1alpha1.StorageReplicaStatus(nil), replicas...)
		cutOff[1].Source = "master-0"
		cutOff[1].Error = "discovering what to copy: reading the image set of main: " +
			"the image set could not be read from 10.0.0.1:5001"
		leading = nil
		for _, self := range []string{"master-1", "master-2"} {
			if run.MayLead(self, replicasOnLiveNodes(self, cutOff, nodes, now), "", lease) {
				leading = append(leading, self)
			}
		}
		assert.Equal(t, []string{"master-1"}, leading, "a failed copy from the dead leader, lease %+v", lease)
	}
}

// TestPresentReplicasReadsTheNodes: the reads go to the API server by name, and a refused one keeps
// the report, as a refused list of pods always has.
func TestPresentReplicasReadsTheNodes(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	lost := nodeIn(corev1.ConditionUnknown, now.Add(-5*time.Minute))
	lost.Name = "master-1"
	alive := nodeIn(corev1.ConditionTrue, now.Add(-time.Hour))
	alive.Name = "master-0"
	pod := func(node string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "registry-storage-" + node, Namespace: "d8-system",
				Labels: map[string]string{"app": "registry-storage"}},
			Spec:   corev1.PodSpec{NodeName: node},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		}
	}
	replicas := []registryv1alpha1.StorageReplicaStatus{{Node: "master-0"}, {Node: "master-1"}, {Node: "master-2"}}

	clientset := fake.NewClientset(alive, lost, pod("master-0"), pod("master-1"), pod("master-2"))
	assert.Equal(t, []string{"master-0"},
		nodeNames(presentReplicas(context.Background(), clientset, "d8-system", "master-0", replicas, now)),
		"master-1's node is unheard from, and master-2's is gone altogether")

	refused := fake.NewClientset(alive, lost, pod("master-0"), pod("master-1"), pod("master-2"))
	refused.PrependReactor("get", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "nodes"}, "", errors.New("no rule"))
	})
	assert.Equal(t, []string{"master-0", "master-1", "master-2"},
		nodeNames(presentReplicas(context.Background(), refused, "d8-system", "master-0", replicas, now)),
		"a node that cannot be read is no evidence about it")
}

// TestTheTrackerSaysWhetherATermStartedOrEnded: the elector calls OnStoppedLeading for an attempt that
// never won as much as for a term that ended, and a start may land after its stop (see
// TestTheLeaderGaugeFollowsTheLease); the callbacks log only what took effect.
func TestTheTrackerSaysWhetherATermStartedOrEnded(t *testing.T) {
	tracker := &leadership{}

	term, cancel := context.WithCancel(context.Background())
	cancel()
	assert.False(t, tracker.stopped(), "an attempt that never won has no term to end, and says so")
	assert.False(t, tracker.started(term), "and a start that came after it takes no effect")
	assert.False(t, tracker.IsLeader(), "the term was over before its start arrived")

	assert.True(t, tracker.started(context.Background()))
	assert.True(t, tracker.IsLeader())
	assert.True(t, tracker.stopped(), "a term that started ends")
	assert.False(t, tracker.IsLeader())
	assert.False(t, tracker.stopped(), "once")
}

// TestTheLoopFollowsTheLeaseAndThePresentReplicas: the loop is given the lease and the election's view
// of the replicas, read from the cluster — see run.Loop.currentLeader for which decides when.
func TestTheLoopFollowsTheLeaseAndThePresentReplicas(t *testing.T) {
	now := time.Now()
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{Name: "registry-storage", Namespace: "d8-system"},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       ptr.To("master-1"),
			RenewTime:            &metav1.MicroTime{Time: now},
			LeaseDurationSeconds: ptr.To[int32](60),
		},
	}
	pod := func(node string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "registry-storage-" + node, Namespace: "d8-system",
				Labels: map[string]string{"app": "registry-storage"}},
			Spec:   corev1.PodSpec{NodeName: node},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		}
	}
	node := func(name string, status corev1.ConditionStatus, since time.Time) *corev1.Node {
		n := nodeIn(status, since)
		n.Name = name
		return n
	}
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0"}, {Node: "master-1"}, {Node: "master-2"},
	}

	clientset := fake.NewClientset(lease,
		node("master-0", corev1.ConditionTrue, now.Add(-time.Hour)),
		node("master-1", corev1.ConditionTrue, now.Add(-time.Hour)),
		node("master-2", corev1.ConditionUnknown, now.Add(-2*time.Minute)),
		pod("master-0"), pod("master-1"), pod("master-2"))
	loop := &run.Loop{}
	followTheLease(loop, clientset, "d8-system", "master-0", "registry-storage")

	ctx := context.Background()
	assert.Equal(t, run.Lease{Known: true, Holder: "master-1", Live: true}, loop.ReadLease(ctx))
	assert.Equal(t, []string{"master-0", "master-1"}, nodeNames(loop.Present(ctx, replicas)),
		"master-2's node is lost")

	clientset.PrependReactor("get", "leases", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("the server is currently unable to handle the request")
	})
	assert.Equal(t, run.Lease{}, loop.ReadLease(ctx), "a lease that cannot be read is unknown")
}

// TestARenewalSeenHereIsLive: a follower whose clock ran a minute ahead of the leader's took every
// renewal for an expired lease, and copied from nobody — in air-gap, never getting the set. A renewal
// it saw happen counts by its own clock; the first read, which saw none, leaves it to the clocks.
func TestARenewalSeenHereIsLive(t *testing.T) {
	// This node's clock is 90 s ahead of the holder's.
	here := time.Date(2026, 9, 30, 18, 37, 0, 0, time.UTC)
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{Name: "registry-storage", Namespace: "d8-system"},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       ptr.To("master-1"),
			RenewTime:            &metav1.MicroTime{Time: here.Add(-90 * time.Second)},
			LeaseDurationSeconds: ptr.To[int32](60),
		},
	}
	clientset := fake.NewClientset(lease)
	renew := func(holder string, at time.Time) {
		lease.Spec.HolderIdentity = ptr.To(holder)
		lease.Spec.RenewTime = &metav1.MicroTime{Time: at}
		_, err := clientset.CoordinationV1().Leases("d8-system").Update(context.Background(), lease, metav1.UpdateOptions{})
		assert.NoError(t, err)
	}
	seen := &renewals{}
	read := func(now time.Time) run.Lease {
		return readRenewedLease(context.Background(), clientset, "d8-system", "registry-storage", seen, now)
	}

	assert.Equal(t, run.Lease{Known: true, Holder: "master-1"}, read(here),
		"the first read saw no renewal, and by the clocks the lease expired half a minute ago")

	renew("master-1", here.Add(-80*time.Second))
	assert.Equal(t, run.Lease{Known: true, Holder: "master-1", Live: true}, read(here.Add(10*time.Second)),
		"renewed since the last read")
	assert.Equal(t, run.Lease{Known: true, Holder: "master-1", Live: true}, read(here.Add(60*time.Second)),
		"a minute after the read before the renewal")

	assert.Equal(t, run.Lease{Known: true}, read(here.Add(61*time.Second)),
		"no renewal seen for longer than the lease lasts, and by the clocks it is long gone")

	renew("master-2", here.Add(-10*time.Second))
	assert.Equal(t, run.Lease{Known: true, Holder: "master-2"}, read(here.Add(80*time.Second)),
		"a new holder's first renewal proves nothing yet")
	renew("master-2", here.Add(-5*time.Second))
	assert.Equal(t, run.Lease{Known: true, Holder: "master-2", Live: true}, read(here.Add(85*time.Second)))
}

// TestARenewalFirstSeenAfterAGapIsNotLive is H2b of a DVP stand. A follower's syncer was cut from the API
// server from 14:28:02 to 14:36:25, while the leader renewed until 14:30:58 and was powered off. On its
// return the follower read that last renewal for the first time, counted the holder live for a minute
// from then, and replicated from the dead leader 343 s after it stopped renewing. A move proves a renewal
// made after the read before it, and is credited to that read: here 14:28:11, so long past. Nor can the
// move say the holder is gone — it may have renewed a second ago — so the lease is unknown, and the
// roles decide among the replicas whose node is there, which left out the dead leader on the stand.
//
// The holder's clock is set 90 s behind this one, so that only a renewal seen here can make it live.
func TestARenewalFirstSeenAfterAGapIsNotLive(t *testing.T) {
	const skew = -90 * time.Second
	at := func(hour, minute, second int) time.Time {
		return time.Date(2026, 10, 30, hour, minute, second, 0, time.UTC)
	}
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{Name: "registry-storage", Namespace: "d8-system"},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       ptr.To("master-1"),
			RenewTime:            &metav1.MicroTime{Time: at(14, 27, 58).Add(skew)},
			LeaseDurationSeconds: ptr.To[int32](60),
		},
	}
	clientset := fake.NewClientset(lease)
	renew := func(at time.Time) {
		lease.Spec.RenewTime = &metav1.MicroTime{Time: at.Add(skew)}
		_, err := clientset.CoordinationV1().Leases("d8-system").Update(context.Background(), lease, metav1.UpdateOptions{})
		assert.NoError(t, err)
	}
	seen := &renewals{}
	read := func(now time.Time) run.Lease {
		return readRenewedLease(context.Background(), clientset, "d8-system", "registry-storage", seen, now)
	}

	read(at(14, 28, 1))
	renew(at(14, 28, 8))
	assert.Equal(t, run.Lease{Known: true, Holder: "master-1", Live: true}, read(at(14, 28, 11)),
		"renewed between two reads ten seconds apart: live")

	// Cut from the API server: no reads, while the leader renews to its last and is powered off.
	renew(at(14, 30, 58))
	assert.Equal(t, run.Lease{}, read(at(14, 36, 25)),
		"a renewal first seen after eight minutes without a read proves nothing about the last minute, "+
			"and the clocks, 90 s apart, cannot tell either")
	assert.Equal(t, run.Lease{Known: true}, read(at(14, 36, 35)),
		"read again, not renewed: by the clocks the lease is past its grace")

	renew(at(14, 36, 33))
	assert.Equal(t, run.Lease{Known: true, Holder: "master-1", Live: true}, read(at(14, 36, 35)),
		"once reads are close again, a renewal between them is live")
}

// TestARenewingHolderAfterAGapInReadsIsUnknown: a follower whose clock runs 65 s ahead of the leader's
// copies from it, reading the lease every 10 s, and then cannot reach the API server for 90 s while the
// leader goes on renewing. The first read back sees renewTime moved since a read longer ago than the
// lease lasts, and by the clocks the lease expired 5 s ago. Taken for not live, that cut the copy from a
// leader that never stopped; unknown, the copy goes on — see run.Loop.rearranged — and the next read
// sees it live again.
func TestARenewingHolderAfterAGapInReadsIsUnknown(t *testing.T) {
	const skew = -65 * time.Second
	t0 := time.Date(2026, 10, 30, 15, 0, 0, 0, time.UTC)
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{Name: "registry-storage", Namespace: "d8-system"},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       ptr.To("master-1"),
			RenewTime:            &metav1.MicroTime{Time: t0.Add(skew)},
			LeaseDurationSeconds: ptr.To[int32](60),
		},
	}
	clientset := fake.NewClientset(lease)
	renew := func(at time.Time) {
		lease.Spec.RenewTime = &metav1.MicroTime{Time: at.Add(skew)}
		_, err := clientset.CoordinationV1().Leases("d8-system").Update(context.Background(), lease, metav1.UpdateOptions{})
		assert.NoError(t, err)
	}
	seen := &renewals{}
	read := func(now time.Time) run.Lease {
		return readRenewedLease(context.Background(), clientset, "d8-system", "registry-storage", seen, now)
	}
	live := run.Lease{Known: true, Holder: "master-1", Live: true}

	read(t0)
	for tick := 10 * time.Second; tick <= time.Minute; tick += 10 * time.Second {
		renew(t0.Add(tick - 5*time.Second))
		assert.Equal(t, live, read(t0.Add(tick)), "at %s", tick)
	}

	// 90 s without the API server; the leader renews to the end of it.
	back := t0.Add(150 * time.Second)
	renew(back.Add(-5 * time.Second))
	assert.Equal(t, run.Lease{}, read(back),
		"renewed some time in the last 90 s, and by the clocks expired 10 s ago: neither live nor gone")
	renew(back.Add(5 * time.Second))
	assert.Equal(t, live, read(back.Add(10*time.Second)), "renewed between two close reads")
}

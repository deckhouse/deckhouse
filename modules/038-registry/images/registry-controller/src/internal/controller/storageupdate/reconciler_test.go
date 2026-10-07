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

package storageupdate

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

const (
	oldRevision = "registry-storage-111"
	newRevision = "registry-storage-222"
)

func storageSet(replicas int32, updateRevision string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: Namespace, Name: StorageName},
		Spec:       appsv1.StatefulSetSpec{Replicas: &replicas},
		Status:     appsv1.StatefulSetStatus{UpdateRevision: updateRevision},
	}
}

func replica(ordinal int, node, revision string, ready bool) *corev1.Pod {
	condition := corev1.ConditionFalse
	if ready {
		condition = corev1.ConditionTrue
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: Namespace,
			Name:      StorageName + "-" + string(rune('0'+ordinal)),
			Labels:    map[string]string{"app": storageAppLabel, revisionLabel: revision},
		},
		Spec: corev1.PodSpec{NodeName: node},
		Status: corev1.PodStatus{
			Phase:      corev1.PodRunning,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: condition}},
		},
	}
}

// storageWithUpstream is the cache as it looks when the cluster still has a registry outside it:
// every node keeps that upstream as a fallback, which is what can serve a replica's new image
// while the replica is away.
func storageWithUpstream(host string) *registryv1alpha1.RegistryStorage {
	return &registryv1alpha1.RegistryStorage{
		ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName},
		Spec: registryv1alpha1.RegistryStorageSpec{
			Upstream: &registryv1alpha1.Upstream{
				Endpoint: registryv1alpha1.Endpoint{
					Scheme: registryv1alpha1.SchemeHTTPS, Host: host, Path: "/deckhouse/ee",
				},
			},
		},
	}
}

// airGappedStorage is the case with nothing to fall back to.
func airGappedStorage() *registryv1alpha1.RegistryStorage {
	return &registryv1alpha1.RegistryStorage{
		ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName},
	}
}

func lease(holderIdentity string) *coordinationv1.Lease {
	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{Namespace: Namespace, Name: StorageLeaseName},
		Spec:       coordinationv1.LeaseSpec{HolderIdentity: &holderIdentity},
	}
}

func newReconciler(t *testing.T, objects ...client.Object) (*Reconciler, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

	r := &Reconciler{}
	r.InjectClient(fakeClient)
	return r, fakeClient
}

func reconcile(t *testing.T, r *Reconciler) ctrl.Result {
	t.Helper()

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: Namespace, Name: StorageName},
	})
	require.NoError(t, err)
	return result
}

func deleted(t *testing.T, c client.Client, name string) bool {
	t.Helper()

	pod := &corev1.Pod{}
	err := c.Get(context.Background(), types.NamespacedName{Namespace: Namespace, Name: name}, pod)
	if apierrors.IsNotFound(err) {
		return true
	}
	require.NoError(t, err)
	return pod.DeletionTimestamp != nil
}

// TestFollowersGoBeforeTheLeader is the rule the whole controller exists for.
//
// A StatefulSet's own RollingUpdate replaces pods by ordinal, and which ordinal holds the fill
// lease is not a property of the ordinal. Replacing the leader first hands the lease to a replica
// that is itself about to be replaced, so one update stops the fill twice.
func TestFollowersGoBeforeTheLeader(t *testing.T) {
	r, c := newReconciler(t,
		storageSet(3, newRevision),
		// The leader is the LOWEST ordinal, which is the case the built-in order gets wrong
		// last — so a test that passed by accident would pass here too. It is asserted anyway
		// because the point is that the ordinal has nothing to do with it.
		replica(0, "master-0", oldRevision, true),
		replica(1, "master-1", oldRevision, true),
		replica(2, "master-2", oldRevision, true),
		lease("master-0"),
		storageWithUpstream("registry.deckhouse.io"),
	)

	reconcile(t, r)

	assert.False(t, deleted(t, c, StorageName+"-0"), "the leader must not be replaced while followers are stale")
	followersGone := deleted(t, c, StorageName+"-1") || deleted(t, c, StorageName+"-2")
	assert.True(t, followersGone, "a follower should have been replaced")
}

// TestTheLeaderGoesLast: with every follower on the new revision and serving, the leader is next.
func TestTheLeaderGoesLast(t *testing.T) {
	r, c := newReconciler(t,
		storageSet(3, newRevision),
		replica(0, "master-0", oldRevision, true),
		replica(1, "master-1", newRevision, true),
		replica(2, "master-2", newRevision, true),
		lease("master-0"),
		storageWithUpstream("registry.deckhouse.io"),
	)

	reconcile(t, r)

	assert.True(t, deleted(t, c, StorageName+"-0"), "the leader is the last one left and must now be replaced")
}

// TestOneAtATime: nothing is replaced while a replica is missing or not serving.
//
// Two replicas down at once turns "a slower pull" into "no pull", and on a two-master cluster it
// leaves nothing behind at all. The absent pod is also how "one at a time" is enforced across
// reconciles without this controller keeping any state.
func TestOneAtATime(t *testing.T) {
	t.Run("a replica is still missing", func(t *testing.T) {
		r, c := newReconciler(t,
			storageSet(3, newRevision),
			replica(0, "master-0", oldRevision, true),
			replica(1, "master-1", oldRevision, true),
			lease("master-2"),
			storageWithUpstream("registry.deckhouse.io"),
		)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"))
		assert.False(t, deleted(t, c, StorageName+"-1"))
	})

	t.Run("a replica is not serving yet", func(t *testing.T) {
		r, c := newReconciler(t,
			storageSet(2, newRevision),
			replica(0, "master-0", oldRevision, true),
			replica(1, "master-1", newRevision, false),
			lease("master-0"),
			storageWithUpstream("registry.deckhouse.io"),
		)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"),
			"replacing this would leave a cluster with no replica able to serve")
	})
}

// TestSomethingHasToBeAbleToServeMeanwhile is the half that only matters when nothing else helps.
//
// A replaced pod pulls its new image AFTER it is deleted, from a registry that pod was part of.
// Another replica covers that, and so does an upstream. With one replica and no upstream there is
// nothing, and the only correct answer is to refuse: the way into such a cluster is `d8 mirror
// pull` and `d8 mirror push`, and until the images are there the update has no business starting.
func TestSomethingHasToBeAbleToServeMeanwhile(t *testing.T) {
	t.Run("one replica, no upstream: refused", func(t *testing.T) {
		r, c := newReconciler(t,
			storageSet(1, newRevision),
			replica(0, "master-0", oldRevision, true),
			lease("master-0"),
			airGappedStorage(),
		)

		result := reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"),
			"replacing the only replica of an air-gapped cache leaves nothing able to serve its new image")
		assert.Equal(t, blockedInterval, result.RequeueAfter,
			"a refusal that will not resolve itself should not be retried like a wait")
	})

	t.Run("one replica with an upstream: allowed", func(t *testing.T) {
		// The node keeps the upstream as a fallback, which is exactly what it is for.
		r, c := newReconciler(t,
			storageSet(1, newRevision),
			replica(0, "master-0", oldRevision, true),
			lease("master-0"),
			storageWithUpstream("registry.deckhouse.io"),
		)

		reconcile(t, r)

		assert.True(t, deleted(t, c, StorageName+"-0"))
	})

	t.Run("air-gapped but with a sibling: allowed", func(t *testing.T) {
		// Two replicas are mirrors of each other on every node, so one can go.
		r, c := newReconciler(t,
			storageSet(2, newRevision),
			replica(0, "master-0", oldRevision, true),
			replica(1, "master-1", oldRevision, true),
			lease("master-0"),
			airGappedStorage(),
		)

		reconcile(t, r)

		assert.True(t, deleted(t, c, StorageName+"-1"), "a follower should have been replaced")
		assert.False(t, deleted(t, c, StorageName+"-0"), "and not the leader")
	})

	t.Run("no RegistryStorage at all: refused", func(t *testing.T) {
		r, c := newReconciler(t,
			storageSet(1, newRevision),
			replica(0, "master-0", oldRevision, true),
			lease("master-0"),
		)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"),
			"nothing is known about a fallback, so nothing may be taken down")
	})
}

// TestNothingToDo covers the states in which this controller must keep its hands off.
func TestNothingToDo(t *testing.T) {
	t.Run("every replica is on the new revision", func(t *testing.T) {
		r, c := newReconciler(t,
			storageSet(2, newRevision),
			replica(0, "master-0", newRevision, true),
			replica(1, "master-1", newRevision, true),
			lease("master-0"),
			storageWithUpstream("registry.deckhouse.io"),
		)

		result := reconcile(t, r)

		assert.Zero(t, result.RequeueAfter, "a settled cache should not be polled")
		assert.False(t, deleted(t, c, StorageName+"-0"))
		assert.False(t, deleted(t, c, StorageName+"-1"))
	})

	t.Run("the StatefulSet has not worked out its revision yet", func(t *testing.T) {
		r, c := newReconciler(t,
			storageSet(1, ""),
			replica(0, "master-0", oldRevision, true),
			lease("master-0"),
			storageWithUpstream("registry.deckhouse.io"),
		)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"),
			"an empty update revision is not a reason to delete anything")
	})

	t.Run("there is no cache at all", func(t *testing.T) {
		r, _ := newReconciler(t)
		assert.Zero(t, reconcile(t, r).RequeueAfter)
	})
}

// TestNoKnownLeaderStillMakesProgress: an unreadable lease must not stall the update of the cache.
//
// Empty is the safe answer in an unobvious way — with no known leader every stale replica looks
// like a follower, so the update proceeds in ordinal order. That is the built-in behaviour this
// controller improves on, and stalling forever would be worse than not improving on it.
func TestNoKnownLeaderStillMakesProgress(t *testing.T) {
	r, c := newReconciler(t,
		storageSet(2, newRevision),
		replica(0, "master-0", oldRevision, true),
		replica(1, "master-1", oldRevision, true),
		storageWithUpstream("registry.deckhouse.io"),
	)

	reconcile(t, r)

	someoneGone := deleted(t, c, StorageName+"-0") || deleted(t, c, StorageName+"-1")
	assert.True(t, someoneGone, "an unreadable lease must not stop the cache from being updated")
}

// publishingStorage is the cache during the air-gap transition: the write endpoint is open, so
// `d8 mirror push` is how the content arrives, and the upstream is still held meanwhile.
func publishingStorage(complete bool) *registryv1alpha1.RegistryStorage {
	storage := storageWithUpstream("registry.deckhouse.io")
	storage.Spec.Publish = true
	storage.Status.SafeToDropUpstream = complete
	return storage
}

// TestAnIncompleteCacheIsUpdatedLikeAComplete one: completeness gates dropping the upstream, which
// the layout controller holds until the leader has the whole set, and has nothing to say about
// replacing a replica. A gate on it here meant a cache that could not complete — a budget reached,
// a fill failing on what the update itself fixes — could never be updated at all.
func TestAnIncompleteCacheIsUpdatedLikeAComplete(t *testing.T) {
	for _, complete := range []bool{false, true} {
		r, c := newReconciler(t,
			storageSet(3, newRevision),
			replica(0, "master-0", oldRevision, true),
			replica(1, "master-1", oldRevision, true),
			replica(2, "master-2", oldRevision, true),
			lease("master-0"),
			publishingStorage(complete),
		)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"), "complete=%v: the leader still goes last", complete)
		assert.True(t, deleted(t, c, StorageName+"-1") || deleted(t, c, StorageName+"-2"),
			"complete=%v: a follower is replaced whether or not the cache holds its set", complete)
	}
}

// TestAReplicaThatCannotStartOnTheOldRevisionIsReplacedAtOnce is the one case where waiting for
// readiness waits forever: the old revision is what keeps the replica from starting.
//
// Measured on an Engine cluster on 2026-10-02: the first render ran before the master NodeGroup
// existed, so the store was created under /opt, which is read-only there. The pod sat in
// ContainerCreating on "mkdir /opt/deckhouse/registry: read-only file system", and the corrected
// revision never reached it, because nothing may move while a replica is not serving.
func TestAReplicaThatCannotStartOnTheOldRevisionIsReplacedAtOnce(t *testing.T) {
	t.Run("the only replica, the leader, with nothing else to pull from", func(t *testing.T) {
		r, c := newReconciler(t,
			storageSet(1, newRevision),
			replica(0, "master-0", oldRevision, false),
			lease("master-0"),
			airGappedStorage(),
		)

		reconcile(t, r)

		// Every gate a serving replica goes through is about what it would stop serving, and this
		// one serves nothing.
		assert.True(t, deleted(t, c, StorageName+"-0"))
	})

	t.Run("beside replicas that serve", func(t *testing.T) {
		r, c := newReconciler(t,
			storageSet(2, newRevision),
			replica(0, "master-0", oldRevision, true),
			replica(1, "master-1", oldRevision, false),
			lease("master-0"),
			storageWithUpstream("registry.deckhouse.io"),
		)

		reconcile(t, r)

		assert.True(t, deleted(t, c, StorageName+"-1"), "the replica that serves nothing goes first")
		assert.False(t, deleted(t, c, StorageName+"-0"), "and the one that serves stays, one at a time")
	})

	// A replica that has served and is failing its readiness probe for a moment — one second
	// timeout, three tries — will serve again. Deleting it past the gates is the deletion they
	// exist to refuse: here the only replica of an air-gapped cache, which would not come back.
	t.Run("not a replica that has run and is only failing its probe", func(t *testing.T) {
		blipping := replica(0, "master-0", oldRevision, false)
		started := true
		blipping.Status.ContainerStatuses = []corev1.ContainerStatus{{
			Name:    "distribution",
			Started: &started,
			State:   corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
		}}
		r, c := newReconciler(t,
			storageSet(1, newRevision),
			blipping,
			lease("master-0"),
			airGappedStorage(),
		)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"))
	})

	// One that has run and crashes on the old revision serves nothing and will not on that
	// revision, so it goes like one that never started — rather than until someone deletes it by
	// hand, with the revision that may fix it held back meanwhile.
	t.Run("a replica crash-looping on the old revision", func(t *testing.T) {
		looping := replica(0, "master-0", oldRevision, false)
		looping.Status.ContainerStatuses = []corev1.ContainerStatus{{
			Name:                 "distribution",
			RestartCount:         7,
			State:                corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
			LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1}},
		}}
		r, c := newReconciler(t,
			storageSet(1, newRevision),
			looping,
			lease("master-0"),
			airGappedStorage(),
		)

		reconcile(t, r)

		assert.True(t, deleted(t, c, StorageName+"-0"))
	})
}

// airGappedWith is an air-gapped cache whose replicas report how much of the set they hold.
func airGappedWith(fullNodes ...string) *registryv1alpha1.RegistryStorage {
	storage := airGappedStorage()
	for _, node := range []string{"master-0", "master-1", "master-2"} {
		storage.Status.Replicas = append(storage.Status.Replicas, registryv1alpha1.StorageReplicaStatus{
			Node: node, Full: slices.Contains(fullNodes, node),
		})
	}
	return storage
}

// TestOnlyTheLeaderServesTheNodes is the rule the agents follow: they pull from the leader and from
// nothing else in the cache, so a follower being up says nothing about whether the nodes can pull.
//
// Measured on the `scenarios` unit on 2026-10-05: in air-gap the leader was replaced with two
// followers behind it that were still copying, the update had nothing that could serve, and every
// agent answered 502 — the store's own replicas in ImagePullBackOff on their own image.
func TestOnlyTheLeaderServesTheNodes(t *testing.T) {
	t.Run("in air-gap the leader stays while no follower holds the whole set", func(t *testing.T) {
		r, c := newReconciler(t,
			storageSet(3, newRevision),
			replica(0, "master-0", oldRevision, true),
			replica(1, "master-1", newRevision, true),
			replica(2, "master-2", newRevision, true),
			lease("master-0"),
			airGappedWith(), // nobody full
		)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"),
			"a follower still copying would lead with images missing, and nothing else serves")
	})

	t.Run("in air-gap the leader goes once a follower holds the whole set", func(t *testing.T) {
		r, c := newReconciler(t,
			storageSet(3, newRevision),
			replica(0, "master-0", oldRevision, true),
			replica(1, "master-1", newRevision, true),
			replica(2, "master-2", newRevision, true),
			lease("master-0"),
			airGappedWith("master-2"),
		)

		reconcile(t, r)

		assert.True(t, deleted(t, c, StorageName+"-0"), "the full follower takes the lease over")
	})
}

// TestAReplicaOnItsWayOutIsNotServing: a pod being deleted still reads Ready until its containers
// stop, and counted as serving it would authorise a second replacement right after the first.
// replicas() leaves such a pod out, so the set looks one short and nothing else is replaced.
func TestAReplicaOnItsWayOutIsNotServing(t *testing.T) {
	going := replica(1, "master-1", oldRevision, true)
	now := metav1.Now()
	going.DeletionTimestamp = &now
	going.Finalizers = []string{"test.example.com/hold"} // the fake client keeps it while finalized

	r, c := newReconciler(t,
		storageSet(3, newRevision),
		replica(0, "master-0", oldRevision, true),
		going,
		replica(2, "master-2", oldRevision, true),
		lease("master-2"),
		storageWithUpstream("registry.deckhouse.io"),
	)

	reconcile(t, r)

	assert.False(t, deleted(t, c, StorageName+"-0"), "one at a time: the previous replacement has not finished")
}

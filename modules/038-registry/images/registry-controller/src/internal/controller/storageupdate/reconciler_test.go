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

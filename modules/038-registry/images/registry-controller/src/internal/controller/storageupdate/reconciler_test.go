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

// holder is the image-holder pod on a node: ready means kubelet pulled every image it names,
// which is the only fact the replacement needs from it.
func holder(node string, generation string, ready bool) *corev1.Pod {
	condition := corev1.ConditionFalse
	if ready {
		condition = corev1.ConditionTrue
	}
	labels := map[string]string{"app": ImageHolderName}
	if generation != "" {
		labels["pod-template-generation"] = generation
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: Namespace,
			Name:      ImageHolderName + "-" + node,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{NodeName: node},
		Status: corev1.PodStatus{
			Phase:      corev1.PodRunning,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: condition}},
		},
	}
}

func holderSet(generation int64) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: Namespace, Name: ImageHolderName, Generation: generation},
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
		holderSet(1),
		holder("master-0", "1", true),
		holder("master-1", "1", true),
		holder("master-2", "1", true),
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
		holderSet(1),
		holder("master-0", "1", true),
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
			holderSet(1),
			holder("master-0", "1", true),
			holder("master-1", "1", true),
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
			holderSet(1),
			holder("master-0", "1", true),
			holder("master-1", "1", true),
		)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"),
			"replacing this would leave a cluster with no replica able to serve")
	})
}

// TestTheImagesHaveToBeOnTheNodeFirst is the other half of the mechanism, and the half that only
// matters when nothing else can help.
//
// A replaced pod pulls after it is deleted, from a registry it is itself part of. With several
// replicas the others cover it; with one replica and no upstream the pull has no source at all.
func TestTheImagesHaveToBeOnTheNodeFirst(t *testing.T) {
	t.Run("the holder is not ready", func(t *testing.T) {
		r, c := newReconciler(t,
			storageSet(1, newRevision),
			replica(0, "master-0", oldRevision, true),
			lease("master-0"),
			holderSet(1),
			holder("master-0", "1", false),
		)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"),
			"the only replica must not be replaced before its node holds the new images")
	})

	t.Run("the holder is still on the previous generation", func(t *testing.T) {
		// Ready, but holding the images of the version being replaced — which is no help at
		// all, and is indistinguishable from the right thing if only readiness is checked.
		r, c := newReconciler(t,
			storageSet(1, newRevision),
			replica(0, "master-0", oldRevision, true),
			lease("master-0"),
			holderSet(2),
			holder("master-0", "1", true),
		)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"))
	})

	t.Run("the holder has no pod on that node", func(t *testing.T) {
		r, c := newReconciler(t,
			storageSet(1, newRevision),
			replica(0, "master-0", oldRevision, true),
			lease("master-0"),
			holderSet(1),
			holder("master-1", "1", true),
		)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"))
	})

	t.Run("the holder is ready on the right generation", func(t *testing.T) {
		r, c := newReconciler(t,
			storageSet(1, newRevision),
			replica(0, "master-0", oldRevision, true),
			lease("master-0"),
			holderSet(3),
			holder("master-0", "3", true),
		)

		reconcile(t, r)

		assert.True(t, deleted(t, c, StorageName+"-0"))
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
			holderSet(1),
			holder("master-0", "1", true),
			holder("master-1", "1", true),
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
			holderSet(1),
			holder("master-0", "1", true),
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
		holderSet(1),
		holder("master-0", "1", true),
		holder("master-1", "1", true),
	)

	reconcile(t, r)

	someoneGone := deleted(t, c, StorageName+"-0") || deleted(t, c, StorageName+"-1")
	assert.True(t, someoneGone, "an unreadable lease must not stop the cache from being updated")
}

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
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	testingclock "k8s.io/utils/clock/testing"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-controller/internal/metrics"
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

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))
	return scheme
}

// newReconciler is the reconciler over one fake client, standing in for the cache and the API server
// alike: a cache that has seen everything, which is what most of these tests are not about.
func newReconciler(t *testing.T, objects ...client.Object) (*Reconciler, client.Client) {
	t.Helper()

	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(objects...).Build()

	r := &Reconciler{}
	r.InjectClient(fakeClient)
	r.InjectAPIReader(fakeClient)
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
// nothing, and unless the new images are already on the node — see
// TestTheOnlyReplicaGoesOnceItsNodeHoldsTheNewImages — the only correct answer is to refuse.
func TestSomethingHasToBeAbleToServeMeanwhile(t *testing.T) {
	t.Run("one replica, no upstream, no image holder: refused", func(t *testing.T) {
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
		// The agents pull from the leader and from nothing else in the cache, so a follower can go:
		// the leader keeps serving while it is away. The leader goes last, after every follower
		// (chooseNext), and then only if a follower holds the whole set.
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

// The fingerprints of the set the cluster needs, as the replicas report them with Full: the one
// after the update, and the one before it.
const (
	currentSet  = "set-after-update"
	previousSet = "set-before-update"
)

// airGappedWith is an air-gapped cache whose replicas report how much of the set they hold: the
// full ones against the current set, as a syncer reports a Full.
func airGappedWith(fullNodes ...string) *registryv1alpha1.RegistryStorage {
	storage := airGappedStorage()
	for _, node := range []string{"master-0", "master-1", "master-2"} {
		replica := registryv1alpha1.StorageReplicaStatus{Node: node, Full: slices.Contains(fullNodes, node)}
		if replica.Full {
			replica.VerifiedSet = currentSet
		}
		storage.Status.Replicas = append(storage.Status.Replicas, replica)
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
			airGappedWith("master-0", "master-2"),
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

// The images of the storage pod in these tests, as the template names them.
var (
	newImages = []string{"registry.example.com/registry@sha256:new-distribution",
		"registry.example.com/registry@sha256:new-auth",
		"registry.example.com/common@sha256:new-kube-rbac-proxy",
		"registry.example.com/registry@sha256:new-syncer"}
	oldImages = []string{"registry.example.com/registry@sha256:old-distribution",
		"registry.example.com/registry@sha256:old-auth",
		"registry.example.com/common@sha256:old-kube-rbac-proxy",
		"registry.example.com/registry@sha256:old-syncer"}
)

// storageSetWith is storageSet with a pod template running images, which is what the holder is
// compared against.
func storageSetWith(replicas int32, images []string) *appsv1.StatefulSet {
	set := storageSet(replicas, newRevision)
	for i, image := range images {
		set.Spec.Template.Spec.Containers = append(set.Spec.Template.Spec.Containers,
			corev1.Container{Name: "c" + string(rune('0'+i)), Image: image})
	}
	return set
}

// holder is the image holder's pod on node, running images; every container is Running, the pod
// Ready.
func holder(node string, images []string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: Namespace,
			Name:      holderDaemonSet + "-" + node,
			Labels:    map[string]string{"app": holderAppLabel},
		},
		Spec: corev1.PodSpec{NodeName: node},
		Status: corev1.PodStatus{
			Phase:      corev1.PodRunning,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
		},
	}
	for i, image := range images {
		name := "image-holder-" + string(rune('0'+i))
		pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: name, Image: image})
		pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, corev1.ContainerStatus{
			Name: name, Image: image, Ready: true,
			State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
		})
	}
	return pod
}

// pulling makes the holder's container for image wait for the kubelet's reason, and the pod not
// ready.
func pulling(pod *corev1.Pod, image, reason string) *corev1.Pod {
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Image != image {
			continue
		}
		pod.Status.ContainerStatuses[i].Ready = false
		pod.Status.ContainerStatuses[i].State = corev1.ContainerState{
			Waiting: &corev1.ContainerStateWaiting{Reason: reason},
		}
	}
	pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}}
	return pod
}

func withRecorder(r *Reconciler) *record.FakeRecorder {
	recorder := record.NewFakeRecorder(16)
	r.InjectRecorder(recorder)
	return recorder
}

// events drains what was recorded so far.
func events(recorder *record.FakeRecorder) []string {
	var out []string
	for {
		select {
		case event := <-recorder.Events:
			out = append(out, event)
		default:
			return out
		}
	}
}

func blockedGauge(reason BlockReason) float64 {
	return testutil.ToFloat64(metrics.StorageUpdateBlocked.WithLabelValues(string(reason)))
}

// TestTheOnlyReplicaGoesOnceItsNodeHoldsTheNewImages is the single-master air-gapped cluster, which
// could not update its cache at all before: nothing serves the replica's new images while it is
// away, and without them on the node it would come back to an empty pull and stay down.
//
// The image holder pulls them from the old replica while it still serves and keeps them, so the
// replacement is safe once the holder on the replica's node runs every image of the template — and
// not a moment before.
func TestTheOnlyReplicaGoesOnceItsNodeHoldsTheNewImages(t *testing.T) {
	t.Run("the holder runs every new image: replaced", func(t *testing.T) {
		r, c := newReconciler(t,
			storageSetWith(1, newImages),
			replica(0, "master-0", oldRevision, true),
			lease("master-0"),
			airGappedStorage(),
			holder("master-0", newImages),
		)
		recorder := withRecorder(r)

		result := reconcile(t, r)

		assert.True(t, deleted(t, c, StorageName+"-0"), "the new images are on the node, the replica comes back")
		assert.Equal(t, settleInterval, result.RequeueAfter)
		recorded := strings.Join(events(recorder), "\n")
		assert.Contains(t, recorded, "ReplacingOnlyReplica",
			"the one replacement the nodes notice is announced where it will be read")
		assert.Contains(t, recorded, "cannot pull images")
	})

	t.Run("with nobody holding the lease as well", func(t *testing.T) {
		// Nothing else could lead, so the lease says nothing here.
		r, c := newReconciler(t,
			storageSetWith(1, newImages),
			replica(0, "master-0", oldRevision, true),
			airGappedStorage(),
			holder("master-0", newImages),
		)

		reconcile(t, r)

		assert.True(t, deleted(t, c, StorageName+"-0"))
	})

	t.Run("the holder still runs the previous images: refused, nothing to push", func(t *testing.T) {
		r, c := newReconciler(t,
			storageSetWith(1, newImages),
			replica(0, "master-0", oldRevision, true),
			lease("master-0"),
			airGappedStorage(),
			holder("master-0", oldImages),
		)
		recorder := withRecorder(r)

		result := reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"))
		assert.Equal(t, blockedInterval, result.RequeueAfter)
		recorded := strings.Join(events(recorder), "\n")
		assert.Contains(t, recorded, "UpdateBlocked")
		assert.Contains(t, recorded, string(BlockedHolderNotReady))
		assert.Contains(t, recorded, "previous images")
		assert.NotContains(t, recorded, "d8 mirror push",
			"the holder only has to roll; pushing images changes nothing about that")
	})

	t.Run("the holder is still pulling: refused, nothing to push", func(t *testing.T) {
		r, c := newReconciler(t,
			storageSetWith(1, newImages),
			replica(0, "master-0", oldRevision, true),
			lease("master-0"),
			airGappedStorage(),
			pulling(holder("master-0", newImages), newImages[3], "ContainerCreating"),
		)
		recorder := withRecorder(r)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"))
		recorded := strings.Join(events(recorder), "\n")
		assert.Contains(t, recorded, newImages[3], "the message names what is not there yet")
		assert.NotContains(t, recorded, "d8 mirror push")
	})

	t.Run("the holder cannot pull an image: refused, and pushing it is the advice", func(t *testing.T) {
		r, c := newReconciler(t,
			storageSetWith(1, newImages),
			replica(0, "master-0", oldRevision, true),
			lease("master-0"),
			airGappedStorage(),
			pulling(holder("master-0", newImages), newImages[0], "ImagePullBackOff"),
		)
		recorder := withRecorder(r)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"),
			"a container that is not running has not got its image, whatever its spec says")
		recorded := strings.Join(events(recorder), "\n")
		assert.Contains(t, recorded, string(BlockedHolderNotReady))
		assert.Contains(t, recorded, "d8 mirror push",
			"the holder pulls from this cache, so the image it cannot get is the one to bring in")
		assert.Contains(t, recorded, newImages[0])
	})

	t.Run("a failed pull is reported past an image still being created ahead of it", func(t *testing.T) {
		pod := pulling(holder("master-0", newImages), newImages[0], "ContainerCreating")
		pod = pulling(pod, newImages[1], "ImagePullBackOff")

		r, c := newReconciler(t,
			storageSetWith(1, newImages),
			replica(0, "master-0", oldRevision, true),
			lease("master-0"),
			airGappedStorage(),
			pod,
		)
		recorder := withRecorder(r)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"))
		recorded := strings.Join(events(recorder), "\n")
		assert.Contains(t, recorded, "cannot pull "+newImages[1]+" (ImagePullBackOff)",
			"the image named is the one that failed, not the first one missing")
		assert.NotContains(t, recorded, "cannot pull "+newImages[0])
		assert.Contains(t, recorded, "d8 mirror push",
			"the one failure an operator can do something about is not hidden by one still in progress")
	})

	t.Run("the holder cannot run an image it has: refused as a failure, nothing to push", func(t *testing.T) {
		for _, reason := range []string{"CrashLoopBackOff", "CreateContainerError"} {
			t.Run(reason, func(t *testing.T) {
				r, c := newReconciler(t,
					storageSetWith(1, newImages),
					replica(0, "master-0", oldRevision, true),
					lease("master-0"),
					airGappedStorage(),
					pulling(holder("master-0", newImages), newImages[2], reason),
				)
				recorder := withRecorder(r)

				reconcile(t, r)

				assert.False(t, deleted(t, c, StorageName+"-0"))
				recorded := strings.Join(events(recorder), "\n")
				assert.Contains(t, recorded, string(BlockedHolderNotReady))
				assert.Contains(t, recorded, "fails to run it ("+reason+")",
					"a container the kubelet cannot run has its image already: not a pull to wait for")
				assert.Contains(t, recorded, newImages[2])
				assert.NotContains(t, recorded, "is pulling them")
				assert.NotContains(t, recorded, "d8 mirror push",
					"the image is on the node already: pushing it changes nothing")
			})
		}
	})

	// Between two restarts of a crash loop the kubelet shows the container terminated, before it
	// shows the back-off. That is the same failure, not a pull that has not started.
	t.Run("a holder container that has just exited: refused as a failure to run", func(t *testing.T) {
		pod := holder("master-0", newImages)
		pod.Status.ContainerStatuses[2].Ready = false
		pod.Status.ContainerStatuses[2].State = corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{Reason: "StartError", ExitCode: 128},
		}
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}}

		r, c := newReconciler(t,
			storageSetWith(1, newImages),
			replica(0, "master-0", oldRevision, true),
			lease("master-0"),
			airGappedStorage(),
			pod,
		)
		recorder := withRecorder(r)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"))
		recorded := strings.Join(events(recorder), "\n")
		assert.Contains(t, recorded, "fails to run it (StartError)")
		assert.Contains(t, recorded, newImages[2])
		assert.NotContains(t, recorded, "is pulling them")
		assert.NotContains(t, recorded, "d8 mirror push")
	})

	t.Run("a failed pull is reported past a container that cannot run", func(t *testing.T) {
		pod := pulling(holder("master-0", newImages), newImages[0], "CrashLoopBackOff")
		pod = pulling(pod, newImages[1], "ImagePullBackOff")

		r, c := newReconciler(t,
			storageSetWith(1, newImages),
			replica(0, "master-0", oldRevision, true),
			lease("master-0"),
			airGappedStorage(),
			pod,
		)
		recorder := withRecorder(r)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"))
		recorded := strings.Join(events(recorder), "\n")
		assert.Contains(t, recorded, "cannot pull "+newImages[1]+" (ImagePullBackOff)",
			"the failure an operator can fix comes first")
		assert.Contains(t, recorded, "d8 mirror push")
	})

	t.Run("the holder is on another node: refused", func(t *testing.T) {
		r, c := newReconciler(t,
			storageSetWith(1, newImages),
			replica(0, "master-0", oldRevision, true),
			lease("master-0"),
			airGappedStorage(),
			holder("master-1", newImages),
		)
		recorder := withRecorder(r)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"), "images on another node do not help this one start")
		recorded := strings.Join(events(recorder), "\n")
		assert.Contains(t, recorded, string(BlockedSingleReplicaWithoutHolder))
		assert.Contains(t, recorded, holderDaemonSet)
		assert.Contains(t, recorded, "no pod on master-0 yet",
			"on the update that brings the holder this is a moment's wait, and it reads as one")
		assert.NotContains(t, recorded, "d8 mirror push")
	})

	t.Run("a holder on its way out holds nothing", func(t *testing.T) {
		going := holder("master-0", newImages)
		now := metav1.Now()
		going.DeletionTimestamp = &now
		going.Finalizers = []string{"test.example.com/hold"}

		r, c := newReconciler(t,
			storageSetWith(1, newImages),
			replica(0, "master-0", oldRevision, true),
			lease("master-0"),
			airGappedStorage(),
			going,
		)
		recorder := withRecorder(r)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"),
			"once it stops, the image garbage collector may take what it held")
		recorded := strings.Join(events(recorder), "\n")
		assert.Contains(t, recorded, string(BlockedHolderNotReady),
			"every roll of the DaemonSet deletes the old pod first: a holder being replaced, not one missing")
		assert.Contains(t, recorded, "being replaced")
		assert.NotContains(t, recorded, "d8 mirror push")
	})

	t.Run("an init container's image is needed as well", func(t *testing.T) {
		set := storageSetWith(1, newImages)
		set.Spec.Template.Spec.InitContainers = []corev1.Container{
			{Name: "init", Image: "registry.example.com/registry@sha256:new-init"},
		}
		r, c := newReconciler(t,
			set,
			replica(0, "master-0", oldRevision, true),
			lease("master-0"),
			airGappedStorage(),
			holder("master-0", newImages),
		)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"), "a replica that cannot start its init container does not start")
	})

	t.Run("with an upstream the holder is not looked at", func(t *testing.T) {
		r, c := newReconciler(t,
			storageSetWith(1, newImages),
			replica(0, "master-0", oldRevision, true),
			lease("master-0"),
			storageWithUpstream("registry.deckhouse.io"),
			// No holder at all, and nothing waits on it.
		)
		recorder := withRecorder(r)

		reconcile(t, r)

		assert.True(t, deleted(t, c, StorageName+"-0"))
		assert.NotContains(t, strings.Join(events(recorder), "\n"), "ReplacingOnlyReplica",
			"the nodes fall back to the upstream, so this replacement costs them nothing")
	})
}

// TestTheHolderDoesNotStandInForAFollower: with several replicas the leader still waits for a
// follower that holds the whole set, holder or not. The lease would otherwise move to a follower
// missing images, and those only the old leader had would be gone from the cluster.
func TestTheHolderDoesNotStandInForAFollower(t *testing.T) {
	r, c := newReconciler(t,
		storageSetWith(3, newImages),
		replica(0, "master-0", oldRevision, true),
		replica(1, "master-1", newRevision, true),
		replica(2, "master-2", newRevision, true),
		lease("master-0"),
		airGappedWith("master-0"), // the leader alone is full
		holder("master-0", newImages),
		holder("master-1", newImages),
		holder("master-2", newImages),
	)

	reconcile(t, r)

	assert.False(t, deleted(t, c, StorageName+"-0"))
}

// TestTheRefusalSaysWhatEndsIt: the leader waiting for a follower is two situations that read the
// same from the outside, and only one of them ends without an operator.
func TestTheRefusalSaysWhatEndsIt(t *testing.T) {
	t.Run("the leader holds the set: wait for the followers, nothing to do", func(t *testing.T) {
		storage := airGappedWith("master-0")
		storage.Status.Replicas[1].VerifiedDigests = 120
		storage.Status.Replicas[1].DeclaredDigests = 556

		r, c := newReconciler(t,
			storageSet(3, newRevision),
			replica(0, "master-0", oldRevision, true),
			replica(1, "master-1", newRevision, true),
			replica(2, "master-2", newRevision, true),
			lease("master-0"),
			storage,
		)
		recorder := withRecorder(r)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"))
		recorded := strings.Join(events(recorder), "\n")
		assert.Contains(t, recorded, string(BlockedLeaderWithoutFullFollower))
		assert.Contains(t, recorded, StorageName+"-1 on master-1: copying, 120 of 556 digests",
			"the message says which follower it is waiting for and how far it has got")
		assert.Contains(t, recorded, StorageName+"-1 and "+StorageName+"-2 are copying it from the leader")
		assert.Contains(t, recorded, "Nothing needs doing")
		assert.NotContains(t, recorded, "d8 mirror push",
			"the followers are copying what the leader has; pushing it again changes nothing")
	})

	t.Run("the leader does not hold the set either: bring it in", func(t *testing.T) {
		storage := airGappedWith()
		storage.Status.Replicas[2].Error = "fetching manifest: connection refused"

		r, c := newReconciler(t,
			storageSet(3, newRevision),
			replica(0, "master-0", oldRevision, true),
			replica(1, "master-1", newRevision, true),
			replica(2, "master-2", newRevision, true),
			lease("master-0"),
			storage,
		)
		recorder := withRecorder(r)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"))
		recorded := strings.Join(events(recorder), "\n")
		assert.Contains(t, recorded, string(BlockedLeaderWithoutFullFollower))
		assert.Contains(t, recorded, "d8 mirror push",
			"the followers copy from the leader, so what it lacks only an operator can bring in")
		assert.Contains(t, recorded, "error: fetching manifest: connection refused")
	})

	t.Run("the leader has not reported yet: nothing to push", func(t *testing.T) {
		storage := airGappedWith()
		storage.Status.Replicas = storage.Status.Replicas[1:] // no report from master-0

		r, c := newReconciler(t,
			storageSet(3, newRevision),
			replica(0, "master-0", oldRevision, true),
			replica(1, "master-1", newRevision, true),
			replica(2, "master-2", newRevision, true),
			lease("master-0"),
			storage,
		)
		recorder := withRecorder(r)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"))
		recorded := strings.Join(events(recorder), "\n")
		assert.Contains(t, recorded, string(BlockedLeaderWithoutFullFollower))
		assert.Contains(t, recorded, "has not reported")
		assert.NotContains(t, recorded, "d8 mirror push",
			"a leader that has not looked yet says nothing about the images being missing")
	})

	t.Run("the leader's last pass failed: its log, not a push", func(t *testing.T) {
		storage := airGappedWith("master-0", "master-1")
		storage.Status.Replicas[0].Error = "reading the catalogue: input/output error"
		storage.Status.Replicas[0].VerifiedSet = "" // a failed pass names no set

		r, c := newReconciler(t,
			storageSet(3, newRevision),
			replica(0, "master-0", oldRevision, true),
			replica(1, "master-1", newRevision, true),
			replica(2, "master-2", newRevision, true),
			lease("master-0"),
			storage,
		)
		recorder := withRecorder(r)

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"))
		recorded := strings.Join(events(recorder), "\n")
		assert.Contains(t, recorded, "reading the catalogue: input/output error")
		assert.Contains(t, recorded, "container syncer of pod d8-system/"+StorageName+"-0")
		assert.NotContains(t, recorded, "d8 mirror push",
			"a pass that could not finish says nothing about the images being missing")
		assert.Contains(t, recorded, StorageName+"-1 on master-1: reports full",
			"a follower's Full cannot be measured against a leader that names no set")
	})

	// The leader holds the set, and the followers are each doing something else: the message says
	// each, because "the followers are copying it" was said of followers that had not started and of
	// one that was failing, followed by "Nothing needs doing".
	followersSay := func(t *testing.T, storage *registryv1alpha1.RegistryStorage) string {
		t.Helper()
		r, c := newReconciler(t,
			storageSet(3, newRevision),
			replica(0, "master-0", oldRevision, true),
			replica(1, "master-1", newRevision, true),
			replica(2, "master-2", newRevision, true),
			lease("master-0"),
			storage,
		)
		recorder := withRecorder(r)

		result := reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"))
		assert.Equal(t, settleInterval, result.RequeueAfter, "what the followers do shows only in RegistryStorage")
		recorded := strings.Join(events(recorder), "\n")
		assert.Contains(t, recorded, string(BlockedLeaderWithoutFullFollower))
		assert.NotContains(t, recorded, "d8 mirror push", "the leader holds the set")
		return recorded
	}

	t.Run("one follower copying, one on an earlier set: the copying one is named", func(t *testing.T) {
		storage := airGappedWith("master-0", "master-2")
		storage.Status.Replicas[1].VerifiedDigests = 120
		storage.Status.Replicas[1].DeclaredDigests = 556
		storage.Status.Replicas[2].VerifiedSet = previousSet

		recorded := followersSay(t, storage)

		assert.Contains(t, recorded, StorageName+"-1 is copying it from the leader, and the update continues "+
			"once it has finished")
		assert.NotContains(t, recorded, StorageName+"-2 is copying")
		assert.NotContains(t, recorded, "not reported copying",
			"a copy under way ends the wait sooner than one yet to start")
		assert.Contains(t, recorded, StorageName+"-2 on master-2: full against another set than the leader's")
		assert.Contains(t, recorded, "Nothing needs doing")
	})

	t.Run("a follower failing, one on an earlier set: the log, and not nothing to do", func(t *testing.T) {
		storage := airGappedWith("master-0", "master-2")
		storage.Status.Replicas[1].Error = "copying from the leader: unexpected EOF"
		storage.Status.Replicas[2].VerifiedSet = previousSet

		recorded := followersSay(t, storage)

		assert.Contains(t, recorded, StorageName+"-1 reports an error, and the cause is in the log of container "+
			"syncer of pod d8-system/"+StorageName+"-1")
		assert.Contains(t, recorded, StorageName+"-2 is full against another set than the leader's and has not "+
			"reported copying the leader's yet")
		assert.NotContains(t, recorded, "Nothing needs doing",
			"a follower is failing: something may well need doing, and its log says what")
		assert.NotContains(t, recorded, "copying it from the leader")
	})

	t.Run("one follower copying, one failing: both are said", func(t *testing.T) {
		storage := airGappedWith("master-0")
		storage.Status.Replicas[1].VerifiedDigests = 120
		storage.Status.Replicas[1].DeclaredDigests = 556
		storage.Status.Replicas[2].Error = "copying from the leader: unexpected EOF"

		recorded := followersSay(t, storage)

		assert.Contains(t, recorded, StorageName+"-1 is copying it from the leader")
		assert.Contains(t, recorded, StorageName+"-2 reports an error, and the cause is in the log of container "+
			"syncer of pod d8-system/"+StorageName+"-2")
		assert.NotContains(t, recorded, "Nothing needs doing")
	})

	t.Run("a follower that has not reported: said so", func(t *testing.T) {
		storage := airGappedWith("master-0", "master-1")
		storage.Status.Replicas[1].VerifiedSet = previousSet
		storage.Status.Replicas = storage.Status.Replicas[:2] // no report from master-2

		recorded := followersSay(t, storage)

		assert.Contains(t, recorded, StorageName+"-2 has not reported yet.")
		assert.Contains(t, recorded, StorageName+"-1 is full against another set than the leader's")
		assert.Contains(t, recorded, "Nothing needs doing")
	})
}

// TestAFollowerFullForThePreviousSetDoesNotTakeOver: a follower replaced in this update comes back
// ready with the Full it was last found with, read back from the status, and copies the new set only
// on a later pass. The pods are watched, so the moment it is ready the leader would have gone — and
// with it, in air-gap, the only replica holding the release the cluster is updating to.
func TestAFollowerFullForThePreviousSetDoesNotTakeOver(t *testing.T) {
	storage := airGappedWith("master-0", "master-1", "master-2")
	storage.Status.Replicas[1].VerifiedSet = previousSet
	storage.Status.Replicas[2].VerifiedSet = previousSet

	r, c := newReconciler(t,
		storageSet(3, newRevision),
		replica(0, "master-0", oldRevision, true),
		replica(1, "master-1", newRevision, true),
		replica(2, "master-2", newRevision, true),
		lease("master-0"),
		storage,
	)
	recorder := withRecorder(r)

	result := reconcile(t, r)

	assert.False(t, deleted(t, c, StorageName+"-0"),
		"neither follower holds the set the leader holds, whatever their Full says")
	assert.Equal(t, settleInterval, result.RequeueAfter,
		"the copy shows only in RegistryStorage, which is not watched: looked at again soon, not in minutes")
	assert.InDelta(t, 1, blockedGauge(BlockedLeaderWithoutFullFollower), 0)
	recorded := strings.Join(events(recorder), "\n")
	assert.Contains(t, recorded, string(BlockedLeaderWithoutFullFollower))
	assert.Contains(t, recorded, StorageName+"-1 on master-1: full against another set than the leader's")
	assert.Contains(t, recorded, StorageName+"-1 and "+StorageName+"-2 are full against another set than the "+
		"leader's and have not reported copying the leader's yet",
		"the reports are all the controller sees, and they say the copy has not started")
	assert.NotContains(t, recorded, "copying it from the leader",
		"said here on a stand while both followers sat on the set before the update, and frozen for minutes")
	assert.NotContains(t, recorded, "d8 mirror push", "the leader holds the set; the followers copy it")

	// master-2 has copied the current set: it takes the lease over, as a full follower always did.
	current := &registryv1alpha1.RegistryStorage{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(storage), current))
	current.Status.Replicas[2].VerifiedSet = currentSet
	require.NoError(t, c.Update(context.Background(), current))

	result = reconcile(t, r)

	assert.True(t, deleted(t, c, StorageName+"-0"))
	assert.Equal(t, settleInterval, result.RequeueAfter)
	assert.InDelta(t, 0, blockedGauge(BlockedLeaderWithoutFullFollower), 0)
}

// TestALeaderThatNamesNoSetKeepsTheLease: what a follower holds is measured against the leader's
// fingerprint, so while the leader names none no follower can be shown to be current — even one
// that reports full. Which of the leader's states that is decides what the message says and how
// soon it is looked at again.
func TestALeaderThatNamesNoSetKeepsTheLease(t *testing.T) {
	t.Run("full, and its last pass could not tell the set: a wait", func(t *testing.T) {
		storage := airGappedWith("master-0", "master-1")
		storage.Status.Replicas[0].VerifiedSet = ""

		r, c := newReconciler(t,
			storageSet(3, newRevision),
			replica(0, "master-0", oldRevision, true),
			replica(1, "master-1", newRevision, true),
			replica(2, "master-2", newRevision, true),
			lease("master-0"),
			storage,
		)
		recorder := withRecorder(r)

		result := reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"))
		assert.Equal(t, settleInterval, result.RequeueAfter,
			"an air-gapped leader names the set again on its next pass")
		recorded := strings.Join(events(recorder), "\n")
		assert.Contains(t, recorded, string(BlockedLeaderWithoutFullFollower))
		assert.Contains(t, recorded, "could not tell which set the cluster needs")
		assert.NotContains(t, recorded, "d8 mirror push")
		assert.NotContains(t, recorded, "Nothing needs doing",
			"a leader that keeps failing to read the set keeps the update waiting for good")
		assert.Contains(t, recorded, "the log of container syncer of pod d8-system/"+StorageName+"-0",
			"what to look at if it does not end")
	})

	t.Run("not full: bring the images in, as before", func(t *testing.T) {
		// A follower full while the leader it copies from is not: its Full is for a set the leader
		// has moved off. It used to take the lease over.
		storage := airGappedWith("master-1")

		r, c := newReconciler(t,
			storageSet(3, newRevision),
			replica(0, "master-0", oldRevision, true),
			replica(1, "master-1", newRevision, true),
			replica(2, "master-2", newRevision, true),
			lease("master-0"),
			storage,
		)
		recorder := withRecorder(r)

		result := reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"))
		assert.Equal(t, blockedInterval, result.RequeueAfter,
			"what ends this is an operator pushing images, on a human timescale")
		recorded := strings.Join(events(recorder), "\n")
		assert.Contains(t, recorded, string(BlockedLeaderWithoutFullFollower))
		assert.Contains(t, recorded, "d8 mirror push")
	})
}

// TestTheBlockIsReported: the refusal leaves nothing behind but an event, so the gauge is what an
// alert can see — and it has to go back to 0 when the block ends, or the alert outlives it.
func TestTheBlockIsReported(t *testing.T) {
	r, c := newReconciler(t,
		storageSetWith(1, newImages),
		replica(0, "master-0", oldRevision, true),
		lease("master-0"),
		airGappedStorage(),
		holder("master-0", oldImages),
	)

	reconcile(t, r)

	assert.False(t, deleted(t, c, StorageName+"-0"))
	assert.InDelta(t, 1, blockedGauge(BlockedHolderNotReady), 0)
	for _, reason := range blockReasons {
		if reason != BlockedHolderNotReady {
			assert.InDelta(t, 0, blockedGauge(reason), 0, "only the reason that applies reads 1: %s", reason)
		}
	}
	assert.InDelta(t, 1, testutil.ToFloat64(metrics.StorageReplicasOutdated), 0)

	// The holder rolls: the block ends, and the replica goes.
	updated := holder("master-0", newImages)
	current := &corev1.Pod{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(updated), current))
	updated.ResourceVersion = current.ResourceVersion
	require.NoError(t, c.Update(context.Background(), updated))

	reconcile(t, r)

	assert.True(t, deleted(t, c, StorageName+"-0"))
	assert.InDelta(t, 0, blockedGauge(BlockedHolderNotReady), 0, "a block that has ended must not keep the alert up")
	assert.InDelta(t, 1, testutil.ToFloat64(metrics.StorageReplicasOutdated), 0,
		"the replica just deleted is not updated yet either")

	// And with the cache turned off there is nothing to be blocked or outdated.
	r, _ = newReconciler(t)
	metrics.StorageUpdateBlocked.WithLabelValues(string(BlockedHolderNotReady)).Set(1)
	reconcile(t, r)
	assert.InDelta(t, 0, blockedGauge(BlockedHolderNotReady), 0)
	assert.InDelta(t, 0, testutil.ToFloat64(metrics.StorageReplicasOutdated), 0)
}

// TestOutdatedCountsTheMissing: a replica taken down for replacement is not updated, and counting
// only the pods that exist would read as progress each time one goes.
func TestOutdatedCountsTheMissing(t *testing.T) {
	set := storageSet(3, newRevision)
	pods := []*corev1.Pod{
		replica(0, "master-0", oldRevision, true),
		replica(1, "master-1", newRevision, true),
	}
	assert.Equal(t, 2, outdatedReplicas(set, pods, newRevision))

	pods = append(pods, replica(2, "master-2", newRevision, true))
	pods[0] = replica(0, "master-0", newRevision, true)
	assert.Equal(t, 0, outdatedReplicas(set, pods, newRevision))
}

// newLaggingReconciler is the reconciler as the manager wires it, with a cache that has not seen the
// deletions yet: the cached client lists the replicas as they were when it was made — the ones
// deleted since last, so that another replica comes first — while the API reader and every write go
// to the API server. deletes records the name of every pod the reconciler deletes.
func newLaggingReconciler(t *testing.T, objects ...client.Object) (r *Reconciler, api client.Client, deletes *[]string) {
	t.Helper()

	api = fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(objects...).Build()
	stale := &corev1.PodList{}
	require.NoError(t, api.List(context.Background(), stale,
		client.InNamespace(Namespace), client.MatchingLabels{"app": storageAppLabel}))

	deletes = &[]string{}
	cached := interceptor.NewClient(api.(client.WithWatch), interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			options := &client.ListOptions{}
			options.ApplyOptions(opts)
			pods, isPods := list.(*corev1.PodList)
			if !isPods || options.LabelSelector == nil ||
				!options.LabelSelector.Matches(labels.Set{"app": storageAppLabel}) {
				return c.List(ctx, list, opts...)
			}
			stale.DeepCopyInto(pods)
			slices.SortStableFunc(pods.Items, func(a, b corev1.Pod) int {
				deletedA, deletedB := slices.Contains(*deletes, a.Name), slices.Contains(*deletes, b.Name)
				switch {
				case deletedA == deletedB:
					return 0
				case deletedA:
					return 1
				}
				return -1
			})
			return nil
		},
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			*deletes = append(*deletes, obj.GetName())
			return c.Delete(ctx, obj, opts...)
		},
	})

	r = &Reconciler{}
	r.InjectClient(cached)
	r.InjectAPIReader(api)
	return r, api, deletes
}

// TestTheReplicaJustDeletedIsSeenGoing is the pass right after a replacement. An event during a pass
// marks the key dirty, so the next pass starts as soon as the first returns, before the cache has
// seen the deletion: on a stand four of the nine replacements deleted the same replica twice, some
// 100 ms apart. With two stale followers, the second pass reading the cache could as well take the
// other one down — two replicas away at once, the one thing "one at a time" is there to prevent.
//
// The replicas carry a finalizer, as the stand's pods are held by their grace period: the Delete
// only marks the replica, and the API server lists it, terminating, to the pass that follows.
func TestTheReplicaJustDeletedIsSeenGoing(t *testing.T) {
	terminating := func(pod *corev1.Pod) *corev1.Pod {
		pod.Finalizers = []string{"test.example.com/hold"}
		return pod
	}
	r, api, deletes := newLaggingReconciler(t,
		storageSet(3, newRevision),
		terminating(replica(0, "master-0", oldRevision, true)),
		terminating(replica(1, "master-1", oldRevision, true)),
		terminating(replica(2, "master-2", oldRevision, true)),
		lease("master-0"),
		storageWithUpstream("registry.deckhouse.io"),
	)

	reconcile(t, r)
	require.Len(t, *deletes, 1, "a follower is replaced")
	first := (*deletes)[0]

	// At once, with the cache listing every replica still there and serving, another follower first.
	result := reconcile(t, r)

	assert.Equal(t, []string{first}, *deletes,
		"the replica just deleted is listed going, so nothing is replaced alongside it, and it is not deleted again")
	assert.Equal(t, settleInterval, result.RequeueAfter)
	assert.True(t, deleted(t, api, first), "still there, terminating, as on the stand")
	assert.False(t, deleted(t, api, StorageName+"-0"))

	// The same pass reading the replicas from the cache, as it did: a second follower goes.
	r.Reader = nil
	reconcile(t, r)

	require.Len(t, *deletes, 2)
	assert.NotEqual(t, first, (*deletes)[1], "what the stand risked: two replicas down at once")
}

// TestTheReplicaIsDeletedByItsUID: the StatefulSet recreates a replica under the same name, and a
// pass that decided on the old pod must not delete its successor, which nobody checked the gates
// against. The API server answers a UID that no longer matches with a Conflict, and that means the
// pod decided on is gone already — as NotFound does — not that the pass failed: neither is said as
// a FailedDelete.
//
// The fake client ignores a UID precondition, so the interceptor checks it and answers the Conflict.
func TestTheReplicaIsDeletedByItsUID(t *testing.T) {
	withUID := func(pod *corev1.Pod) *corev1.Pod {
		pod.UID = types.UID("uid-" + pod.Name)
		return pod
	}
	conflict := func(name string) error {
		return apierrors.NewConflict(schema.GroupResource{Resource: "pods"}, name,
			errors.New("the UID in the precondition does not match the UID in record"))
	}
	type outcome struct {
		result        ctrl.Result
		api           client.Client
		preconditions []types.UID
		recorded      string
	}
	replaced := func(t *testing.T, answer func(name string) error, objects ...client.Object) outcome {
		t.Helper()

		api := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(objects...).Build()
		var preconditions []types.UID
		r := &Reconciler{}
		r.InjectClient(interceptor.NewClient(api, interceptor.Funcs{
			Delete: func(_ context.Context, _ client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				options := &client.DeleteOptions{}
				options.ApplyOptions(opts)
				if options.Preconditions == nil || options.Preconditions.UID == nil {
					preconditions = append(preconditions, "")
				} else {
					preconditions = append(preconditions, *options.Preconditions.UID)
				}
				return answer(obj.GetName())
			},
		}))
		r.InjectAPIReader(api)
		recorder := withRecorder(r)
		result := reconcile(t, r)
		return outcome{result: result, api: api, preconditions: preconditions,
			recorded: strings.Join(events(recorder), "\n")}
	}

	t.Run("a follower replaced past the gates", func(t *testing.T) {
		got := replaced(t, conflict,
			storageSet(3, newRevision),
			withUID(replica(0, "master-0", oldRevision, true)),
			withUID(replica(1, "master-1", oldRevision, true)),
			withUID(replica(2, "master-2", newRevision, true)),
			lease("master-0"),
			storageWithUpstream("registry.deckhouse.io"),
		)

		assert.Equal(t, []types.UID{"uid-" + StorageName + "-1"}, got.preconditions)
		assert.Equal(t, settleInterval, got.result.RequeueAfter, "looked at again, as after any replacement")
		assert.False(t, deleted(t, got.api, StorageName+"-1"))
		assert.NotContains(t, got.recorded, "FailedDelete", "the pod decided on is gone, nothing failed")
	})

	t.Run("a replica that never ran on the old revision", func(t *testing.T) {
		got := replaced(t, conflict,
			storageSet(3, newRevision),
			withUID(replica(0, "master-0", oldRevision, true)),
			withUID(replica(1, "master-1", oldRevision, false)),
			withUID(replica(2, "master-2", newRevision, true)),
			lease("master-0"),
			storageWithUpstream("registry.deckhouse.io"),
		)

		assert.Equal(t, []types.UID{"uid-" + StorageName + "-1"}, got.preconditions)
		assert.Equal(t, settleInterval, got.result.RequeueAfter)
		assert.NotContains(t, got.recorded, "FailedDelete")
	})

	t.Run("a replica deleted meanwhile by something else", func(t *testing.T) {
		got := replaced(t, func(name string) error {
			return apierrors.NewNotFound(schema.GroupResource{Resource: "pods"}, name)
		},
			storageSet(3, newRevision),
			withUID(replica(0, "master-0", oldRevision, true)),
			withUID(replica(1, "master-1", oldRevision, true)),
			withUID(replica(2, "master-2", newRevision, true)),
			lease("master-0"),
			storageWithUpstream("registry.deckhouse.io"),
		)

		assert.Equal(t, []types.UID{"uid-" + StorageName + "-1"}, got.preconditions)
		assert.Equal(t, settleInterval, got.result.RequeueAfter)
		assert.NotContains(t, got.recorded, "FailedDelete", "the pod decided on is gone, nothing failed")
	})
}

// TestAFailedDeletionIsTriedAgainSoon: a Delete that fails used to end the pass in its error, which
// the workqueue retries with a back-off doubling up to 1000 s, and while every replica serves nothing
// else wakes this controller. On a stand an admission policy denied the Delete; once it was lifted,
// nothing tried again for almost five minutes. Now the pass reports the failure and is looked at
// again after settleInterval, as after a Delete that went through, so no back-off builds up.
//
// The interceptor denies the Delete as an admission policy does, then fails it on the API server's
// side, then lets it through.
func TestAFailedDeletionIsTriedAgainSoon(t *testing.T) {
	withUID := func(pod *corev1.Pod) *corev1.Pod {
		pod.UID = types.UID("uid-" + pod.Name)
		return pod
	}
	type harness struct {
		r             *Reconciler
		api           client.Client
		recorder      *record.FakeRecorder
		preconditions *[]types.UID
	}
	failingTwice := func(t *testing.T, objects ...client.Object) harness {
		t.Helper()

		api := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(objects...).Build()
		preconditions := &[]types.UID{}
		r := &Reconciler{}
		r.InjectClient(interceptor.NewClient(api, interceptor.Funcs{
			Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				options := &client.DeleteOptions{}
				options.ApplyOptions(opts)
				if options.Preconditions == nil || options.Preconditions.UID == nil {
					*preconditions = append(*preconditions, "")
				} else {
					*preconditions = append(*preconditions, *options.Preconditions.UID)
				}
				switch len(*preconditions) {
				case 1:
					return apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, obj.GetName(),
						errors.New("ValidatingAdmissionPolicy 'hold-storage-pod' denied request"))
				case 2:
					return apierrors.NewInternalError(errors.New("etcdserver: request timed out"))
				}
				return c.Delete(ctx, obj, opts...)
			},
		}))
		r.InjectAPIReader(api)
		return harness{r: r, api: api, recorder: withRecorder(r), preconditions: preconditions}
	}
	// replacedOnTheThirdTry runs the three passes and checks what each of them did.
	replacedOnTheThirdTry := func(t *testing.T, h harness, name string) {
		t.Helper()

		for try := 1; try <= 2; try++ {
			result := reconcile(t, h.r)
			assert.Equal(t, settleInterval, result.RequeueAfter, "try %d: looked at again soon, with no back-off", try)
			assert.False(t, deleted(t, h.api, name), "try %d: the Delete failed", try)
			assert.Contains(t, strings.Join(events(h.recorder), "\n"), "FailedDelete", "try %d", try)
		}

		result := reconcile(t, h.r)

		assert.Equal(t, settleInterval, result.RequeueAfter)
		assert.True(t, deleted(t, h.api, name), "replaced once the Delete is let through")
		uid := types.UID("uid-" + name)
		assert.Equal(t, []types.UID{uid, uid, uid}, *h.preconditions, "every try deletes the pod decided on, by its UID")
	}

	t.Run("a follower behind an upstream", func(t *testing.T) {
		h := failingTwice(t,
			storageSet(3, newRevision),
			withUID(replica(0, "master-0", oldRevision, true)),
			withUID(replica(1, "master-1", oldRevision, true)),
			withUID(replica(2, "master-2", newRevision, true)),
			lease("master-0"),
			storageWithUpstream("registry.deckhouse.io"),
		)

		replacedOnTheThirdTry(t, h, StorageName+"-1")
	})

	t.Run("a replica that never ran on the old revision", func(t *testing.T) {
		h := failingTwice(t,
			storageSet(3, newRevision),
			withUID(replica(0, "master-0", oldRevision, true)),
			withUID(replica(1, "master-1", oldRevision, false)),
			withUID(replica(2, "master-2", newRevision, true)),
			lease("master-0"),
			storageWithUpstream("registry.deckhouse.io"),
		)

		replacedOnTheThirdTry(t, h, StorageName+"-1")
	})

	t.Run("the only air-gapped replica: no outage announced until the Delete goes through", func(t *testing.T) {
		h := failingTwice(t,
			storageSetWith(1, newImages),
			withUID(replica(0, "master-0", oldRevision, true)),
			lease("master-0"),
			airGappedStorage(),
			holder("master-0", newImages),
		)

		reconcile(t, h.r)

		recorded := strings.Join(events(h.recorder), "\n")
		assert.Contains(t, recorded, "FailedDelete")
		assert.Contains(t, recorded, "denied request", "the error says why")
		assert.Contains(t, recorded, "Retrying in "+settleInterval.String())
		assert.NotContains(t, recorded, "ReplacingOnlyReplica", "the replica did not go, and the nodes still pull")

		reconcile(t, h.r)
		assert.NotContains(t, strings.Join(events(h.recorder), "\n"), "ReplacingOnlyReplica")

		reconcile(t, h.r)

		assert.True(t, deleted(t, h.api, StorageName+"-0"))
		recorded = strings.Join(events(h.recorder), "\n")
		assert.Contains(t, recorded, "ReplacingOnlyReplica")
		assert.NotContains(t, recorded, "FailedDelete")
	})

	t.Run("the pass is reported: the replica is outdated, and nothing refused", func(t *testing.T) {
		h := failingTwice(t,
			storageSetWith(1, newImages),
			withUID(replica(0, "master-0", oldRevision, true)),
			lease("master-0"),
			airGappedStorage(),
			holder("master-0", newImages),
		)
		metrics.StorageUpdateBlocked.WithLabelValues(string(BlockedHolderNotReady)).Set(1)
		metrics.StorageReplicasOutdated.Set(0)

		reconcile(t, h.r)

		assert.False(t, deleted(t, h.api, StorageName+"-0"))
		assert.InDelta(t, 1, testutil.ToFloat64(metrics.StorageReplicasOutdated), 0, "the replica is still there")
		for _, reason := range blockReasons {
			assert.InDelta(t, 0, blockedGauge(reason), 0, "every gate passed, nothing was refused: %s", reason)
		}
	})
}

// TestAFailedDeletionIsSaidOnceUntilItChanges: the recorder's spam filter gives every Warning event of
// the StatefulSet one allowance, so a FailedDelete on every retry of a long denial would use up what
// an UpdateBlocked said after it needs. It is said again when the error's reason changes, and once
// a blocked interval has passed; every try is still made a settle interval after the last.
func TestAFailedDeletionIsSaidOnceUntilItChanges(t *testing.T) {
	api := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(
		storageSet(3, newRevision),
		replica(0, "master-0", oldRevision, true),
		replica(1, "master-1", oldRevision, true),
		replica(2, "master-2", newRevision, true),
		lease("master-0"),
		storageWithUpstream("registry.deckhouse.io"),
	).Build()
	failure := apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, StorageName+"-1",
		errors.New("ValidatingAdmissionPolicy 'hold-storage-pod' denied request"))
	r := &Reconciler{}
	r.InjectClient(interceptor.NewClient(api, interceptor.Funcs{
		Delete: func(context.Context, client.WithWatch, client.Object, ...client.DeleteOption) error {
			return failure
		},
	}))
	r.InjectAPIReader(api)
	recorder := withRecorder(r)
	clock := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return clock }
	failedDelete := func() int {
		said := 0
		for _, recorded := range events(recorder) {
			if strings.Contains(recorded, "FailedDelete") {
				said++
			}
		}
		return said
	}
	// tryAgain runs a pass, moves the clock on to its requeue, and counts the FailedDelete it said —
	// drained on every pass, so that one said too often fails here rather than filling the recorder.
	tryAgain := func() int {
		t.Helper()
		assert.Equal(t, settleInterval, reconcile(t, r).RequeueAfter)
		clock = clock.Add(settleInterval)
		return failedDelete()
	}

	assert.Equal(t, 1, tryAgain())

	repeated := 0
	for range int(blockedInterval/settleInterval) - 1 {
		repeated += tryAgain()
	}
	assert.Zero(t, repeated, "the same denial, retried within a blocked interval")

	assert.Equal(t, 1, tryAgain(), "a blocked interval later the denial is still said to go on")

	failure = apierrors.NewInternalError(errors.New("etcdserver: request timed out"))
	assert.Equal(t, 1, tryAgain(), "another reason is news")
	assert.False(t, deleted(t, api, StorageName+"-1"))
}

// TestAFailedReadStillEndsInAnError: what a pass cannot read, it cannot decide on, so it says
// nothing about the update and the gauges keep what the last pass that got through published.
func TestAFailedReadStillEndsInAnError(t *testing.T) {
	r, _ := newReconciler(t,
		storageSet(3, newRevision),
		replica(0, "master-0", oldRevision, true),
		replica(1, "master-1", oldRevision, true),
		replica(2, "master-2", newRevision, true),
		lease("master-0"),
		storageWithUpstream("registry.deckhouse.io"),
	)
	api := fake.NewClientBuilder().WithScheme(newScheme(t)).Build()
	r.InjectAPIReader(interceptor.NewClient(api, interceptor.Funcs{
		List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
			return apierrors.NewInternalError(errors.New("etcdserver: request timed out"))
		},
	}))
	metrics.StorageUpdateBlocked.WithLabelValues(string(BlockedHolderNotReady)).Set(1)
	metrics.StorageReplicasOutdated.Set(3)

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: Namespace, Name: StorageName},
	})

	require.Error(t, err)
	assert.InDelta(t, 1, blockedGauge(BlockedHolderNotReady), 0)
	assert.InDelta(t, 3, testutil.ToFloat64(metrics.StorageReplicasOutdated), 0)
}

// TestARefusalIsSaidOnceUntilItChanges: a pass runs on every event of a replica or a holder as well
// as on the requeue, and each used to repeat the refusal the pass before had made — on a stand both
// waits began with it said twice within 50 ms. The event is said again when what decides it changes,
// and once a blocked interval has passed, so a wait still shows as going on without spending what the
// spam filter allows the StatefulSet's Warning events; the gauge is published every pass.
func TestARefusalIsSaidOnceUntilItChanges(t *testing.T) {
	storage := airGappedWith("master-0")
	storage.Status.Replicas[1].VerifiedDigests = 120
	storage.Status.Replicas[1].DeclaredDigests = 556

	r, c := newReconciler(t,
		storageSet(3, newRevision),
		replica(0, "master-0", oldRevision, true),
		replica(1, "master-1", newRevision, true),
		replica(2, "master-2", newRevision, true),
		lease("master-0"),
		storage,
	)
	recorder := withRecorder(r)
	clock := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return clock }
	blocked := func() []string {
		var out []string
		for _, recorded := range events(recorder) {
			if strings.Contains(recorded, "UpdateBlocked") {
				out = append(out, recorded)
			}
		}
		return out
	}
	follower := func(change func(*registryv1alpha1.StorageReplicaStatus)) {
		current := &registryv1alpha1.RegistryStorage{}
		require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(storage), current))
		change(&current.Status.Replicas[1])
		require.NoError(t, c.Update(context.Background(), current))
	}

	reconcile(t, r)
	reconcile(t, r)

	assert.Len(t, blocked(), 1, "the second pass, woken by some pod, says nothing the first did not")
	assert.InDelta(t, 1, blockedGauge(BlockedLeaderWithoutFullFollower), 0, "the gauge is set on every pass")

	// The follower copies on. Its count is in the message, and is not news.
	follower(func(replica *registryv1alpha1.StorageReplicaStatus) { replica.VerifiedDigests = 300 })
	clock = clock.Add(time.Second)
	reconcile(t, r)

	assert.Empty(t, blocked(), "a digest count changes on every pass while a follower copies")

	// Its pass fails. That is news.
	follower(func(replica *registryv1alpha1.StorageReplicaStatus) { replica.Error = "unexpected EOF" })
	reconcile(t, r)

	said := blocked()
	require.Len(t, said, 1)
	assert.Contains(t, said[0], StorageName+"-1 reports an error")

	// The same refusal, a settle interval later: in the log, not in another event.
	saidAt := clock
	clock = clock.Add(settleInterval)
	reconcile(t, r)

	assert.Empty(t, blocked(), "an event per settle interval used up the StatefulSet's Warning allowance")

	// Nor after a pass that is not refused: a replica stops serving, which is a wait, not a refusal,
	// and serves again. A replica whose readiness flaps would otherwise say the refusal on every flap.
	ready := func(serving bool) {
		pod := &corev1.Pod{}
		require.NoError(t, c.Get(context.Background(),
			client.ObjectKey{Namespace: Namespace, Name: StorageName + "-2"}, pod))
		pod.Status.Conditions = replica(2, "master-2", newRevision, serving).Status.Conditions
		require.NoError(t, c.Status().Update(context.Background(), pod))
	}
	ready(false)
	reconcile(t, r)

	assert.Empty(t, blocked(), "waiting for a replica to serve is not a refusal")
	assert.InDelta(t, 0, blockedGauge(BlockedLeaderWithoutFullFollower), 0)

	ready(true)
	reconcile(t, r)

	assert.Empty(t, blocked(), "the same refusal, a moment after the same event")
	assert.InDelta(t, 1, blockedGauge(BlockedLeaderWithoutFullFollower), 0)

	// A blocked interval after it was said, the wait is said to go on.
	clock = saidAt.Add(blockedInterval)
	reconcile(t, r)

	again := blocked()
	require.Len(t, again, 1)
	assert.Contains(t, again[0], StorageName+"-1 reports an error")
}

// spamFilteredRecorder records events the way the recorder of a running controller does: through
// client-go's correlator, whose spam filter gives every Warning event of an object one allowance —
// a burst of 25, then one per five minutes — and drops what exceeds it.
type spamFilteredRecorder struct {
	correlator *record.EventCorrelator

	// recorded is the reason of every event the filter let through.
	recorded []string
}

func (s *spamFilteredRecorder) Event(object runtime.Object, eventtype, reason, message string) {
	meta := object.(metav1.Object)
	result, err := s.correlator.EventCorrelate(&corev1.Event{
		InvolvedObject: corev1.ObjectReference{
			Kind: "StatefulSet", APIVersion: "apps/v1",
			Namespace: meta.GetNamespace(), Name: meta.GetName(), UID: meta.GetUID(),
		},
		Type: eventtype, Reason: reason, Message: message,
		Source: corev1.EventSource{Component: ControllerName},
	})
	if err == nil && !result.Skip {
		s.recorded = append(s.recorded, reason)
	}
}

func (s *spamFilteredRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...any) {
	s.Event(object, eventtype, reason, fmt.Sprintf(messageFmt, args...))
}

func (s *spamFilteredRecorder) AnnotatedEventf(
	object runtime.Object, _ map[string]string, eventtype, reason, messageFmt string, args ...any,
) {
	s.Eventf(object, eventtype, reason, messageFmt, args...)
}

// TestALongWaitLeavesRoomForOtherWarnings: a leader waiting an hour for a follower to copy the set
// is a settling refusal, looked at again every settle interval. Said in an event on each of those,
// it used up the spam filter's allowance for the StatefulSet within minutes, and the FailedDelete
// of the replacement the wait ended in was dropped.
func TestALongWaitLeavesRoomForOtherWarnings(t *testing.T) {
	storage := airGappedWith("master-0")
	storage.Status.Replicas[1].VerifiedDigests = 120
	storage.Status.Replicas[1].DeclaredDigests = 556

	api := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(
		storageSet(3, newRevision),
		replica(0, "master-0", oldRevision, true),
		replica(1, "master-1", newRevision, true),
		replica(2, "master-2", newRevision, true),
		lease("master-0"),
		storage,
	).Build()
	r := &Reconciler{}
	r.InjectClient(interceptor.NewClient(api, interceptor.Funcs{
		Delete: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.DeleteOption) error {
			return apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, obj.GetName(),
				errors.New("ValidatingAdmissionPolicy 'hold-storage-pod' denied request"))
		},
	}))
	r.InjectAPIReader(api)
	clock := testingclock.NewFakeClock(time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC))
	r.now = clock.Now
	recorder := &spamFilteredRecorder{correlator: record.NewEventCorrelatorWithOptions(record.CorrelatorOptions{
		Clock: clock,
	})}
	r.InjectRecorder(recorder)

	// An hour and half the filter's refill period: the denial lands between two refills, so what it
	// finds is what the wait has left, not a token that happened to come back that very moment.
	const wait = time.Hour + 150*time.Second
	for range int(wait / settleInterval) {
		require.Equal(t, settleInterval, reconcile(t, r).RequeueAfter)
		clock.Step(settleInterval)
	}

	lastPass := wait - settleInterval
	assert.Equal(t, slices.Repeat([]string{"UpdateBlocked"}, int(lastPass/blockedInterval)+1), recorder.recorded,
		"the same wait, said once per blocked interval")

	// The follower holds the set, and the replacement it allows is denied.
	current := &registryv1alpha1.RegistryStorage{}
	require.NoError(t, api.Get(context.Background(), client.ObjectKeyFromObject(storage), current))
	current.Status.Replicas[1].Full = true
	current.Status.Replicas[1].VerifiedSet = currentSet
	require.NoError(t, api.Update(context.Background(), current))
	recorder.recorded = nil

	reconcile(t, r)

	assert.Equal(t, []string{"FailedDelete"}, recorder.recorded,
		"what a wait leaves of the allowance is enough for the failure it ends in")
}

// TestAPullFailingOnEveryRetryIsOneRefusal: the kubelet's reason for an image it cannot pull goes from
// ErrImagePull to ImagePullBackOff and back on every retry, and each change of the holder wakes a
// pass. It is the same failure, and said again it would spend the StatefulSet's Warning allowance.
func TestAPullFailingOnEveryRetryIsOneRefusal(t *testing.T) {
	r, c := newReconciler(t,
		storageSetWith(1, newImages),
		replica(0, "master-0", oldRevision, true),
		lease("master-0"),
		airGappedStorage(),
		pulling(holder("master-0", newImages), newImages[0], "ErrImagePull"),
	)
	recorder := withRecorder(r)
	clock := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return clock }

	said := 0
	for retry := range 8 {
		reason := "ErrImagePull"
		if retry%2 == 1 {
			reason = "ImagePullBackOff"
		}
		updated := pulling(holder("master-0", newImages), newImages[0], reason)
		current := &corev1.Pod{}
		require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(updated), current))
		updated.ResourceVersion = current.ResourceVersion
		// The fake client, like the API server, changes a pod's status only through the subresource.
		require.NoError(t, c.Status().Update(context.Background(), updated))

		reconcile(t, r)
		clock = clock.Add(20 * time.Second)

		for _, recorded := range events(recorder) {
			if strings.Contains(recorded, "UpdateBlocked") {
				said++
			}
		}
	}

	assert.Equal(t, 1, said)
	assert.False(t, deleted(t, c, StorageName+"-0"))
}

// TestAHolderCrashLoopIsOneRefusal: between two restarts of a crash loop the kubelet shows the
// holder's container terminated, then in CrashLoopBackOff, and each change of the holder wakes a
// pass. It is the same failure to run, said once like a pull failing on every retry — and still
// told apart from a failure to pull the same image, which asks for something else.
func TestAHolderCrashLoopIsOneRefusal(t *testing.T) {
	crashing := func(retry int) *corev1.Pod {
		pod := holder("master-0", newImages)
		if retry%2 == 1 {
			return pulling(pod, newImages[0], "CrashLoopBackOff")
		}
		pod.Status.ContainerStatuses[0].Ready = false
		pod.Status.ContainerStatuses[0].State = corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{Reason: "StartError", ExitCode: 128},
		}
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}}
		return pod
	}

	r, c := newReconciler(t,
		storageSetWith(1, newImages),
		replica(0, "master-0", oldRevision, true),
		lease("master-0"),
		airGappedStorage(),
		crashing(0),
	)
	recorder := withRecorder(r)
	clock := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return clock }

	update := func(updated *corev1.Pod) {
		current := &corev1.Pod{}
		require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(updated), current))
		updated.ResourceVersion = current.ResourceVersion
		require.NoError(t, c.Status().Update(context.Background(), updated))
	}
	blocked := func() []string {
		var said []string
		for _, recorded := range events(recorder) {
			if strings.Contains(recorded, "UpdateBlocked") {
				said = append(said, recorded)
			}
		}
		return said
	}

	var said []string
	for retry := range 8 {
		update(crashing(retry))
		reconcile(t, r)
		clock = clock.Add(20 * time.Second)
		said = append(said, blocked()...)
	}

	require.Len(t, said, 1)
	assert.Contains(t, said[0], "fails to run it")
	assert.False(t, deleted(t, c, StorageName+"-0"))

	// The same image failing to pull instead is another refusal, said at once.
	update(pulling(holder("master-0", newImages), newImages[0], "ImagePullBackOff"))
	reconcile(t, r)
	said = blocked()
	require.Len(t, said, 1, "a failure to pull is not the failure to run said before")
	assert.Contains(t, said[0], "cannot pull "+newImages[0])
}

// TestALeaderWhoseLastPassFailedKeepsTheLease: a leader report with an error, or not full, next to a
// fingerprint names a set an earlier pass found — the syncer clears it on a pass that counts and
// fails — so a follower full against it is not shown to hold what the leader holds now. The leader
// stays until a pass of it succeeds, as the FAQ says, and the follower is not said to hold the set.
func TestALeaderWhoseLastPassFailedKeepsTheLease(t *testing.T) {
	storage := airGappedWith("master-0", "master-1")
	storage.Status.Replicas[0].Error = "this replica's registry is not listening: connection refused"

	r, c := newReconciler(t,
		storageSet(3, newRevision),
		replica(0, "master-0", oldRevision, true),
		replica(1, "master-1", newRevision, true),
		replica(2, "master-2", newRevision, true),
		lease("master-0"),
		storage,
	)
	recorder := withRecorder(r)

	result := reconcile(t, r)

	assert.False(t, deleted(t, c, StorageName+"-0"))
	assert.Equal(t, blockedInterval, result.RequeueAfter)
	assert.InDelta(t, 1, blockedGauge(BlockedLeaderWithoutFullFollower), 0)
	recorded := strings.Join(events(recorder), "\n")
	assert.Contains(t, recorded, "once a pass of the leader succeeds")
	assert.NotContains(t, recorded, "d8 mirror push")
	assert.Contains(t, recorded, StorageName+"-1 on master-1: reports full")
	assert.NotContains(t, recorded, "holds the whole set;")

	// Its next pass succeeds: the follower holding the same set takes over.
	current := &registryv1alpha1.RegistryStorage{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(storage), current))
	current.Status.Replicas[0].Error = ""
	require.NoError(t, c.Update(context.Background(), current))

	reconcile(t, r)

	assert.True(t, deleted(t, c, StorageName+"-0"))
}

// TestALeaderNotFullKeepsTheLease: a leader that reports not full, with a fingerprint carried over
// next to it, is not one a follower full against that fingerprint can replace: the followers copy
// from it, and what it lacks only a push brings in.
func TestALeaderNotFullKeepsTheLease(t *testing.T) {
	storage := airGappedWith("master-0", "master-1")
	storage.Status.Replicas[0].Full = false

	r, c := newReconciler(t,
		storageSet(3, newRevision),
		replica(0, "master-0", oldRevision, true),
		replica(1, "master-1", newRevision, true),
		replica(2, "master-2", newRevision, true),
		lease("master-0"),
		storage,
	)
	recorder := withRecorder(r)

	reconcile(t, r)

	assert.False(t, deleted(t, c, StorageName+"-0"))
	assert.InDelta(t, 1, blockedGauge(BlockedLeaderWithoutFullFollower), 0)
	recorded := strings.Join(events(recorder), "\n")
	assert.Contains(t, recorded, "d8 mirror push")
	assert.Contains(t, recorded, StorageName+"-1 on master-1: reports full")
	assert.NotContains(t, recorded, "no follower holds it")
}

// TestOnlyTheCachePodsWakeTheController: every pass lists the replicas from the API server, and a pod
// of any other component in the namespace changing says nothing about the update.
func TestOnlyTheCachePodsWakeTheController(t *testing.T) {
	pod := func(namespace, app string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace, Name: app + "-0", Labels: map[string]string{"app": app},
		}}
	}
	filter := cachePods()

	assert.True(t, filter.Create(event.CreateEvent{Object: pod(Namespace, storageAppLabel)}))
	assert.True(t, filter.Delete(event.DeleteEvent{Object: pod(Namespace, storageAppLabel)}),
		"a replica gone is what lets the next one go")
	assert.True(t, filter.Update(event.UpdateEvent{
		ObjectOld: pod(Namespace, holderAppLabel), ObjectNew: pod(Namespace, holderAppLabel),
	}), "a holder finishing its pull is what lets the only replica go")
	assert.False(t, filter.Update(event.UpdateEvent{
		ObjectOld: pod(Namespace, "registry-controller"), ObjectNew: pod(Namespace, "registry-controller"),
	}))
	assert.False(t, filter.Create(event.CreateEvent{Object: pod("default", storageAppLabel)}))
}

// watches records what a reconciler asks the controller to watch.
type watches struct {
	objects []client.Object
	options [][]builder.WatchesOption
}

func (w *watches) Owns(client.Object, ...builder.OwnsOption) {}
func (w *watches) WatchesRawSource(source.Source)            {}
func (w *watches) WithEventFilter(predicate.Predicate)       {}
func (w *watches) Watches(object client.Object, _ handler.EventHandler, opts ...builder.WatchesOption) {
	w.objects = append(w.objects, object)
	w.options = append(w.options, opts)
}

// TestThePodWatchIsFiltered: cachePods is what keeps other pods from waking the controller only
// while the pod watch carries it. The builder keeps a predicate out of reach of a test, so this
// checks that the one pod watch has the predicates option, and TestOnlyTheCachePodsWakeTheController
// what that predicate lets through.
func TestThePodWatchIsFiltered(t *testing.T) {
	w := &watches{}
	(&Reconciler{}).SetupWatches(w)

	require.Len(t, w.objects, 1)
	assert.IsType(t, &corev1.Pod{}, w.objects[0])
	require.Len(t, w.options[0], 1)
	assert.IsType(t, builder.Predicates{}, w.options[0][0])
}

// TestTheLeaderReplacedNamesEveryPossibleSuccessor: which replica leads after the leader goes is the
// election's to decide — whichever replica holding the whole set asks first, the leader's own
// replacement on its node among them, since it comes back over the same store. The log used to name
// the first full follower in list order as the one that "takes the lease over", and two passes on a
// stand named two different ones for the same deletion.
func TestTheLeaderReplacedNamesEveryPossibleSuccessor(t *testing.T) {
	pods := []*corev1.Pod{
		replica(0, "master-0", oldRevision, true),
		replica(1, "master-1", newRevision, true),
		replica(2, "master-2", newRevision, true),
	}
	source := func(t *testing.T, storage *registryv1alpha1.RegistryStorage) string {
		t.Helper()

		r, _ := newReconciler(t, storage)
		decision, err := r.imageSourceWhile(context.Background(), storageSet(3, newRevision), pods[0], pods, "master-0")
		require.NoError(t, err)
		require.Nil(t, decision.refused)
		return decision.source
	}

	t.Run("both followers hold the set", func(t *testing.T) {
		assert.Equal(t, "a replica holding the whole set, whichever the election picks: "+
			StorageName+"-1, "+StorageName+"-2, or the replacement of "+StorageName+"-0 on master-0",
			source(t, airGappedWith("master-0", "master-1", "master-2")))
	})

	t.Run("one follower holds it", func(t *testing.T) {
		storage := airGappedWith("master-0", "master-1", "master-2")
		storage.Status.Replicas[1].VerifiedSet = previousSet

		assert.Equal(t, "a replica holding the whole set, whichever the election picks: "+
			StorageName+"-2, or the replacement of "+StorageName+"-0 on master-0",
			source(t, storage))
	})
}

// servingOn gives a replica the `registry` port its pod spec declares.
func servingOn(pod *corev1.Pod, port int32) *corev1.Pod {
	pod.Spec.Containers = []corev1.Container{{
		Name:  "distribution",
		Ports: []corev1.ContainerPort{{Name: "registry", ContainerPort: port}},
	}}
	return pod
}

// reporting is an air-gapped cache in which every replica holds the whole current set and reports
// the address given for its node.
func reporting(addresses map[string]string) *registryv1alpha1.RegistryStorage {
	storage := airGappedStorage()
	for _, node := range []string{"master-0", "master-1", "master-2"} {
		storage.Status.Replicas = append(storage.Status.Replicas, registryv1alpha1.StorageReplicaStatus{
			Node: node, Full: true, VerifiedSet: currentSet, Address: addresses[node],
		})
	}
	return storage
}

// TestAReplacedReplicaReportsWhereItServesBeforeTheNextGoes is the update to the store's current
// ports. The registry controller points the agents at the port each replica reported, and a replaced
// replica's entry keeps its predecessor's port until the new syncer writes its own. Replacing the
// leader in that window moved the lease to a replica reported on a port nothing listened on, with
// every other replica reported the same way: an air-gapped cluster with nothing to pull from.
func TestAReplacedReplicaReportsWhereItServesBeforeTheNextGoes(t *testing.T) {
	pods := func() []client.Object {
		return []client.Object{
			servingOn(replica(0, "master-0", oldRevision, true), 5001),
			servingOn(replica(1, "master-1", newRevision, true), 4281),
			servingOn(replica(2, "master-2", newRevision, true), 4281),
		}
	}

	t.Run("a replaced replica still reported on its predecessor's port holds the leader", func(t *testing.T) {
		objects := append(pods(), storageSet(3, newRevision), lease("master-0"),
			reporting(map[string]string{
				"master-0": "10.0.0.1:5001", "master-1": "10.0.0.2:4281", "master-2": "10.0.0.3:5001",
			}))
		r, c := newReconciler(t, objects...)

		events := record.NewFakeRecorder(10)
		r.InjectRecorder(events)

		result := reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-0"))
		assert.Equal(t, settleInterval, result.RequeueAfter)

		// The wait ends only when a syncer writes its report, so it is said where an operator
		// looking at a stalled update looks, and not only at a log level nobody runs with.
		require.Len(t, events.Events, 1)
		event := <-events.Events
		assert.Contains(t, event, "Normal WaitingForAddress")
		assert.Contains(t, event, StorageName+"-2", "names the replica that has not reported")
	})

	t.Run("once both have reported, the leader goes", func(t *testing.T) {
		objects := append(pods(), storageSet(3, newRevision), lease("master-0"),
			reporting(map[string]string{
				"master-0": "10.0.0.1:5001", "master-1": "10.0.0.2:4281", "master-2": "10.0.0.3:4281",
			}))
		r, c := newReconciler(t, objects...)

		reconcile(t, r)

		assert.True(t, deleted(t, c, StorageName+"-0"))
	})

	t.Run("a follower waits the same way", func(t *testing.T) {
		r, c := newReconciler(t,
			storageSet(3, newRevision), lease("master-0"),
			servingOn(replica(0, "master-0", oldRevision, true), 5001),
			servingOn(replica(1, "master-1", oldRevision, true), 5001),
			servingOn(replica(2, "master-2", newRevision, true), 4281),
			reporting(map[string]string{
				"master-0": "10.0.0.1:5001", "master-1": "10.0.0.2:5001", "master-2": "[fd00::3]:5001",
			}))

		reconcile(t, r)

		assert.False(t, deleted(t, c, StorageName+"-1"))
	})

	t.Run("no report, or one that cannot be read, is not waited for", func(t *testing.T) {
		r, c := newReconciler(t,
			storageSet(3, newRevision), lease("master-0"),
			servingOn(replica(0, "master-0", oldRevision, true), 5001),
			servingOn(replica(1, "master-1", oldRevision, true), 5001),
			servingOn(replica(2, "master-2", newRevision, true), 4281),
			reporting(map[string]string{"master-0": "10.0.0.1:5001", "master-1": "10.0.0.2:5001", "master-2": "10.0.0.3"}))

		reconcile(t, r)

		assert.True(t, deleted(t, c, StorageName+"-1"))
	})
}

func TestAddressNotReported(t *testing.T) {
	reports := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Address: "10.0.0.1:5001"},
		{Node: "master-1", Address: "[fd00::2]:4281"},
		{Node: "master-2", Address: "10.0.0.3:5001"},
	}
	pods := []*corev1.Pod{
		servingOn(replica(0, "master-0", oldRevision, true), 5001),
		servingOn(replica(1, "master-1", newRevision, true), 4281),
		servingOn(replica(2, "master-2", newRevision, true), 4281),
		replica(3, "master-3", newRevision, true), // declares no port
	}

	assert.Equal(t, []string{StorageName + "-2"}, addressNotReported(pods, newRevision, reports),
		"only a replica on the target revision whose report names another port than it serves on")
	assert.Empty(t, addressNotReported(pods, newRevision, nil))
}

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

package operation

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

var accepted = time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

func newReconciler(t *testing.T, now time.Time, objects ...client.Object) (*Reconciler, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, coordinationv1.AddToScheme(scheme))

	// Every replica of the storage on a master node of its own, unless the test says which nodes there
	// are — which is how a master that left is written.
	nodesGiven := false
	for _, object := range objects {
		if _, ok := object.(*corev1.Node); ok {
			nodesGiven = true
		}
	}
	if !nodesGiven {
		for _, object := range objects {
			if storage, ok := object.(*registryv1alpha1.RegistryStorage); ok {
				for _, replica := range storage.Status.Replicas {
					objects = append(objects, master(replica.Node))
				}
			}
		}
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithStatusSubresource(&registryv1alpha1.RegistryOperation{}, &registryv1alpha1.RegistryStorage{}).
		Build()

	r := &Reconciler{Now: func() time.Time { return now }}
	r.InjectClient(fakeClient)
	return r, fakeClient
}

func master(name string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name: name, Labels: map[string]string{"node-role.kubernetes.io/control-plane": ""},
	}}
}

// notReadyMaster is a master whose Ready condition has had the status since then.
func notReadyMaster(name string, status corev1.ConditionStatus, since time.Time) *corev1.Node {
	node := master(name)
	node.Status.Conditions = []corev1.NodeCondition{{
		Type: corev1.NodeReady, Status: status, LastTransitionTime: metav1.NewTime(since),
	}}
	return node
}

// collectionLease is the syncers' collection lease as client-go leaves it: held and renewed, or given
// up (no holder, acquired and renewed at the release, one second long).
func collectionLease(holder string, acquired, renewed time.Time, seconds int32) *coordinationv1.Lease {
	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{Namespace: Namespace, Name: CollectionLeaseName},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       ptr.To(holder),
			AcquireTime:          ptr.To(metav1.NewMicroTime(acquired)),
			RenewTime:            ptr.To(metav1.NewMicroTime(renewed)),
			LeaseDurationSeconds: ptr.To(seconds),
		},
	}
}

func storageWith(nodes ...string) *registryv1alpha1.RegistryStorage {
	storage := &registryv1alpha1.RegistryStorage{
		ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName},
	}
	for _, node := range nodes {
		storage.Status.Replicas = append(storage.Status.Replicas, registryv1alpha1.StorageReplicaStatus{Node: node})
	}
	return storage
}

func newRequest(name string, created time.Time) *registryv1alpha1.RegistryOperation {
	return &registryv1alpha1.RegistryOperation{
		ObjectMeta: metav1.ObjectMeta{Name: name, CreationTimestamp: metav1.NewTime(created)},
		Spec:       registryv1alpha1.RegistryOperationSpec{Type: registryv1alpha1.OperationGarbageCollection},
	}
}

// acceptedRequest is a request the controller has already handed to the replicas on nodes.
func acceptedRequest(name string, nodes ...string) *registryv1alpha1.RegistryOperation {
	request := newRequest(name, accepted)
	at := metav1.NewTime(accepted)
	deadline := metav1.NewTime(accepted.Add(registryv1alpha1.OperationStartDeadline))
	request.Status = registryv1alpha1.RegistryOperationStatus{
		Phase:      registryv1alpha1.OperationPending,
		Reason:     registryv1alpha1.ReasonOperationAccepted,
		AcceptedAt: &at,
		Deadline:   &deadline,
	}
	for _, node := range nodes {
		request.Status.Replicas = append(request.Status.Replicas, registryv1alpha1.ReplicaOperationStatus{
			Node: node, Phase: registryv1alpha1.ReplicaOperationPending,
		})
	}
	return request
}

func reconcileRequest(t *testing.T, r *Reconciler, name string) ctrl.Result {
	t.Helper()

	result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
	require.NoError(t, err)
	return result
}

func getRequest(t *testing.T, c client.Client, name string) *registryv1alpha1.RegistryOperation {
	t.Helper()

	request := &registryv1alpha1.RegistryOperation{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: name}, request))
	return request
}

func at(d time.Duration) *metav1.Time {
	stamp := metav1.NewTime(accepted.Add(d))
	return &stamp
}

func TestAcceptHandsTheRequestToEveryReplica(t *testing.T) {
	r, c := newReconciler(t, accepted,
		storageWith("master-2", "master-0", "master-1"), newRequest("manual-1", accepted))

	result := reconcileRequest(t, r, "manual-1")

	got := getRequest(t, c, "manual-1").Status
	assert.Equal(t, registryv1alpha1.OperationPending, got.Phase)
	assert.Equal(t, registryv1alpha1.ReasonOperationAccepted, got.Reason)
	require.NotNil(t, got.AcceptedAt)
	assert.True(t, got.AcceptedAt.Time.Equal(accepted))
	require.NotNil(t, got.Deadline)
	assert.True(t, got.Deadline.Time.Equal(accepted.Add(time.Hour)))
	assert.Nil(t, got.FinishedAt)

	require.Len(t, got.Replicas, 3)
	for i, node := range []string{"master-0", "master-1", "master-2"} {
		assert.Equal(t, node, got.Replicas[i].Node)
		assert.Equal(t, registryv1alpha1.ReplicaOperationPending, got.Replicas[i].Phase)
	}

	// Looked at again at the deadline, when a replica that never picked it up is given up on.
	assert.Equal(t, time.Hour+time.Second, result.RequeueAfter)
}

func TestAcceptRefuses(t *testing.T) {
	canceled := newRequest("manual-2", accepted)
	canceled.Labels = map[string]string{registryv1alpha1.OperationCancelLabel: "true"}

	tests := []struct {
		name    string
		objects []client.Object
		want    registryv1alpha1.RegistryOperationPhase
		reason  string
	}{
		{
			name:    "the cache is turned off",
			objects: []client.Object{newRequest("manual-2", accepted)},
			want:    registryv1alpha1.OperationFailed,
			reason:  registryv1alpha1.ReasonOperationNoCache,
		},
		{
			name:    "no replica has reported",
			objects: []client.Object{storageWith(), newRequest("manual-2", accepted)},
			want:    registryv1alpha1.OperationFailed,
			reason:  registryv1alpha1.ReasonOperationNoReplicas,
		},
		{
			name: "an older request is running",
			objects: []client.Object{
				storageWith("master-0"),
				acceptedRequest("manual-1", "master-0"),
				newRequest("manual-2", accepted.Add(time.Minute)),
			},
			want:   registryv1alpha1.OperationFailed,
			reason: registryv1alpha1.ReasonOperationAnotherActive,
		},
		{
			name: "an older request is not even accepted yet",
			objects: []client.Object{
				storageWith("master-0"),
				newRequest("manual-1", accepted),
				newRequest("manual-2", accepted.Add(time.Second)),
			},
			want:   registryv1alpha1.OperationFailed,
			reason: registryv1alpha1.ReasonOperationAnotherActive,
		},
		{
			name:    "canceled before acceptance",
			objects: []client.Object{storageWith("master-0"), canceled},
			want:    registryv1alpha1.OperationCanceled,
			reason:  registryv1alpha1.ReasonOperationCanceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, c := newReconciler(t, accepted, tt.objects...)

			result := reconcileRequest(t, r, "manual-2")
			assert.Equal(t, registryv1alpha1.OperationRetention, result.RequeueAfter, "kept for a day, then deleted")

			got := getRequest(t, c, "manual-2").Status
			assert.Equal(t, tt.want, got.Phase)
			assert.Equal(t, tt.reason, got.Reason)
			assert.NotEmpty(t, got.Message)
			require.NotNil(t, got.FinishedAt)
			assert.Empty(t, got.Replicas)
		})
	}
}

func TestAcceptIsNotHeldUpByAFinishedOrNewerRequest(t *testing.T) {
	finished := acceptedRequest("manual-0", "master-0")
	finished.Status.Phase = registryv1alpha1.OperationSucceeded
	finished.Status.FinishedAt = at(time.Minute)

	r, c := newReconciler(t, accepted,
		storageWith("master-0"), finished,
		newRequest("manual-1", accepted), newRequest("manual-2", accepted.Add(time.Second)))

	reconcileRequest(t, r, "manual-1")

	assert.Equal(t, registryv1alpha1.OperationPending, getRequest(t, c, "manual-1").Status.Phase)
}

func TestSummarizeAddsUpWhatTheReplicasReported(t *testing.T) {
	request := acceptedRequest("manual-1", "master-0", "master-1")
	request.Status.Replicas[0] = registryv1alpha1.ReplicaOperationStatus{
		Node: "master-0", Phase: registryv1alpha1.ReplicaOperationSucceeded,
		StartedAt: at(2 * time.Minute), FinishedAt: at(5 * time.Minute),
		GarbageCollection: &registryv1alpha1.GarbageCollectionResult{
			RemovedManifests: 10, RemovedBlobs: ptr.To[int32](40), RemovedVersions: []string{"v1.71.3", "v1.71.2"}, ReclaimedBytes: ptr.To[int64](300),
		},
	}
	request.Status.Replicas[1] = registryv1alpha1.ReplicaOperationStatus{
		Node: "master-1", Phase: registryv1alpha1.ReplicaOperationRunning, StartedAt: at(5 * time.Minute),
	}

	r, c := newReconciler(t, accepted.Add(6*time.Minute), storageWith("master-0", "master-1"), request)
	reconcileRequest(t, r, "manual-1")

	got := getRequest(t, c, "manual-1").Status
	assert.Equal(t, registryv1alpha1.OperationRunning, got.Phase)
	assert.Equal(t, registryv1alpha1.ReasonOperationInProgress, got.Reason)
	require.NotNil(t, got.StartedAt)
	assert.True(t, got.StartedAt.Time.Equal(accepted.Add(2*time.Minute)))
	assert.Nil(t, got.FinishedAt)

	// The second replica finishes.
	got.Replicas[1].Phase = registryv1alpha1.ReplicaOperationSucceeded
	got.Replicas[1].FinishedAt = at(9 * time.Minute)
	got.Replicas[1].GarbageCollection = &registryv1alpha1.GarbageCollectionResult{
		RemovedManifests: 4, RemovedBlobs: ptr.To[int32](2), RemovedVersions: []string{"v1.71.2"}, ReclaimedBytes: ptr.To[int64](200),
	}
	finishing := getRequest(t, c, "manual-1")
	finishing.Status = got
	require.NoError(t, c.Status().Update(context.Background(), finishing))

	reconcileRequest(t, r, "manual-1")

	got = getRequest(t, c, "manual-1").Status
	assert.Equal(t, registryv1alpha1.OperationSucceeded, got.Phase)
	assert.Equal(t, registryv1alpha1.ReasonOperationCompleted, got.Reason)
	require.NotNil(t, got.FinishedAt)
	assert.True(t, got.FinishedAt.Time.Equal(accepted.Add(9*time.Minute)), "the last replica's finish, not the clock")
	require.NotNil(t, got.GarbageCollection)
	assert.EqualValues(t, 14, got.GarbageCollection.RemovedManifests)
	require.NotNil(t, got.GarbageCollection.RemovedBlobs)
	assert.EqualValues(t, 42, *got.GarbageCollection.RemovedBlobs)
	require.NotNil(t, got.GarbageCollection.ReclaimedBytes)
	assert.EqualValues(t, 500, *got.GarbageCollection.ReclaimedBytes)
	assert.Equal(t, []string{"v1.71.2", "v1.71.3"}, got.GarbageCollection.RemovedVersions)
}

func TestWaitingIsNamedInTheMessage(t *testing.T) {
	request := acceptedRequest("manual-1", "master-0", "master-1")
	request.Status.Replicas[0].Phase = registryv1alpha1.ReplicaOperationWaiting
	request.Status.Replicas[0].Reason = registryv1alpha1.ReasonOperationFilling

	r, c := newReconciler(t, accepted.Add(time.Minute), storageWith("master-0", "master-1"), request)
	reconcileRequest(t, r, "manual-1")

	got := getRequest(t, c, "manual-1").Status
	assert.Equal(t, registryv1alpha1.OperationPending, got.Phase)
	assert.Equal(t, "Waiting: master-0 (Filling).", got.Message)
}

func TestAReplicaThatLeftIsSkipped(t *testing.T) {
	request := acceptedRequest("manual-1", "master-0", "master-1")
	request.Status.Replicas[0].Phase = registryv1alpha1.ReplicaOperationSucceeded
	request.Status.Replicas[0].StartedAt = at(time.Minute)
	request.Status.Replicas[0].FinishedAt = at(2 * time.Minute)

	r, c := newReconciler(t, accepted.Add(3*time.Minute), storageWith("master-0"), request)
	reconcileRequest(t, r, "manual-1")

	got := getRequest(t, c, "manual-1").Status
	assert.Equal(t, registryv1alpha1.ReplicaOperationSkipped, got.Replicas[1].Phase)
	assert.Equal(t, registryv1alpha1.ReasonOperationReplicaGone, got.Replicas[1].Reason)
	assert.Equal(t, registryv1alpha1.OperationSucceeded, got.Phase,
		"a replica that is gone has no store left to collect")
}

func TestTurningTheCacheOffEndsTheRequest(t *testing.T) {
	r, c := newReconciler(t, accepted.Add(time.Minute), acceptedRequest("manual-1", "master-0"))
	reconcileRequest(t, r, "manual-1")

	got := getRequest(t, c, "manual-1").Status
	assert.Equal(t, registryv1alpha1.ReplicaOperationSkipped, got.Replicas[0].Phase)
	assert.Equal(t, registryv1alpha1.ReasonOperationNoCache, got.Replicas[0].Reason)
	assert.Equal(t, registryv1alpha1.OperationFailed, got.Phase)
	assert.Equal(t, registryv1alpha1.ReasonOperationNoCache, got.Reason, "why, rather than that no replica is left")
}

func TestAReplicaThatNeverPickedTheRequestUpFailsAtTheDeadline(t *testing.T) {
	request := acceptedRequest("manual-1", "master-0", "master-1")
	request.Status.Replicas[0].Phase = registryv1alpha1.ReplicaOperationSucceeded
	request.Status.Replicas[0].StartedAt = at(time.Minute)
	request.Status.Replicas[0].FinishedAt = at(2 * time.Minute)

	// Before the deadline nothing is decided for it.
	r, c := newReconciler(t, accepted.Add(59*time.Minute), storageWith("master-0", "master-1"), request)
	result := reconcileRequest(t, r, "manual-1")
	assert.Equal(t, registryv1alpha1.ReplicaOperationPending, getRequest(t, c, "manual-1").Status.Replicas[1].Phase)
	assert.Equal(t, time.Minute+time.Second, result.RequeueAfter)

	r.Now = func() time.Time { return accepted.Add(time.Hour + time.Second) }
	reconcileRequest(t, r, "manual-1")

	got := getRequest(t, c, "manual-1").Status
	assert.Equal(t, registryv1alpha1.ReplicaOperationFailed, got.Replicas[1].Phase)
	assert.Equal(t, registryv1alpha1.ReasonOperationDeadlineExceeded, got.Replicas[1].Reason)
	assert.Equal(t, registryv1alpha1.OperationFailed, got.Phase)
	assert.Equal(t, registryv1alpha1.ReasonOperationReplicaFailed, got.Reason)
	assert.Contains(t, got.Message, "master-1 (DeadlineExceeded)")
}

func TestAWaitingReplicaIsLeftToItselfUntilTheTimeout(t *testing.T) {
	request := acceptedRequest("manual-1", "master-0")
	request.Status.Replicas[0].Phase = registryv1alpha1.ReplicaOperationWaiting
	request.Status.Replicas[0].Reason = registryv1alpha1.ReasonOperationTurn

	// Past the start deadline: the replica has picked the request up and decides that itself.
	r, c := newReconciler(t, accepted.Add(2*time.Hour), storageWith("master-0"), request)
	reconcileRequest(t, r, "manual-1")
	assert.Equal(t, registryv1alpha1.ReplicaOperationWaiting, getRequest(t, c, "manual-1").Status.Replicas[0].Phase)

	r.Now = func() time.Time { return accepted.Add(registryv1alpha1.OperationTimeout) }
	reconcileRequest(t, r, "manual-1")

	got := getRequest(t, c, "manual-1").Status
	assert.Equal(t, registryv1alpha1.ReplicaOperationFailed, got.Replicas[0].Phase)
	assert.Equal(t, registryv1alpha1.ReasonOperationTimedOut, got.Replicas[0].Reason)
	assert.Equal(t, registryv1alpha1.OperationFailed, got.Phase)
}

func TestCancelStopsWhatHasNotStartedAndWaitsForWhatHas(t *testing.T) {
	request := acceptedRequest("manual-1", "master-0", "master-1")
	request.Labels = map[string]string{registryv1alpha1.OperationCancelLabel: "true"}
	request.Status.Replicas[0].Phase = registryv1alpha1.ReplicaOperationRunning
	request.Status.Replicas[0].StartedAt = at(time.Minute)

	r, c := newReconciler(t, accepted.Add(2*time.Minute), storageWith("master-0", "master-1"), request)
	reconcileRequest(t, r, "manual-1")

	got := getRequest(t, c, "manual-1").Status
	assert.Equal(t, registryv1alpha1.ReplicaOperationRunning, got.Replicas[0].Phase, "the running replica stops itself")
	assert.Equal(t, registryv1alpha1.ReplicaOperationCanceled, got.Replicas[1].Phase)
	assert.Equal(t, registryv1alpha1.OperationRunning, got.Phase)

	// The running replica reports that it stopped.
	stopped := getRequest(t, c, "manual-1")
	stopped.Status.Replicas[0].Phase = registryv1alpha1.ReplicaOperationCanceled
	stopped.Status.Replicas[0].FinishedAt = at(3 * time.Minute)
	require.NoError(t, c.Status().Update(context.Background(), stopped))

	reconcileRequest(t, r, "manual-1")

	got = getRequest(t, c, "manual-1").Status
	assert.Equal(t, registryv1alpha1.OperationCanceled, got.Phase)
	assert.Equal(t, registryv1alpha1.ReasonOperationCanceled, got.Reason)
}

func TestACancelThatCameTooLateDoesNotUndoACollection(t *testing.T) {
	request := acceptedRequest("manual-1", "master-0")
	request.Labels = map[string]string{registryv1alpha1.OperationCancelLabel: "true"}
	request.Status.Replicas[0].Phase = registryv1alpha1.ReplicaOperationSucceeded
	request.Status.Replicas[0].StartedAt = at(time.Minute)
	request.Status.Replicas[0].FinishedAt = at(2 * time.Minute)

	r, c := newReconciler(t, accepted.Add(3*time.Minute), storageWith("master-0"), request)
	reconcileRequest(t, r, "manual-1")

	assert.Equal(t, registryv1alpha1.OperationSucceeded, getRequest(t, c, "manual-1").Status.Phase)
}

// TestAFinishedOperationIsDeletedADayLater is the retention of NodeOperation: the record stays for a
// day, then goes.
func TestAFinishedOperationIsDeletedADayLater(t *testing.T) {
	finished := acceptedRequest("manual-1", "master-0")
	finished.Status.Phase = registryv1alpha1.OperationSucceeded
	finished.Status.FinishedAt = at(time.Hour)

	r, c := newReconciler(t, accepted.Add(5*time.Hour), storageWith("master-0"), finished)
	result := reconcileRequest(t, r, "manual-1")
	assert.Equal(t, 20*time.Hour, result.RequeueAfter, "looked at again when it is a day old")
	getRequest(t, c, "manual-1")

	r.Now = func() time.Time { return accepted.Add(25 * time.Hour) }
	reconcileRequest(t, r, "manual-1")

	err := c.Get(context.Background(), types.NamespacedName{Name: "manual-1"}, &registryv1alpha1.RegistryOperation{})
	assert.True(t, apierrors.IsNotFound(err), "a finished operation a day old is deleted")
}

func TestARefusedOperationIsKeptForADayToo(t *testing.T) {
	r, c := newReconciler(t, accepted, newRequest("manual-1", accepted))

	result := reconcileRequest(t, r, "manual-1")
	assert.Equal(t, registryv1alpha1.OperationFailed, getRequest(t, c, "manual-1").Status.Phase)
	assert.Equal(t, registryv1alpha1.OperationRetention, result.RequeueAfter)
}

// TestTheEntryOfAMasterThatLeftIsNotHandedAnything is the master replaced: its entry stays in the
// storage status, nothing removes it, and an operation handed to it failed at its deadline alongside
// replicas that had done all the work.
func TestTheEntryOfAMasterThatLeftIsNotHandedAnything(t *testing.T) {
	r, c := newReconciler(t, accepted,
		storageWith("master-0", "master-old"), master("master-0"), newRequest("manual-1", accepted))
	reconcileRequest(t, r, "manual-1")

	got := getRequest(t, c, "manual-1").Status
	require.Len(t, got.Replicas, 1)
	assert.Equal(t, "master-0", got.Replicas[0].Node)
}

// TestAMasterLeavingMidOperationIsSkipped: accepted while it was there, gone before it collected.
func TestAMasterLeavingMidOperationIsSkipped(t *testing.T) {
	request := acceptedRequest("manual-1", "master-0", "master-1")
	request.Status.Replicas[0].Phase = registryv1alpha1.ReplicaOperationSucceeded
	request.Status.Replicas[0].StartedAt = at(time.Minute)
	request.Status.Replicas[0].FinishedAt = at(2 * time.Minute)

	// master-1's entry is still in the storage status; the node is not.
	r, c := newReconciler(t, accepted.Add(3*time.Minute),
		storageWith("master-0", "master-1"), master("master-0"), request)
	reconcileRequest(t, r, "manual-1")

	got := getRequest(t, c, "manual-1").Status
	assert.Equal(t, registryv1alpha1.ReplicaOperationSkipped, got.Replicas[1].Phase)
	assert.Equal(t, registryv1alpha1.OperationSucceeded, got.Phase, "what is left collected")
}

// TestAReplicaStillHeldBackByItsStoreFailsAtTheDeadline: its syncer settles that itself, unless it
// stopped answering — a master gone NotReady, a pod in a crash loop. Left to it, the operation stayed
// active for a day, and every collection with it.
func TestAReplicaStillHeldBackByItsStoreFailsAtTheDeadline(t *testing.T) {
	for _, reason := range []string{registryv1alpha1.ReasonOperationFilling, registryv1alpha1.ReasonOperationPushing} {
		t.Run(reason, func(t *testing.T) {
			request := acceptedRequest("manual-1", "master-0")
			request.Status.Replicas[0].Phase = registryv1alpha1.ReplicaOperationWaiting
			request.Status.Replicas[0].Reason = reason

			r, c := newReconciler(t, accepted.Add(30*time.Minute), storageWith("master-0"), request)
			result := reconcileRequest(t, r, "manual-1")
			assert.Equal(t, registryv1alpha1.ReplicaOperationWaiting, getRequest(t, c, "manual-1").Status.Replicas[0].Phase)
			assert.Equal(t, 30*time.Minute+time.Second, result.RequeueAfter, "looked at again at the deadline")

			r.Now = func() time.Time { return accepted.Add(registryv1alpha1.OperationStartDeadline) }
			reconcileRequest(t, r, "manual-1")

			got := getRequest(t, c, "manual-1").Status
			assert.Equal(t, registryv1alpha1.ReplicaOperationFailed, got.Replicas[0].Phase)
			assert.Equal(t, registryv1alpha1.ReasonOperationDeadlineExceeded, got.Replicas[0].Reason)
			assert.Contains(t, got.Replicas[0].Message, reason)
			assert.Equal(t, registryv1alpha1.OperationFailed, got.Phase)
		})
	}
}

// TestCancelStopsAReplicaWaitingForItsStoreOrItsTurn: the label promises that, and a replica whose
// syncer stopped answering would otherwise hold the operation open for a day.
func TestCancelStopsAReplicaWaitingForItsStoreOrItsTurn(t *testing.T) {
	request := acceptedRequest("manual-1", "master-0", "master-1")
	request.Labels = map[string]string{registryv1alpha1.OperationCancelLabel: "true"}
	request.Status.Replicas[0].Phase = registryv1alpha1.ReplicaOperationWaiting
	request.Status.Replicas[0].Reason = registryv1alpha1.ReasonOperationFilling
	request.Status.Replicas[1].Phase = registryv1alpha1.ReplicaOperationWaiting
	request.Status.Replicas[1].Reason = registryv1alpha1.ReasonOperationTurn

	r, c := newReconciler(t, accepted.Add(2*time.Minute), storageWith("master-0", "master-1"), request)
	reconcileRequest(t, r, "manual-1")

	got := getRequest(t, c, "manual-1").Status
	assert.Equal(t, registryv1alpha1.ReplicaOperationCanceled, got.Replicas[0].Phase)
	assert.Equal(t, registryv1alpha1.ReplicaOperationCanceled, got.Replicas[1].Phase)
	assert.Equal(t, registryv1alpha1.OperationCanceled, got.Phase)
}

// running is a request master-0 has been collecting on since a minute after it was accepted, while
// master-1 has finished.
func running() *registryv1alpha1.RegistryOperation {
	request := acceptedRequest("manual-1", "master-0", "master-1")
	request.Status.Replicas[0].Phase = registryv1alpha1.ReplicaOperationRunning
	request.Status.Replicas[0].StartedAt = at(time.Minute)
	request.Status.Replicas[1].Phase = registryv1alpha1.ReplicaOperationSucceeded
	request.Status.Replicas[1].StartedAt = at(time.Minute)
	request.Status.Replicas[1].FinishedAt = at(2 * time.Minute)
	return request
}

// TestARunningReplicaIsLeftToItWhileItHoldsTheLease: a collection takes hours and writes nothing
// meanwhile. The lease it renews is what says it is still at work, and is read again when it would lapse.
func TestARunningReplicaIsLeftToItWhileItHoldsTheLease(t *testing.T) {
	renewed := accepted.Add(2 * time.Hour)
	r, c := newReconciler(t, renewed.Add(30*time.Second),
		storageWith("master-0", "master-1"), running(),
		collectionLease("master-0", accepted.Add(time.Minute), renewed, 60))

	result := reconcileRequest(t, r, "manual-1")

	got := getRequest(t, c, "manual-1").Status
	assert.Equal(t, registryv1alpha1.ReplicaOperationRunning, got.Replicas[0].Phase)
	assert.Equal(t, registryv1alpha1.OperationRunning, got.Phase)
	assert.Equal(t, 30*time.Second+lostLeaseGrace+time.Second, result.RequeueAfter,
		"looked at again when the lease, not renewed, would have lapsed for the grace")
}

// TestARunningReplicaThatStoppedHoldingTheLeaseIsLost: its syncer died or lost the API server while it
// collected. Nothing else settles its entry, and left to the timeout it held the operation, and every
// collection with it, for a day.
func TestARunningReplicaThatStoppedHoldingTheLeaseIsLost(t *testing.T) {
	stopped := accepted.Add(time.Hour)

	tests := []struct {
		name  string
		lease *coordinationv1.Lease
		// lostAt is when the entry is given up on.
		lostAt time.Time
	}{
		{
			name:   "it stopped renewing the lease",
			lease:  collectionLease("master-0", accepted.Add(time.Minute), stopped, 60),
			lostAt: stopped.Add(time.Minute + lostLeaseGrace),
		},
		{
			name:   "it gave the lease up and did not report",
			lease:  collectionLease("", stopped, stopped, 1),
			lostAt: stopped.Add(lostLeaseGrace),
		},
		{
			name:   "another replica took the lease over",
			lease:  collectionLease("master-1", stopped, stopped.Add(10*time.Minute), 60),
			lostAt: stopped.Add(lostLeaseGrace),
		},
		{
			name:   "there is no lease",
			lostAt: accepted.Add(time.Minute + lostLeaseGrace),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := []client.Object{storageWith("master-0", "master-1"), running()}
			if tt.lease != nil {
				objects = append(objects, tt.lease)
			}

			r, c := newReconciler(t, tt.lostAt.Add(-time.Second), objects...)
			result := reconcileRequest(t, r, "manual-1")
			assert.Equal(t, registryv1alpha1.ReplicaOperationRunning, getRequest(t, c, "manual-1").Status.Replicas[0].Phase,
				"within the grace the replica may still be writing its result")
			assert.Equal(t, 2*time.Second, result.RequeueAfter, "looked at again when the grace is over")

			r.Now = func() time.Time { return tt.lostAt }
			reconcileRequest(t, r, "manual-1")

			got := getRequest(t, c, "manual-1").Status
			assert.Equal(t, registryv1alpha1.ReplicaOperationFailed, got.Replicas[0].Phase)
			assert.Equal(t, registryv1alpha1.ReasonOperationReplicaLost, got.Replicas[0].Reason)
			assert.NotEmpty(t, got.Replicas[0].Message)
			assert.Equal(t, registryv1alpha1.OperationFailed, got.Phase)
			assert.Equal(t, registryv1alpha1.ReasonOperationReplicaFailed, got.Reason)
			assert.Contains(t, got.Message, "master-0 (ReplicaLost)")
		})
	}
}

// TestAReplicaWaitingOnANodeThatIsNotReadyIsLost: a replica waiting for its turn holds nothing to show
// it is alive, and its node is the sign. A master that went away for good otherwise kept the operation
// open for a day.
func TestAReplicaWaitingOnANodeThatIsNotReadyIsLost(t *testing.T) {
	for _, status := range []corev1.ConditionStatus{corev1.ConditionFalse, corev1.ConditionUnknown} {
		t.Run(string(status), func(t *testing.T) {
			request := acceptedRequest("manual-1", "master-0", "master-1")
			request.Status.Replicas[0].Phase = registryv1alpha1.ReplicaOperationWaiting
			request.Status.Replicas[0].Reason = registryv1alpha1.ReasonOperationTurn
			request.Status.Replicas[1].Phase = registryv1alpha1.ReplicaOperationRunning
			request.Status.Replicas[1].StartedAt = at(time.Minute)

			since := accepted.Add(2 * time.Hour)
			r, c := newReconciler(t, since.Add(5*time.Minute),
				storageWith("master-0", "master-1"),
				notReadyMaster("master-0", status, since), master("master-1"), request,
				collectionLease("master-1", accepted.Add(time.Minute), since.Add(5*time.Minute), 60))

			result := reconcileRequest(t, r, "manual-1")
			assert.Equal(t, registryv1alpha1.ReplicaOperationWaiting, getRequest(t, c, "manual-1").Status.Replicas[0].Phase,
				"a master that reboots keeps its turn")
			assert.Equal(t, 5*time.Minute+time.Second, result.RequeueAfter)

			r.Now = func() time.Time { return since.Add(notReadyGrace) }
			reconcileRequest(t, r, "manual-1")

			got := getRequest(t, c, "manual-1").Status
			assert.Equal(t, registryv1alpha1.ReplicaOperationFailed, got.Replicas[0].Phase)
			assert.Equal(t, registryv1alpha1.ReasonOperationReplicaLost, got.Replicas[0].Reason)
			assert.Equal(t, registryv1alpha1.ReplicaOperationRunning, got.Replicas[1].Phase, "the replica collecting is left to it")
			assert.Equal(t, registryv1alpha1.OperationRunning, got.Phase)
		})
	}
}

// TestANodeThatIsNotReadyDoesNotTakeACollectionAway: a running replica is judged by the lease alone. A
// node can drop out of Ready while its syncer still renews the lease and deletes, and its entry is not
// settled under it.
func TestANodeThatIsNotReadyDoesNotTakeACollectionAway(t *testing.T) {
	now := accepted.Add(3 * time.Hour)
	r, c := newReconciler(t, now,
		storageWith("master-0", "master-1"),
		notReadyMaster("master-0", corev1.ConditionUnknown, accepted.Add(time.Hour)), master("master-1"), running(),
		collectionLease("master-0", accepted.Add(time.Minute), now.Add(-5*time.Second), 60))

	reconcileRequest(t, r, "manual-1")

	assert.Equal(t, registryv1alpha1.ReplicaOperationRunning, getRequest(t, c, "manual-1").Status.Replicas[0].Phase)
}

// TestANodeThatCameBackKeepsItsReplicasTurn: Ready again, however long it was away.
func TestANodeThatCameBackKeepsItsReplicasTurn(t *testing.T) {
	request := acceptedRequest("manual-1", "master-0")
	request.Status.Replicas[0].Phase = registryv1alpha1.ReplicaOperationWaiting
	request.Status.Replicas[0].Reason = registryv1alpha1.ReasonOperationTurn

	r, c := newReconciler(t, accepted.Add(3*time.Hour),
		storageWith("master-0"), notReadyMaster("master-0", corev1.ConditionTrue, accepted.Add(2*time.Hour)), request)
	reconcileRequest(t, r, "manual-1")

	assert.Equal(t, registryv1alpha1.ReplicaOperationWaiting, getRequest(t, c, "manual-1").Status.Replicas[0].Phase)
}

// TestOnlyAMasterChangingReadinessWakesTheOperations: kubelet writes the Ready condition of every node
// every few seconds, and only a change of its status on a master is news here.
func TestOnlyAMasterChangingReadinessWakesTheOperations(t *testing.T) {
	ready := notReadyMaster("master-0", corev1.ConditionTrue, accepted)
	heartbeat := ready.DeepCopy()
	heartbeat.Status.Conditions[0].LastHeartbeatTime = metav1.NewTime(accepted.Add(time.Minute))
	gone := notReadyMaster("master-0", corev1.ConditionUnknown, accepted.Add(time.Minute))

	worker := func(node *corev1.Node) *corev1.Node {
		node = node.DeepCopy()
		node.Labels = nil
		return node
	}

	changed := storageNodeReadinessChanged()
	assert.True(t, changed.Update(event.UpdateEvent{ObjectOld: ready, ObjectNew: gone}))
	assert.True(t, changed.Update(event.UpdateEvent{ObjectOld: gone, ObjectNew: ready}))
	assert.False(t, changed.Update(event.UpdateEvent{ObjectOld: ready, ObjectNew: heartbeat}))
	assert.False(t, changed.Update(event.UpdateEvent{ObjectOld: worker(ready), ObjectNew: worker(gone)}))
}

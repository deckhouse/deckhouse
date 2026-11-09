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

package collection

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// threeAM is a firing of the schedule every test uses.
var threeAM = time.Date(2026, 10, 21, 3, 17, 0, 0, time.UTC)

const nightly = "17 3 * * *"

func newReconciler(t *testing.T, now time.Time, objects ...client.Object) (*Reconciler, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

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
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "DeckhouseRelease"},
		&unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(releaseGVK, &unstructured.UnstructuredList{})

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithStatusSubresource(&registryv1alpha1.RegistryOperation{}, &registryv1alpha1.RegistryStorage{}).
		Build()

	r := &Reconciler{Now: func() time.Time { return now }}
	r.InjectClient(c)
	return r, c
}

// storage is a storage with the schedule, its replicas holding set, and the annotations given.
func storage(enabled bool, set string, annotations map[string]string) *registryv1alpha1.RegistryStorage {
	s := &registryv1alpha1.RegistryStorage{
		ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName, Annotations: annotations},
		Spec: registryv1alpha1.RegistryStorageSpec{
			GarbageCollection: &registryv1alpha1.GarbageCollection{Enabled: enabled, Schedule: nightly},
		},
	}
	for _, node := range []string{"master-0", "master-1"} {
		s.Status.Replicas = append(s.Status.Replicas, registryv1alpha1.StorageReplicaStatus{
			Node: node, Full: set != "", VerifiedSet: set,
		})
	}
	return s
}

func release(version, phase string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "DeckhouseRelease"})
	u.SetName(version)
	u.Object["spec"] = map[string]any{"version": version}
	u.Object["status"] = map[string]any{"phase": phase}
	return u
}

func reconcileStorage(t *testing.T, r *Reconciler) ctrl.Result {
	t.Helper()

	result, err := r.Reconcile(context.Background(),
		ctrl.Request{NamespacedName: types.NamespacedName{Name: registryv1alpha1.SingletonName}})
	require.NoError(t, err)
	return result
}

func operations(t *testing.T, c client.Client) []registryv1alpha1.RegistryOperation {
	t.Helper()

	list := &registryv1alpha1.RegistryOperationList{}
	require.NoError(t, c.List(context.Background(), list))
	return list.Items
}

func annotations(t *testing.T, c client.Client) map[string]string {
	t.Helper()

	s := &registryv1alpha1.RegistryStorage{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: registryv1alpha1.SingletonName}, s))
	return s.Annotations
}

func scheduledAt(at time.Time) map[string]string {
	return map[string]string{
		registryv1alpha1.ScheduledCollectionAnnotation: at.UTC().Format(time.RFC3339),
		registryv1alpha1.ScheduleAnnotation:            nightly,
	}
}

func running(name string) *registryv1alpha1.RegistryOperation {
	return &registryv1alpha1.RegistryOperation{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       registryv1alpha1.RegistryOperationSpec{Type: registryv1alpha1.OperationGarbageCollection},
		Status:     registryv1alpha1.RegistryOperationStatus{Phase: registryv1alpha1.OperationRunning},
	}
}

func TestTheScheduleCreatesACollection(t *testing.T) {
	r, c := newReconciler(t, threeAM.Add(time.Minute),
		storage(true, "", scheduledAt(threeAM.Add(-24*time.Hour))))

	result := reconcileStorage(t, r)

	ops := operations(t, c)
	require.Len(t, ops, 1)
	assert.Equal(t, registryv1alpha1.OperationGarbageCollection, ops[0].Spec.Type)
	assert.Contains(t, ops[0].Spec.Cause, nightly)
	assert.Equal(t, threeAM.Format(time.RFC3339), annotations(t, c)[registryv1alpha1.ScheduledCollectionAnnotation])
	assert.Equal(t, 24*time.Hour-time.Minute, result.RequeueAfter, "looked at again at the next firing")

	// The same firing is not acted on twice.
	reconcileStorage(t, r)
	assert.Len(t, operations(t, c), 1)
}

func TestAFirstLookOwesNothing(t *testing.T) {
	r, c := newReconciler(t, threeAM.Add(time.Minute), storage(true, "set-a", nil))

	reconcileStorage(t, r)

	assert.Empty(t, operations(t, c), "neither a firing nor a set it replaced is known yet")
	got := annotations(t, c)
	assert.Equal(t, threeAM.Add(time.Minute).Format(time.RFC3339), got[registryv1alpha1.ScheduledCollectionAnnotation])
	assert.Equal(t, "set-a", got[registryv1alpha1.CollectedSetAnnotation])
}

func TestAFiringLongMissedIsSkipped(t *testing.T) {
	r, c := newReconciler(t, threeAM.Add(10*time.Hour),
		storage(true, "", scheduledAt(threeAM.Add(-24*time.Hour))))

	reconcileStorage(t, r)

	assert.Empty(t, operations(t, c), "not at an hour nobody chose")
	assert.Equal(t, threeAM.Format(time.RFC3339), annotations(t, c)[registryv1alpha1.ScheduledCollectionAnnotation])
}

func TestAFiringDuringAnotherOperationIsSkipped(t *testing.T) {
	r, c := newReconciler(t, threeAM.Add(time.Minute),
		storage(true, "", scheduledAt(threeAM.Add(-24*time.Hour))), running("gc-manual-x"))

	reconcileStorage(t, r)

	assert.Len(t, operations(t, c), 1, "only the one already running")
	assert.Equal(t, threeAM.Format(time.RFC3339), annotations(t, c)[registryv1alpha1.ScheduledCollectionAnnotation])
}

func TestANewSetIsCollectedOnceEveryReplicaHoldsIt(t *testing.T) {
	annotated := scheduledAt(threeAM)
	annotated[registryv1alpha1.CollectedSetAnnotation] = "set-a"
	annotated[registryv1alpha1.CollectedReleaseAnnotation] = "v1.77.3"

	// One replica still on the old set: nothing yet.
	waiting := storage(true, "set-b", annotated)
	waiting.Status.Replicas[1].VerifiedSet = "set-a"
	r, c := newReconciler(t, threeAM.Add(time.Hour), waiting, release("v1.77.3", "Deployed"))
	reconcileStorage(t, r)
	assert.Empty(t, operations(t, c))

	// Both hold it, and the platform is the same: a change of modules.
	r, c = newReconciler(t, threeAM.Add(time.Hour), storage(true, "set-b", annotated), release("v1.77.3", "Deployed"))
	reconcileStorage(t, r)

	ops := operations(t, c)
	require.Len(t, ops, 1)
	assert.Equal(t, "The cache holds a new image set.", ops[0].Spec.Cause)
	assert.Equal(t, "set-b", annotations(t, c)[registryv1alpha1.CollectedSetAnnotation])

	// And not again for the same set.
	reconcileStorage(t, r)
	assert.Len(t, operations(t, c), 1)
}

func TestAnUpdateOfDeckhouseIsNamedInTheCause(t *testing.T) {
	annotated := scheduledAt(threeAM)
	annotated[registryv1alpha1.CollectedSetAnnotation] = "set-a"
	annotated[registryv1alpha1.CollectedReleaseAnnotation] = "v1.77.3"

	r, c := newReconciler(t, threeAM.Add(time.Hour), storage(true, "set-b", annotated),
		release("v1.77.3", "Superseded"), release("v1.78.0", "Deployed"))
	reconcileStorage(t, r)

	ops := operations(t, c)
	require.Len(t, ops, 1)
	assert.Equal(t, "Deckhouse was updated to v1.78.0.", ops[0].Spec.Cause)
	assert.Equal(t, "v1.78.0", annotations(t, c)[registryv1alpha1.CollectedReleaseAnnotation])
}

func TestANewSetWaitsForTheOperationInProgress(t *testing.T) {
	annotated := scheduledAt(threeAM)
	annotated[registryv1alpha1.CollectedSetAnnotation] = "set-a"

	r, c := newReconciler(t, threeAM.Add(time.Hour), storage(true, "set-b", annotated), running("gc-manual-x"))
	reconcileStorage(t, r)

	assert.Len(t, operations(t, c), 1)
	assert.Equal(t, "set-a", annotations(t, c)[registryv1alpha1.CollectedSetAnnotation],
		"still owed, so it is collected once the running one finishes")
}

func TestAScheduledCollectionCoversTheNewSet(t *testing.T) {
	annotated := scheduledAt(threeAM.Add(-24 * time.Hour))
	annotated[registryv1alpha1.CollectedSetAnnotation] = "set-a"

	r, c := newReconciler(t, threeAM.Add(time.Minute), storage(true, "set-b", annotated))
	reconcileStorage(t, r)

	assert.Len(t, operations(t, c), 1, "one collection, not one for the schedule and one for the set")
	assert.Equal(t, "set-b", annotations(t, c)[registryv1alpha1.CollectedSetAnnotation])
}

func TestNothingIsCreatedWhileCollectionIsOff(t *testing.T) {
	annotated := scheduledAt(threeAM.Add(-24 * time.Hour))
	annotated[registryv1alpha1.CollectedSetAnnotation] = "set-a"

	r, c := newReconciler(t, threeAM.Add(time.Minute), storage(false, "set-b", annotated))
	result := reconcileStorage(t, r)

	assert.Empty(t, operations(t, c))
	assert.Zero(t, result.RequeueAfter)
}

func deckhouse(image string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "d8-system", Name: "deckhouse"},
		Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "kube-rbac-proxy", Image: "registry.example.com/deckhouse:rbac-proxy"},
				{Name: "deckhouse", Image: image},
			},
		}}},
	}
}

// TestADevelopmentImageNamesItsRelease: a cluster installed from a development image deploys no
// DeckhouseRelease, and the image it runs is what says which release that is.
func TestADevelopmentImageNamesItsRelease(t *testing.T) {
	annotated := scheduledAt(threeAM)
	annotated[registryv1alpha1.CollectedSetAnnotation] = "set-a"
	annotated[registryv1alpha1.CollectedReleaseAnnotation] = "pr879"

	r, c := newReconciler(t, threeAM.Add(time.Hour), storage(true, "set-b", annotated),
		deckhouse("dev-registry.deckhouse.io/sys/deckhouse-oss:pr880"))
	reconcileStorage(t, r)

	ops := operations(t, c)
	require.Len(t, ops, 1)
	assert.Equal(t, "Deckhouse was updated to pr880.", ops[0].Spec.Cause)
	assert.Equal(t, "pr880", annotations(t, c)[registryv1alpha1.CollectedReleaseAnnotation])
}

func TestImageTag(t *testing.T) {
	for image, want := range map[string]string{
		"dev-registry.deckhouse.io/sys/deckhouse-oss:pr880":            "pr880",
		"registry.example.com:5000/deckhouse/ee:v1.78.0":               "v1.78.0",
		"registry.example.com:5000/deckhouse/ee:v1.78.0@sha256:abc123": "v1.78.0",
		"registry.example.com:5000/deckhouse/ee@sha256:abc123":         "",
		"registry.example.com:5000/deckhouse/ee":                       "",
		"":                                                             "",
	} {
		assert.Equal(t, want, imageTag(image), image)
	}
}

// full is a storage whose replicas hold no set and refuse writes: the state a live cluster sat in with
// 26.8 GB in a 26.8 GB budget, its fill failing at the first blob.
func full(annotations map[string]string) *registryv1alpha1.RegistryStorage {
	s := storage(true, "", annotations)
	for i := range s.Status.Replicas {
		s.Status.Replicas[i].Store = &registryv1alpha1.StoreUsage{Writable: false, Reason: registryv1alpha1.StoreReserveExhausted}
	}
	return s
}

// TestAStoreThatRefusesWritesIsCollectedWithoutAHeldSet: the set trigger waits for a set the store can
// never take in, so a store that refuses writes is collected anyway — once per release.
func TestAStoreThatRefusesWritesIsCollectedWithoutAHeldSet(t *testing.T) {
	r, c := newReconciler(t, threeAM.Add(time.Hour), full(scheduledAt(threeAM)),
		deckhouse("dev-registry.deckhouse.io/sys/deckhouse-oss:pr880"))
	reconcileStorage(t, r)

	ops := operations(t, c)
	require.Len(t, ops, 1)
	assert.Contains(t, ops[0].Name, "gc-full-")
	assert.Contains(t, ops[0].Spec.Cause, "master-0, master-1 refuses writes")
	assert.Equal(t, "pr880", annotations(t, c)[registryv1alpha1.RefusalCollectedAnnotation])

	// The collection ran and the store still refuses: not again for the same release.
	finished := ops[0].DeepCopy()
	finished.Status.Phase = registryv1alpha1.OperationSucceeded
	require.NoError(t, c.Status().Update(context.Background(), finished))
	reconcileStorage(t, r)
	assert.Len(t, operations(t, c), 1)
}

func TestARefusingStoreIsCollectedAgainAfterAnUpdate(t *testing.T) {
	annotated := scheduledAt(threeAM)
	annotated[registryv1alpha1.RefusalCollectedAnnotation] = "pr879"

	r, c := newReconciler(t, threeAM.Add(time.Hour), full(annotated),
		deckhouse("dev-registry.deckhouse.io/sys/deckhouse-oss:pr880"))
	reconcileStorage(t, r)

	require.Len(t, operations(t, c), 1)
	assert.Equal(t, "pr880", annotations(t, c)[registryv1alpha1.RefusalCollectedAnnotation])
}

func TestARefusingStoreWaitsForTheOperationInProgress(t *testing.T) {
	r, c := newReconciler(t, threeAM.Add(time.Hour), full(scheduledAt(threeAM)), running("gc-manual-x"))
	reconcileStorage(t, r)

	assert.Len(t, operations(t, c), 1, "only the one already running")
	assert.Empty(t, annotations(t, c)[registryv1alpha1.RefusalCollectedAnnotation])
}

func TestAStoreAcceptingWritesAgainForgetsTheRefusal(t *testing.T) {
	annotated := scheduledAt(threeAM)
	annotated[registryv1alpha1.RefusalCollectedAnnotation] = "pr880"

	r, c := newReconciler(t, threeAM.Add(time.Hour), storage(true, "", annotated))
	reconcileStorage(t, r)

	assert.Empty(t, operations(t, c))
	assert.Empty(t, annotations(t, c)[registryv1alpha1.RefusalCollectedAnnotation],
		"a later refusal is a new one")
}

func master(name string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name: name, Labels: map[string]string{"node-role.kubernetes.io/control-plane": ""},
	}}
}

// TestTheEntryOfAMasterThatLeftDoesNotHoldBackANewSet: the entry stays with the set it last had, and
// "every replica holds the new set" would never be true again.
func TestTheEntryOfAMasterThatLeftDoesNotHoldBackANewSet(t *testing.T) {
	annotated := scheduledAt(threeAM)
	annotated[registryv1alpha1.CollectedSetAnnotation] = "set-a"

	s := storage(true, "set-b", annotated)
	s.Status.Replicas = append(s.Status.Replicas, registryv1alpha1.StorageReplicaStatus{
		Node: "master-old", Full: true, VerifiedSet: "set-a",
	})
	r, c := newReconciler(t, threeAM.Add(time.Hour), s, master("master-0"), master("master-1"))
	reconcileStorage(t, r)

	ops := operations(t, c)
	require.Len(t, ops, 1)
	assert.Equal(t, "The cache holds a new image set.", ops[0].Spec.Cause)
}

// TestAScheduleThatNeverFiresIsNotSpunOn: the 30th of February parses, and has no next firing.
func TestAScheduleThatNeverFiresIsNotSpunOn(t *testing.T) {
	s := storage(true, "", scheduledAt(threeAM))
	s.Spec.GarbageCollection.Schedule = "0 0 30 2 *"

	r, c := newReconciler(t, threeAM.Add(time.Hour), s)
	result := reconcileStorage(t, r)

	assert.Zero(t, result.RequeueAfter)
	assert.Empty(t, operations(t, c))
}

// TestAChangedScheduleCountsFromWhenItChanged: the firing last acted on belongs to the old schedule.
// Counted from it, "0 9 * * *" set at ten fired at once, for a nine o'clock that had passed.
func TestAChangedScheduleCountsFromWhenItChanged(t *testing.T) {
	changed := storage(true, "", scheduledAt(threeAM))
	changed.Spec.GarbageCollection.Schedule = "0 9 * * *"
	ten := time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)

	r, c := newReconciler(t, ten, changed)
	result := reconcileStorage(t, r)

	assert.Empty(t, operations(t, c), "not at an hour nobody chose")
	got := annotations(t, c)
	assert.Equal(t, ten.Format(time.RFC3339), got[registryv1alpha1.ScheduledCollectionAnnotation])
	assert.Equal(t, "0 9 * * *", got[registryv1alpha1.ScheduleAnnotation])
	assert.Equal(t, 23*time.Hour, result.RequeueAfter, "the first firing of the new schedule is tomorrow's")
}

// TestTurningCollectionOnAgainOwesNothing: a firing that passed while collection was off is not one
// anybody asked for.
func TestTurningCollectionOnAgainOwesNothing(t *testing.T) {
	r, c := newReconciler(t, threeAM.Add(-time.Hour), storage(false, "", scheduledAt(threeAM.Add(-24*time.Hour))))
	reconcileStorage(t, r)
	assert.Empty(t, annotations(t, c)[registryv1alpha1.ScheduledCollectionAnnotation], "forgotten while off")

	on := &registryv1alpha1.RegistryStorage{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: registryv1alpha1.SingletonName}, on))
	on.Spec.GarbageCollection.Enabled = true
	require.NoError(t, c.Update(context.Background(), on))

	r.Now = func() time.Time { return threeAM.Add(time.Hour) }
	reconcileStorage(t, r)
	assert.Empty(t, operations(t, c), "the firing at three passed while it was off")
}

// TestAFailedCollectionOfARefusingStoreIsTriedAgain: the nightly schedule fails the same way on such a
// store, and the store stayed full until somebody raised its size.
func TestAFailedCollectionOfARefusingStoreIsTriedAgain(t *testing.T) {
	r, c := newReconciler(t, threeAM.Add(time.Hour), full(scheduledAt(threeAM)),
		deckhouse("dev-registry.deckhouse.io/sys/deckhouse-oss:pr880"))
	reconcileStorage(t, r)
	ops := operations(t, c)
	require.Len(t, ops, 1)

	failed := ops[0].DeepCopy()
	failed.Status.Phase = registryv1alpha1.OperationFailed
	failed.Status.FinishedAt = &metav1.Time{Time: threeAM.Add(2 * time.Hour)}
	require.NoError(t, c.Status().Update(context.Background(), failed))

	r.Now = func() time.Time { return threeAM.Add(3 * time.Hour) }
	result := reconcileStorage(t, r)
	assert.Len(t, operations(t, c), 1, "not straight away")
	assert.Equal(t, 3*time.Hour+time.Second, result.RequeueAfter, "looked at again when it may be")

	r.Now = func() time.Time { return threeAM.Add(6 * time.Hour) }
	reconcileStorage(t, r)
	assert.Len(t, operations(t, c), 2)
}

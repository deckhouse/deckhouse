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

package repair

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-controller/internal/layout"
	"github.com/deckhouse/registry-controller/internal/register"
)

var now = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

func master(name string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{layout.ControlPlaneLabel: ""}}}
}

func storage(corrupted map[string][]string) *registryv1alpha1.RegistryStorage {
	storage := &registryv1alpha1.RegistryStorage{ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName}}
	for _, node := range []string{"master-0", "master-1"} {
		storage.Status.Replicas = append(storage.Status.Replicas, registryv1alpha1.StorageReplicaStatus{
			Node: node, CorruptedBlobs: corrupted[node],
		})
	}
	return storage
}

func newReconciler(t *testing.T, objects ...client.Object) (*Reconciler, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))
	objects = append(objects, master("master-0"), master("master-1"))
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).
		WithStatusSubresource(&registryv1alpha1.RegistryStorage{}, &registryv1alpha1.RegistryOperation{}).Build()
	clock := now
	return &Reconciler{Base: register.Base{Client: c}, Now: func() time.Time { return clock }}, c
}

func reconcileStorage(t *testing.T, r *Reconciler) ctrl.Result {
	t.Helper()
	result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: registryv1alpha1.SingletonName}})
	require.NoError(t, err)
	return result
}

func operations(t *testing.T, c client.Client) []registryv1alpha1.RegistryOperation {
	t.Helper()
	list := &registryv1alpha1.RegistryOperationList{}
	require.NoError(t, c.List(context.Background(), list))
	return list.Items
}

func TestNothingBrokenAsksForNothing(t *testing.T) {
	r, c := newReconciler(t, storage(nil))
	reconcileStorage(t, r)
	assert.Empty(t, operations(t, c))
}

func TestBrokenBlobsAreRepairedWhereTheyWereFound(t *testing.T) {
	r, c := newReconciler(t, storage(map[string][]string{"master-1": {"sha256:bb", "sha256:aa"}}))
	result := reconcileStorage(t, r)

	created := operations(t, c)
	require.Len(t, created, 1)
	spec := created[0].Spec
	assert.Equal(t, registryv1alpha1.OperationRepair, spec.Type)
	assert.Equal(t, registryv1alpha1.CorruptionDetectedCause, spec.Cause)
	require.NotNil(t, spec.Repair)
	assert.Equal(t, []string{"sha256:aa", "sha256:bb"}, spec.Repair.Digests)
	assert.Equal(t, []string{"master-1"}, spec.Repair.Nodes)
	assert.Equal(t, retryAfter+time.Second, result.RequeueAfter)

	// The same findings are not asked for again until the retry.
	created[0].Status.Phase = registryv1alpha1.OperationFailed
	require.NoError(t, c.Status().Update(context.Background(), &created[0]))
	result = reconcileStorage(t, r)
	assert.Len(t, operations(t, c), 1)
	assert.Equal(t, retryAfter+time.Second, result.RequeueAfter)

	r.Now = func() time.Time { return now.Add(retryAfter) }
	reconcileStorage(t, r)
	assert.Len(t, operations(t, c), 2, "a repair that did not fix them is tried again after a while")
}

func TestANewFindingIsAskedForAtOnceButWaitsForTheQueue(t *testing.T) {
	busy := &registryv1alpha1.RegistryOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "gc-schedule-1"},
		Spec:       registryv1alpha1.RegistryOperationSpec{Type: registryv1alpha1.OperationGarbageCollection},
		Status:     registryv1alpha1.RegistryOperationStatus{Phase: registryv1alpha1.OperationRunning},
	}
	r, c := newReconciler(t, storage(map[string][]string{"master-0": {"sha256:aa"}}), busy)
	reconcileStorage(t, r)
	assert.Len(t, operations(t, c), 1, "one operation at a time: the repair waits for the collection")

	busy.Status.Phase = registryv1alpha1.OperationSucceeded
	require.NoError(t, c.Status().Update(context.Background(), busy))
	reconcileStorage(t, r)
	assert.Len(t, operations(t, c), 2)
}

func TestTheFindingsOfAMasterThatLeftAreNotRepaired(t *testing.T) {
	gone := storage(map[string][]string{"master-0": {"sha256:aa"}})
	gone.Status.Replicas[0].Node = "master-9"
	r, c := newReconciler(t, gone)
	reconcileStorage(t, r)
	assert.Empty(t, operations(t, c))
}

func TestWantedListsAtMostWhatAnOperationTakes(t *testing.T) {
	var many []string
	for i := range registryv1alpha1.MaxOperationDigests + 10 {
		many = append(many, "sha256:"+string(rune('a'+i%26))+string(rune('a'+i/26)))
	}
	request := Wanted(storage(map[string][]string{"master-0": many}))
	require.NotNil(t, request)
	assert.Len(t, request.Digests, registryv1alpha1.MaxOperationDigests)
	assert.NotEmpty(t, request.Fingerprint)
}

// A Repair refused for another operation never ran, and is no reason to wait four hours.
func TestARefusedRepairIsAskedForAgainAtOnce(t *testing.T) {
	r, c := newReconciler(t, storage(map[string][]string{"master-0": {"sha256:aa"}}))
	reconcileStorage(t, r)
	created := operations(t, c)
	require.Len(t, created, 1)

	created[0].Status.Phase = registryv1alpha1.OperationFailed
	created[0].Status.Reason = registryv1alpha1.ReasonOperationAnotherActive
	require.NoError(t, c.Status().Update(context.Background(), &created[0]))
	reconcileStorage(t, r)
	assert.Len(t, operations(t, c), 2)
}

// Blobs nothing can repair must not keep the rest out of every request: asked for again, the next
// window of them is.
func TestARepeatedRequestTakesTheNextBlobs(t *testing.T) {
	var many []string
	for i := range registryv1alpha1.MaxOperationDigests + 10 {
		many = append(many, fmt.Sprintf("sha256:%064x", i))
	}
	r, c := newReconciler(t, storage(map[string][]string{"master-0": many}))
	reconcileStorage(t, r)
	first := operations(t, c)
	require.Len(t, first, 1)
	assert.Equal(t, many[:registryv1alpha1.MaxOperationDigests], first[0].Spec.Repair.Digests)

	first[0].Status.Phase = registryv1alpha1.OperationSucceeded
	require.NoError(t, c.Status().Update(context.Background(), &first[0]))
	r.Now = func() time.Time { return now.Add(retryAfter) }
	reconcileStorage(t, r)

	all := operations(t, c)
	require.Len(t, all, 2)
	var second []string
	for _, operation := range all {
		if operation.Name != first[0].Name {
			second = operation.Spec.Repair.Digests
		}
	}
	assert.Subset(t, second, many[registryv1alpha1.MaxOperationDigests:], "the ten left out the first time")
	assert.Len(t, second, registryv1alpha1.MaxOperationDigests)
}

// The same blob broken again on the same replica, after a Repair repaired it and the finding went, is a
// new finding: the stand waited four hours for it, the fingerprint being the one of the first request.
func TestABlobBrokenAgainAfterARepairIsAskedForAtOnce(t *testing.T) {
	broken := storage(map[string][]string{"master-0": {"sha256:aa"}})
	r, c := newReconciler(t, broken)
	reconcileStorage(t, r)
	first := operations(t, c)
	require.Len(t, first, 1)
	first[0].Status.Phase = registryv1alpha1.OperationSucceeded
	require.NoError(t, c.Status().Update(context.Background(), &first[0]))

	// Still recorded: the same findings wait out the pause.
	reconcileStorage(t, r)
	assert.Len(t, operations(t, c), 1)

	setFindings := func(blobs []string) {
		current := &registryv1alpha1.RegistryStorage{}
		require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: registryv1alpha1.SingletonName}, current))
		current.Status.Replicas[0].CorruptedBlobs = blobs
		require.NoError(t, c.Status().Update(context.Background(), current))
	}

	setFindings(nil)
	reconcileStorage(t, r)
	current := &registryv1alpha1.RegistryStorage{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: registryv1alpha1.SingletonName}, current))
	assert.NotContains(t, current.Annotations, registryv1alpha1.RepairRequestedAnnotation)

	setFindings([]string{"sha256:aa"})
	reconcileStorage(t, r)
	assert.Len(t, operations(t, c), 2)
}

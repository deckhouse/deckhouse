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

package gc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

var operationsEpoch = time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

func clusterOperations(t *testing.T, objects ...client.Object) (*ClusterOperations, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithStatusSubresource(&registryv1alpha1.RegistryOperation{}).
		Build()
	return &ClusterOperations{Client: c, Node: "master-0"}, c
}

// gcOperation is a garbage collection created at the epoch plus created, in phase, with master-0's entry
// in entry.
func gcOperation(
	name string, created time.Duration,
	phase registryv1alpha1.RegistryOperationPhase, entry registryv1alpha1.ReplicaOperationPhase,
) *registryv1alpha1.RegistryOperation {
	deadline := metav1.NewTime(operationsEpoch.Add(time.Hour))
	operation := &registryv1alpha1.RegistryOperation{
		ObjectMeta: metav1.ObjectMeta{Name: name, CreationTimestamp: metav1.NewTime(operationsEpoch.Add(created))},
		Spec:       registryv1alpha1.RegistryOperationSpec{Type: registryv1alpha1.OperationGarbageCollection},
		Status: registryv1alpha1.RegistryOperationStatus{
			Phase:    phase,
			Deadline: &deadline,
			Replicas: []registryv1alpha1.ReplicaOperationStatus{
				{Node: "master-1", Phase: registryv1alpha1.ReplicaOperationPending},
			},
		},
	}
	if entry != "" {
		operation.Status.Replicas = append(operation.Status.Replicas,
			registryv1alpha1.ReplicaOperationStatus{Node: "master-0", Phase: entry})
	}
	return operation
}

func TestNextPicksTheOldestOperationThisReplicaOwes(t *testing.T) {
	operations, _ := clusterOperations(t,
		// Not accepted yet: the controller decides whether it is served at all.
		gcOperation("unaccepted", 0, "", ""),
		// Finished.
		gcOperation("finished", time.Second, registryv1alpha1.OperationSucceeded, registryv1alpha1.ReplicaOperationSucceeded),
		// Accepted, but this replica's part is settled.
		gcOperation("settled", 2*time.Second, registryv1alpha1.OperationRunning, registryv1alpha1.ReplicaOperationSkipped),
		// Accepted before this replica joined.
		gcOperation("not-ours", 3*time.Second, registryv1alpha1.OperationPending, ""),
		gcOperation("newer", 5*time.Second, registryv1alpha1.OperationPending, registryv1alpha1.ReplicaOperationPending),
		gcOperation("owed", 4*time.Second, registryv1alpha1.OperationRunning, registryv1alpha1.ReplicaOperationWaiting),
	)

	operation, err := operations.Next(context.Background())
	require.NoError(t, err)
	require.NotNil(t, operation)
	assert.Equal(t, "owed", operation.Name)
	assert.True(t, operation.Deadline.Equal(operationsEpoch.Add(time.Hour)))
	assert.False(t, operation.Canceled)
}

func TestNextFindsNothingWhenNothingIsOwed(t *testing.T) {
	operations, _ := clusterOperations(t,
		gcOperation("finished", 0, registryv1alpha1.OperationFailed, registryv1alpha1.ReplicaOperationFailed))

	operation, err := operations.Next(context.Background())
	require.NoError(t, err)
	assert.Nil(t, operation)
}

func TestGetSeesTheCancelLabel(t *testing.T) {
	canceled := gcOperation("manual-1", 0, registryv1alpha1.OperationRunning, registryv1alpha1.ReplicaOperationRunning)
	canceled.Labels = map[string]string{registryv1alpha1.OperationCancelLabel: "true"}
	operations, _ := clusterOperations(t, canceled)

	operation, err := operations.Get(context.Background(), "manual-1")
	require.NoError(t, err)
	require.NotNil(t, operation)
	assert.True(t, operation.Canceled)

	gone, err := operations.Get(context.Background(), "deleted")
	require.NoError(t, err)
	assert.Nil(t, gone)
}

func TestReportWritesOnlyThisReplicasEntry(t *testing.T) {
	operations, c := clusterOperations(t,
		gcOperation("manual-1", 0, registryv1alpha1.OperationRunning, registryv1alpha1.ReplicaOperationWaiting))

	started := operationsEpoch.Add(time.Minute + 500*time.Millisecond)
	finished := operationsEpoch.Add(3 * time.Minute)
	reclaimed := int64(1 << 30)
	require.NoError(t, operations.Report(context.Background(), "manual-1", Progress{
		Phase:      registryv1alpha1.ReplicaOperationSucceeded,
		Message:    "Removed 3 references.",
		StartedAt:  &started,
		FinishedAt: &finished,
		Report: &Report{
			Deleted:          []string{"a", "b", "c"},
			RemovedVersions:  []string{"v1.71.2"},
			RemovedManifests: 2,
			RemovedBlobs:     1199,
			ReclaimedBytes:   &reclaimed,
		},
	}))

	got := &registryv1alpha1.RegistryOperation{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "manual-1"}, got))

	other := got.Status.Replica("master-1")
	require.NotNil(t, other)
	assert.Equal(t, registryv1alpha1.ReplicaOperationPending, other.Phase, "another replica's entry is left alone")

	entry := got.Status.Replica("master-0")
	require.NotNil(t, entry)
	assert.Equal(t, registryv1alpha1.ReplicaOperationSucceeded, entry.Phase)
	assert.Equal(t, "Removed 3 references.", entry.Message)
	require.NotNil(t, entry.StartedAt)
	assert.True(t, entry.StartedAt.Time.Equal(operationsEpoch.Add(time.Minute)), "stored the way the API server stores it")
	require.NotNil(t, entry.FinishedAt)
	require.NotNil(t, entry.GarbageCollection)
	assert.EqualValues(t, 2, entry.GarbageCollection.RemovedManifests)
	require.NotNil(t, entry.GarbageCollection.RemovedBlobs)
	assert.EqualValues(t, 1199, *entry.GarbageCollection.RemovedBlobs)
	assert.Equal(t, []string{"v1.71.2"}, entry.GarbageCollection.RemovedVersions)
	require.NotNil(t, entry.GarbageCollection.ReclaimedBytes)
	assert.Equal(t, reclaimed, *entry.GarbageCollection.ReclaimedBytes)
}

// TestReportDoesNotReopenASettledEntry: the controller gave up on this replica, and a report that
// arrives after that changes nothing.
func TestReportDoesNotReopenASettledEntry(t *testing.T) {
	operations, c := clusterOperations(t,
		gcOperation("manual-1", 0, registryv1alpha1.OperationRunning, registryv1alpha1.ReplicaOperationFailed))

	require.ErrorIs(t, operations.Report(context.Background(), "manual-1", Progress{
		Phase: registryv1alpha1.ReplicaOperationRunning,
	}), ErrNotOwed, "said, so a replica about to start does not")

	got := &registryv1alpha1.RegistryOperation{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "manual-1"}, got))
	assert.Equal(t, registryv1alpha1.ReplicaOperationFailed, got.Status.Replica("master-0").Phase)

	// An operation that is gone is nothing to report to either.
	require.ErrorIs(t, operations.Report(context.Background(), "deleted", Progress{
		Phase: registryv1alpha1.ReplicaOperationRunning,
	}), ErrNotOwed)
}

// TestAnotherTypeIsNotOurs: a replica collects only for a GarbageCollection, whatever else the type
// list grows to.
func TestAnotherTypeIsNotOurs(t *testing.T) {
	other := gcOperation("other", 0, registryv1alpha1.OperationPending, registryv1alpha1.ReplicaOperationPending)
	other.Spec.Type = "SomethingElse"
	operations, _ := clusterOperations(t, other)

	operation, err := operations.Next(context.Background())
	require.NoError(t, err)
	assert.Nil(t, operation)
}

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

package observe

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-controller/internal/metrics"
)

func newReconciler(t *testing.T, objects ...client.Object) *Reconciler {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
	r := &Reconciler{}
	r.InjectClient(c)

	// The gauges are process-wide, so a test that did not clear them would read another
	// test's cluster.
	resetAll()
	return r
}

func resetAll() {
	metrics.Managed.Set(0)
	metrics.ConfigValid.Set(0)
	metrics.Nodes.Set(0)
	metrics.StorageReplicas.Set(0)
	metrics.StorageAllReplicasFull.Set(0)
	metrics.StorageSafeToDropUpstream.Set(0)
	metrics.EffectiveUpstream.Reset()
	metrics.NodeReconciled.Reset()
	metrics.NodeFromCache.Reset()
	metrics.StorageReplicaFull.Reset()
	metrics.StorageStaleData.Reset()
}

func run(t *testing.T, r *Reconciler) {
	t.Helper()

	result, err := r.Reconcile(context.Background(), ctrl.Request{})
	require.NoError(t, err)
	// Re-queued unconditionally, because the gauges live in a process that can restart:
	// after a restart nothing has changed, so nothing would trigger a reconciliation.
	assert.Positive(t, result.RequeueAfter)
}

func config(spec registryv1alpha1.RegistryConfigSpec, status registryv1alpha1.RegistryConfigStatus) *registryv1alpha1.RegistryConfig {
	return &registryv1alpha1.RegistryConfig{
		ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName},
		Spec:       spec,
		Status:     status,
	}
}

func TestObserveManagedConfig(t *testing.T) {
	r := newReconciler(t, config(
		registryv1alpha1.RegistryConfigSpec{Mode: registryv1alpha1.ModeManaged},
		registryv1alpha1.RegistryConfigStatus{
			Conditions: []metav1.Condition{{
				Type:   registryv1alpha1.ConditionValid,
				Status: metav1.ConditionTrue,
				Reason: registryv1alpha1.ReasonResolved,
			}},
			EffectiveUpstream: &registryv1alpha1.Upstream{
				Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io"},
			},
		}))

	run(t, r)

	assert.EqualValues(t, 1, testutil.ToFloat64(metrics.Managed))
	assert.EqualValues(t, 1, testutil.ToFloat64(metrics.ConfigValid))
	assert.EqualValues(t, 1,
		testutil.ToFloat64(metrics.EffectiveUpstream.WithLabelValues("registry.deckhouse.io")))
}

// TestObserveUnmanagedConfig matters because every other metric is meaningless without it:
// a cluster that manages nothing has no cache to be incomplete and no nodes to be
// unreconciled, and an alert on those would fire at a module doing exactly what it was told.
func TestObserveUnmanagedConfig(t *testing.T) {
	r := newReconciler(t, config(
		registryv1alpha1.RegistryConfigSpec{Mode: registryv1alpha1.ModeUnmanaged},
		registryv1alpha1.RegistryConfigStatus{}))

	run(t, r)
	assert.Zero(t, testutil.ToFloat64(metrics.Managed))
}

// TestObserveWithoutAConfig covers a cluster where the module manages nothing at all. The
// gauges must read zero rather than keep their last value.
func TestObserveWithoutAConfig(t *testing.T) {
	r := newReconciler(t)
	metrics.ConfigValid.Set(1)

	run(t, r)
	assert.Zero(t, testutil.ToFloat64(metrics.ConfigValid))
}

func TestObserveStorage(t *testing.T) {
	storage := &registryv1alpha1.RegistryStorage{
		ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName},
		Status: registryv1alpha1.RegistryStorageStatus{
			SafeToDropUpstream: true,
			Replicas: []registryv1alpha1.StorageReplicaStatus{
				{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true},
				{Node: "master-1", Role: registryv1alpha1.ReplicaRoleFollower, Full: false},
				{Node: "master-2", Role: registryv1alpha1.ReplicaRoleFollower, Full: true,
					Error: "3 of 459 references could not be copied"},
			},
		},
	}

	r := newReconciler(t, storage)
	run(t, r)

	assert.EqualValues(t, 3, testutil.ToFloat64(metrics.StorageReplicas))
	assert.EqualValues(t, 1, testutil.ToFloat64(metrics.StorageSafeToDropUpstream))
	assert.EqualValues(t, 1, testutil.ToFloat64(
		metrics.StorageReplicaFull.WithLabelValues("master-0", string(registryv1alpha1.ReplicaRoleLeader))))
	assert.Zero(t, testutil.ToFloat64(
		metrics.StorageReplicaFull.WithLabelValues("master-1", string(registryv1alpha1.ReplicaRoleFollower))))
	// Counters that look complete beside an error are not completeness: `full` says what
	// it holds, the error says whether its last pass finished.
	assert.Zero(t, testutil.ToFloat64(
		metrics.StorageReplicaFull.WithLabelValues("master-2", string(registryv1alpha1.ReplicaRoleFollower))))
}

// TestObserveStorageGoesAwayWithIt: a cache that was turned off must stop being reported,
// or the gauges keep describing a storage that no longer exists.
func TestObserveStorageGoesAwayWithIt(t *testing.T) {
	r := newReconciler(t)
	metrics.StorageReplicas.Set(3)
	metrics.StorageReplicaFull.WithLabelValues("master-0", string(registryv1alpha1.ReplicaRoleLeader)).Set(1)

	run(t, r)

	assert.Zero(t, testutil.ToFloat64(metrics.StorageReplicas))
	assert.Zero(t, testutil.CollectAndCount(metrics.StorageReplicaFull))
}

func node(name string, generation int64, status registryv1alpha1.RegistryNodeStatus) *registryv1alpha1.RegistryNode {
	return &registryv1alpha1.RegistryNode{
		ObjectMeta: metav1.ObjectMeta{Name: name, Generation: generation},
		Status:     status,
	}
}

func TestObserveNodes(t *testing.T) {
	r := newReconciler(t,
		node("worker-1", 7, registryv1alpha1.RegistryNodeStatus{
			Reconciled: true, ObservedGeneration: 7,
		}),
		// Applied an older layout: reconciled by its own account, behind by the
		// cluster's. Those are the same field to the agent and different questions to an
		// operator.
		node("worker-2", 7, registryv1alpha1.RegistryNodeStatus{
			Reconciled: true, ObservedGeneration: 6,
		}),
	)

	run(t, r)

	assert.EqualValues(t, 2, testutil.ToFloat64(metrics.Nodes))
	assert.EqualValues(t, 1, testutil.ToFloat64(metrics.NodeReconciled.WithLabelValues("worker-1")))
	assert.Zero(t, testutil.ToFloat64(metrics.NodeReconciled.WithLabelValues("worker-2")))
}

// TestObserveNodeRunningFromDisk is the state that looks like success from every other
// angle: the node pulls images, its condition is True, and its configuration may be behind
// the cluster's by any amount.
func TestObserveNodeRunningFromDisk(t *testing.T) {
	r := newReconciler(t, node("worker-1", 7, registryv1alpha1.RegistryNodeStatus{
		Reconciled: true, ObservedGeneration: 7,
		Conditions: []metav1.Condition{{
			Type:   registryv1alpha1.ConditionReconciled,
			Status: metav1.ConditionTrue,
			Reason: registryv1alpha1.ReasonAppliedFromCache,
		}},
	}))

	run(t, r)
	assert.EqualValues(t, 1, testutil.ToFloat64(metrics.NodeFromCache.WithLabelValues("worker-1")))
}

// TestObserveStaleStorageData is the one metric that could not exist without this
// reconciler. The agent measures it, and the agent cannot be scraped: it is a static pod
// that has to work when the API server does not, so it can carry no kube-rbac-proxy.
func TestObserveStaleStorageData(t *testing.T) {
	r := newReconciler(t,
		node("master-0", 1, registryv1alpha1.RegistryNodeStatus{StaleStorageDataBytes: 42 << 30}),
		node("worker-1", 1, registryv1alpha1.RegistryNodeStatus{}),
	)

	run(t, r)

	assert.EqualValues(t, 42<<30,
		testutil.ToFloat64(metrics.StorageStaleData.WithLabelValues("master-0")))
	// A node with nothing left behind gets no series at all, rather than a zero: an alert
	// on "greater than zero" would be noise either way, but a series per node in the
	// cluster is a cost paid for nothing.
	assert.EqualValues(t, 1, testutil.CollectAndCount(metrics.StorageStaleData))
}

// TestObserveDoesNotRequireLeadership states the design choice: this reconciler converges
// nothing, and a picture of the cluster that depended on which pod holds a lease would make
// the reporting stop exactly when the leader is the thing in trouble.
func TestObserveDoesNotRequireLeadership(t *testing.T) {
	assert.True(t, (&Reconciler{}).SkipLeaderElection())
}

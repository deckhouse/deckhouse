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

package operationsapprover

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

func TestNewApprover_ConcurrencyLimits(t *testing.T) {
	t.Parallel()

	t.Run("single node uses workload limit 1", func(t *testing.T) {
		t.Parallel()
		a := newApprover(nodeCounts{masters: 1}, nil)
		etcd := a.approveChain.components[controlplanev1alpha1.OperationComponentEtcd]
		require.Equal(t, 1, etcd.concurrencyLimit)
		api := a.approveChain.nextLink.components[controlplanev1alpha1.OperationComponentKubeAPIServer]
		require.Equal(t, 1, api.concurrencyLimit)
	})

	t.Run("three nodes workload limit is nodesCount-1", func(t *testing.T) {
		t.Parallel()
		a := newApprover(nodeCounts{masters: 3}, nil)
		api := a.approveChain.nextLink.components[controlplanev1alpha1.OperationComponentKubeAPIServer]
		require.Equal(t, 2, api.concurrencyLimit)
	})
}

func TestApprover_TryApprove_Etcd(t *testing.T) {
	t.Parallel()

	t.Run("allows first etcd operation", func(t *testing.T) {
		t.Parallel()
		a := newApprover(nodeCounts{masters: 3}, nil)
		op := newOperation("etcd-1", "node-a", controlplanev1alpha1.OperationComponentEtcd, false)
		require.True(t, a.tryApprove(op))
	})

	t.Run("rejects second etcd while first is reserved in same approver", func(t *testing.T) {
		t.Parallel()
		a := newApprover(nodeCounts{masters: 3}, nil)
		require.True(t, a.tryApprove(newOperation("etcd-1", "node-a", controlplanev1alpha1.OperationComponentEtcd, false)))
		require.False(t, a.tryApprove(newOperation("etcd-2", "node-b", controlplanev1alpha1.OperationComponentEtcd, false)))
	})

	t.Run("rejects second etcd on same node", func(t *testing.T) {
		t.Parallel()
		a := newApprover(nodeCounts{masters: 3}, nil)
		require.True(t, a.tryApprove(newOperation("etcd-1", "node-a", controlplanev1alpha1.OperationComponentEtcd, false)))
		require.False(t, a.tryApprove(newOperation("etcd-2", "node-a", controlplanev1alpha1.OperationComponentEtcd, false)))
	})

	t.Run("seed in-flight etcd blocks another etcd", func(t *testing.T) {
		t.Parallel()
		seed := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("etcd-running", "node-a", controlplanev1alpha1.OperationComponentEtcd, true),
		}
		a := newApprover(nodeCounts{masters: 3}, seed)
		require.False(t, a.tryApprove(newOperation("etcd-new", "node-b", controlplanev1alpha1.OperationComponentEtcd, false)))
	})
}

func TestApprover_TryApprove_StageOrdering(t *testing.T) {
	t.Parallel()

	t.Run("blocks apiserver while etcd stage has reservation", func(t *testing.T) {
		t.Parallel()
		a := newApprover(nodeCounts{masters: 3}, nil)
		require.True(t, a.tryApprove(newOperation("e1", "n1", controlplanev1alpha1.OperationComponentEtcd, false)))
		require.False(t, a.tryApprove(newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false)))
	})

	t.Run("blocks apiserver on another node while etcd stage has reservation", func(t *testing.T) {
		t.Parallel()
		a := newApprover(nodeCounts{masters: 3}, nil)
		require.True(t, a.tryApprove(newOperation("e1", "n1", controlplanev1alpha1.OperationComponentEtcd, false)))
		require.False(t, a.tryApprove(newOperation("a1", "n2", controlplanev1alpha1.OperationComponentKubeAPIServer, false)))
	})

	t.Run("allows apiserver when etcd stage is empty", func(t *testing.T) {
		t.Parallel()
		a := newApprover(nodeCounts{masters: 3}, nil)
		require.True(t, a.tryApprove(newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false)))
	})

	t.Run("blocks kcm while apiserver stage has reservation", func(t *testing.T) {
		t.Parallel()
		a := newApprover(nodeCounts{masters: 3}, nil)
		require.True(t, a.tryApprove(newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false)))
		require.False(t, a.tryApprove(newOperation("k1", "n1", controlplanev1alpha1.OperationComponentKubeControllerManager, false)))
	})

	t.Run("allows kcm on another node while apiserver stage has reservation", func(t *testing.T) {
		t.Parallel()
		a := newApprover(nodeCounts{masters: 3}, nil)
		require.True(t, a.tryApprove(newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false)))
		require.True(t, a.tryApprove(newOperation("k1", "n2", controlplanev1alpha1.OperationComponentKubeControllerManager, false)))
	})

	t.Run("queued apiserver blocks kcm on same node but not on other nodes", func(t *testing.T) {
		t.Parallel()
		// 3 nodes -> apiserver concurrency limit 2
		a := newApprover(nodeCounts{masters: 3}, nil)
		require.True(t, a.tryApprove(newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false)))
		require.True(t, a.tryApprove(newOperation("a2", "n2", controlplanev1alpha1.OperationComponentKubeAPIServer, false)))
		// a3 hits concurrency limit -> goes to queue
		require.False(t, a.tryApprove(newOperation("a3", "n3", controlplanev1alpha1.OperationComponentKubeAPIServer, false)))
		// kcm on n3 must be blocked (apiserver queued on n3)
		require.False(t, a.tryApprove(newOperation("k1", "n3", controlplanev1alpha1.OperationComponentKubeControllerManager, false)))
		// kcm on n1 must be blocked (apiserver approved on n1)
		require.False(t, a.tryApprove(newOperation("k2", "n1", controlplanev1alpha1.OperationComponentKubeControllerManager, false)))
	})

	t.Run("allows kcm and scheduler concurrently on same pipeline stage", func(t *testing.T) {
		t.Parallel()
		a := newApprover(nodeCounts{masters: 3}, nil)
		require.True(t, a.tryApprove(newOperation("k1", "n1", controlplanev1alpha1.OperationComponentKubeControllerManager, false)))
		require.True(t, a.tryApprove(newOperation("s1", "n2", controlplanev1alpha1.OperationComponentKubeScheduler, false)))
	})
}

func TestApprover_TryApprove_WorkloadConcurrencyAndPerNode(t *testing.T) {
	t.Parallel()

	t.Run("allows up to workload concurrency limit on distinct nodes", func(t *testing.T) {
		t.Parallel()
		// 3 nodes -> limit 2 for apiserver
		a := newApprover(nodeCounts{masters: 3}, nil)
		require.True(t, a.tryApprove(newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false)))
		require.True(t, a.tryApprove(newOperation("a2", "n2", controlplanev1alpha1.OperationComponentKubeAPIServer, false)))
		require.False(t, a.tryApprove(newOperation("a3", "n3", controlplanev1alpha1.OperationComponentKubeAPIServer, false)))
	})

	t.Run("rejects second apiserver on same node", func(t *testing.T) {
		t.Parallel()
		a := newApprover(nodeCounts{masters: 3}, nil)
		require.True(t, a.tryApprove(newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false)))
		require.False(t, a.tryApprove(newOperation("a2", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false)))
	})
}

func TestNewApprover_PartitionAndOrder(t *testing.T) {
	t.Parallel()

	t.Run("unapproved operation is only in approveQueue", func(t *testing.T) {
		t.Parallel()
		op := newOperation("x", "n1", controlplanev1alpha1.OperationComponentEtcd, false)
		a := newApprover(nodeCounts{masters: 1}, []controlplanev1alpha1.ControlPlaneOperation{op})
		require.Empty(t, a.approveChain.components[controlplanev1alpha1.OperationComponentEtcd].approvedOperationsPerNode)
		require.Len(t, a.approveQueue, 1)
		require.Equal(t, "x", a.approveQueue[0].Name)
	})

	t.Run("approved and incompleted is seed only empty queue", func(t *testing.T) {
		t.Parallel()
		op := newOperation("x", "n1", controlplanev1alpha1.OperationComponentEtcd, true)
		a := newApprover(nodeCounts{masters: 1}, []controlplanev1alpha1.ControlPlaneOperation{op})
		require.Empty(t, a.approveQueue)
		require.Equal(t, 1, a.approveChain.components[controlplanev1alpha1.OperationComponentEtcd].approvedOperationsTotal)
	})

	t.Run("approved and completed is excluded from seed and queue", func(t *testing.T) {
		t.Parallel()
		op := newOperation("x", "n1", controlplanev1alpha1.OperationComponentEtcd, true)
		meta.SetStatusCondition(&op.Status.Conditions, metav1.Condition{
			Type:               "Completed",
			Status:             metav1.ConditionTrue,
			Reason:             controlplanev1alpha1.CPOReasonOperationCompleted,
			LastTransitionTime: metav1.Now(),
		})
		a := newApprover(nodeCounts{masters: 1}, []controlplanev1alpha1.ControlPlaneOperation{op})
		require.Empty(t, a.approveQueue)
		require.Zero(t, a.approveChain.components[controlplanev1alpha1.OperationComponentEtcd].approvedOperationsTotal)
	})

	t.Run("approveQueue sorted by pipeline stage then name", func(t *testing.T) {
		t.Parallel()
		api := newOperation("aaa-apiserver", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false)
		etcd := newOperation("zzz-etcd", "n2", controlplanev1alpha1.OperationComponentEtcd, false)
		kcm := newOperation("m-kcm", "n1", controlplanev1alpha1.OperationComponentKubeControllerManager, false)
		a := newApprover(nodeCounts{masters: 1}, []controlplanev1alpha1.ControlPlaneOperation{api, etcd, kcm})
		require.Equal(t, []string{"zzz-etcd", "aaa-apiserver", "m-kcm"}, []string{
			a.approveQueue[0].Name, a.approveQueue[1].Name, a.approveQueue[2].Name,
		})
	})
}

func newOperation(name, node string, component controlplanev1alpha1.OperationComponent, approved bool) controlplanev1alpha1.ControlPlaneOperation {
	return controlplanev1alpha1.ControlPlaneOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				constants.ControlPlaneNodeNameLabelKey: node,
			},
		},
		Spec: controlplanev1alpha1.ControlPlaneOperationSpec{
			NodeName:  node,
			Component: component,
			Approved:  approved,
		},
	}
}

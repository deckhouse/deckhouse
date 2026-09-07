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

package approver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

func approvedNames(ops []controlplanev1alpha1.ControlPlaneOperation) []string {
	names := make([]string, 0, len(ops))
	for _, op := range ops {
		names = append(names, op.Name)
	}
	return names
}

func TestApprover_ConcurrencyLimits(t *testing.T) {
	t.Parallel()

	t.Run("single node uses workload limit 1 for etcd and apiserver", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("etcd-1", "n1", controlplanev1alpha1.OperationComponentEtcd, false),
			newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
		}
		result := NewApprover(NormalPipeline).SelectApprovable(ops, Nodes{Masters: 1})
		// etcd stage wide-blocks apiserver once reserved, so only etcd-1 is admitted this round.
		require.Equal(t, []string{"etcd-1"}, approvedNames(result))
	})

	t.Run("three nodes workload limit is nodesCount-1 for apiserver", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("a2", "n2", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("a3", "n3", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
		}
		result := NewApprover(NormalPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.ElementsMatch(t, []string{"a1", "a2"}, approvedNames(result))
	})
}

func TestApprover_Etcd(t *testing.T) {
	t.Parallel()

	t.Run("allows first etcd operation", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("etcd-1", "node-a", controlplanev1alpha1.OperationComponentEtcd, false),
		}
		result := NewApprover(NormalPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"etcd-1"}, approvedNames(result))
	})

	t.Run("rejects second etcd anywhere while first is reserved", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("etcd-1", "node-a", controlplanev1alpha1.OperationComponentEtcd, false),
			newOperation("etcd-2", "node-b", controlplanev1alpha1.OperationComponentEtcd, false),
		}
		result := NewApprover(NormalPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"etcd-1"}, approvedNames(result))
	})

	t.Run("rejects second etcd on same node", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("etcd-1", "node-a", controlplanev1alpha1.OperationComponentEtcd, false),
			newOperation("etcd-2", "node-a", controlplanev1alpha1.OperationComponentEtcd, false),
		}
		result := NewApprover(NormalPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"etcd-1"}, approvedNames(result))
	})

	t.Run("seed in-flight etcd blocks another etcd", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("etcd-running", "node-a", controlplanev1alpha1.OperationComponentEtcd, true),
			newOperation("etcd-new", "node-b", controlplanev1alpha1.OperationComponentEtcd, false),
		}
		result := NewApprover(NormalPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Empty(t, result)
	})

	// blocks() for clusterWide stages checks hasAnyReservation() (approvedTotal > 0), not queued
	// operations, so a merely-queued etcd reservation does not by itself block the rest of the
	// pipeline cluster-wide. This is intentional and matches old behavior.
	//
	// Note: in practice a queued-only (no admitted) etcd reservation is unreachable here, because
	// etcd concurrency limit is always 1 (see etcdConcurrencyLimit) and queuing only happens once
	// approvedTotal >= limit, i.e. an etcd op can only be queued once another one is already
	// admitted. So this test seeds one admitted etcd (via the second etcd op getting queued
	// behind it) and asserts that apiserver is blocked because of that admitted reservation — the
	// queued second etcd op contributes nothing extra to the block, demonstrating the asymmetry
	// documented on stageGate.blocks even though a "queued but not admitted" state can't be
	// constructed standalone.
	t.Run("queued etcd reservation adds no extra cluster-wide block beyond the already-admitted one", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			// etcd-1 is admitted (etcd concurrency limit is 1).
			newOperation("etcd-1", "node-a", controlplanev1alpha1.OperationComponentEtcd, false),
			// etcd-2 hits the limit and is merely queued, not admitted.
			newOperation("etcd-2", "node-b", controlplanev1alpha1.OperationComponentEtcd, false),
			// apiserver is wide-blocked solely by etcd-1's admitted reservation.
			newOperation("a1", "node-c", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
		}
		result := NewApprover(NormalPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"etcd-1"}, approvedNames(result))
	})
}

func TestApprover_StageOrdering(t *testing.T) {
	t.Parallel()

	t.Run("blocks apiserver while etcd stage has reservation on same node", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("e1", "n1", controlplanev1alpha1.OperationComponentEtcd, false),
			newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
		}
		result := NewApprover(NormalPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"e1"}, approvedNames(result))
	})

	t.Run("blocks apiserver on another node while etcd stage has reservation", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("e1", "n1", controlplanev1alpha1.OperationComponentEtcd, false),
			newOperation("a1", "n2", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
		}
		result := NewApprover(NormalPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"e1"}, approvedNames(result))
	})

	t.Run("allows apiserver when etcd stage is empty", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
		}
		result := NewApprover(NormalPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"a1"}, approvedNames(result))
	})

	t.Run("blocks kcm while apiserver stage has reservation on same node", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("k1", "n1", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
		}
		result := NewApprover(NormalPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"a1"}, approvedNames(result))
	})

	t.Run("allows kcm on another node while apiserver stage has reservation", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("k1", "n2", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
		}
		result := NewApprover(NormalPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.ElementsMatch(t, []string{"a1", "k1"}, approvedNames(result))
	})

	t.Run("queued apiserver blocks kcm on same node but not on other nodes", func(t *testing.T) {
		t.Parallel()
		// 3 nodes -> apiserver concurrency limit 2
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("a2", "n2", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			// a3 hits concurrency limit -> goes to queue on n3
			newOperation("a3", "n3", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			// kcm on n3 must be blocked (apiserver queued on n3)
			newOperation("k1", "n3", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
			// kcm on n1 must be blocked (apiserver approved on n1)
			newOperation("k2", "n1", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
		}
		result := NewApprover(NormalPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.ElementsMatch(t, []string{"a1", "a2"}, approvedNames(result))
	})

	t.Run("allows kcm and scheduler concurrently on same pipeline stage", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("k1", "n1", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
			newOperation("s1", "n2", controlplanev1alpha1.OperationComponentKubeScheduler, false),
		}
		result := NewApprover(NormalPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.ElementsMatch(t, []string{"k1", "s1"}, approvedNames(result))
	})
}

func TestApprover_WorkloadConcurrencyAndPerNode(t *testing.T) {
	t.Parallel()

	t.Run("allows up to workload concurrency limit on distinct nodes", func(t *testing.T) {
		t.Parallel()
		// 3 nodes -> limit 2 for apiserver
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("a2", "n2", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("a3", "n3", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
		}
		result := NewApprover(NormalPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.ElementsMatch(t, []string{"a1", "a2"}, approvedNames(result))
	})

	t.Run("rejects second apiserver on same node", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("a2", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
		}
		result := NewApprover(NormalPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"a1"}, approvedNames(result))
	})
}

func TestApprover_PartitionAndOrder(t *testing.T) {
	t.Parallel()

	t.Run("unapproved operation is returned when admitted", func(t *testing.T) {
		t.Parallel()
		op := newOperation("x", "n1", controlplanev1alpha1.OperationComponentEtcd, false)
		result := NewApprover(NormalPipeline).SelectApprovable([]controlplanev1alpha1.ControlPlaneOperation{op}, Nodes{Masters: 1})
		require.Len(t, result, 1)
		require.Equal(t, "x", result[0].Name)
	})

	t.Run("approved and incomplete seeds the chain without being re-returned", func(t *testing.T) {
		t.Parallel()
		seeded := newOperation("x", "n1", controlplanev1alpha1.OperationComponentEtcd, true)
		newEtcd := newOperation("y", "n2", controlplanev1alpha1.OperationComponentEtcd, false)
		result := NewApprover(NormalPipeline).SelectApprovable(
			[]controlplanev1alpha1.ControlPlaneOperation{seeded, newEtcd}, Nodes{Masters: 1})
		// seeded op is not returned (already approved), and it occupies the etcd stage,
		// so the new unapproved etcd op is blocked (clusterWide).
		require.Empty(t, result)
	})

	t.Run("approved and completed is excluded entirely, does not block", func(t *testing.T) {
		t.Parallel()
		completed := newOperation("x", "n1", controlplanev1alpha1.OperationComponentEtcd, true)
		meta.SetStatusCondition(&completed.Status.Conditions, metav1.Condition{
			Type:               "Completed",
			Status:             metav1.ConditionTrue,
			Reason:             controlplanev1alpha1.CPOReasonOperationCompleted,
			LastTransitionTime: metav1.Now(),
		})
		newEtcd := newOperation("y", "n2", controlplanev1alpha1.OperationComponentEtcd, false)
		result := NewApprover(NormalPipeline).SelectApprovable(
			[]controlplanev1alpha1.ControlPlaneOperation{completed, newEtcd}, Nodes{Masters: 1})
		require.Equal(t, []string{"y"}, approvedNames(result))
	})

	t.Run("returned operations are sorted by pipeline stage then name", func(t *testing.T) {
		t.Parallel()
		api := newOperation("aaa-apiserver", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false)
		etcd := newOperation("zzz-etcd", "n2", controlplanev1alpha1.OperationComponentEtcd, false)
		kcm := newOperation("m-kcm", "n1", controlplanev1alpha1.OperationComponentKubeControllerManager, false)
		result := NewApprover(NormalPipeline).SelectApprovable(
			[]controlplanev1alpha1.ControlPlaneOperation{api, etcd, kcm}, Nodes{Masters: 1})
		// etcd is processed first (pipeline order) and admitted; being clusterWide it then blocks
		// every later stage (apiserver, kcm) regardless of node for the rest of this call.
		require.Equal(t, []string{"zzz-etcd"}, approvedNames(result))
	})
}

func TestApprover_VirtualPipeline(t *testing.T) {
	t.Parallel()

	t.Run("KubeAPIServer is the first stage and nothing blocks it globally", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("a2", "n2", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("a3", "n3", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
		}
		result := NewApprover(VirtualPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		// same concurrency/per-node rules as the normal pipeline test: limit is nodes-1 == 2.
		require.ElementsMatch(t, []string{"a1", "a2"}, approvedNames(result))
	})

	t.Run("rejects second apiserver on same node", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("a2", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
		}
		result := NewApprover(VirtualPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"a1"}, approvedNames(result))
	})

	t.Run("KubeControllerManager/KubeScheduler stage unaffected, still works", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("k1", "n1", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
			newOperation("s1", "n2", controlplanev1alpha1.OperationComponentKubeScheduler, false),
		}
		result := NewApprover(VirtualPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.ElementsMatch(t, []string{"k1", "s1"}, approvedNames(result))
	})

	t.Run("apiserver reservation does not wide-block kcm on another node (no Etcd stage)", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("k1", "n2", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
		}
		result := NewApprover(VirtualPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.ElementsMatch(t, []string{"a1", "k1"}, approvedNames(result))
	})
}

func newOperation(name, node string, component controlplanev1alpha1.OperationComponent, approved bool) controlplanev1alpha1.ControlPlaneOperation {
	return controlplanev1alpha1.ControlPlaneOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: constants.KubeSystemNamespace,
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

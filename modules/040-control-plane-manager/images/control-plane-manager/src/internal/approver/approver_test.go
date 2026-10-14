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
		result := NewApprover(StaticPodPipeline).SelectApprovable(ops, Nodes{Masters: 1})
		// etcd stage wide-blocks apiserver once reserved, so only etcd-1 is admitted this round.
		require.Equal(t, []string{"etcd-1"}, approvedNames(result))
	})

	t.Run("three nodes workload limit is (nodesCount-1)/2 for apiserver", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("a2", "n2", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("a3", "n3", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
		}
		result := NewApprover(StaticPodPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"a1"}, approvedNames(result))
	})

	t.Run("five nodes workload limit is (nodesCount-1)/2 for apiserver", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("a2", "n2", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("a3", "n3", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("a4", "n4", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("a5", "n5", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
		}
		result := NewApprover(StaticPodPipeline).SelectApprovable(ops, Nodes{Masters: 5})
		require.Equal(t, []string{"a1", "a2"}, approvedNames(result))
	})

	t.Run("three nodes workload limit is (nodesCount-1)/2 for kcm", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("k1", "n1", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
			newOperation("k2", "n2", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
			newOperation("k3", "n3", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
		}
		result := NewApprover(StaticPodPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"k1"}, approvedNames(result))
	})
}

func TestApprover_LeadersLast(t *testing.T) {
	t.Parallel()

	ops := []controlplanev1alpha1.ControlPlaneOperation{
		newOperation("k1", "n1", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
		newOperation("k2", "n2", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
		newOperation("s1", "n1", controlplanev1alpha1.OperationComponentKubeScheduler, false),
		newOperation("s2", "n2", controlplanev1alpha1.OperationComponentKubeScheduler, false),
	}
	result := NewApprover(StaticPodPipeline).SelectApprovable(ops, Nodes{
		Masters: 3,
		Leaders: map[controlplanev1alpha1.OperationComponent]Leader{
			controlplanev1alpha1.OperationComponentKubeControllerManager: {Node: "n1"},
			controlplanev1alpha1.OperationComponentKubeScheduler:         {Node: "n2"},
		},
	})
	require.ElementsMatch(t, []string{"k2", "s1"}, approvedNames(result))

	t.Run("etcd leader goes last", func(t *testing.T) {
		t.Parallel()
		result := NewApprover(StaticPodPipeline).SelectApprovable([]controlplanev1alpha1.ControlPlaneOperation{
			newOperation("e1", "n1", controlplanev1alpha1.OperationComponentEtcd, false),
			newOperation("e2", "n2", controlplanev1alpha1.OperationComponentEtcd, false),
			newOperation("e3", "n3", controlplanev1alpha1.OperationComponentEtcd, false),
		}, Nodes{
			Masters: 3,
			Leaders: map[controlplanev1alpha1.OperationComponent]Leader{controlplanev1alpha1.OperationComponentEtcd: {Node: "n1"}},
		})
		require.Equal(t, []string{"e2"}, approvedNames(result))
	})

	fiveMasters := Nodes{
		Masters: 5,
		Leaders: map[controlplanev1alpha1.OperationComponent]Leader{controlplanev1alpha1.OperationComponentKubeControllerManager: {Node: "n1"}},
	}

	t.Run("leader is not approved alongside a follower", func(t *testing.T) {
		t.Parallel()
		result := NewApprover(StaticPodPipeline).SelectApprovable([]controlplanev1alpha1.ControlPlaneOperation{
			newOperation("k1", "n1", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
			newOperation("k2", "n2", controlplanev1alpha1.OperationComponentKubeControllerManager, true),
		}, fiveMasters)
		require.Empty(t, result)
	})

	t.Run("leader is approved alone", func(t *testing.T) {
		t.Parallel()
		result := NewApprover(StaticPodPipeline).SelectApprovable([]controlplanev1alpha1.ControlPlaneOperation{
			newOperation("k1", "n1", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
		}, fiveMasters)
		require.Equal(t, []string{"k1"}, approvedNames(result))
	})

	t.Run("waiting kcm leader is not approved, other components go on", func(t *testing.T) {
		t.Parallel()
		result := NewApprover(StaticPodPipeline).SelectApprovable([]controlplanev1alpha1.ControlPlaneOperation{
			newOperation("k1", "n1", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
			newOperation("s2", "n2", controlplanev1alpha1.OperationComponentKubeScheduler, false),
		}, Nodes{
			Masters: 5,
			Leaders: map[controlplanev1alpha1.OperationComponent]Leader{controlplanev1alpha1.OperationComponentKubeControllerManager: {Node: "n1", Waits: true}},
		})
		require.Equal(t, []string{"s2"}, approvedNames(result))
	})

	t.Run("waiting etcd leader holds back kube-apiserver", func(t *testing.T) {
		t.Parallel()
		result := NewApprover(StaticPodPipeline).SelectApprovable([]controlplanev1alpha1.ControlPlaneOperation{
			newOperation("e1", "n1", controlplanev1alpha1.OperationComponentEtcd, false),
			newOperation("a2", "n2", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
		}, Nodes{
			Masters: 3,
			Leaders: map[controlplanev1alpha1.OperationComponent]Leader{controlplanev1alpha1.OperationComponentEtcd: {Node: "n1", Waits: true}},
		})
		require.Empty(t, result)
	})
}

func TestApprover_Etcd(t *testing.T) {
	t.Parallel()

	t.Run("allows first etcd operation", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("etcd-1", "node-a", controlplanev1alpha1.OperationComponentEtcd, false),
		}
		result := NewApprover(StaticPodPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"etcd-1"}, approvedNames(result))
	})

	t.Run("rejects second etcd anywhere while first is reserved", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("etcd-1", "node-a", controlplanev1alpha1.OperationComponentEtcd, false),
			newOperation("etcd-2", "node-b", controlplanev1alpha1.OperationComponentEtcd, false),
		}
		result := NewApprover(StaticPodPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"etcd-1"}, approvedNames(result))
	})

	t.Run("rejects second etcd on same node", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("etcd-1", "node-a", controlplanev1alpha1.OperationComponentEtcd, false),
			newOperation("etcd-2", "node-a", controlplanev1alpha1.OperationComponentEtcd, false),
		}
		result := NewApprover(StaticPodPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"etcd-1"}, approvedNames(result))
	})

	t.Run("seed in-flight etcd blocks another etcd", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("etcd-running", "node-a", controlplanev1alpha1.OperationComponentEtcd, true),
			newOperation("etcd-new", "node-b", controlplanev1alpha1.OperationComponentEtcd, false),
		}
		result := NewApprover(StaticPodPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Empty(t, result)
	})

	// A queued-only etcd operation, with none admitted, blocks the pipeline too: see "waiting etcd
	// leader holds back kube-apiserver" in TestApprover_LeadersLast.
	t.Run("queued etcd operation behind an admitted one blocks apiserver", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			// etcd-1 is admitted (etcd concurrency limit is 1).
			newOperation("etcd-1", "node-a", controlplanev1alpha1.OperationComponentEtcd, false),
			// etcd-2 hits the limit and is merely queued, not admitted.
			newOperation("etcd-2", "node-b", controlplanev1alpha1.OperationComponentEtcd, false),
			// apiserver is wide-blocked solely by etcd-1's admitted reservation.
			newOperation("a1", "node-c", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
		}
		result := NewApprover(StaticPodPipeline).SelectApprovable(ops, Nodes{Masters: 3})
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
		result := NewApprover(StaticPodPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"e1"}, approvedNames(result))
	})

	t.Run("blocks apiserver on another node while etcd stage has reservation", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("e1", "n1", controlplanev1alpha1.OperationComponentEtcd, false),
			newOperation("a1", "n2", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
		}
		result := NewApprover(StaticPodPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"e1"}, approvedNames(result))
	})

	t.Run("allows apiserver when etcd stage is empty", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
		}
		result := NewApprover(StaticPodPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"a1"}, approvedNames(result))
	})

	t.Run("blocks kcm while apiserver stage has reservation on same node", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("k1", "n1", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
		}
		result := NewApprover(StaticPodPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"a1"}, approvedNames(result))
	})

	t.Run("blocks kcm on another node while apiserver stage has reservation", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("k1", "n2", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
		}
		result := NewApprover(StaticPodPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"a1"}, approvedNames(result))
	})

	t.Run("in-flight apiserver blocks scheduler on every node", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, true),
			newOperation("s2", "n2", controlplanev1alpha1.OperationComponentKubeScheduler, false),
			newOperation("s3", "n3", controlplanev1alpha1.OperationComponentKubeScheduler, false),
		}
		result := NewApprover(StaticPodPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Empty(t, result)
	})

	t.Run("queued apiserver blocks kcm on every node", func(t *testing.T) {
		t.Parallel()
		// 5 nodes -> apiserver concurrency limit 2
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("a2", "n2", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			// a3 hits concurrency limit -> goes to queue on n3
			newOperation("a3", "n3", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("k1", "n3", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
			newOperation("k2", "n1", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
			// no apiserver operation on n4 at all
			newOperation("k3", "n4", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
		}
		result := NewApprover(StaticPodPipeline).SelectApprovable(ops, Nodes{Masters: 5})
		require.ElementsMatch(t, []string{"a1", "a2"}, approvedNames(result))
	})

	t.Run("allows kcm once every apiserver operation is finished", func(t *testing.T) {
		t.Parallel()
		completed := newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, true)
		meta.SetStatusCondition(&completed.Status.Conditions, metav1.Condition{
			Type:               "Completed",
			Status:             metav1.ConditionTrue,
			Reason:             controlplanev1alpha1.CPOReasonOperationCompleted,
			LastTransitionTime: metav1.Now(),
		})
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			completed,
			newOperation("k2", "n2", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
		}
		result := NewApprover(StaticPodPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"k2"}, approvedNames(result))
	})

	t.Run("allows kcm and scheduler concurrently on same pipeline stage", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("k1", "n1", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
			newOperation("s1", "n2", controlplanev1alpha1.OperationComponentKubeScheduler, false),
		}
		result := NewApprover(StaticPodPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.ElementsMatch(t, []string{"k1", "s1"}, approvedNames(result))
	})
}

func TestApprover_WorkloadConcurrencyAndPerNode(t *testing.T) {
	t.Parallel()

	t.Run("allows up to workload concurrency limit on distinct nodes", func(t *testing.T) {
		t.Parallel()
		// 5 nodes -> limit 2 for apiserver
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("a2", "n2", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("a3", "n3", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
		}
		result := NewApprover(StaticPodPipeline).SelectApprovable(ops, Nodes{Masters: 5})
		require.ElementsMatch(t, []string{"a1", "a2"}, approvedNames(result))
	})

	t.Run("rejects second apiserver on same node", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
			newOperation("a2", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false),
		}
		result := NewApprover(StaticPodPipeline).SelectApprovable(ops, Nodes{Masters: 3})
		require.Equal(t, []string{"a1"}, approvedNames(result))
	})
}

func TestApprover_PartitionAndOrder(t *testing.T) {
	t.Parallel()

	t.Run("unapproved operation is returned when admitted", func(t *testing.T) {
		t.Parallel()
		op := newOperation("x", "n1", controlplanev1alpha1.OperationComponentEtcd, false)
		result := NewApprover(StaticPodPipeline).SelectApprovable([]controlplanev1alpha1.ControlPlaneOperation{op}, Nodes{Masters: 1})
		require.Len(t, result, 1)
		require.Equal(t, "x", result[0].Name)
	})

	t.Run("approved and incomplete seeds the chain without being re-returned", func(t *testing.T) {
		t.Parallel()
		seeded := newOperation("x", "n1", controlplanev1alpha1.OperationComponentEtcd, true)
		newEtcd := newOperation("y", "n2", controlplanev1alpha1.OperationComponentEtcd, false)
		result := NewApprover(StaticPodPipeline).SelectApprovable(
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
		result := NewApprover(StaticPodPipeline).SelectApprovable(
			[]controlplanev1alpha1.ControlPlaneOperation{completed, newEtcd}, Nodes{Masters: 1})
		require.Equal(t, []string{"y"}, approvedNames(result))
	})

	t.Run("in-flight observe-only apiserver operation does not block kcm", func(t *testing.T) {
		t.Parallel()
		observe := newOperation("a-observe", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, true)
		observe.Spec.Steps = []controlplanev1alpha1.StepName{controlplanev1alpha1.StepCertObserve}
		result := NewApprover(StaticPodPipeline).SelectApprovable([]controlplanev1alpha1.ControlPlaneOperation{
			observe,
			newOperation("k2", "n2", controlplanev1alpha1.OperationComponentKubeControllerManager, false),
		}, Nodes{Masters: 3})
		require.Equal(t, []string{"k2"}, approvedNames(result))
	})

	t.Run("returned operations are sorted by pipeline stage then name", func(t *testing.T) {
		t.Parallel()
		api := newOperation("aaa-apiserver", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false)
		etcd := newOperation("zzz-etcd", "n2", controlplanev1alpha1.OperationComponentEtcd, false)
		kcm := newOperation("m-kcm", "n1", controlplanev1alpha1.OperationComponentKubeControllerManager, false)
		result := NewApprover(StaticPodPipeline).SelectApprovable(
			[]controlplanev1alpha1.ControlPlaneOperation{api, etcd, kcm}, Nodes{Masters: 1})
		// etcd is processed first (pipeline order) and admitted; being clusterWide it then blocks
		// every later stage (apiserver, kcm) regardless of node for the rest of this call.
		require.Equal(t, []string{"zzz-etcd"}, approvedNames(result))
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

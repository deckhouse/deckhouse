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

package nodeconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// TestNeedDrain: a group of one is interrupted without a drain (nowhere for the
// workload to go), but status.nodes == 0 means "not counted yet", not "one".
func TestNeedDrain(t *testing.T) {
	tests := []struct {
		name string
		ng   *v1.NodeGroup
		want bool
	}{
		{
			name: "a group of one has nowhere to drain to",
			ng:   nodeGroupWithNodes(1),
			want: false,
		},
		{
			name: "a group whose nodes have not been counted yet is drained",
			ng:   nodeGroupWithNodes(0),
			want: true,
		},
		{
			name: "a group of many is drained",
			ng:   nodeGroupWithNodes(50),
			want: true,
		},
		{
			name: "the operator can turn the drain off",
			ng: func() *v1.NodeGroup {
				ng := nodeGroupWithNodes(50)
				ng.Spec.Disruptions = &v1.DisruptionsSpec{
					Automatic: &v1.AutomaticDisruptionSpec{DrainBeforeApproval: ptr.To(false)},
				}
				return ng
			}(),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, needDrain(tt.ng))
		})
	}
}

// An operation is owned by the node and outlives the NodeConfig by a day, so a
// recreated config — counting from generation 1 again — matched the completed
// operation of its predecessor and waited for a disruption nobody would carry
// out, holding a rollout slot of its group until the operation was collected.
func TestFindApprovalIgnoresAnOperationOfAnEarlierNodeConfig(t *testing.T) {
	const previousUID, recreatedUID = "5f5d1c7e-0000-4000-8000-000000000001", "5f5d1c7e-0000-4000-8000-000000000002"

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	done := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "approve-worker-0-abcde",
			Labels: map[string]string{
				v1alpha1.NodeOperationNodeLabel: "worker-0",
				nodeConfigUIDLabel:              previousUID,
			},
		},
		Spec: v1alpha1.NodeOperationSpec{
			Type:             v1alpha1.NodeOperationTypeApproveDisruption,
			NodeName:         "worker-0",
			ConfigGeneration: ptr.To(int64(2)),
		},
		Status: v1alpha1.NodeOperationStatus{Phase: v1alpha1.NodeOperationPhaseCompleted},
	}
	r := &Reconciler{sources: &sourceReader{Reader: fake.NewClientBuilder().WithScheme(scheme).WithObjects(done).Build()}}

	recreated, err := r.findApproval(t.Context(), nodeConfigOf("worker-0", recreatedUID, 2))
	require.NoError(t, err)
	require.Nil(t, recreated, "the operation was carried out for an object that no longer exists")

	previous, err := r.findApproval(t.Context(), nodeConfigOf("worker-0", previousUID, 2))
	require.NoError(t, err)
	require.NotNil(t, previous, "the object it was issued for must still find it, or it is approved twice")
}

// A NodeOperation fails on a deadline or on a pod nothing can evict. The node
// keeps asking, so a failed attempt must not be mistaken for one in flight:
// that leaves the node holding a rollout slot for a disruption nobody will
// carry out.
func TestFindApprovalAllowsAFreshAttemptAfterAFailure(t *testing.T) {
	const uid = "5f5d1c7e-0000-4000-8000-000000000003"

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	failed := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "approve-worker-0-fghij",
			Labels: map[string]string{
				v1alpha1.NodeOperationNodeLabel: "worker-0",
				nodeConfigUIDLabel:              uid,
			},
		},
		Spec: v1alpha1.NodeOperationSpec{
			Type:             v1alpha1.NodeOperationTypeApproveDisruption,
			NodeName:         "worker-0",
			ConfigGeneration: ptr.To(int64(2)),
		},
		Status: v1alpha1.NodeOperationStatus{Phase: v1alpha1.NodeOperationPhaseFailed},
	}
	r := &Reconciler{sources: &sourceReader{Reader: fake.NewClientBuilder().WithScheme(scheme).WithObjects(failed).Build()}}

	found, err := r.findApproval(t.Context(), nodeConfigOf("worker-0", uid, 2))
	require.NoError(t, err)
	require.Nil(t, found, "a failed operation must not stand in for one in flight")
}

func nodeConfigOf(name, uid string, generation int64) *internalv1alpha1.NodeConfig {
	return &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name, UID: types.UID(uid), Generation: generation},
	}
}

func nodeGroupWithNodes(count int32) *v1.NodeGroup {
	ng := &v1.NodeGroup{}
	ng.Name = "worker"
	ng.Status.Nodes = count
	return ng
}

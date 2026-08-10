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
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// nodeConfigWith builds one node's config: what it was asked to install, and
// what it reports back.
func nodeConfigWith(name string, spec []internalv1alpha1.Extension, status []internalv1alpha1.ExtensionStatus) *internalv1alpha1.NodeConfig {
	return &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       internalv1alpha1.NodeSpec{NodeName: name, Extensions: spec},
		Status:     internalv1alpha1.NodeConfigStatus{Extensions: status},
	}
}

// The whole point of reading the nodes back: a request that resolves here can
// still be refused by every node it reaches, and until this was counted the two
// looked identical from the cluster.
func TestNEROutcomesCountWhatTheNodesReport(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, internalv1alpha1.AddToScheme(scheme))

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		// Two nodes refuse bob and accept bob-signed.
		nodeConfigWith("master-0",
			[]internalv1alpha1.Extension{
				{Name: "bob", RequestedBy: nerRequestedByPrefix + "bob-request"},
				{Name: "bob-signed", RequestedBy: nerRequestedByPrefix + "bob-signed-request"},
			},
			[]internalv1alpha1.ExtensionStatus{
				{Name: "bob", State: "Failed", Message: "Required key not available"},
				{Name: "bob-signed", State: "Ready"},
			}),
		nodeConfigWith("worker-0",
			[]internalv1alpha1.Extension{
				{Name: "bob", RequestedBy: nerRequestedByPrefix + "bob-request"},
				{Name: "bob-signed", RequestedBy: nerRequestedByPrefix + "bob-signed-request"},
			},
			[]internalv1alpha1.ExtensionStatus{
				{Name: "bob", State: "Failed", Message: "Required key not available"},
				{Name: "bob-signed", State: "Ready"},
			}),
		// A node that has not answered yet counts as neither.
		nodeConfigWith("worker-1",
			[]internalv1alpha1.Extension{{Name: "bob", RequestedBy: nerRequestedByPrefix + "bob-request"}},
			nil),
		// A platform extension: node-manager asked for it, no NER owns it.
		nodeConfigWith("worker-2",
			[]internalv1alpha1.Extension{{Name: "containerd", RequestedBy: "node-manager"}},
			[]internalv1alpha1.ExtensionStatus{{Name: "containerd", State: "Ready"}}),
	).Build()

	outcomes, err := readNEROutcomes(context.Background(), client)
	require.NoError(t, err)

	require.Equal(t, int32(2), outcomes["bob-request"].failed)
	require.Equal(t, int32(0), outcomes["bob-request"].applied)
	require.Equal(t, "Required key not available", outcomes["bob-request"].message,
		"the reason must reach the cluster, so nobody has to read a node's journal for it")

	require.Equal(t, int32(2), outcomes["bob-signed-request"].applied)
	require.Equal(t, int32(0), outcomes["bob-signed-request"].failed)
	require.Empty(t, outcomes["bob-signed-request"].message)

	// requestedBy is the join key, and "node-manager" is not a request.
	require.NotContains(t, outcomes, "node-manager")
}

// A node reporting Pending is still working; counting it either way would be an
// answer it has not given.
func TestNEROutcomesIgnorePendingNodes(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, internalv1alpha1.AddToScheme(scheme))

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		nodeConfigWith("worker-0",
			[]internalv1alpha1.Extension{{Name: "bob", RequestedBy: nerRequestedByPrefix + "bob-request"}},
			[]internalv1alpha1.ExtensionStatus{{Name: "bob", State: "Pending"}}),
	).Build()

	outcomes, err := readNEROutcomes(context.Background(), client)
	require.NoError(t, err)
	require.Equal(t, int32(0), outcomes["bob-request"].applied)
	require.Equal(t, int32(0), outcomes["bob-request"].failed)
}


// A node that rejects the configuration wholesale keeps running the last one
// it accepted, so the refused extension's Failed entry is gone from its status
// by the next pass — measured on a live cluster: the NER read Ready while the
// node sat Degraded over it. The durable trace of the refusal is the
// ConfigurationApplied condition, and the count has to come from there.
func TestNEROutcomesCountWholesaleRejections(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, internalv1alpha1.AddToScheme(scheme))

	rejected := nodeConfigWith("master-0",
		[]internalv1alpha1.Extension{
			{Name: "containerd", RequestedBy: "node-manager"},
			{Name: "bob-signed", RequestedBy: nerRequestedByPrefix + "bob-signed-request"},
			{Name: "bob", RequestedBy: nerRequestedByPrefix + "bob-request"},
		},
		// Last-known-good statuses: bob-signed made it in an earlier
		// generation, bob never appears.
		[]internalv1alpha1.ExtensionStatus{
			{Name: "containerd", State: "Ready"},
			{Name: "bob-signed", State: "Ready"},
		})
	rejected.Status.Conditions = []metav1.Condition{{
		Type:    configurationAppliedCondition,
		Status:  metav1.ConditionFalse,
		Reason:  "Rejected",
		Message: "bob: config rejected: its roothash signature is not one this node trusts",
	}}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(rejected).Build()

	outcomes, err := readNEROutcomes(context.Background(), client)
	require.NoError(t, err)

	require.Equal(t, int32(1), outcomes["bob-request"].failed)
	require.Equal(t, int32(0), outcomes["bob-request"].applied)
	require.Contains(t, outcomes["bob-request"].message, "not one this node trusts")

	// The extension that did make it is not blamed for the rejection.
	require.Equal(t, int32(1), outcomes["bob-signed-request"].applied)
	require.Equal(t, int32(0), outcomes["bob-signed-request"].failed)
}

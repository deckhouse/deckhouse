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

package v1alpha2

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testAddress   = "192.168.199.10"
	testNodeGroup = "master"
)

// adoptableNode returns a Node that passes every pre-adoption check, so that a test case can
// break exactly the one it is about.
func adoptableNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{NodeGroupLabel: testNodeGroup},
		},
		Spec: corev1.NodeSpec{ProviderID: "static://"},
		Status: corev1.NodeStatus{
			Addresses:  []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: testAddress}},
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
		},
	}
}

func nodeWithAddresses(name string, addresses ...string) *corev1.Node {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}}
	for _, address := range addresses {
		node.Status.Addresses = append(node.Status.Addresses, corev1.NodeAddress{
			Type:    corev1.NodeInternalIP,
			Address: address,
		})
	}

	return node
}

func fakeClientWithNodes(t *testing.T, nodes ...*corev1.Node) client.Reader {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	builder := fake.NewClientBuilder().WithScheme(scheme)
	for _, node := range nodes {
		builder = builder.WithObjects(node)
	}

	return builder.Build()
}

func staticInstanceWithAnnotations(address string, annotations map[string]string) *StaticInstance {
	return &StaticInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "master-1", Annotations: annotations},
		Spec:       StaticInstanceSpec{Address: address},
	}
}

func TestStaticInstance_ResolveAdoption(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		annotations map[string]string
		labels      map[string]string
		address     string
		nodes       []*corev1.Node
		action      AdoptAction
		reason      string
	}{
		"no annotations, node exists: bootstrap": {
			annotations: nil,
			address:     testAddress,
			nodes:       []*corev1.Node{adoptableNode("existing")},
			action:      AdoptActionBootstrap,
			reason:      AdoptReasonNotRequested,
		},
		"adopt-if-node-exists, adoptable node exists: adopt": {
			annotations: map[string]string{AdoptIfNodeExistsAnnotation: ""},
			address:     testAddress,
			nodes:       []*corev1.Node{adoptableNode("existing")},
			action:      AdoptActionAdopt,
			reason:      AdoptReasonNodeIsAdoptable,
		},
		"adopt-if-node-exists, node is not ready: wait": {
			annotations: map[string]string{AdoptIfNodeExistsAnnotation: ""},
			address:     testAddress,
			nodes:       []*corev1.Node{notReadyNode("existing")},
			action:      AdoptActionWait,
			reason:      AdoptReasonNodeIsNotReady,
		},
		"adopt-if-node-exists, node is in another node group: reject": {
			annotations: map[string]string{AdoptIfNodeExistsAnnotation: ""},
			address:     testAddress,
			nodes:       []*corev1.Node{nodeOfAnotherGroup("existing")},
			action:      AdoptActionReject,
			reason:      AdoptReasonNodeInOtherNodeGroup,
		},
		"adopt-if-node-exists, no node with that address: bootstrap": {
			annotations: map[string]string{AdoptIfNodeExistsAnnotation: ""},
			address:     testAddress,
			nodes:       []*corev1.Node{nodeWithAddresses("other", "192.168.199.11")},
			action:      AdoptActionBootstrap,
			reason:      AdoptReasonNoNodeWithAddress,
		},
		"adopt-if-node-exists, no nodes at all: bootstrap": {
			annotations: map[string]string{AdoptIfNodeExistsAnnotation: ""},
			address:     testAddress,
			nodes:       nil,
			action:      AdoptActionBootstrap,
			reason:      AdoptReasonNoNodeWithAddress,
		},
		"adopt-if-node-exists, empty address: bootstrap": {
			annotations: map[string]string{AdoptIfNodeExistsAnnotation: ""},
			address:     "",
			nodes:       []*corev1.Node{adoptableNode("existing")},
			action:      AdoptActionBootstrap,
			reason:      AdoptReasonNoNodeWithAddress,
		},
		"skip-bootstrap-phase without any node: adopt unconditionally": {
			annotations: map[string]string{SkipBootstrapPhaseAnnotation: ""},
			address:     testAddress,
			nodes:       nil,
			action:      AdoptActionAdopt,
			reason:      AdoptReasonRequestedImperatively,
		},
		"skip-bootstrap-phase is not held back by an unfit node": {
			annotations: map[string]string{SkipBootstrapPhaseAnnotation: ""},
			address:     testAddress,
			nodes:       []*corev1.Node{notReadyNode("existing")},
			action:      AdoptActionAdopt,
			reason:      AdoptReasonRequestedImperatively,
		},
		"unrelated annotation only: bootstrap": {
			annotations: map[string]string{"example.com/unrelated": "true"},
			address:     testAddress,
			nodes:       []*corev1.Node{adoptableNode("existing")},
			action:      AdoptActionBootstrap,
			reason:      AdoptReasonNotRequested,
		},
		"adopt marker set as a label instead of an annotation: bootstrap": {
			annotations: nil,
			labels:      map[string]string{AdoptIfNodeExistsAnnotation: ""},
			address:     testAddress,
			nodes:       []*corev1.Node{adoptableNode("existing")},
			action:      AdoptActionBootstrap,
			reason:      AdoptReasonNotRequested,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			instance := staticInstanceWithAnnotations(tt.address, tt.annotations)
			instance.Labels = tt.labels

			decision, err := instance.ResolveAdoption(t.Context(), fakeClientWithNodes(t, tt.nodes...), testNodeGroup)

			require.NoError(t, err)
			require.Equal(t, tt.action, decision.Action)
			require.Equal(t, tt.reason, decision.Reason)
			require.NotEmpty(t, decision.Message)
		})
	}
}

func notReadyNode(name string) *corev1.Node {
	node := adoptableNode(name)
	node.Status.Conditions[0].Status = corev1.ConditionFalse

	return node
}

func nodeOfAnotherGroup(name string) *corev1.Node {
	node := adoptableNode(name)
	node.Labels[NodeGroupLabel] = "another-group"

	return node
}

func TestStaticInstance_FindNodeWithSameAddress(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		address  string
		nodes    []*corev1.Node
		expected string
	}{
		"matching internal ip": {
			address:  testAddress,
			nodes:    []*corev1.Node{nodeWithAddresses("matching", testAddress)},
			expected: "matching",
		},
		"matches among several addresses of one node": {
			address:  testAddress,
			nodes:    []*corev1.Node{nodeWithAddresses("matching", "10.0.0.1", testAddress)},
			expected: "matching",
		},
		"matches the right node among several": {
			address: testAddress,
			nodes: []*corev1.Node{
				nodeWithAddresses("other", "192.168.199.11"),
				nodeWithAddresses("matching", testAddress),
			},
			expected: "matching",
		},
		"no match": {
			address:  testAddress,
			nodes:    []*corev1.Node{nodeWithAddresses("other", "192.168.199.11")},
			expected: "",
		},
		"empty address never matches": {
			address:  "",
			nodes:    []*corev1.Node{nodeWithAddresses("nodeWithoutAddresses")},
			expected: "",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			instance := staticInstanceWithAnnotations(tt.address, nil)

			node, err := instance.FindNodeWithSameAddress(t.Context(), fakeClientWithNodes(t, tt.nodes...))
			require.NoError(t, err)

			if tt.expected == "" {
				require.Nil(t, node)
			} else {
				require.NotNil(t, node)
				require.Equal(t, tt.expected, node.Name)
			}
		})
	}
}

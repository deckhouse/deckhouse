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

const testAddress = "192.168.199.10"

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

func TestStaticInstance_ShouldAdopt(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		annotations map[string]string
		address     string
		nodes       []*corev1.Node
		expected    bool
	}{
		"no annotations, node exists: bootstrap": {
			annotations: nil,
			address:     testAddress,
			nodes:       []*corev1.Node{nodeWithAddresses("existing", testAddress)},
			expected:    false,
		},
		"adopt-if-node-exists, node exists: adopt": {
			annotations: map[string]string{AdoptIfNodeExistsAnnotation: ""},
			address:     testAddress,
			nodes:       []*corev1.Node{nodeWithAddresses("existing", testAddress)},
			expected:    true,
		},
		"adopt-if-node-exists, no node with that address: bootstrap": {
			annotations: map[string]string{AdoptIfNodeExistsAnnotation: ""},
			address:     testAddress,
			nodes:       []*corev1.Node{nodeWithAddresses("other", "192.168.199.11")},
			expected:    false,
		},
		"adopt-if-node-exists, no nodes at all: bootstrap": {
			annotations: map[string]string{AdoptIfNodeExistsAnnotation: ""},
			address:     testAddress,
			nodes:       nil,
			expected:    false,
		},
		"adopt-if-node-exists, empty address: bootstrap": {
			annotations: map[string]string{AdoptIfNodeExistsAnnotation: ""},
			address:     "",
			nodes:       []*corev1.Node{nodeWithAddresses("existing", testAddress)},
			expected:    false,
		},
		"skip-bootstrap-phase without any node: adopt unconditionally": {
			annotations: map[string]string{SkipBootstrapPhaseAnnotation: ""},
			address:     testAddress,
			nodes:       nil,
			expected:    true,
		},
		"unrelated annotation only: bootstrap": {
			annotations: map[string]string{"example.com/unrelated": "true"},
			address:     testAddress,
			nodes:       []*corev1.Node{nodeWithAddresses("existing", testAddress)},
			expected:    false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			instance := staticInstanceWithAnnotations(tt.address, tt.annotations)

			shouldAdopt, err := instance.ShouldAdopt(t.Context(), fakeClientWithNodes(t, tt.nodes...))
			require.NoError(t, err)
			require.Equal(t, tt.expected, shouldAdopt)
		})
	}
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

			nodeName, err := instance.FindNodeWithSameAddress(t.Context(), fakeClientWithNodes(t, tt.nodes...))
			require.NoError(t, err)
			require.Equal(t, tt.expected, nodeName)
		})
	}
}

func TestStaticInstance_validateAddressUnlessAdopting(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		annotations map[string]string
		address     string
		nodes       []*corev1.Node
		errContains string
	}{
		"free address without annotations is allowed": {
			address: testAddress,
			nodes:   []*corev1.Node{nodeWithAddresses("other", "192.168.199.11")},
		},
		"taken address without annotations is rejected": {
			address:     testAddress,
			nodes:       []*corev1.Node{nodeWithAddresses("existing", testAddress)},
			errContains: "already exists on node",
		},
		"rejection hints at the declarative annotation": {
			address:     testAddress,
			nodes:       []*corev1.Node{nodeWithAddresses("existing", testAddress)},
			errContains: AdoptIfNodeExistsAnnotation,
		},
		"taken address is allowed with adopt-if-node-exists": {
			annotations: map[string]string{AdoptIfNodeExistsAnnotation: ""},
			address:     testAddress,
			nodes:       []*corev1.Node{nodeWithAddresses("existing", testAddress)},
		},
		"taken address is allowed with skip-bootstrap-phase": {
			annotations: map[string]string{SkipBootstrapPhaseAnnotation: ""},
			address:     testAddress,
			nodes:       []*corev1.Node{nodeWithAddresses("existing", testAddress)},
		},
		"empty address without annotations is rejected": {
			address:     "",
			nodes:       nil,
			errContains: "must not be empty",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			instance := staticInstanceWithAnnotations(tt.address, tt.annotations)

			err := instance.validateAddressUnlessAdopting(t.Context(), fakeClientWithNodes(t, tt.nodes...))

			if tt.errContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.errContains)
			}
		})
	}
}

// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package converge

import (
	"testing"

	klient "github.com/flant/kube-client/client"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

var nodeGroupGVR = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "nodegroups"}

func nodeGroup(name, systemType string) *unstructured.Unstructured {
	spec := map[string]any{"nodeType": "CloudPermanent"}
	if systemType != "" {
		spec["systemType"] = systemType
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "NodeGroup",
		"metadata":   map[string]any{"name": name},
		"spec":       spec,
	}}
}

func gateClient(t *testing.T, nodeGroups ...*unstructured.Unstructured) (*kubernetes.SimpleKubeClientGetter, *dynamicfake.FakeDynamicClient) {
	t.Helper()

	kubeCl := client.NewFakeKubernetesClientWithListGVR(
		map[schema.GroupVersionResource]string{nodeGroupGVR: "NodeGroupList"},
	)

	fakeClient, ok := kubeCl.KubeClient.(*klient.Client)
	require.True(t, ok, "fake kube client is not backed by kube-client")

	dyn, ok := fakeClient.Dynamic().(*dynamicfake.FakeDynamicClient)
	require.True(t, ok, "fake kube client is not backed by a fake dynamic client")

	for _, ng := range nodeGroups {
		_, err := dyn.Resource(nodeGroupGVR).Create(t.Context(), ng, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	return kubernetes.NewSimpleKubeClientGetter(kubeCl), dyn
}

// The control plane decides whether converge may use SSH at all: an immutable master
// answers no sshd, and the node-user switch that every other cluster needs would hang
// on a port nothing listens on.
func TestMasterGroupIsImmutable(t *testing.T) {
	t.Run("a classic control plane keeps the SSH path", func(t *testing.T) {
		kubeGetter, _ := gateClient(t, nodeGroup("master", ""), nodeGroup("worker", "Immutable"))

		immutableMasters, err := masterGroupIsImmutable(t.Context(), kubeGetter)
		require.NoError(t, err)
		require.False(t, immutableMasters)
	})

	t.Run("an immutable control plane drops it", func(t *testing.T) {
		kubeGetter, _ := gateClient(t, nodeGroup("master", "Immutable"), nodeGroup("worker", ""))

		immutableMasters, err := masterGroupIsImmutable(t.Context(), kubeGetter)
		require.NoError(t, err)
		require.True(t, immutableMasters)
	})

	// A cluster whose master group was never written keeps the path every other
	// cluster needs.
	t.Run("no master group at all", func(t *testing.T) {
		kubeGetter, _ := gateClient(t, nodeGroup("worker", "Immutable"))

		immutableMasters, err := masterGroupIsImmutable(t.Context(), kubeGetter)
		require.NoError(t, err)
		require.False(t, immutableMasters)
	})
}

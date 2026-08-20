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
	"fmt"
	"testing"

	klient "github.com/flant/kube-client/client"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
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

func TestRefuseImmutableNodeGroups(t *testing.T) {
	convergedGroups := []string{"master", "worker"}

	t.Run("all groups mutable", func(t *testing.T) {
		kubeGetter, _ := gateClient(t, nodeGroup("master", ""), nodeGroup("worker", "Mutable"))

		require.NoError(t, refuseImmutableNodeGroups(t.Context(), kubeGetter, convergedGroups))
	})

	t.Run("one group immutable", func(t *testing.T) {
		kubeGetter, _ := gateClient(t, nodeGroup("master", "Immutable"), nodeGroup("worker", ""))

		err := refuseImmutableNodeGroups(t.Context(), kubeGetter, convergedGroups)

		require.ErrorContains(t, err, "master")
		require.ErrorContains(t, err, "not supported yet")
	})

	// An immutable CloudEphemeral group is reconciled by the bootstrap provider, and
	// converge walks no such group: a bashible cluster keeps converging while its
	// ephemeral nodes migrate.
	t.Run("immutable group converge never walks", func(t *testing.T) {
		kubeGetter, _ := gateClient(t, nodeGroup("master", ""), nodeGroup("ephemeral", "Immutable"))

		require.NoError(t, refuseImmutableNodeGroups(t.Context(), kubeGetter, convergedGroups))
	})

	t.Run("unreadable node groups refuse the phase", func(t *testing.T) {
		kubeGetter, dyn := gateClient(t)
		dyn.PrependReactor("list", "nodegroups", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf("apiserver is unavailable")
		})

		require.Error(t, refuseImmutableNodeGroups(t.Context(), kubeGetter, convergedGroups))
	})
}

func TestConvergedNodeGroupNames(t *testing.T) {
	metaConfig := &config.MetaConfig{
		TerraNodeGroupSpecs: []config.TerraNodeGroupSpec{
			{Name: "worker", Replicas: 1},
			// Not in the cluster yet: converge is about to create its nodes.
			{Name: "fresh", Replicas: 2},
		},
	}
	nodesState := map[string]state.NodeGroupInfrastructureState{
		"worker": {},
		// Gone from the config but still holds state: converge deletes its nodes.
		"dropped": {},
	}

	require.Equal(t,
		[]string{"dropped", "fresh", "master", "worker"},
		convergedNodeGroupNames(metaConfig, nodesState),
	)
}

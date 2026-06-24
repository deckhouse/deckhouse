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
package gatekeeper

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	fakediscovery "k8s.io/client-go/discovery/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetMutations_V1Alpha1Only(t *testing.T) {
	obj := newMutationObject("mutations.gatekeeper.sh/v1alpha1", "AssignImage", "assign-image-v1alpha1")
	client := ctrlfake.NewClientBuilder().WithRuntimeObjects(obj).Build()
	kubeClient := k8sfake.NewSimpleClientset()

	discovery := kubeClient.Discovery().(*fakediscovery.FakeDiscovery)
	discovery.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "mutations.gatekeeper.sh/v1alpha1",
			APIResources: []metav1.APIResource{
				{Name: "assignimages", Kind: "AssignImage", Verbs: metav1.Verbs{"list"}},
			},
		},
	}

	mutations, err := GetMutations(client, kubeClient)
	require.NoError(t, err)
	require.Len(t, mutations, 1)
	require.Equal(t, "AssignImage", mutations[0].Meta.Kind)
	require.Equal(t, "assign-image-v1alpha1", mutations[0].Meta.Name)
	require.Equal(t, []MatchKind{{APIGroups: []string{""}, Kinds: []string{"Pod"}}}, mutations[0].Spec.Match.Kinds)
}

func TestGetMutations_V1Only(t *testing.T) {
	obj := newMutationObject("mutations.gatekeeper.sh/v1", "AssignImage", "assign-image-v1")
	client := ctrlfake.NewClientBuilder().WithRuntimeObjects(obj).Build()
	kubeClient := k8sfake.NewSimpleClientset()

	discovery := kubeClient.Discovery().(*fakediscovery.FakeDiscovery)
	discovery.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "mutations.gatekeeper.sh/v1",
			APIResources: []metav1.APIResource{
				{Name: "assignimages", Kind: "AssignImage", Verbs: metav1.Verbs{"list"}},
			},
		},
	}

	mutations, err := GetMutations(client, kubeClient)
	require.NoError(t, err)
	require.Len(t, mutations, 1)
	require.Equal(t, "AssignImage", mutations[0].Meta.Kind)
	require.Equal(t, "assign-image-v1", mutations[0].Meta.Name)
	require.Equal(t, []MatchKind{{APIGroups: []string{""}, Kinds: []string{"Pod"}}}, mutations[0].Spec.Match.Kinds)
}

func newMutationObject(apiVersion, kind, name string) runtime.Object {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata": map[string]interface{}{
			"name": name,
		},
		"spec": map[string]interface{}{
			"match": map[string]interface{}{
				"kinds": []interface{}{
					map[string]interface{}{
						"apiGroups": []interface{}{""},
						"kinds":     []interface{}{"Pod"},
					},
				},
			},
		},
	}}
}

/*
Copyright 2023 Flant JSC

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

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

type cluster struct {
	APIVersion        string
	Kind              string
	Name              string
	Namespace         string
	UID               string
	InfrastructureRef *corev1.ObjectReference
}

type capiCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec clusterSpec `json:"spec,omitempty"`
}

type clusterSpec struct {
	InfrastructureRef *corev1.ObjectReference `json:"infrastructureRef,omitempty"`
}

// filterDynamicProbeNodeGroups returns the name of a nodegroup to consider or emptystring if it should be skipped
func filterClusters(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var c capiCluster
	err := sdk.FromUnstructured(obj, &c)
	if err != nil {
		return nil, err
	}

	return cluster{
		APIVersion:        c.APIVersion,
		Kind:              c.Kind,
		Name:              c.Name,
		Namespace:         c.Namespace,
		UID:               string(c.UID),
		InfrastructureRef: c.Spec.InfrastructureRef,
	}, nil
}

// This hook discovers nodegroup names for dynamic probes in upmeter
var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		Queue: "/modules/node-manager",
		OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:                         "clusters",
				ApiVersion:                   "cluster.x-k8s.io/v1beta1",
				Kind:                         "Cluster",
				WaitForSynchronization:       pointer.Bool(false),
				ExecuteHookOnSynchronization: pointer.Bool(false),
				ExecuteHookOnEvents:          pointer.Bool(true),
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{"d8-cloud-instance-manager"},
					},
				},
				FilterFunc: filterClusters,
			},
		},
	},
	updateCluster,
)

// collectDynamicProbeConfig sets names of objects to internal values
func updateCluster(input *go_hook.HookInput) error {
	statusPatch := map[string]interface{}{
		"status": map[string]interface{}{
			"infrastructureReady": true,
		},
	}

	snap := input.Snapshots["clusters"]

	if len(snap) == 0 {
		return nil
	}

	if len(snap) > 1 {
		return fmt.Errorf("more than one CAPI cluster resource is found: %v", snap)
	}

	c := snap[0].(cluster)

	if c.InfrastructureRef == nil {
		return fmt.Errorf("cluster resource does not have infrastructureRef field: %v", c)
	}

	ownerRefPatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"ownerReferences": []map[string]interface{}{
				{
					"apiVersion": c.APIVersion,
					"kind":       c.Kind,
					"name":       c.Name,
					"namespace":  c.Namespace,
					"uid":        c.UID,
				},
			},
		},
	}
	// patch infrastructure cluster ownerRef
	input.PatchCollector.MergePatch(ownerRefPatch, c.InfrastructureRef.APIVersion, c.InfrastructureRef.Kind, c.InfrastructureRef.Namespace, c.InfrastructureRef.Name, object_patch.IgnoreMissingObject())

	// patch ready status
	input.PatchCollector.MergePatch(statusPatch, c.APIVersion, c.Kind, c.Namespace, c.Name, object_patch.IgnoreMissingObject(), object_patch.WithSubresource("/status"))

	return nil
}

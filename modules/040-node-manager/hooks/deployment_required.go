/*
Copyright 2021 Flant JSC

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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"

	v1 "github.com/deckhouse/deckhouse/go_lib/api/v1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "node_group",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
			FilterFunc: depRequiredFilterNG,
		},
		{
			Name:       "machine_deployment",
			ApiVersion: "machine.sapcloud.io/v1alpha1",
			Kind:       "MachineDeployment",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: nameFilter,
		},
		{
			Name:       "machine_set",
			ApiVersion: "machine.sapcloud.io/v1alpha1",
			Kind:       "MachineSet",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: nameFilter,
		},
		{
			Name:       "machine",
			ApiVersion: "machine.sapcloud.io/v1alpha1",
			Kind:       "Machine",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: nameFilter,
		},
	},
}, handleDeploymentRequired)

func nameFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

type depRequiredNG struct {
	Name    string
	IsCloud bool
}

func depRequiredFilterNG(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng v1.NodeGroup

	err := sdk.FromUnstructured(obj, &ng)
	if err != nil {
		return nil, err
	}

	return depRequiredNG{
		Name:    ng.Name,
		IsCloud: ng.Spec.NodeType == v1.NodeTypeCloudEphemeral,
	}, nil
}

func handleDeploymentRequired(input *go_hook.HookInput) error {
	var totalCount int

	snap := input.Snapshots["node_group"]
	for _, sn := range snap {
		ng := sn.(depRequiredNG)
		if ng.IsCloud {
			totalCount++
			break // we need at least one NG
		}
	}

	snapM := input.Snapshots["machine"]
	totalCount += len(snapM)
	snapMD := input.Snapshots["machine_deployment"]
	totalCount += len(snapMD)
	snapMS := input.Snapshots["machine_set"]
	totalCount += len(snapMS)

	if totalCount > 0 {
		input.Values.Set("nodeManager.internal.machineControllerManagerEnabled", true)
		return nil
	}

	input.Values.Remove("nodeManager.internal.machineControllerManagerEnabled")

	return nil
}

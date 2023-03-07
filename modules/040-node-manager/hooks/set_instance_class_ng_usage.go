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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/update_instance_class_ng",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "ings",
			Kind:                   "NodeGroup",
			ApiVersion:             "deckhouse.io/v1",
			WaitForSynchronization: pointer.BoolPtr(false),
			FilterFunc:             filterCloudEphemeralNG,
		},
	},
}, setInstanceClassUsage)

func filterCloudEphemeralNG(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng ngv1.NodeGroup

	err := sdk.FromUnstructured(obj, &ng)
	if err != nil {
		return nil, err
	}

	if ng.Spec.NodeType != ngv1.NodeTypeCloudEphemeral {
		return nil, nil
	}

	return usedInstanceClass{
		Kind:          ng.Spec.CloudInstances.ClassReference.Kind,
		Name:          ng.Spec.CloudInstances.ClassReference.Name,
		NodeGroupName: ng.Name,
	}, nil
}

func setInstanceClassUsage(input *go_hook.HookInput) error {
	snap := input.Snapshots["ings"]
	if len(snap) == 0 {
		return nil
	}

	m := make(map[usedInstanceClass][]string)

	for _, sn := range snap {
		if sn == nil {
			// not ephemeral
			continue
		}

		usedIC := sn.(usedInstanceClass)

		m[usedIC] = append(m[usedIC], usedIC.NodeGroupName)
	}

	for ic, ngNames := range m {
		statusPatch := map[string]interface{}{
			"status": map[string]interface{}{
				"nodeGroupConsumers": ngNames,
			},
		}
		input.PatchCollector.MergePatch(statusPatch, "deckhouse.io/v1", ic.Kind, "", ic.Name)
	}

	return nil
}

type usedInstanceClass struct {
	Kind string
	Name string

	NodeGroupName string
}

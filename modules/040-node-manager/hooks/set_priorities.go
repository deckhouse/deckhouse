/*
Copyright 2022 Flant JSC

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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/set_priorities",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "ngs",
			ApiVersion:             "deckhouse.io/v1",
			Kind:                   "NodeGroup",
			WaitForSynchronization: pointer.BoolPtr(false),
			FilterFunc:             setPriorityFilterNG,
		},
	},
}, handleSetPriorities)

type setPriorityNodeGroup struct {
	Name     string
	Priority *int
}

func setPriorityFilterNG(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng ngv1.NodeGroup

	err := sdk.FromUnstructured(obj, &ng)
	if err != nil {
		return nil, err
	}

	return setPriorityNodeGroup{
		Name:     ng.Name,
		Priority: ng.Spec.Priority,
	}, nil
}

func handleSetPriorities(input *go_hook.HookInput) error {
	priorities := make(map[int][]string)
	prefix := input.Values.Get("nodeManager.internal.instancePrefix")

	snap := input.Snapshots["ngs"]
	for _, sn := range snap {
		ng := sn.(setPriorityNodeGroup)
		if ng.Priority != nil {
			key := fmt.Sprintf("%s-%s-.*", prefix, ng.Name)
			priorities[*ng.Priority] = append(priorities[*ng.Priority], key)
		}
	}

	if len(priorities) > 0 {
		input.Values.Set("nodeManager.internal.clusterAutoscalerPriorities", priorities)
	} else {
		input.Values.Remove("nodeManager.internal.clusterAutoscalerPriorities")
	}

	return nil
}

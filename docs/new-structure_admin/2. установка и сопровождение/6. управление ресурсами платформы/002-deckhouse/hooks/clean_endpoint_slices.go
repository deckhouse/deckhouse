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
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

// this hook clean orphan EndpointSlices from the previous version of the `deckhouse` Service
// TODO: remove after the release 1.55
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:       "/modules/deckhouse/sync-configs",
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "endpointslices",
			ApiVersion:                   "discovery.k8s.io/v1",
			Kind:                         "EndpointSlice",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"app":                                    "deckhouse",
					"heritage":                               "deckhouse",
					"endpointslice.kubernetes.io/managed-by": "endpointslice-controller.k8s.io",
				},
			},
			FilterFunc: filterEndpointSlices,
		},
	},
}, deleteOrphanEndpoints)

func filterEndpointSlices(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func deleteOrphanEndpoints(input *go_hook.HookInput) error {
	snap := input.Snapshots["endpointslices"]

	for _, sn := range snap {
		endpointSliceName := sn.(string)
		input.PatchCollector.Delete("discovery.k8s.io/v1", "EndpointSlice", "d8-system", endpointSliceName, object_patch.InBackground())
	}

	return nil
}

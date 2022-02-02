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

/*
LINSTOR removal may stuck in case if operator deployment was removed before the resources.
We're removing whole namespace so there is no reason to wait for their graceful termination.
*/

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterDeleteHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "linstorcontrollers",
			ApiVersion: "piraeus.linbit.com/v1",
			Kind:       "LinstorController",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{linstorNamespace},
				},
			},
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
			FilterFunc:                   applyFinalizersFilter,
		},
		{
			Name:       "linstorsatellitesets",
			ApiVersion: "piraeus.linbit.com/v1",
			Kind:       "LinstorSatelliteSet",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{linstorNamespace},
				},
			},
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
			FilterFunc:                   applyFinalizersFilter,
		},
		{
			Name:       "linstorcsidrivers",
			ApiVersion: "piraeus.linbit.com/v1",
			Kind:       "LinstorCSIDriver",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{linstorNamespace},
				},
			},
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
			FilterFunc:                   applyFinalizersFilter,
		},
	},
}, removeFinalizers)

type LinstorCRSnapshot struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Name       string   `json:"name"`
	Namespace  string   `json:"namespace"`
	Finalizers []string `json:"finalizers"`
}

func applyFinalizersFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return LinstorCRSnapshot{
		APIVersion: obj.GetAPIVersion(),
		Kind:       obj.GetKind(),
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
		Finalizers: obj.GetFinalizers(),
	}, nil
}

func removeFinalizers(input *go_hook.HookInput) error {
	for _, kind := range []string{"linstorcontrollers", "linstorsatellitesets", "linstorcsidrivers"} {
		snaps := input.Snapshots[kind]
		if len(snaps) == 0 {
			input.LogEntry.Debugln(kind + " are not found. Skip")
			continue
		}

		for _, snap := range snaps {
			cr := snap.(LinstorCRSnapshot)
			if cr.Finalizers != nil {
				mergePatch := map[string]interface{}{
					"metadata": map[string]interface{}{
						"finalizers": nil,
					},
				}
				input.PatchCollector.MergePatch(mergePatch, cr.APIVersion, cr.Kind, cr.Namespace, cr.Name)
			}
		}

	}
	return nil
}

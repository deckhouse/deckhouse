/*
Copyright 2021 Flant CJSC

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
/*
	Temporary hook, that removes monitoringPDB without helm owner label to render new templates
	It could be removed after 01.11.2021
*/
package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/prometheus/remove_pdb",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "pdb",
			ApiVersion: "policy/v1beta1",
			Kind:       "PodDisruptionBudget",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"module": "prometheus",
				},
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "app.kubernetes.io/managed-by",
						Operator: v1.LabelSelectorOpNotIn,
						Values:   []string{"Helm"},
					},
				},
			},
			ExecuteHookOnEvents: pointer.BoolPtr(false),
			FilterFunc:          filterPDBWithAnnotations,
		},
	},
}, removePDBHandler)

func filterPDBWithAnnotations(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func removePDBHandler(input *go_hook.HookInput) error {
	snap := input.Snapshots["pdb"]
	for _, sn := range snap {
		pdbName := sn.(string)
		err := input.ObjectPatcher().DeleteObject("policy/v1beta1", "PodDisruptionBudget", "d8-monitoring", pdbName, "")
		if err != nil {
			return err
		}
	}

	return nil
}

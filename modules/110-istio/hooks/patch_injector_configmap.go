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

// The hook will lose relevance after solving the issue — https://github.com/istio/istio/issues/40078.

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

const istioInjectorCPULimitPath = "global.proxy.resources.limits.cpu"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: lib.Queue("patch-injector-configmap"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "injector_configmap",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			FilterFunc: applyInjectorConfigmapFilter,
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "operator.istio.io/version",
						Operator: metav1.LabelSelectorOpExists,
					},
					{
						Key:      "operator.istio.io/component",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"Pilot"},
					},
					{
						Key:      "release",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"istio"},
					},
				},
			},
			NamespaceSelector: lib.NsSelector(),
		},
	},
}, patchInjectorConfigmap)

type injectorConfigMap struct {
	Name   string
	Values string
}

func applyInjectorConfigmapFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cm := v1.ConfigMap{}
	err := sdk.FromUnstructured(obj, &cm)
	if err != nil {
		return nil, fmt.Errorf("cannot convert ConfigMap object to ConfigMap: %v", err)
	}
	values, ok := cm.Data["values"]
	// missing values to Patch -> skip it
	if !ok {
		return nil, nil
	}
	if !gjson.Get(values, istioInjectorCPULimitPath).Exists() {
		return nil, nil
	}
	return injectorConfigMap{
		Name:   cm.Name,
		Values: values,
	}, nil
}

func patchInjectorConfigmap(input *go_hook.HookInput) error {
	for _, imRaw := range input.Snapshots["injector_configmap"] {
		if imRaw == nil {
			continue
		}
		im := imRaw.(injectorConfigMap)
		patchedValues, err := sjson.Delete(im.Values, istioInjectorCPULimitPath)
		if err != nil {
			return err
		}
		cmPatch := map[string]interface{}{
			"data": map[string]interface{}{
				"values": patchedValues,
			},
		}
		input.PatchCollector.MergePatch(cmPatch, "v1", "ConfigMap", "d8-istio", im.Name)
	}
	return nil
}

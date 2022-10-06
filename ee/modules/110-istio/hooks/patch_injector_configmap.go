/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// The hook will lose relevance after solving the issue — https://github.com/istio/istio/issues/40078.

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/tidwall/sjson"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("patch-injector-configmap"),
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
			NamespaceSelector: internal.NsSelector(),
		},
	},
}, patchInjectorConfigmap)

func applyInjectorConfigmapFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cm := v1.ConfigMap{}
	err := sdk.FromUnstructured(obj, &cm)
	if err != nil {
		return nil, fmt.Errorf("cannot convert ConfigMap object to ConfigMap: %v", err)
	}
	return cm, nil
}

func patchInjectorConfigmap(input *go_hook.HookInput) error {
	for _, cmRaw := range input.Snapshots["injector_configmap"] {
		cm := cmRaw.(v1.ConfigMap)
		values, ok := cm.Data["values"]
		// missing values to Patch -> skip it
		if !ok {
			continue
		}
		patchedValues, err := sjson.Delete(values, "global.proxy.resources.limits.cpu")
		if err != nil {
			return err
		}
		cmPatch := map[string]interface{}{
			"data": map[string]interface{}{
				"values": patchedValues,
			},
		}
		input.PatchCollector.MergePatch(cmPatch, "v1", "ConfigMap", cm.Namespace, cm.Name)
	}
	return nil
}

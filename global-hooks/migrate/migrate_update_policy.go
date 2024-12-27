/*
Copyright 2024 Flant JSC

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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

// TODO(ipaqsa): remove it after 1.68
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "moduleUpdatePolicies",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleUpdatePolicy",
			FilterFunc: mupFilter,
		},
		{
			Name:                         "modules",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "Module",
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   moduleFilter,
		},
	},
}, fireMupAlerts)

func mupFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	mup := new(v1alpha1.ModuleUpdatePolicy)
	if err := sdk.FromUnstructured(obj, mup); err != nil {
		return nil, err
	}
	if mup.Spec.ModuleReleaseSelector.LabelSelector == nil {
		return nil, nil
	}
	return &filteredMup{LabelSelector: mup.Spec.ModuleReleaseSelector.LabelSelector}, nil
}

type filteredMup struct {
	LabelSelector *metav1.LabelSelector
}

func moduleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	module := new(v1alpha1.Module)
	if err := sdk.FromUnstructured(obj, module); err != nil {
		return nil, err
	}
	if module.Properties.UpdatePolicy != "" {
		return nil, nil
	}
	if !module.ConditionStatus(v1alpha1.ModuleConditionEnabledByModuleConfig) {
		return nil, nil
	}
	return &filteredModule{Name: module.Name, Source: module.Properties.Source}, nil
}

type filteredModule struct {
	Name   string
	Source string
}

func fireMupAlerts(input *go_hook.HookInput) error {
	input.MetricsCollector.Expire("d8_update_policy")

	for _, moduleSnapshot := range input.Snapshots["modules"] {
		if moduleSnapshot == nil {
			continue
		}

		module := moduleSnapshot.(*filteredModule)

		var labelsSet labels.Set = map[string]string{"module": module.Name, "source": module.Source}

		for _, policySnapshot := range input.Snapshots["moduleUpdatePolicies"] {
			if policySnapshot == nil {
				continue
			}

			policy := policySnapshot.(*filteredMup)

			selector, err := metav1.LabelSelectorAsSelector(policy.LabelSelector)
			if err != nil {
				continue
			}

			if selectorSourceName, exists := selector.RequiresExactMatch("source"); exists && selectorSourceName != module.Source {
				continue
			}

			if selector.Matches(labelsSet) {
				input.MetricsCollector.Set("d8_deprecated_update_policy", 1.0, map[string]string{
					"moduleName": module.Name,
				}, metrics.WithGroup("d8_update_policy"))
				continue
			}
		}
	}

	return nil
}

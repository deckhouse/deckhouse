/*
Copyright 2025 Flant JSC

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
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

const (
	promppModuleName = "prompp"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 9},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "prompp_module",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "Module",
			NameSelector: &types.NameSelector{
				MatchNames: []string{promppModuleName},
			},
			FilterFunc: applyModuleFilter,
		},
		{
			Name:       "prompp_moduleconfig",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{promppModuleName},
			},
			FilterFunc: applyModuleConfigFilter,
		},
	},
}, enablePrompp)

func applyModuleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	mc := &v1alpha1.Module{}
	err := sdk.FromUnstructured(obj, mc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert object to module: %v", err)
	}

	return mc.Name, nil
}

func applyModuleConfigFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	mc := &v1alpha1.ModuleConfig{}
	err := sdk.FromUnstructured(obj, mc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert object to moduleconfig: %v", err)
	}

	return mc.Name, nil
}

func enablePrompp(input *go_hook.HookInput) error {
	hasModule := len(input.Snapshots["prompp_module"]) > 0
	hasModuleConfig := len(input.Snapshots["prompp_moduleconfig"]) > 0

	if !hasModule {
		input.Logger.Info("no prompp module found, won't create ModuleConfig")
		return nil
	}

	if hasModuleConfig {
		input.Logger.Info("prompp ModuleConfig is present, nothing to do")
		return nil
	}

	mc := unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleConfig",
		"metadata": map[string]any{
			"name": promppModuleName,
		},
		"spec": map[string]any{
			"enabled": true,
			"source":  "deckhouse",
		},
	}}

	input.PatchCollector.CreateIfNotExists(&mc)

	return nil
}

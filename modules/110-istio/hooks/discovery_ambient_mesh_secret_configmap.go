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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	ambientModeValuesKey = "istio.internal.enableAmbientMode"
)

type ConfigMapInfo struct {
	Name      string
	Namespace string
	Exists    bool
}

func applyConfigMapFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	if obj == nil {
		return ConfigMapInfo{Exists: false}, nil
	}

	return ConfigMapInfo{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Exists:    true,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/istio/ambient_mode_monitor",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "ambientmode_configmap",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			// Use proper field names from the go_hook package
			NameSelector: &types.NameSelector{
				MatchNames: []string{"istio-ambientmode"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-istio"},
				},
			},
			FilterFunc: applyConfigMapFilter,
			// Explicitly enable hook execution on events and synchronization
			ExecuteHookOnEvents:          go_hook.Bool(true),
			ExecuteHookOnSynchronization: go_hook.Bool(true),
		},
	},
}, monitorAmbientModeConfigMap)

func monitorAmbientModeConfigMap(input *go_hook.HookInput) error {
	if len(input.Snapshots["ambientmode_configmap"]) == 0 {
		// ConfigMap doesn't exist
		input.Values.Set(ambientModeValuesKey, false)
		return nil
	}

	configMapInfo := input.Snapshots["ambientmode_configmap"][0].(ConfigMapInfo)
	if configMapInfo.Exists {
		// ConfigMap exists - enable ambient mode
		input.Values.Set(ambientModeValuesKey, true)
	} else {
		// ConfigMap doesn't exist - disable ambient mode
		input.Values.Set(ambientModeValuesKey, false)
	}

	return nil
}

/*
Copyright 2026 Flant JSC

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
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/control-plane-manager/alerting",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "module_config_authn",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"user-authn"},
			},
			FilterFunc: applyModuleConfigFilterForAlerts,
		},
	},
}, checkMcForNonMigratedConfig)

type ModuleConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModuleConfigSpec   `json:"spec"`
	Status ModuleConfigStatus `json:"status,omitempty"`
}

type ModuleConfigSpec struct {
	Version  int            `json:"version,omitempty"`
	Settings SettingsValues `json:"settings,omitempty"`
	Enabled  bool           `json:"enabled,omitempty"`
}

type SettingsValues struct {
	PublishAPI *struct{} `json:"publishAPI" yaml:"publishAPI"`
}

type ModuleConfigStatus struct {
	Version string `json:"version"`
	Message string `json:"message"`
}

func applyModuleConfigFilterForAlerts(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	mc := &ModuleConfig{}
	err := sdk.FromUnstructured(obj, mc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert user-authn ModuleConfig: %v", err)
	}

	return mc.Spec.Settings.PublishAPI, nil
}

func checkMcForNonMigratedConfig(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire("D8ObsoletePublishAPIInUserAuthn")

	mcSnaps := input.Snapshots.Get("module_config_authn")
	if len(mcSnaps) != 1 {
		return nil
	}

	settings := new(SettingsValues)

	err := mcSnaps[0].UnmarshalTo(settings)
	if err != nil {
		return fmt.Errorf("cannot unmarshal ModuleConfig: %w", err)
	}
	fmt.Println(settings)
	if settings != nil {
		input.MetricsCollector.Set("d8_obsolete_publishapi_in_user_authn", 1,
			map[string]string{},
			metrics.WithGroup("D8ObsoletePublishAPIInUserAuthn"))
	}
	return nil
}

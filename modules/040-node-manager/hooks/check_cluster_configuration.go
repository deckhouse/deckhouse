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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	CheckClusterConfigRaw = "checkClusterConfigRaw"
)

// TODO: Remove this hook after 1.70.0 release
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/requirements/check_cluster_configuration",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "check_cluster_configuration",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{
					"kube-system",
				}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{
				"d8-cluster-configuration",
			}},
			FilterFunc: applyProviderClusterConfigurationSecretFilter,
		},
	},
}, CheckClusterConfig)

func CheckClusterConfig(input *go_hook.HookInput) error {
	snap := input.Snapshots["check_cluster_configuration"]
	if len(snap) == 0 {
		return nil
	}
	secret := snap[0].(*v1.Secret)
	if YAML, ok := secret.Data["cluster-configuration.yaml"]; ok && len(YAML) > 0 {
		err := config.ValidateConf(&YAML)
		if err != nil {
			requirements.SaveValue(CheckClusterConfigRaw, true)
			input.MetricsCollector.Set("d8_check_cluster_config", 1, nil)
			errText := err.Error()
			if str := findErrorLines(errText); str != "" {
				input.Logger.Error(str)
			} else {
				input.Logger.Error(errText)
			}
			return nil
		}
		requirements.SaveValue(CheckClusterConfigRaw, false)
		input.MetricsCollector.Expire("d8_check_cluster_config")
	}
	return nil
}

func findErrorLines(text string) string {
	lines := strings.Split(text, "\n")
	var result strings.Builder
	for i := 0; i < len(lines); i++ {
		if strings.Contains(lines[i], "error occurred:") {
			result.WriteString(lines[i] + "\n")
			if i+1 < len(lines) {
				result.WriteString(lines[i+1] + "\n")
			}
		}
	}
	return result.String()
}

func applyProviderClusterConfigurationSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret = &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert secret from unstructured: %v", err)
	}

	return secret, nil
}

// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "deckhouse_cm",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(true),
			FilterFunc:                   applyDeckhouseConfigmapFilter,
		},
	},
}, migrationRemoveDeprecatedConfigmapDeckhouse)

func applyDeckhouseConfigmapFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm v1.ConfigMap
	err := sdk.FromUnstructured(obj, &cm)
	if err != nil {
		return "", err
	}
	for labelName := range cm.Labels {
		if strings.Contains(labelName, "argocd") {
			return 1.0, nil
		}
	}
	return 0.0, nil
}

func migrationRemoveDeprecatedConfigmapDeckhouse(input *go_hook.HookInput) error {
	deckhouseConfigSnap := input.Snapshots["deckhouse_cm"]
	if len(deckhouseConfigSnap) == 0 {
		return nil
	}

	input.MetricsCollector.Set(
		"d8_deprecated_configmap_managed_by_argocd",
		deckhouseConfigSnap[0].(float64),
		map[string]string{
			"namespace": "d8-system",
			"configmap": "deckhouse",
		},
		metrics.WithGroup("migration_remove_deprecated_deckhouse_cm"),
	)

	if deckhouseConfigSnap[0].(float64) == 0.0 {
		input.PatchCollector.Delete("v1", "ConfigMap", "d8-system", "deckhouse")
	}
	return nil
}

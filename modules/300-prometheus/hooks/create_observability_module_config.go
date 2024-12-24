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
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const observabilityMCSnapshotName = "observability_moduleconfig"

var observabilityMCManifest = map[string]interface{}{
	"apiVersion": "deckhouse.io/v1alpha1",
	"kind":       "ModuleConfig",
	"metadata": map[string]interface{}{
		"name": "observability",
	},
	"spec": map[string]interface{}{
		"enabled": "true",
	},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       observabilityMCSnapshotName,
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"observability"},
			},
			FilterFunc: applyObservabilityMCFilter,
		},
	},
}, observabilityMCHookHandler)

func applyObservabilityMCFilter(_ *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return true, nil
}

func observabilityMCHookHandler(input *go_hook.HookInput) error {
	observabilityMCSnapshots := input.Snapshots[observabilityMCSnapshotName]

	if len(observabilityMCSnapshots) == 0 {
		input.PatchCollector.Create(&unstructured.Unstructured{
			Object: observabilityMCManifest,
		})
	}

	return nil
}

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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

// TODO: migrate ModuleSource deckhouse to adopt it with helm release
//   it could be deleted after 1.60 release

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/deckhouse/adopt_module_source",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:         "ms",
			ApiVersion:   "deckhouse.io/v1alpha1",
			Kind:         "ModuleSource",
			NameSelector: &types.NameSelector{MatchNames: []string{"deckhouse"}},
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "app.kubernetes.io/managed-by",
						Operator: "NotIn",
						Values:   []string{"Helm"},
					},
				},
			},
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			FilterFunc:                   filterModuleSource,
		},
	},
}, adoptModuleSource)

func filterModuleSource(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return unstructured.GetName(), nil
}

func adoptModuleSource(input *go_hook.HookInput) error {
	snap := input.Snapshots["ms"]
	if len(snap) == 0 {
		return nil
	}

	name := snap[0].(string)
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				"meta.helm.sh/release-name":      "deckhouse",
				"meta.helm.sh/release-namespace": "d8-system",
			},
			"labels": map[string]string{
				"app.kubernetes.io/managed-by": "Helm",
				"heritage":                     "deckhouse",
				"module":                       "deckhouse",
			},
		},
	}
	input.PatchCollector.MergePatch(patch, "deckhouse.io/v1alpha1", "ModuleSource", "", name)

	return nil
}

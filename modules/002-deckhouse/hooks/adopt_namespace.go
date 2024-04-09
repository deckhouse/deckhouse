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
	"k8s.io/utils/pointer"
)

// TODO: migrate ns d8-cloud-instance-manager from node-manager helm release to deckhouse helm release
//   it could be deleted after 1.60 release

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/deckhouse/adopt_namespace",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "ns",
			ApiVersion:                   "v1",
			Kind:                         "Namespace",
			NameSelector:                 &types.NameSelector{MatchNames: []string{"d8-cloud-instance-manager"}},
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			FilterFunc:                   filterNS,
		},
	},
}, adoptNS)

func filterNS(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	if unstructured.GetAnnotations()["meta.helm.sh/release-namespace"] == "d8-system" {
		return nil, nil
	}
	return unstructured.GetName(), nil
}

func adoptNS(input *go_hook.HookInput) error {
	snap := input.Snapshots["ns"]
	if len(snap) == 0 {
		return nil
	}

	if snap[0] == nil {
		return nil
	}

	name := snap[0].(string)
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				"meta.helm.sh/release-name":      "node-manager",
				"meta.helm.sh/release-namespace": "d8-system",
			},
		},
	}
	input.PatchCollector.MergePatch(patch, "v1", "Namespace", "", name)

	return nil
}

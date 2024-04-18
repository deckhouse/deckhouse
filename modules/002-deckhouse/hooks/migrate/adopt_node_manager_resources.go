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

package migrate

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
	Queue:        "/modules/deckhouse/adopt_node_manager_resources",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "ns",
			ApiVersion:                   "v1",
			Kind:                         "Namespace",
			NameSelector:                 &types.NameSelector{MatchNames: []string{"d8-cloud-instance-manager"}},
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			FilterFunc:                   filterResource,
		},
		{
			Name:       "cm",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			NameSelector:                 &types.NameSelector{MatchNames: []string{"kube-rbac-proxy-ca.crt"}},
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			FilterFunc:                   filterResource,
		},
	},
}, adoptResources)

func filterResource(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	if unstructured.GetAnnotations()["meta.helm.sh/release-name"] == "deckhouse" {
		return nil, nil
	}
	return unstructured.GetName(), nil
}

func adoptResources(input *go_hook.HookInput) error {
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				"meta.helm.sh/release-name": "deckhouse",
			},
		},
	}

	snap := input.Snapshots["ns"]
	if len(snap) == 1 {
		if snap[0] != nil {
			name := snap[0].(string)
			input.PatchCollector.MergePatch(patch, "v1", "Namespace", "", name)
		}
	}

	snap = input.Snapshots["cm"]
	if len(snap) == 1 {
		if snap[0] != nil {
			name := snap[0].(string)
			input.PatchCollector.MergePatch(patch, "v1", "ConfigMap", "d8-cloud-instance-manager", name)
		}
	}
	return nil
}

/*
Copyright 2021 Flant JSC

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
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

// If istio-cni is enabled and DaemonSet istio-cni is created.
// We need to set cni-exclusive: "false" to avoid a conflict writing to /etc/cni/net.d/*.conflist.

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/cni-cilium/set-exclusive",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "istio-cni-daemonset",
			ApiVersion: "apps/v1",
			Kind:       "DaemonSet",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"istio-cni-node"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-istio"},
				},
			},
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   nameFilter,
		},
	},
}, setExclusiveMode)

func nameFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func setExclusiveMode(input *go_hook.HookInput) error {
	istioCniDaemonSet := input.Snapshots["istio-cni-daemonset"]
	if len(istioCniDaemonSet) < 1 {
		return nil // No need to change default value?
	}

	cm := map[string]interface{}{
		"data": map[string]string{
			"cni-exclusive": "false",
		},
	}
	input.PatchCollector.MergePatch(cm, "v1", "ConfigMap", "d8-cni-cilium", "cilium-config", object_patch.IgnoreMissingObject())
	return nil
}

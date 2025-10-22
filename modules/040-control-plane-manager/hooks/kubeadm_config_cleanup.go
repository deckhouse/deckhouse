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
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: moduleQueue,
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cm_kubeadm_config",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"kubeadm-config"},
			},
			FilterFunc: func(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
				return unstructured, nil
			},
		},
	},
}, handleKubeadmConfig)

func handleKubeadmConfig(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("cm_kubeadm_config")

	if len(snaps) == 0 {
		input.Logger.Debug("No kubeadm-config found or snapshot not configured")
		return nil
	}
	input.Logger.Info("Deleting CM kubeadm-config")
	input.PatchCollector.Delete("v1", "ConfigMap", "kube-system", "kubeadm-config")

	return nil
}

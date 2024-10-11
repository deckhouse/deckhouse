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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

const (
	clusterConfiguration  = "d8-cluster-configuration"
	providerConfiguration = "d8-provider-cluster-configuration"
	staticConfiguration   = "d8-static-cluster-configuration"
)

// we need to change therese secrets without deckhouse pod
// and we need to migrate existing secrets

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/deckhouse/migrate",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       clusterConfiguration,
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector:                 &types.NameSelector{MatchNames: []string{clusterConfiguration}},
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(false),
			FilterFunc:                   filterHasLabelHeritageDeckhouse,
		},
		{
			Name:       providerConfiguration,
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector:                 &types.NameSelector{MatchNames: []string{providerConfiguration}},
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(false),
			FilterFunc:                   filterHasLabelHeritageDeckhouse,
		},
		{
			Name:       staticConfiguration,
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector:                 &types.NameSelector{MatchNames: []string{staticConfiguration}},
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(false),
			FilterFunc:                   filterHasLabelHeritageDeckhouse,
		},
	},
}, removeLabelHeritageDeckhouse)

func filterHasLabelHeritageDeckhouse(secret *unstructured.Unstructured) (go_hook.FilterResult, error) {
	labels := secret.GetLabels()
	if len(labels) == 0 {
		return false, nil
	}
	val, ok := labels["heritage"]
	if !ok {
		return false, nil
	}
	return val == "deckhouse", nil
}

func removeLabelHeritageDeckhouse(input *go_hook.HookInput) error {
	removeLabelIfNeed := func(input *go_hook.HookInput, snapSecretName string) {
		snap := input.Snapshots[snapSecretName]

		if len(snap) == 0 {
			input.LogEntry.Debugf("Skip removing label 'heritage: deckhouse' for secret %s - secret not found", snapSecretName)
			return
		}

		if !snap[0].(bool) {
			input.LogEntry.Debugf("Skip removing label 'heritage: deckhouse' for secret %s - label not found", snapSecretName)
			return
		}

		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"heritage": nil,
				},
			},
		}

		input.LogEntry.Warnf(fmt.Sprintf("Remove label 'heritage: deckhouse' from %s", snapSecretName))
		input.PatchCollector.MergePatch(patch, "v1", "Secret", "kube-system", snapSecretName)
	}

	removeLabelIfNeed(input, clusterConfiguration)
	removeLabelIfNeed(input, providerConfiguration)
	removeLabelIfNeed(input, staticConfiguration)

	return nil
}

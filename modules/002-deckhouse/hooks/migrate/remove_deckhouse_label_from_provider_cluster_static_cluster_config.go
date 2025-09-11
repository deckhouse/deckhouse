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
	"context"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
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

func removeLabelHeritageDeckhouse(_ context.Context, input *go_hook.HookInput) error {
	removeLabelIfNeed := func(snapSecretName string) error {
		snapBools, err := sdkobjectpatch.UnmarshalToStruct[bool](input.Snapshots, snapSecretName)
		if err != nil {
			return fmt.Errorf("failed to unmarshal snapshot %q: %w", snapSecretName, err)
		}

		if len(snapBools) == 0 {
			input.Logger.Debug("Skip removing label 'heritage: deckhouse' - secret not found", slog.String("name", snapSecretName))
			return nil
		}

		if !snapBools[0] {
			input.Logger.Debug("Skip removing label 'heritage: deckhouse' - label not found", slog.String("name", snapSecretName))
			return nil
		}

		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"heritage": nil,
				},
			},
		}

		input.Logger.Warn("Remove label 'heritage: deckhouse' from secret", slog.String("name", snapSecretName))
		input.PatchCollector.PatchWithMerge(patch, "v1", "Secret", "kube-system", snapSecretName)
		return nil
	}

	if err := removeLabelIfNeed(clusterConfiguration); err != nil {
		return err
	}
	if err := removeLabelIfNeed(providerConfiguration); err != nil {
		return err
	}
	if err := removeLabelIfNeed(staticConfiguration); err != nil {
		return err
	}

	return nil
}

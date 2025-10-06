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

// add label name: d8-provider-cluster-configuration to provider cluster config secret
// add label name: d8-static-cluster-configuration to static cluster config secret
// cluster config secrete already has this label

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/deckhouse/migrate",
	Kubernetes: []go_hook.KubernetesConfig{
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
			FilterFunc:                   filterHasNotLabelName,
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
			FilterFunc:                   filterHasNotLabelName,
		},
	},
}, addLabelName)

func filterHasNotLabelName(secret *unstructured.Unstructured) (go_hook.FilterResult, error) {
	labels := secret.GetLabels()
	if len(labels) == 0 {
		return true, nil
	}

	if labels["name"] != secret.GetName() {
		return true, nil
	}

	return false, nil
}

func addLabelName(_ context.Context, input *go_hook.HookInput) error {
	addLabelFn := func(snapSecretName string) error {
		snapBools, err := sdkobjectpatch.UnmarshalToStruct[bool](input.Snapshots, snapSecretName)
		if err != nil {
			return fmt.Errorf("failed to unmarshal snapshot %q: %w", snapSecretName, err)
		}

		if len(snapBools) == 0 {
			input.Logger.Debug("Skip adding label 'name' - secret not found", slog.String("name", snapSecretName))
			return nil
		}

		if !snapBools[0] {
			input.Logger.Debug("Skip adding label 'name' - label found", slog.String("name", snapSecretName))
			return nil
		}

		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"name": snapSecretName,
				},
			},
		}

		input.Logger.Warn("Add label 'name' to secret", slog.String("name", snapSecretName))
		input.PatchCollector.PatchWithMerge(patch, "v1", "Secret", "kube-system", snapSecretName)
		return nil
	}

	if err := addLabelFn(providerConfiguration); err != nil {
		return err
	}
	if err := addLabelFn(staticConfiguration); err != nil {
		return err
	}

	return nil
}

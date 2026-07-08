/*
Copyright 2026 Flant JSC

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

package initsecret

import (
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	init_secret "github.com/deckhouse/deckhouse/go_lib/registry/models/initsecret"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const (
	initSecretName              = "registry-init"
	initSecretNamespace         = "d8-system"
	initSecretSnapName          = "init-secret"
	initSecretAppliedAnnotation = "registry.deckhouse.io/is-applied"
)

func snapName(prefix, name string) string {
	return fmt.Sprintf("%s-->%s", prefix, name)
}

func KubernetsConfig(name string) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:       snapName(name, initSecretSnapName),
		ApiVersion: "v1",
		Kind:       "Secret",
		NameSelector: &types.NameSelector{
			MatchNames: []string{initSecretName},
		},
		NamespaceSelector: &types.NamespaceSelector{
			NameSelector: &types.NameSelector{
				MatchNames: []string{initSecretNamespace},
			},
		},
		FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
			var secret v1core.Secret

			err := sdk.FromUnstructured(obj, &secret)
			if err != nil {
				return nil, fmt.Errorf("failed to convert init secret to struct: %v", err)
			}

			_, applied := secret.Annotations[initSecretAppliedAnnotation]
			ret := initSecretSnap{
				Applied: applied,
				Config:  secret.Data["config"],
			}
			return ret, nil
		},
	}
}

func InputsFromSnapshot(input *go_hook.HookInput, name string) (Inputs, error) {
	snapName := snapName(name, initSecretSnapName)

	initSecret, err := helpers.SnapshotToSingle[initSecretSnap](input, snapName)
	if err != nil {
		if errors.Is(err, helpers.ErrNoSnapshot) {
			// No initialization secret found - nothing to process
			return Inputs{}, nil
		}
		// Unexpected error while reading snapshot
		return Inputs{}, err
	}

	// Secret already applied in a previous run - skip processing
	if initSecret.Applied {
		return Inputs{}, nil
	}

	// Secret exists and needs to be processed
	var config init_secret.Config
	if err = yaml.Unmarshal(initSecret.Config, &config); err != nil {
		return Inputs{}, fmt.Errorf("cannot unmarshal init secret YAML from snapshot %s: %w", snapName, err)
	}

	if err = config.Validate(); err != nil {
		return Inputs{}, fmt.Errorf("init secret validation failed for snapshot %s: %w", snapName, err)
	}

	return Inputs{
		Applied: false,
		Config:  config,
	}, nil
}

func SetApplied(input *go_hook.HookInput, inputs Inputs) {
	if !inputs.Applied {
		input.Logger.Debug("Marking init secret as applied by setting annotation")

		patch := map[string]any{
			"metadata": map[string]any{
				"annotations": map[string]any{
					initSecretAppliedAnnotation: "",
				},
			},
		}

		input.PatchCollector.PatchWithMerge(
			patch, "v1", "Secret", initSecretNamespace, initSecretName)
	}
}

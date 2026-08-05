/*
Copyright 2025 Flant JSC

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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/utils/ptr"

	deckhousev1alpha1 "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/hooks/internal"
	ycpccv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/hooks/internal/api/pcc/v1"
	ycsettingsv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/hooks/internal/api/settings/v1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		// Binding 0: PCC secret - read-only snapshot; no events (deletion is handled by yandex_cluster_configuration.go).
		{
			Name:       "provider_cluster_configuration",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{internal.PCCSecretName},
			},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   internal.FilterPCCSecret,
		},
		// Binding 1: ModuleConfig - read-only snapshot for State B value override.
		// ExecuteHookOnSynchronization=false: hook must not fire before the namespace exists (created by Helm).
		{
			Name:       "module_config",
			ApiVersion: deckhousev1alpha1.SchemeGroupVersion.String(),
			Kind:       deckhousev1alpha1.ModuleConfigKind,
			NameSelector: &types.NameSelector{
				MatchNames: []string{internal.ModuleName},
			},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   internal.FilterModuleConfig,
		},
	},
}, handleMigrationResources)

func handleMigrationResources(_ context.Context, input *go_hook.HookInput) error {
	pccSnaps := input.Snapshots.Get("provider_cluster_configuration")
	if len(pccSnaps) == 0 {
		// State A: no PCC - nothing to create; deletion is handled by yandex_cluster_configuration.go.
		return nil
	}

	var pccResult internal.PCCSecretFilterResult
	if err := pccSnaps[0].UnmarshalTo(&pccResult); err != nil {
		return fmt.Errorf("unmarshal PCC snapshot: %w", err)
	}

	var pcc ycpccv1.YandexProviderClusterConfiguration
	if len(pccResult.ProviderClusterConfig) > 0 {
		if err := convertStructsUsingJSON(pccResult.ProviderClusterConfig, &pcc); err != nil {
			return fmt.Errorf("parse PCC: %w", err)
		}
	}

	// State B: PCC present, migration in progress - create artifacts in namespace (which now exists after Helm).
	// State C (migration complete) is detected and handled by yandex_cluster_configuration.go (OnBeforeHelm),
	// which fires on NodeGroup/YandexInstanceClass/ModuleConfig/Secret events and calls deleteMigrationArtifacts.
	// Running createProviderClusterConfigurationResources in State C is safe: CreateOrUpdate is idempotent
	// and yandex_cluster_configuration.go will delete the secret on the next (or concurrent) cycle.
	if err := validateProviderClusterConfig(pcc); err != nil {
		return fmt.Errorf("validate provider cluster config: %w", err)
	}

	var mc ycsettingsv1.ModuleConfigSettings
	mcSnaps := input.Snapshots.Get("module_config")

	if len(mcSnaps) != 0 {
		var mcResult internal.ModuleConfigFilterResult
		if err := mcSnaps[0].UnmarshalTo(&mcResult); err != nil {
			return fmt.Errorf("unmarshal ModuleConfig snapshot: %w", err)
		}

		// The v1 settings payload is what matters here: an operator may keep version 1 settings
		// while the module is disabled, and they still have to be projected.
		if len(mcResult.SettingsV1) > 0 {
			if err := convertStructsUsingJSON(mcResult.SettingsV1, &mc); err != nil {
				return fmt.Errorf("parse ModuleConfig v1: %w", err)
			}
		}
	}

	if err := internal.CreateMigrationResourcesSecret(input, pcc, mc); err != nil {
		return fmt.Errorf("create migration resources: %w", err)
	}

	internal.CreateMigrationConfigMap(input)
	return nil
}

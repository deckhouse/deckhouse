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
		// Binding 2: the existing master NodeGroup - used to detect a hybrid cluster (Static master).
		{
			Name:       "master_node_group",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"master"},
			},
			FilterFunc: internal.FilterNodeGroup,
		},
		// Binding 3: the candi discovery-data Secret - the infrastructure run's recorded output,
		// read when the legacy PCC does not carry discovery data.
		{
			Name:       "candi_discovery_data",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{internal.Namespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{internal.CandiDiscoverySecretName},
			},
			FilterFunc: internal.FilterCandiDiscoverySecret,
		},
	},
}, handleMigrationResources)

func handleMigrationResources(_ context.Context, input *go_hook.HookInput) error {
	pccResult, pccFound, err := decodePCCSnapshot(input)
	if err != nil {
		return err
	}

	if !pccFound {
		// State A: no PCC - nothing to create; deletion is handled by yandex_cluster_configuration.go.
		return nil
	}

	// State B: PCC present, migration in progress - create artifacts in namespace (which now exists after Helm).
	// State C (migration complete) is detected and handled by yandex_cluster_configuration.go (OnBeforeHelm),
	// which fires on NodeGroup/YandexInstanceClass/ModuleConfig/Secret events and calls deleteMigrationArtifacts.
	// Running createProviderClusterConfigurationResources in State C is safe: CreateOrUpdate is idempotent
	// and yandex_cluster_configuration.go will delete the secret on the next (or concurrent) cycle.
	var pcc ycpccv1.YandexProviderClusterConfiguration
	if err := convertStructsUsingJSON(pccResult.ProviderClusterConfig, &pcc); err != nil {
		return fmt.Errorf("unmarshal PCC: %w", err)
	}

	if err := validateProviderClusterConfig(pcc); err != nil {
		return fmt.Errorf("validate provider cluster config: %w", err)
	}

	mcSettings, err := decodeModuleConfigSettingsV1(input)
	if err != nil {
		return err
	}

	// Discovery data comes from the candi Secret first, then the legacy PCC discovery payload.
	discoveryData, candiPresent := resolveCandiDiscoveryData(input)
	if err := applyPCCDiscoveryFallback(&discoveryData, pccResult, candiPresent); err != nil {
		return err
	}

	isHybrid := isHybridCluster(input, "master_node_group")
	if err := internal.CreateMigrationResourcesSecret(input, pcc, mcSettings, discoveryData, isHybrid); err != nil {
		return fmt.Errorf("create migration resources: %w", err)
	}

	internal.CreateMigrationConfigMap(input)
	return nil
}

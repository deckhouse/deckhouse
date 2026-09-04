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
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	"github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/hooks/internal"
	ycicv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/hooks/internal/api/instanceclass/v1"
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
		// Binding 2: the existing master NodeGroup - used to detect a hybrid cluster (Static master).
		// Read-only snapshot: the hook writes into the module namespace, so it must run from
		// OnAfterHelm only - see the note on binding 1.
		{
			Name:       "master_node_group",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"master"},
			},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   internal.FilterNodeGroup,
		},
		// Binding 3: the candi discovery-data Secret - the infrastructure run's recorded output,
		// read when the legacy PCC does not carry discovery data. Read-only snapshot for the same
		// reason as binding 2.
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
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   internal.FilterCandiDiscoverySecret,
		},
		// Bindings 4-6: the new-model resources, needed to answer the same question
		// yandex_cluster_configuration.go asks - is the migration already complete? Without them
		// this hook cannot tell State B from State C and would re-create the artifacts that the
		// OnBeforeHelm hook has just deleted. Read-only snapshots for the same reason as binding 2.
		{
			Name:       "credential_secrets",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{internal.Namespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{
					cpapi.CredentialSecretName,
					internal.ExporterCredentialSecretName,
				},
			},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   internal.FilterCredentialSecret,
		},
		{
			Name:                         "node_groups",
			ApiVersion:                   "deckhouse.io/v1",
			Kind:                         "NodeGroup",
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   internal.FilterNodeGroup,
		},
		{
			Name:                         "yandex_instance_classes",
			ApiVersion:                   ycicv1.GroupVersionKind.GroupVersion().String(),
			Kind:                         ycicv1.YandexInstanceClassKind,
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   internal.FilterNamedResource,
		},
	},
}, handleMigrationResources)

func handleMigrationResources(_ context.Context, input *go_hook.HookInput) error {
	pccResult, ok, err := unmarshalToOneStruct[internal.PCCSecretFilterResult](input.Snapshots, "provider_cluster_configuration")
	if err != nil {
		return fmt.Errorf("unmarshal provider_cluster_configuration snapshots: %w", err)
	}
	if !ok || pccResult.ProviderClusterConfig == nil {
		return nil
	}
	pcc := *pccResult.ProviderClusterConfig

	// State C: the new-model resources are all in place, so the migration is over even though the
	// legacy PCC Secret is still around. yandex_cluster_configuration.go (OnBeforeHelm) deletes the
	// artifacts in this state; re-creating them here would delete and re-create the
	// d8-module-is-migrating ConfigMap on every module cycle. That churn is not cosmetic:
	// migration_pending_metric.go watches that ConfigMap, so the metric and its alert would flap,
	// and cpapi.ShouldSkipNewModelValidation keys off MigrationPending - the admission webhook
	// would accept or reject the same NodeGroup write depending on which side of the flap it lands
	// on. Both hooks must therefore agree on when the migration is complete.
	if internal.IsMigrationResourcesApplied(input, pcc) {
		return nil
	}

	// State B: PCC present, migration in progress - create artifacts in namespace (which now exists after Helm).
	if err := validateProviderClusterConfig(pcc); err != nil {
		return fmt.Errorf("validate provider cluster config: %w", err)
	}

	mcResult, ok, err := unmarshalToOneStruct[internal.ModuleConfigFilterResult](input.Snapshots, "module_config")
	if err != nil {
		return fmt.Errorf("unmarshal module_config snapshots: %w", err)
	}
	var mcSettingsV1 ycsettingsv1.ModuleConfigSettings
	if ok && mcResult.SettingsV1 != nil {
		mcSettingsV1 = *mcResult.SettingsV1
	}

	// The candi Secret first, then the legacy PCC payload, with the type markers and the region
	// default stamped on. Shared with yandex_cluster_configuration.go so the projection written
	// into the migration Secret matches the values that hook renders from.
	discoveryData, err := internal.ResolveDiscoveryData(input, &pccResult)
	if err != nil {
		return err
	}

	isHybrid := isHybridCluster(input, "master_node_group")
	if err := internal.CreateMigrationResourcesSecret(input, pcc, mcSettingsV1, discoveryData, isHybrid); err != nil {
		return fmt.Errorf("create migration resources: %w", err)
	}

	internal.CreateMigrationConfigMap(input)
	return nil
}

/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/hooks/internal"
	zicv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/hooks/internal/api/instanceclass/v1"
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "provider_cluster_configuration",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{internal.PCCSecretNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{internal.PCCSecretName},
			},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   internal.FilterPCCSecret,
		},
		{
			Name:       "module_config",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{internal.ModuleName},
			},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   internal.FilterModuleConfig,
		},
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
				MatchNames: []string{cpapi.CredentialSecretName},
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
			Name:                         "zvirt_instance_classes",
			ApiVersion:                   zicv1.GroupVersionKind.GroupVersion().String(),
			Kind:                         zicv1.ZvirtInstanceClassKind,
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   internal.FilterNamedResource,
		},
	},
}, handleCreateMigrationResources)

func handleCreateMigrationResources(_ context.Context, input *go_hook.HookInput) error {
	pccResult, pccPresent, err := unmarshalToOneStruct[internal.PCCSecretFilterResult](input.Snapshots, "provider_cluster_configuration")
	if err != nil {
		return fmt.Errorf("unmarshal provider_cluster_configuration snapshots: %w", err)
	}

	if !pccPresent || pccResult.ProviderClusterConfig == nil {
		// Nothing to migrate from. A cluster that never had the legacy configuration is already
		// on the new model.
		return nil
	}

	pcc := *pccResult.ProviderClusterConfig

	// The bundle has already been applied: zvirt_cluster_configuration.go removes the artifacts,
	// and regenerating them here would resurrect what it just deleted.
	if internal.IsMigrationResourcesApplied(input, pcc) {
		return nil
	}

	nodeGroups, err := sdkobjectpatch.UnmarshalToStruct[internal.NodeGroupFilterResult](input.Snapshots, "node_groups")
	if err != nil {
		return fmt.Errorf("unmarshal node_groups snapshots: %w", err)
	}

	if err := internal.CreateMigrationResourcesSecret(input, pcc, internal.IsHybridCluster(nodeGroups)); err != nil {
		return fmt.Errorf("create migration resources: %w", err)
	}

	internal.CreateMigrationConfigMap(input)

	return nil
}

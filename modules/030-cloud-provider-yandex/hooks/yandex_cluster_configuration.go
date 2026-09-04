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
	"encoding/json"
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	deckhousev1alpha1 "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	"github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/hooks/internal"
	ycicv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/hooks/internal/api/instanceclass/v1"
	ycpccv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/hooks/internal/api/pcc/v1"
	ycsettingsv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/hooks/internal/api/settings/v1"
	ycsettingsv2 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/hooks/internal/api/settings/v2"
)

const discoveryDataValuesPath = "cloudProviderYandex.internal.providerDiscoveryData"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
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
			FilterFunc: internal.FilterPCCSecret,
		},
		{
			Name:       "module_config",
			ApiVersion: deckhousev1alpha1.SchemeGroupVersion.String(),
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{internal.ModuleName},
			},
			FilterFunc: internal.FilterModuleConfig,
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
				MatchNames: []string{
					cpapi.CredentialSecretName,
					internal.ExporterCredentialSecretName,
				},
			},
			FilterFunc: internal.FilterCredentialSecret,
		},
		{
			Name:       "node_groups",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
			FilterFunc: internal.FilterNodeGroup,
		},
		{
			Name:       "yandex_instance_classes",
			ApiVersion: ycicv1.GroupVersionKind.GroupVersion().String(),
			Kind:       ycicv1.YandexInstanceClassKind,
			FilterFunc: internal.FilterNamedResource,
		},
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
}, handleYandexClusterConfiguration)

func handleYandexClusterConfiguration(_ context.Context, input *go_hook.HookInput) error {
	pccResult, pccPresent, err := unmarshalToOneStruct[internal.PCCSecretFilterResult](input.Snapshots, "provider_cluster_configuration")
	if err != nil {
		return fmt.Errorf("unmarshal provider_cluster_configuration snapshots: %w", err)
	}

	// The candi Secret first, then the legacy PCC payload, with the type markers and the region
	// default stamped on. Shared with create_migration_resources.go so both hooks resolve the
	// same data - see internal.ResolveDiscoveryData.
	var pccForDiscovery *internal.PCCSecretFilterResult
	if pccPresent {
		pccForDiscovery = &pccResult
	}
	discoveryData, err := internal.ResolveDiscoveryData(input, pccForDiscovery)
	if err != nil {
		return err
	}

	if !pccPresent || pccResult.ProviderClusterConfig == nil {
		// The legacy PCC is gone. Once credentials live in the cluster, the new model is the only
		// source of truth and the migration artifacts have to go: while d8-module-is-migrating
		// exists, ShouldSkipNewModelValidation keeps new-model validation switched off.
		// Without credentials the cluster is not migrated yet, so the artifacts stay.
		if hasCredentialSecrets(input) && internal.HasMigratedModuleConfig(input) {
			internal.DeleteMigrationArtifacts(input)
		}

		input.Values.Set(discoveryDataValuesPath, discoveryData)
		return nil
	}

	pcc := *pccResult.ProviderClusterConfig

	if internal.IsMigrationResourcesApplied(input, pcc) {
		// migration done: use MC v2 values
		internal.DeleteMigrationArtifacts(input)
		input.Values.Set(discoveryDataValuesPath, discoveryData)
		return nil
	}

	// State B: migration in progress - populate values from PCC so templates render
	if err := validateProviderClusterConfig(pcc); err != nil {
		return fmt.Errorf("validate provider cluster config: %w", err)
	}

	mcResult, ok, err := unmarshalToOneStruct[internal.ModuleConfigFilterResult](input.Snapshots, "module_config")
	if err != nil {
		return fmt.Errorf("unmarshal module_config snapshots: %w", err)
	}
	if ok && mcResult.SettingsV2 != nil {
		input.Values.Set(discoveryDataValuesPath, discoveryData)
		return nil
	}
	var mcSettingsV1 ycsettingsv1.ModuleConfigSettings
	if ok && mcResult.SettingsV1 != nil {
		mcSettingsV1 = *mcResult.SettingsV1
	}
	mcSettingsV2 := internal.BuildModuleConfigSettingsV2(
		pcc,
		mcSettingsV1,
		isHybridCluster(input, "node_groups"),
		discoveryData,
	)

	if err := setPCCAndMCtoRootValues(input, pcc, mcSettingsV2); err != nil {
		return fmt.Errorf("map PCC and MC v1 to root values: %w", err)
	}

	input.Values.Set(discoveryDataValuesPath, discoveryData)
	return nil
}

// isHybridCluster reports whether the NodeGroups in snapshotName describe a hybrid cluster, i.e.
// one whose master is Static. A snapshot that fails to decode counts as non-hybrid, which keeps
// the projection the one a cloud cluster gets.
func isHybridCluster(input *go_hook.HookInput, snapshotName string) bool {
	nodeGroups, err := sdkobjectpatch.UnmarshalToStruct[internal.NodeGroupFilterResult](input.Snapshots, snapshotName)
	if err != nil {
		return false
	}

	return internal.IsHybridCluster(nodeGroups)
}

// setPCCAndMCtoRootValues writes PCC fields into cloudProviderYandex settings paths
// in State B, following Yandex's leaf-only pattern:
func setPCCAndMCtoRootValues(
	input *go_hook.HookInput,
	pcc ycpccv1.YandexProviderClusterConfiguration,
	mcSettings ycsettingsv2.ModuleConfigSettings,
) error {
	input.Values.Set("cloudProviderYandex.provider", mcSettings.Provider)
	setSectionParameters(input, "cloudProviderYandex.nodes", mcSettings.Nodes.Parameters)
	setSectionParameters(input, "cloudProviderYandex.storage", mcSettings.Storage.Parameters)
	setSectionParameters(input, "cloudProviderYandex.ccm", mcSettings.CCM.Parameters)

	return setCredentialSecretsValuesIfAbsent(input, pcc)
}

// setSectionParameters replaces the parameters of a settings section, keeping the `disabled` flag
// the section already carries - that one belongs to the operator, through config values, and must
// survive the projection. The nodes, storage and ccm sections hold nothing else.
func setSectionParameters(input *go_hook.HookInput, path string, parameters any) {
	section := map[string]any{"parameters": parameters}
	if input.Values.Get(path + ".disabled").Bool() {
		section["disabled"] = true
	}

	input.Values.Set(path, section)
}

func setCredentialSecretsValuesIfAbsent(input *go_hook.HookInput, pcc ycpccv1.YandexProviderClusterConfiguration) error {
	existingSecrets := make(map[string]any)
	if v, ok := input.Values.GetOk("cloudProviderYandex.internal.credentialSecrets"); ok {
		if err := json.Unmarshal([]byte(v.Raw), &existingSecrets); err != nil {
			return fmt.Errorf("unmarshal credentialSecrets: %w", err)
		}
	}

	_, ok := existingSecrets[cpapi.CredentialSecretName]
	if !ok && pcc.Provider.ServiceAccountJSON != "" {
		existingSecrets[cpapi.CredentialSecretName] = map[string]any{
			"authScheme": cpapi.AuthSchemeServiceAccount,
			"secret":     pcc.Provider.ServiceAccountJSON,
		}
	}

	_, ok = existingSecrets[internal.ExporterCredentialSecretName]
	if !ok && pcc.WithNATInstance != nil && pcc.WithNATInstance.ExporterAPIKey != nil && *pcc.WithNATInstance.ExporterAPIKey != "" {
		existingSecrets[internal.ExporterCredentialSecretName] = map[string]any{
			"authScheme": cpapi.AuthSchemeAPIToken,
			"secret":     *pcc.WithNATInstance.ExporterAPIKey,
		}
	}

	input.Values.Set("cloudProviderYandex.internal.credentialSecrets", existingSecrets)

	return nil
}

// hasCredentialSecrets checks whether the managed credential Secret is available —
// either already populated by credentials.go (Order 19) or present as a Kubernetes
// Secret in the snapshot. Yandex does not store credentials inside ModuleConfig.
//
// Both checks match by name on purpose: the snapshot also carries the NAT-instance exporter
// Secret, which shares the credentials type, and a cluster holding only that one is not migrated.
func hasCredentialSecrets(input *go_hook.HookInput) bool {
	valuesPath := fmt.Sprintf("cloudProviderYandex.internal.credentialSecrets.%s", cpapi.CredentialSecretName)
	if _, exists := input.Values.GetOk(valuesPath); exists {
		return true
	}

	return internal.HasCredentialSecret(input)
}

// validateProviderClusterConfig ensures the PCC has the required Yandex provider fields.
func validateProviderClusterConfig(p ycpccv1.YandexProviderClusterConfiguration) error {
	if p.Provider.CloudID == "" {
		return errors.New("provider.cloudID cannot be empty")
	}
	if p.Provider.FolderID == "" {
		return errors.New("provider.folderID cannot be empty")
	}
	if p.Provider.ServiceAccountJSON == "" {
		return errors.New("provider.serviceAccountJSON cannot be empty")
	}

	return nil
}

func unmarshalToOneStruct[T any](s pkg.Snapshots, key string) (T, bool, error) {
	datas, err := sdkobjectpatch.UnmarshalToStruct[T](s, key)
	if err != nil {
		return *new(T), false, err
	}

	if len(datas) == 0 {
		return *new(T), false, nil
	}

	return datas[0], true, nil
}

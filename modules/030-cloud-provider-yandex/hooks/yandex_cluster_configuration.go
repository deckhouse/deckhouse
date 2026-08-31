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

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	deckhousev1alpha1 "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	clouddatav1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
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
	// Source priority, high to low:
	//  1. the candi Secret - the infrastructure run's recorded output;
	//  2. the legacy PCC discovery payload - also the infrastructure run's recorded output.
	// Both describe an infrastructure run, so a cluster whose infrastructure DKP does not create
	// has neither. That is why openapi/values.yaml requires almost nothing here: the workloads
	// read the network facts through templates/_helpers.tpl, which falls back to the operator's
	// own nodes.parameters.existing* whenever this payload does not carry them.
	discoveryData, candiPresent := resolveCandiDiscoveryData(input)

	// Yandex has no discover.go hook: this hook is the only writer of providerDiscoveryData, so the
	// merge has no second source to preserve. It stays because it also stamps the type markers and
	// the region default onto every payload written below.
	discoveryData = internal.MergeDiscoveryData(discoveryData, clouddatav1.YandexCloudDiscoveryData{})

	pccResult, pccFound, err := decodePCCSnapshot(input)
	if err != nil {
		return err
	}

	if !pccFound {
		// The legacy PCC is gone. Once credentials live in the cluster, the new model is the only
		// source of truth and the migration artifacts have to go: while d8-module-is-migrating
		// exists, ShouldSkipNewModelValidation keeps new-model validation switched off.
		// Without credentials the cluster is not migrated yet, so the artifacts stay.
		if hasCredentialSecrets(input) {
			internal.DeleteMigrationArtifacts(input)
		}

		input.Values.Set(discoveryDataValuesPath, discoveryData)
		return nil
	}

	if err := applyPCCDiscoveryFallback(&discoveryData, pccResult, candiPresent); err != nil {
		return err
	}

	var pcc ycpccv1.YandexProviderClusterConfiguration
	if err := convertStructsUsingJSON(pccResult.ProviderClusterConfig, &pcc); err != nil {
		return fmt.Errorf("unmarshal PCC: %w", err)
	}

	if internal.IsMigrationResourcesApplied(input, pcc) {
		// migration done: use MC v2 values
		internal.DeleteMigrationArtifacts(input)
		input.Values.Set(discoveryDataValuesPath, discoveryData)
		return nil
	}

	// State B: migration in progress - populate values from PCC so templates render
	mcSettings, err := decodeModuleConfigSettingsV1(input)
	if err != nil {
		return err
	}

	mcSettingsV2 := internal.BuildModuleConfigSettingsV2(
		pcc,
		mcSettings,
		isHybridCluster(input, "node_groups"),
		discoveryData,
	)

	if err := setPCCAndMCtoRootValues(input, pcc, mcSettingsV2); err != nil {
		return fmt.Errorf("map PCC and MC v1 to root values: %w", err)
	}

	if err := validateProviderClusterConfig(pcc); err != nil {
		return fmt.Errorf("validate provider cluster config: %w", err)
	}

	input.Values.Set(discoveryDataValuesPath, discoveryData)
	return nil
}

func decodePCCSnapshot(input *go_hook.HookInput) (internal.PCCSecretFilterResult, bool, error) {
	snaps := input.Snapshots.Get("provider_cluster_configuration")
	if len(snaps) == 0 {
		return internal.PCCSecretFilterResult{}, false, nil
	}

	var result internal.PCCSecretFilterResult
	if err := snaps[0].UnmarshalTo(&result); err != nil {
		return internal.PCCSecretFilterResult{}, false, fmt.Errorf("unmarshal PCC snapshot: %w", err)
	}

	return result, true, nil
}

// decodeModuleConfigSettingsV1 reads the v1 settings of the module's ModuleConfig.
//
// The v1 settings payload is what matters here: an operator may keep version 1 settings
// while the module is disabled, and they still have to be projected.
func decodeModuleConfigSettingsV1(input *go_hook.HookInput) (ycsettingsv1.ModuleConfigSettings, error) {
	var settings ycsettingsv1.ModuleConfigSettings

	snaps := input.Snapshots.Get("module_config")
	if len(snaps) == 0 {
		return settings, nil
	}

	var result internal.ModuleConfigFilterResult
	if err := snaps[0].UnmarshalTo(&result); err != nil {
		return settings, fmt.Errorf("unmarshal ModuleConfig snapshot: %w", err)
	}

	if len(result.SettingsV1) == 0 {
		return settings, nil
	}

	if err := json.Unmarshal(result.SettingsV1, &settings); err != nil {
		return settings, fmt.Errorf("parse ModuleConfig v1: %w", err)
	}

	return settings, nil
}

// resolveCandiDiscoveryData reads discovery data from the candi Secret written by the
// infrastructure run. The boolean reports whether the Secret carries a usable payload; an
// unparsable payload still reports true so that the PCC/projection fallback is deliberately
// suppressed rather than silently masked by another source.
func resolveCandiDiscoveryData(input *go_hook.HookInput) (clouddatav1.YandexCloudDiscoveryData, bool) {
	candiSnaps := input.Snapshots.Get("candi_discovery_data")
	if len(candiSnaps) == 0 {
		return clouddatav1.YandexCloudDiscoveryData{}, false
	}

	var candiResult internal.CandiDiscoveryDataFilterResult
	if err := candiSnaps[0].UnmarshalTo(&candiResult); err != nil {
		input.Logger.Warn("failed to unmarshal candi discovery snapshot; PCC fallback suppressed", "error", err)
		return clouddatav1.YandexCloudDiscoveryData{}, true
	}

	if len(candiResult.DiscoveryDataJSON) == 0 {
		return clouddatav1.YandexCloudDiscoveryData{}, false
	}

	var discoveryData clouddatav1.YandexCloudDiscoveryData
	if err := json.Unmarshal(candiResult.DiscoveryDataJSON, &discoveryData); err != nil {
		input.Logger.Warn("failed to parse candi discovery data JSON; PCC fallback suppressed", "error", err)
		return clouddatav1.YandexCloudDiscoveryData{}, true
	}

	return discoveryData, true
}

// applyPCCDiscoveryFallback overlays the legacy PCC discovery payload when the candi Secret carries
// nothing. Both record the same infrastructure run, so the candi one wins and the PCC one only
// stands in for it.
func applyPCCDiscoveryFallback(
	discoveryData *clouddatav1.YandexCloudDiscoveryData,
	pccResult internal.PCCSecretFilterResult,
	candiPresent bool,
) error {
	if candiPresent || len(pccResult.ProviderDiscoveryDataJSON) == 0 {
		return nil
	}

	if err := json.Unmarshal(pccResult.ProviderDiscoveryDataJSON, discoveryData); err != nil {
		return fmt.Errorf("unmarshal discovery data from PCC: %w", err)
	}

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

// hasCredentialSecrets checks whether credential secrets are available —
// either already populated by credentials.go (Order 19) or present as Kubernetes
// Secrets in the snapshot. Yandex does not store credentials inside ModuleConfig.
func hasCredentialSecrets(input *go_hook.HookInput) bool {
	if _, exists := input.Values.GetOk("cloudProviderYandex.internal.credentialSecrets.d8-credentials"); exists {
		return true
	}

	return len(input.Snapshots.Get("credential_secrets")) > 0
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

func convertStructsUsingJSON(in any, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}

	return json.Unmarshal(b, out)
}

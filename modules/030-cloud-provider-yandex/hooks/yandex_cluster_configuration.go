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

	ycicv1 "github.com/deckhouse/deckhouse/cloud-provider-yandex/pkg/api/instanceclass/v1"
	ycpccv1 "github.com/deckhouse/deckhouse/cloud-provider-yandex/pkg/api/pcc/v1"
	ycsettingsv1 "github.com/deckhouse/deckhouse/cloud-provider-yandex/pkg/api/settings/v1"
	ycsettingsv2 "github.com/deckhouse/deckhouse/cloud-provider-yandex/pkg/api/settings/v2"
	ycmeta "github.com/deckhouse/deckhouse/cloud-provider-yandex/pkg/meta"
	deckhousev1alpha1 "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	clouddatav1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	"github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/hooks/internal"
)

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
				MatchNames: []string{ycmeta.ModuleName},
			},
			FilterFunc: internal.FilterModuleConfig,
		},
		{
			Name:       "credential_secrets",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{ycmeta.Namespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{
					cpapi.CredentialSecretName,
					ycmeta.ExporterCredentialSecretName,
				},
			},
			FilterFunc: internal.FilterCredentialSecret,
		},
		{
			Name:       "node_groups",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
			FilterFunc: internal.FilterNamedResource,
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
					MatchNames: []string{ycmeta.Namespace},
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
	// candi takes priority over PCC
	discoveryData, candiPresent := resolveDiscoveryData(input)

	pccSnaps := input.Snapshots.Get("provider_cluster_configuration")
	if len(pccSnaps) == 0 {
		// The legacy PCC is gone. Once credentials live in the cluster, the new model is the only
		// source of truth and the migration artifacts have to go: while d8-module-is-migrating
		// exists, ShouldSkipNewModelValidation keeps new-model validation switched off.
		// Without credentials the cluster is not migrated yet, so the artifacts stay.
		if hasCredentialSecrets(input) {
			internal.DeleteMigrationArtifacts(input)
		}

		return mergeAndSetDiscoveryData(input, discoveryData)
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

	// fall back to PCC discovery when no candi secret
	if !candiPresent && len(pccResult.ProviderDiscoveryDataJSON) > 0 {
		if err := json.Unmarshal(pccResult.ProviderDiscoveryDataJSON, &discoveryData); err != nil {
			return fmt.Errorf("unmarshal discovery data from PCC: %w", err)
		}
	}

	if internal.IsMigrationResourcesApplied(input, pcc) {
		// migration done: use MC v2 values
		internal.DeleteMigrationArtifacts(input)
		return mergeAndSetDiscoveryData(input, discoveryData)
	}

	// State B: migration in progress — populate values from PCC so templates render
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

	if err := setPCCAndMCtoRootValues(input, pcc, mc); err != nil {
		return fmt.Errorf("map PCC and MC v1 to root values: %w", err)
	}

	if err := validateProviderClusterConfig(pcc); err != nil {
		return fmt.Errorf("validate provider cluster config: %w", err)
	}

	return mergeAndSetDiscoveryData(input, discoveryData)
}

func resolveDiscoveryData(input *go_hook.HookInput) (clouddatav1.YandexCloudDiscoveryData, bool) {
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
		return clouddatav1.YandexCloudDiscoveryData{}, true
	}

	var discoveryData clouddatav1.YandexCloudDiscoveryData
	if err := json.Unmarshal(candiResult.DiscoveryDataJSON, &discoveryData); err != nil {
		input.Logger.Warn("failed to parse candi discovery data JSON; PCC fallback suppressed", "error", err)
		return clouddatav1.YandexCloudDiscoveryData{}, true
	}

	return discoveryData, true
}

// setPCCAndMCtoRootValues writes PCC fields into cloudProviderYandex settings paths
// in State B, following Yandex's leaf-only pattern:
func setPCCAndMCtoRootValues(input *go_hook.HookInput, pcc ycpccv1.YandexProviderClusterConfiguration, mcSettings ycsettingsv1.ModuleConfigSettings) error {
	mcSettingsV2 := internal.BuildModuleConfigSettingsV2(pcc, mcSettings)

	input.Values.Set("cloudProviderYandex.provider", mcSettingsV2.Provider)

	if err := setNodesParametersValuesIfAbsent(input, mcSettingsV2); err != nil {
		return err
	}
	if err := setStorageParametersValuesIfAbsent(input, mcSettingsV2); err != nil {
		return err
	}
	if err := setCCMParametersValuesIfAbsent(input, mcSettingsV2); err != nil {
		return err
	}
	if err := setCredentialSecretsValuesIfAbsent(input, pcc); err != nil {
		return err
	}

	// Backward compatibility: storage_classes.go and the CCM template still read the
	// ModuleConfig v1 paths, so the v1 settings are mirrored as they are.
	input.Values.Set("cloudProviderYandex.storageClass", mcSettings.StorageClass)

	return nil
}

func setNodesParametersValuesIfAbsent(input *go_hook.HookInput, mcSettings ycsettingsv2.ModuleConfigSettings) error {
	var nodesSection ycsettingsv2.Nodes
	if v, ok := input.Values.GetOk("cloudProviderYandex.nodes"); ok {
		if err := json.Unmarshal([]byte(v.Raw), &nodesSection); err != nil {
			return fmt.Errorf("unmarshal nodes section: %w", err)
		}
	}

	nodesSection.Parameters = mcSettings.Nodes.Parameters
	input.Values.Set("cloudProviderYandex.nodes", nodesSection)
	return nil
}

func setStorageParametersValuesIfAbsent(input *go_hook.HookInput, mcSettings ycsettingsv2.ModuleConfigSettings) error {
	var storageSection ycsettingsv2.Storage
	if v, ok := input.Values.GetOk("cloudProviderYandex.storage"); ok {
		if err := json.Unmarshal([]byte(v.Raw), &storageSection); err != nil {
			return fmt.Errorf("unmarshal storage section: %w", err)
		}
	}

	storageSection.Parameters = mcSettings.Storage.Parameters
	input.Values.Set("cloudProviderYandex.storage", storageSection)

	return nil
}

func setCCMParametersValuesIfAbsent(input *go_hook.HookInput, mcSettings ycsettingsv2.ModuleConfigSettings) error {
	var ccmSection ycsettingsv2.CCM
	if v, ok := input.Values.GetOk("cloudProviderYandex.ccm"); ok {
		if err := json.Unmarshal([]byte(v.Raw), &ccmSection); err != nil {
			return fmt.Errorf("unmarshal ccm section: %w", err)
		}
	}

	ccmSection.Parameters = mcSettings.CCM.Parameters
	input.Values.Set("cloudProviderYandex.ccm", ccmSection)
	return nil
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

	_, ok = existingSecrets[ycmeta.ExporterCredentialSecretName]
	if !ok && pcc.WithNATInstance != nil && pcc.WithNATInstance.ExporterAPIKey != nil && *pcc.WithNATInstance.ExporterAPIKey != "" {
		existingSecrets[ycmeta.ExporterCredentialSecretName] = map[string]any{
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

	credSnaps := input.Snapshots.Get("credential_secrets")
	return len(credSnaps) > 0
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

// mergeAndSetDiscoveryData merges the resolved discovery data with any existing
// values and writes the result to cloudProviderYandex.internal.providerDiscoveryData.
func mergeAndSetDiscoveryData(input *go_hook.HookInput, discoveryData clouddatav1.YandexCloudDiscoveryData) error {
	if v, ok := input.Values.GetOk("cloudProviderYandex.internal.providerDiscoveryData"); ok && len(v.String()) != 0 {
		var existing clouddatav1.YandexCloudDiscoveryData
		if err := json.Unmarshal([]byte(v.String()), &existing); err != nil {
			return fmt.Errorf("unmarshal existing discovery data: %w", err)
		}
		discoveryData = mergeDiscoveryData(discoveryData, existing)
	}

	if discoveryData.APIVersion == "" {
		discoveryData.APIVersion = clouddatav1.APIVersion
	}
	if discoveryData.Kind == "" {
		discoveryData.Kind = clouddatav1.YandexCloudDiscoveryDataKind
	}

	input.Values.Set("cloudProviderYandex.internal.providerDiscoveryData", discoveryData)
	return nil
}

// mergeDiscoveryData grafts new discovery-data fields onto the existing set
// without overwriting already-populated values.
func mergeDiscoveryData(newValue, currentValue clouddatav1.YandexCloudDiscoveryData) clouddatav1.YandexCloudDiscoveryData {
	result := currentValue

	if newValue.APIVersion != "" && result.APIVersion == "" {
		result.APIVersion = newValue.APIVersion
	}
	if newValue.Kind != "" && result.Kind == "" {
		result.Kind = newValue.Kind
	}
	if newValue.Region != "" && result.Region == "" {
		result.Region = newValue.Region
	}
	if newValue.RouteTableID != "" && result.RouteTableID == "" {
		result.RouteTableID = newValue.RouteTableID
	}
	if newValue.DefaultLbTargetGroupNetworkID != "" && result.DefaultLbTargetGroupNetworkID == "" {
		result.DefaultLbTargetGroupNetworkID = newValue.DefaultLbTargetGroupNetworkID
	}
	if len(newValue.InternalNetworkIDs) > 0 && len(result.InternalNetworkIDs) == 0 {
		result.InternalNetworkIDs = newValue.InternalNetworkIDs
	}
	if len(newValue.Zones) > 0 && len(result.Zones) == 0 {
		result.Zones = newValue.Zones
	}
	if len(newValue.ZoneToSubnetIDMap) > 0 && len(result.ZoneToSubnetIDMap) == 0 {
		result.ZoneToSubnetIDMap = newValue.ZoneToSubnetIDMap
	}
	if newValue.ShouldAssignPublicIPAddress {
		result.ShouldAssignPublicIPAddress = true
	}
	if newValue.NATInstanceName != "" && result.NATInstanceName == "" {
		result.NATInstanceName = newValue.NATInstanceName
	}
	if newValue.NATInstanceZone != "" && result.NATInstanceZone == "" {
		result.NATInstanceZone = newValue.NATInstanceZone
	}
	if newValue.MonitoringAPIKey != "" && result.MonitoringAPIKey == "" {
		result.MonitoringAPIKey = newValue.MonitoringAPIKey
	}

	return result
}

func convertStructsUsingJSON(in any, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

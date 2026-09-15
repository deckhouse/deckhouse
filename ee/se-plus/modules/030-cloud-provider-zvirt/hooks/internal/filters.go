/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package internal

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	deckhousev1alpha1 "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	zpccv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/hooks/internal/api/pcc/v1"
	zsettingsv2 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/hooks/internal/api/settings/v2"
	clouddatav1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

// PCCSecretFilterResult carries both payloads of kube-system/d8-provider-cluster-configuration.
type PCCSecretFilterResult struct {
	ProviderClusterConfig *zpccv1.ZvirtProviderClusterConfiguration    `json:"providerClusterConfig,omitempty"`
	ProviderDiscoveryData *clouddatav1.ZvirtCloudProviderDiscoveryData `json:"providerDiscoveryData,omitempty"`
}

// ModuleConfigFilterResult is the decoded ModuleConfig of this module. Only v2 settings are
// decoded: version 1 of the cloud-provider-zvirt schema carried no settings at all.
type ModuleConfigFilterResult struct {
	Version    int64                             `json:"version"`
	Enabled    bool                              `json:"enabled"`
	SettingsV2 *zsettingsv2.ModuleConfigSettings `json:"settingsV2,omitempty"`
}

type NamedResourceFilterResult struct {
	Name string `json:"name"`
}

type NodeGroupFilterResult struct {
	Name     string `json:"name"`
	NodeType string `json:"nodeType"`
}

// CredentialSecretFilterResult carries the decoded managed credential Secret. The payload is
// needed by credentials.go, which projects it into internal values for the templates.
type CredentialSecretFilterResult struct {
	Name       string `json:"name"`
	AuthScheme string `json:"authScheme,omitempty"`
	Identity   string `json:"identity,omitempty"`
	Secret     string `json:"secret,omitempty"`
}

func FilterPCCSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// The fake k8s dynamic client ignores field selectors, so we guard by name here.
	if obj.GetName() != PCCSecretName {
		return nil, nil
	}

	secret := &corev1.Secret{}
	if err := sdk.FromUnstructured(obj, secret); err != nil {
		return nil, fmt.Errorf("cannot convert PCC secret from unstructured: %v", err)
	}

	result := &PCCSecretFilterResult{}

	if discoveryDataJSON, ok := secret.Data[PCCDiscoveryDataFilename]; ok && len(discoveryDataJSON) > 0 {
		if _, err := config.ValidateDiscoveryData(&discoveryDataJSON, nil, nil); err != nil {
			return nil, fmt.Errorf("validate %s: %v", PCCDiscoveryDataFilename, err)
		}

		var discoveryData clouddatav1.ZvirtCloudProviderDiscoveryData
		if err := json.Unmarshal(discoveryDataJSON, &discoveryData); err != nil {
			return nil, fmt.Errorf("unmarshal %s: %v", PCCDiscoveryDataFilename, err)
		}
		discoveryData.SetDefaults()

		result.ProviderDiscoveryData = &discoveryData
	}

	if clusterConfigYAML, ok := secret.Data[PCCClusterConfigFilename]; ok && len(clusterConfigYAML) > 0 {
		metaConfig, err := config.ParseConfigFromData(
			context.Background(),
			string(clusterConfigYAML),
			infrastructureprovider.MetaConfigValidatorProvider(),
			nil,
		)
		if err != nil {
			return nil, fmt.Errorf("validate %s: %v", PCCClusterConfigFilename, err)
		}

		var providerClusterConfig zpccv1.ZvirtProviderClusterConfiguration
		if err := ConvertStructsUsingJSON(metaConfig.ProviderClusterConfig, &providerClusterConfig); err != nil {
			return nil, fmt.Errorf("unmarshal %s: %v", PCCClusterConfigFilename, err)
		}

		result.ProviderClusterConfig = &providerClusterConfig
	}

	return result, nil
}

func FilterModuleConfig(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	mc := &deckhousev1alpha1.ModuleConfig{}
	if err := sdk.FromUnstructured(obj, mc); err != nil {
		return nil, fmt.Errorf("convert ModuleConfig from unstructured: %w", err)
	}

	result := ModuleConfigFilterResult{
		Version: int64(mc.Spec.Version),
		Enabled: mc.Spec.Enabled != nil && *mc.Spec.Enabled,
	}

	if mc.Spec.Settings != nil && mc.Spec.Version == 2 {
		settingsJSON, err := json.Marshal(mc.Spec.Settings.GetMap())
		if err != nil {
			return nil, fmt.Errorf("marshal ModuleConfig settings: %w", err)
		}

		var settings zsettingsv2.ModuleConfigSettings
		if err := json.Unmarshal(settingsJSON, &settings); err != nil {
			return nil, fmt.Errorf("unmarshal module config settings v2: %v", err)
		}
		result.SettingsV2 = &settings
	}

	return result, nil
}

// FilterCredentialSecret keeps only Secrets of the managed provider credentials type, so that an
// unrelated Secret living in the module namespace never reaches the values.
func FilterCredentialSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &corev1.Secret{}
	if err := sdk.FromUnstructured(obj, secret); err != nil {
		return nil, fmt.Errorf("convert credential Secret from unstructured: %w", err)
	}

	if string(secret.Type) != cpapi.CredentialsSecretType {
		return nil, nil
	}

	return CredentialSecretFilterResult{
		Name:       secret.Name,
		AuthScheme: string(secret.Data[cpapi.CredentialSecretAuthSchemeKey]),
		Identity:   string(secret.Data[cpapi.CredentialSecretIdentityKey]),
		Secret:     string(secret.Data[cpapi.CredentialSecretSecretKey]),
	}, nil
}

func FilterNamedResource(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return NamedResourceFilterResult{Name: obj.GetName()}, nil
}

func FilterNodeGroup(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ng := &cpapi.NodeGroup{}
	if err := sdk.FromUnstructured(obj, ng); err != nil {
		return nil, fmt.Errorf("convert NodeGroup from unstructured: %w", err)
	}

	return NodeGroupFilterResult{
		Name:     ng.Name,
		NodeType: string(ng.Spec.NodeType),
	}, nil
}

func ConvertStructsUsingJSON(in any, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}

	return json.Unmarshal(b, out)
}

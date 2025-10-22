/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "vcd_api_version",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-cloud-provider-discovery-data"},
			},
			FilterFunc: applyCloudProviderDiscoveryDataSecretVCDAPIVersionFilter,
		},
		{
			Name:       "legacy_mode",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-provider-cluster-configuration"},
			},
			FilterFunc: applyProviderClusterConfigurationSecretLegacyModeFilter,
		},
	},
}, handleLegacyMode)

func applyProviderClusterConfigurationSecretLegacyModeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	clusterConfig := &v1.Secret{}
	err := sdk.FromUnstructured(obj, clusterConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to convert kubernetes object: %v", err)
	}

	configDataJSON, ok := clusterConfig.Data["cloud-provider-cluster-configuration.yaml"]
	if !ok {
		return nil, fmt.Errorf("failed to find 'cloud-provider-cluster-configuration.yaml' in 'd8-provider-cluster-configuration' secret")
	}

	var configData map[string]any
	err = yaml.Unmarshal(configDataJSON, &configData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal 'cloud-provider-cluster-configuration.yaml' from 'd8-provider-cluster-configuration' secret: %v", err)
	}

	var legacyMode bool

	value, ok := configData["legacyMode"]
	if ok {
		legacyMode = value.(bool)
	}

	return legacyMode, nil
}

func applyCloudProviderDiscoveryDataSecretVCDAPIVersionFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	discoveryDataSecret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, discoveryDataSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to convert kubernetes object: %v", err)
	}

	discoveryDataJSON, ok := discoveryDataSecret.Data["discovery-data.json"]
	if !ok {
		return nil, fmt.Errorf("failed to find 'discovery-data.json' in 'd8-cloud-provider-discovery-data' secret")
	}

	var discoveryData v1alpha1.VCDCloudProviderDiscoveryData
	err = json.Unmarshal(discoveryDataJSON, &discoveryData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal 'discovery-data.json' from 'd8-cloud-provider-discovery-data' secret: %v", err)
	}

	return discoveryData.VCDAPIVersion, nil
}

func handleLegacyMode(_ context.Context, input *go_hook.HookInput) error {
	legacyModeBools, err := sdkobjectpatch.UnmarshalToStruct[bool](input.Snapshots, "legacy_mode")
	if err != nil {
		return fmt.Errorf("failed to unmarshal legacy_mode snapshot: %w", err)
	}

	if len(legacyModeBools) == 0 {
		input.Logger.Warn("Legacy mode not defined")

		return nil
	}

	if len(legacyModeBools) > 0 {
		// legacyMode is set in the provider cluster configuration secret
		input.Values.Set("cloudProviderVcd.internal.legacyMode", legacyModeBools[0])

		return nil
	}

	vcdAPIVers, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, "vcd_api_version")
	if err != nil {
		return fmt.Errorf("failed to unmarshal vcd_api_version snapshot: %w", err)
	}

	if len(vcdAPIVers) == 0 {
		input.Logger.Warn("VCD API version not defined")

		snaps, err := sdkobjectpatch.UnmarshalToStruct[bool](input.Snapshots, "legacy_mode")
		if err != nil {
			return fmt.Errorf("failed to unmarshal 'legacy_mode' snapshot: %w", err)
		}
		if len(snaps) == 0 {
			return fmt.Errorf("'legacy_mode' snapshot is empty")
		}
		legacyMode := snaps[0]
		if legacyMode {
			// legacyMode is set in the provider cluster configuration secret
			input.Values.Set("cloudProviderVcd.internal.legacyMode", legacyMode)
		}
		return nil
	}

	version, err := semver.NewVersion(vcdAPIVers[0])
	if err != nil {
		return fmt.Errorf("failed to parse VCD API version '%s': %v", vcdAPIVers[0], err)
	}

	versionConstraint, err := semver.NewConstraint("<37.2")
	if err != nil {
		return fmt.Errorf("failed to parse version constraint '%s': %v", versionConstraint, err)
	}

	// Set legacyMode to true if the VCD API version is less than 37.2
	input.Values.Set("cloudProviderVcd.internal.legacyMode", versionConstraint.Check(version))

	return nil
}

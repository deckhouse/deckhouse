/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"

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
	},
}, handleLegacyMode)

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

func handleLegacyMode(input *go_hook.HookInput) error {
	if len(input.Snapshots["vcd_api_version"]) == 0 {
		input.Logger.Warn("VCD API version not defined")

		return nil
	}

	vcdAPIVersion := input.Snapshots["vcd_api_version"][0].(string)

	version, err := semver.NewVersion(vcdAPIVersion)
	if err != nil {
		return fmt.Errorf("failed to parse VCD API version '%s': %v", vcdAPIVersion, err)
	}

	versionConstraint, err := semver.NewConstraint("<37.2")
	if err != nil {
		return fmt.Errorf("failed to parse version constraint '%s': %v", versionConstraint, err)
	}

	// Set legacyMode to true if the VCD API version is less than 37.2
	input.Values.Set("cloudProviderVcd.internal.legacyMode", versionConstraint.Check(version))

	return nil
}

/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/ee/LICENSE
*/

package pricing

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type ProviderClusterConfigurationSecret struct {
	ProviderClusterConfiguration []byte
}

func ApplyProviderClusterConfigurationSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes secret to secret: %v", err)
	}

	return ProviderClusterConfigurationSecret{ProviderClusterConfiguration: secret.Data["cloud-provider-cluster-configuration.yaml"]}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              "providerClusterConfigurationSecret",
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{"kube-system"}}},
			NameSelector:      &types.NameSelector{MatchNames: []string{"d8-provider-cluster-configuration"}},
			FilterFunc:        ApplyProviderClusterConfigurationSecretFilter,
		},
	},
}, providerClusterConfigurationSecret)

func providerClusterConfigurationSecret(input *go_hook.HookInput) error {
	snaps, ok := input.Snapshots["providerClusterConfigurationSecret"]
	if !ok {
		input.LogEntry.Info("No provider-cluster-configuration secret received, skipping setting values")
		return nil
	}

	ps := snaps[0].(ProviderClusterConfigurationSecret)
	var pcc map[string]interface{}
	err := yaml.Unmarshal(ps.ProviderClusterConfiguration, &pcc)
	if err != nil {
		input.LogEntry.Errorf("Failed to unmarshall provider-cluster-configuration from the secret: %v", err)
		return nil
	}

	if cloudLayout, ok := pcc["layout"]; ok {
		input.Values.Set("flantIntegration.internal.cloudLayout", cloudLayout)
		return nil
	}

	input.LogEntry.Error("Key `layout` not found in provider-cluster-configuration ")

	return nil
}

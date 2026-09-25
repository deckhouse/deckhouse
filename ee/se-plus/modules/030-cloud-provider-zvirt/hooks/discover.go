/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/lib-dhctl/pkg/yaml/validation"

	"github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/hooks/internal"
	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
)

const (
	discoveryDataSnapshotName = "cloud_provider_discovery_data"
	discoveryDataSecretName   = "d8-cloud-provider-discovery-data"
	discoveryDataSecretKey    = "discovery-data.json"
)

// This hook does one thing: it moves the discovery data the cloud-data-discoverer wrote into
// module values, validating it on the way. What the module makes of that data — StorageClasses,
// for one — belongs to the hooks that consume the values.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       discoveryDataSnapshotName,
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{discoveryDataSecretName},
			},
			FilterFunc: applyCloudProviderDiscoveryDataSecretFilter,
		},
	},
}, handleCloudProviderDiscoveryDataSecret)

func applyCloudProviderDiscoveryDataSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to convert kubernetes object: %v", err)
	}

	return secret, nil
}

func handleCloudProviderDiscoveryDataSecret(_ context.Context, input *go_hook.HookInput) error {
	secrets := input.Snapshots.Get(discoveryDataSnapshotName)
	if len(secrets) == 0 {
		input.Logger.Warn("failed to find secret '" + discoveryDataSecretName + "' in namespace 'kube-system'")

		return nil
	}

	secret := new(v1.Secret)
	if err := secrets[0].UnmarshalTo(secret); err != nil {
		return fmt.Errorf("failed to unmarshal '%s' snapshot: %w", discoveryDataSnapshotName, err)
	}

	discoveryDataJSON := secret.Data[discoveryDataSecretKey]

	if err := validation.ValidateData(internal.DiscoveryDataSchemaPaths, &discoveryDataJSON); err != nil {
		return fmt.Errorf("failed to validate '%s' from '%s' secret: %v", discoveryDataSecretKey, discoveryDataSecretName, err)
	}

	var discoveryData cloudDataV1.ZvirtCloudProviderDiscoveryData
	if err := json.Unmarshal(discoveryDataJSON, &discoveryData); err != nil {
		return fmt.Errorf("failed to unmarshal '%s' from '%s' secret: %v", discoveryDataSecretKey, discoveryDataSecretName, err)
	}

	discoveryData.SetDefaults()

	input.Values.Set("cloudProviderZvirt.internal.providerDiscoveryData", discoveryData)

	return nil
}

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
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/lib-dhctl/pkg/yaml/validation"

	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	"github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/hooks/internal"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 30},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cloud_provider_discovery_data",
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
			FilterFunc: applyCloudProviderDiscoveryDataSecretFilter,
		},
		{
			Name:       internal.StorageClassesSnapshotName,
			ApiVersion: "storage.k8s.io/v1",
			Kind:       "StorageClass",
			FilterFunc: internal.ApplyStorageClassFilter,
			LabelSelector: &meta.LabelSelector{
				MatchLabels: map[string]string{
					"heritage": "deckhouse",
					"module":   dvpModuleName,
				},
			},
			ExecuteHookOnSynchronization: ptr.To(false),
		},
	},
}, handleCloudProviderDiscoveryDataSecret)

func applyCloudProviderDiscoveryDataSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &corev1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to convert kubernetes object: %v", err)
	}

	return secret, nil
}

func handleCloudProviderDiscoveryDataSecret(_ context.Context, input *go_hook.HookInput) error {
	// On fresh install without a ModuleConfig, nodes/provider are absent; any Values.Set
	// triggers full-object schema validation which rejects the patch. Defer to OnBeforeHelm
	// where dvp_cluster_configuration.go (Order 20) has already populated required fields.
	if _, ok := input.Values.GetOk("cloudProviderDvp.provider"); !ok {
		input.Logger.Warn("cloudProviderDvp.provider not set, skipping discovery (will run on OnBeforeHelm)")
		return nil
	}

	secrets := input.Snapshots.Get("cloud_provider_discovery_data")
	if len(secrets) == 0 {
		input.Logger.Warn("failed to find secret 'd8-cloud-provider-discovery-data' in namespace 'kube-system'")

		if len(input.Snapshots.Get(internal.StorageClassesSnapshotName)) == 0 {
			input.Logger.Warn("failed to find storage classes for dvp provisioner")

			return nil
		}

		return internal.HandleStorageClassesFromSnapshots(input)
	}

	secret := new(corev1.Secret)
	err := secrets[0].UnmarshalTo(secret)
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'cloud_provider_discovery_data' snapshot: %w", err)
	}

	discoveryDataJSON := secret.Data["discovery-data.json"]

	if err := validation.ValidateData([]string{"/deckhouse/candi/cloud-providers/dvp/openapi", "/deckhouse/modules/030-cloud-provider-dvp/candi/openapi"}, &discoveryDataJSON); err != nil {
		return fmt.Errorf("failed to validate 'discovery-data.json' from 'd8-cloud-provider-discovery-data' secret: %v", err)
	}

	var discoveryData cloudDataV1.DVPCloudProviderDiscoveryData
	err = json.Unmarshal(discoveryDataJSON, &discoveryData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'discovery-data.json' from 'd8-cloud-provider-discovery-data' secret: %v", err)
	}

	input.Values.Set("cloudProviderDvp.internal.providerDiscoveryData", discoveryData)

	if err = internal.HandleStorageClassesFromDiscoveryData(input, discoveryData.StorageClassList); err != nil {
		return fmt.Errorf("failed to handle discovery data storage classes: %v", err)
	}

	return nil
}

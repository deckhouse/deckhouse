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
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeAll: &go_hook.OrderedConfig{Order: 20},
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
			FilterFunc: applyCloudProviderSecretFilter,
		},
	},
}, handleCloudProviderDefaultStorageClass)

type discoveryData struct {
	StorageClasses []storageClassInfo `json:"storageClasses"`
}

type storageClassInfo struct {
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}

func applyCloudProviderSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &corev1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	// Extract discovery-data.json from secret
	discoveryJSON, ok := secret.Data["discovery-data.json"]
	if !ok {
		return "", nil
	}

	// Parse JSON to find default storage class
	var data discoveryData
	if err := json.Unmarshal(discoveryJSON, &data); err != nil {
		return "", nil
	}

	// Find default storage class
	for _, sc := range data.StorageClasses {
		if sc.IsDefault {
			return sc.Name, nil
		}
	}

	return "", nil
}

func handleCloudProviderDefaultStorageClass(_ context.Context, input *go_hook.HookInput) error {
	const discoveryPath = "global.discovery.cloudProviderDefaultStorageClass"

	// Read default storage class from Secret snapshot using UnmarshalToStruct
	defaultSCSnap, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, "cloud_provider_discovery_data")
	if err != nil {
		return fmt.Errorf("failed to unmarshal cloud_provider_discovery_data snapshot: %w", err)
	}

	var defaultSC string
	if len(defaultSCSnap) > 0 {
		defaultSC = defaultSCSnap[0]
	}

	if defaultSC != "" {
		input.Values.Set(discoveryPath, defaultSC)
		input.Logger.Info("Set cloud provider default storage class to global values", slog.String("storage_class", defaultSC))
	} else {
		input.Logger.Info("No default storage class found from cloud provider")
		input.Values.Remove(discoveryPath)
	}

	return nil
}

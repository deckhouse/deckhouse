/*
Copyright 2024 Flant JSC

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
	"sort"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type EmbeddedRegistryDataDevice struct {
	NodeName   string `json:"nodeName"`
	DeviceName string `json:"deviceName"`
}

const (
	embeddedRegistryDataDevicesSecretName         = "d8-masters-system-registry-data-device-path"
	embeddedRegistryDataDevicesSecretNamespace    = "d8-system"
	embeddedRegistryDataDevicesInternalValuesPath = "nodeManager.internal.systemRegistry.dataDevices"
	embeddedRegistryDataDevicesSnapshotName       = "embedded_registry_data_devices_secret"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/node-manager/discover-embedded-registry-data-devices",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       embeddedRegistryDataDevicesSnapshotName,
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{embeddedRegistryDataDevicesSecretNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{embeddedRegistryDataDevicesSecretName},
			},
			FilterFunc: filterRegistryDataDevicesSecret,
		},
	},
}, handleRegistryDataDevicesSecret)

func filterRegistryDataDevicesSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := new(corev1.Secret)
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}
	return secret.Data, nil
}

func handleRegistryDataDevicesSecret(_ context.Context, input *go_hook.HookInput) error {
	if !input.Values.Exists(embeddedRegistryDataDevicesInternalValuesPath) {
		input.Values.Set(embeddedRegistryDataDevicesInternalValuesPath, []interface{}{})
	}

	secretData := input.Snapshots.Get(embeddedRegistryDataDevicesSnapshotName)
	if len(secretData) == 0 {
		input.Values.Set(embeddedRegistryDataDevicesInternalValuesPath, []interface{}{})
		return nil
	}

	secretDataStructured := map[string][]byte{}
	if err := secretData[0].UnmarshalTo(&secretDataStructured); err != nil {
		return err
	}
	if len(secretDataStructured) == 0 {
		input.Values.Set(embeddedRegistryDataDevicesInternalValuesPath, []interface{}{})
		return nil
	}

	dataDevices := make([]EmbeddedRegistryDataDevice, 0, len(secretDataStructured))

	sortedNodes := make([]string, 0, len(secretDataStructured))
	for node := range secretDataStructured {
		sortedNodes = append(sortedNodes, node)
	}
	sort.Strings(sortedNodes)

	for _, nodeName := range sortedNodes {
		deviceName := secretDataStructured[nodeName]
		dataDevices = append(dataDevices, EmbeddedRegistryDataDevice{
			NodeName:   nodeName,
			DeviceName: strings.TrimSpace(string(deviceName)),
		})
	}

	input.Values.Set(embeddedRegistryDataDevicesInternalValuesPath, dataDevices)
	return nil
}

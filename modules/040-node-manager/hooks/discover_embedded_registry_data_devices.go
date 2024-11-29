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
	embeddedRegistryDataDevicesSecretName      = "d8-masters-system-registry-data-device-path"
	embeddedRegistryDataDevicesSecretNamespace = "d8-system"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/node-manager/discover-embedded-registry-data-devices",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "embedded_registry_data_devices_secret",
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

func handleRegistryDataDevicesSecret(input *go_hook.HookInput) error {
	secretData := input.Snapshots["embedded_registry_data_devices_secret"]
	if len(secretData) == 0 {
		if input.Values.Exists("nodeManager.internal.systemRegistry.dataDevices") {
			input.Values.Remove("nodeManager.internal.systemRegistry.dataDevices")
		}
		return nil
	}

	secretDataStructured := secretData[0].(map[string][]byte)

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

	input.Values.Set("nodeManager.internal.systemRegistry.dataDevices", dataDevices)
	return nil
}

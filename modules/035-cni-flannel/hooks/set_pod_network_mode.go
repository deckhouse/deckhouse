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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

type FlannelConfig struct {
	PodNetworkMode string `json:"podNetworkMode"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
	Queue:     "/modules/cni-flannel",
}, setPodNetworkMode)

func setPodNetworkMode(input *go_hook.HookInput) error {
	var podNetworkMode = "host-gw"

	if input.ConfigValues.Exists("cniFlannel.podNetworkMode") {
		configPodNetworkMode := input.ConfigValues.Get("cniFlannel.podNetworkMode").String()
		switch configPodNetworkMode {
		case "HostGW":
			podNetworkMode = "host-gw"

		case "VXLAN":
			podNetworkMode = "vxlan"
		}
	}

	input.Values.Set("cniFlannel.internal.podNetworkMode", podNetworkMode)

	return nil
}

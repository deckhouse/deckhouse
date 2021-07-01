/*
Copyright 2021 Flant CJSC

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

// This is temporary migration hook, that could be deleted after this MR (1.07.2021) will get to rock-solid

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:     "/modules/cni-flannel-migrate",
	OnStartup: &go_hook.OrderedConfig{Order: 5},
}, migratePodNetworkMode)

func migratePodNetworkMode(input *go_hook.HookInput) error {
	if !input.ConfigValues.Exists("cniFlannel.podNetworkMode") {
		return nil
	}

	configPodNetworkMode := input.ConfigValues.Get("cniFlannel.podNetworkMode").String()
	var podNetworkMode string
	switch configPodNetworkMode {
	case "host-gw":
		podNetworkMode = "HostGW"

	case "vxlan":
		podNetworkMode = "VXLAN"

	default:
		// already migrated
		return nil
	}

	input.ConfigValues.Set("cniFlannel.podNetworkMode", podNetworkMode)

	return nil
}

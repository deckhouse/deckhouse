/*
Copyright 2022 Flant JSC

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

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/cni-cilium",
}, setCiliumMode)

func setCiliumMode(input *go_hook.HookInput) error {
	value, ok := input.ConfigValues.GetOk("cniCilium.tunnelMode")
	if ok {
		switch value.String() {
		case "VXLAN":
			input.Values.Set("cniCilium.internal.mode", "VXLAN")
			return nil
		case "Disabled":
			// to recover default value if it was discovered before
			input.Values.Set("cniCilium.internal.mode", "Direct")
		}
	}

	value, ok = input.ConfigValues.GetOk("cniCilium.createNodeRoutes")
	if ok && value.Bool() {
		input.Values.Set("cniCilium.internal.mode", "DirectWithNodeRoutes")
	}

	// for static clusters we should use DirectWithNodeRoutes mode
	value, ok = input.Values.GetOk("global.clusterConfiguration.clusterType")
	if ok && value.String() == "Static" {
		input.Values.Set("cniCilium.internal.mode", "DirectWithNodeRoutes")
	}

	// default
	// mode = Direct
	// masqueradeMode = BPF
	return nil
}

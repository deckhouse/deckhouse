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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

// This hook enables nodeRoutes for Openstack and VSphere providers

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/cni-cilium/node-routes",
}, enableNodeRoutes)

func enableNodeRoutes(input *go_hook.HookInput) error {
	input.Values.Set("cniCilium.createNodeRoutes", shouldEnableNodeRoutes(input))

	return nil
}

// use value from config
// if not present, enable everywhere except clouds (other than openstack and vsphere)
func shouldEnableNodeRoutes(input *go_hook.HookInput) bool {
	// Works only with `tunnelMode: Disabled`
	if input.ConfigValues.Get("cniCilium.tunnelMode").String() == "VXLAN" {
		return false
	}

	// if value is set directly - skip this hook
	value, ok := input.ConfigValues.GetOk("cniCilium.createNodeRoutes")
	if ok {
		return value.Bool()
	}

	providerRaw, ok := input.Values.GetOk("global.clusterConfiguration.cloud.provider")
	if ok {
		switch strings.ToLower(providerRaw.String()) {
		case "openstack", "vsphere":
			return true
		}

		return false
	}

	return true
}

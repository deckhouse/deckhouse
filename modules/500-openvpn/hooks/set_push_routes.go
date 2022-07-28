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

	"github.com/deckhouse/deckhouse/go_lib/set"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, setPushToClientRoutes)

const (
	globalPodSubnetPath            = "global.discovery.podSubnet"
	globalServiceSubnetPath        = "global.discovery.serviceSubnet"
	clientRoutesValuesPath         = "openvpn.pushToClientRoutes"
	clientRoutesInternalValuesPath = "openvpn.internal.pushToClientRoutes"
)

// setPushToClientRoutes create routes list for client
// from module config values and global discovery.
// Routes in list are unique.
func setPushToClientRoutes(input *go_hook.HookInput) error {
	routes := set.New()

	userDefinedSubnets, ok := input.ConfigValues.GetOk(clientRoutesValuesPath)
	if ok {
		for _, subnet := range userDefinedSubnets.Array() {
			routes.Add(subnet.String())
		}
	}

	podSubnet := input.Values.Get("global.discovery.podSubnet").String()
	if podSubnet != "" {
		routes.Add(podSubnet)
	}

	serviceSubnet := input.Values.Get("global.discovery.serviceSubnet").String()
	if serviceSubnet != "" {
		routes.Add(serviceSubnet)
	}

	input.Values.Set(clientRoutesInternalValuesPath, routes.Slice())

	return nil
}

/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, setPushToClientRoutes)

func setPushToClientRoutes(input *go_hook.HookInput) error {
	var routeList []string

	podSubnet := input.Values.Get("global.discovery.podSubnet").String()
	serviceSubnet := input.Values.Get("global.discovery.serviceSubnet").String()

	routeList = append(routeList, podSubnet)
	routeList = append(routeList, serviceSubnet)

	if !input.ConfigValues.Exists("openvpn.pushToClientRoutes") {
		input.ConfigValues.Set("openvpn.pushToClientRoutes", routeList)
	}

	return nil
}

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
	// if value is set directly - skip this hook
	_, ok := input.ConfigValues.GetOk("cniCilium.createNodeRoutes")
	if ok {
		return nil
	}

	providerRaw, ok := input.Values.GetOk("global.clusterConfiguration.cloud.provider")
	if !ok {
		return nil
	}

	switch strings.ToLower(providerRaw.String()) {
	case "openstack", "vsphere":
		input.Values.Set("cniCilium.createNodeRoutes", true)
	default:
		return nil
	}

	return nil
}

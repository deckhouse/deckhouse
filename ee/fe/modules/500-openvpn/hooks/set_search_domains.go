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
}, setSearchDomain)

func setSearchDomain(input *go_hook.HookInput) error {
	clusterDomain := input.Values.Get("global.discovery.clusterDomain").String()

	input.ConfigValues.Set("openvpn.pushToClientSearchDomains", []string{clusterDomain})

	return nil
}

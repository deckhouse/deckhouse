/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterDeleteHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:             "/modules/l2-load-balancer/discovery",
}, handleDeleteFlag)

func handleDeleteFlag(input *go_hook.HookInput) error {
	_ = input.Snapshots
	requirements.RemoveValue(l2LoadBalancerModuleDeprecatedKey)
	return nil
}

/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	l2LoadBalancerModuleDeprecatedKey = "l2LoadBalancer:isModuleEnabled"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, alertModuleDeprecatedL2lbExists)

func alertModuleDeprecatedL2lbExists(input *go_hook.HookInput) error {
	requirements.SaveValue(l2LoadBalancerModuleDeprecatedKey, true)

	input.MetricsCollector.Set(
		"d8_l2_load_balancer_module_enabled",
		1,
		map[string]string{},
		metrics.WithGroup("d8_l2_load_balancer_deprecated"),
	)
	return nil
}

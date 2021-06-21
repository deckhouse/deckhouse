package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 100},
}, handleSetInstancePrefix)

func handleSetInstancePrefix(input *go_hook.HookInput) error {
	prefix, exists := input.Values.GetOk("nodeManager.instancePrefix")
	if !exists {
		prefix = input.Values.Get("global.clusterConfiguration.cloud.prefix")
	}

	input.Values.Set("nodeManager.internal.instancePrefix", prefix.String())

	return nil
}

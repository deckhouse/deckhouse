package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	deckhouse_config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
)

// TODO: Remove this hook after Deckhouse release 1.56
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 25},
}, deleteConfigModule)

func deleteConfigModule(input *go_hook.HookInput) error {
	//deckhouse-config
	srv := deckhouse_config.Service()

	return srv.DeleteModule("deckhouse-config")
}

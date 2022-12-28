package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/telemetry"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{
		Order: 20,
	},
}, setCNITelemetry)

func setCNITelemetry(input *go_hook.HookInput) error {
	input.MetricsCollector.Set(telemetry.WrapName("cni_plugin"), 1, map[string]string{"name": "flannel"})
	return nil
}

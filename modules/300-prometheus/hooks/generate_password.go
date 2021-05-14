package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/pwgen"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/prometheus/generate_password",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, generatePassword)

func generatePassword(input *go_hook.HookInput) error {
	if input.Values.Exists("prometheus.auth.externalAuthentication") {
		input.ConfigValues.Remove("prometheus.auth.password")
		if input.ConfigValues.Exists("prometheus.auth") && len(input.ConfigValues.Get("prometheus.auth").Map()) == 0 {
			input.ConfigValues.Remove("prometheus.auth")
		}

		return nil
	}

	if input.Values.Exists("prometheus.auth.password") {
		return nil
	}

	generatedPass := pwgen.AlphaNum(20)

	input.Values.Set("prometheus.auth.password", generatedPass)

	return nil
}

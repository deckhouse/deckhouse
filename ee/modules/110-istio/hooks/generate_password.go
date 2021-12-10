/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/json"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/pwgen"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/istio/generate_password",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, generatePassword)

func generatePassword(input *go_hook.HookInput) error {
	if input.Values.Exists("istio.auth.externalAuthentication") {
		input.ConfigValues.Remove("istio.auth.password")
		if input.ConfigValues.Exists("istio.auth") && len(input.ConfigValues.Get("istio.auth").Map()) == 0 {
			input.ConfigValues.Remove("istio.auth")
		}

		return nil
	}

	if input.Values.Exists("istio.auth.password") {
		return nil
	}

	if !input.ConfigValues.Exists("istio.auth") {
		input.ConfigValues.Set("istio.auth", json.RawMessage("{}"))
	}

	generatedPass := pwgen.AlphaNum(20)

	input.ConfigValues.Set("istio.auth.password", generatedPass)

	return nil
}

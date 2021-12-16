/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"encoding/json"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/pwgen"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, generatePassword)

func generatePassword(input *go_hook.HookInput) error {
	if input.Values.Exists("deckhouseWeb.auth.externalAuthentication") {
		input.ConfigValues.Remove("deckhouseWeb.auth.password")
		if input.ConfigValues.Exists("deckhouseWeb.auth") && len(input.ConfigValues.Get("deckhouseWeb.auth").Map()) == 0 {
			input.ConfigValues.Remove("deckhouseWeb.auth")
		}

		return nil
	}

	if input.Values.Exists("deckhouseWeb.auth.password") {
		return nil
	}

	if !input.ConfigValues.Exists("deckhouseWeb.auth") {
		input.ConfigValues.Set("deckhouseWeb.auth", json.RawMessage("{}"))
	}

	input.ConfigValues.Set("deckhouseWeb.auth.password", pwgen.AlphaNum(20))

	return nil
}

/*
Copyright 2021 Flant CJSC

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

	if !input.ConfigValues.Exists("prometheus.auth") {
		input.ConfigValues.Set("prometheus.auth", json.RawMessage("{}"))
	}

	generatedPass := pwgen.AlphaNum(20)

	input.ConfigValues.Set("prometheus.auth.password", generatedPass)

	return nil
}

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

package generate_password

import (
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/pwgen"
)

func RegisterHook(moduleValuesPath string) bool {
	return sdk.RegisterFunc(&go_hook.HookConfig{
		Queue:        fmt.Sprintf("/modules/%s/generate_password", moduleValuesPath),
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	}, generatePassword(moduleValuesPath))
}

func generatePasswordWithArgs(input *go_hook.HookInput, moduleValuesPath string) error {
	if input.Values.Exists(fmt.Sprintf("%s.auth.externalAuthentication", moduleValuesPath)) {
		input.ConfigValues.Remove(fmt.Sprintf("%s.auth.password", moduleValuesPath))
		if input.ConfigValues.Exists(fmt.Sprintf("%s.auth", moduleValuesPath)) && len(input.ConfigValues.Get(fmt.Sprintf("%s.auth", moduleValuesPath)).Map()) == 0 {
			input.ConfigValues.Remove(fmt.Sprintf("%s.auth", moduleValuesPath))
		}

		return nil
	}

	if input.Values.Exists(fmt.Sprintf("%s.auth.password", moduleValuesPath)) {
		return nil
	}

	if !input.ConfigValues.Exists(fmt.Sprintf("%s.auth", moduleValuesPath)) {
		input.ConfigValues.Set(fmt.Sprintf("%s.auth", moduleValuesPath), json.RawMessage("{}"))
	}

	generatedPass := pwgen.AlphaNum(20)

	input.ConfigValues.Set(fmt.Sprintf("%s.auth.password", moduleValuesPath), generatedPass)

	return nil
}

func generatePassword(moduleValuesPath string) func(input *go_hook.HookInput) error {
	return func(input *go_hook.HookInput) error {
		err := generatePasswordWithArgs(input, moduleValuesPath)
		if err != nil {
			return err
		}
		return nil
	}
}

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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/pwgen"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/upmeter/generate_password",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, generatePassword)

func generatePassword(input *go_hook.HookInput) error {
	const rootAuthKey = "upmeter.auth"

	values, config, err := parseValuesAndConfig(rootAuthKey, input)
	if err != nil {
		return err
	}

	config.Status = setPassword(config.Status, values.Status)
	config.Webui = setPassword(config.Webui, values.Webui)

	if config.IsEmpty() {
		input.ConfigValues.Remove(rootAuthKey)
		return nil
	}

	input.ConfigValues.Set(rootAuthKey, config)
	return nil
}

// setPassword returns config value for an app auth settings. It sets missing password or cleans it
// up when is not used. Returns nil to clean config.
func setPassword(config, values *appAuth) *appAuth {
	// Guard
	if values == nil {
		values = &appAuth{}
	}

	// Remove password if external auth is on
	if values.External != nil {
		if config == nil || config.External == nil {
			// Nothing to remove
			return config
		}

		config.Password = ""
		if config.IsEmpty() {
			return nil
		}
		return config
	}

	// Avoid changing existing password
	if values.Password != "" {
		return config
	}

	// Set password
	if config == nil {
		config = &appAuth{}
	}
	config.Password = pwgen.AlphaNum(20)
	return config
}

type authValues struct {
	Status *appAuth `json:"status,omitempty"`
	Webui  *appAuth `json:"webui,omitempty"`
}

func (a *authValues) IsEmpty() bool {
	return a.Webui.IsEmpty() && a.Status.IsEmpty()
}

type appAuth struct {
	Password              string        `json:"password,omitempty"`
	External              *externalAuth `json:"externalAuthentication,omitempty"`
	AllowedUserGroups     []string      `json:"allowedUserGroups,omitempty"`
	WhitelistSourceRanges []string      `json:"whitelistSourceRanges,omitempty"`
}

func (a *appAuth) IsEmpty() bool {
	if a == nil {
		return true
	}
	return a.Password == "" &&
		a.External == nil &&
		len(a.AllowedUserGroups) == 0 &&
		len(a.WhitelistSourceRanges) == 0
}

type externalAuth struct {
	AuthURL       string `json:"authURL"`
	AuthSignInURL string `json:"authSignInURL"`
}

func parseValuesAndConfig(rootAuthKey string, input *go_hook.HookInput) (*authValues, *authValues, error) {
	values, err := parseAuth(rootAuthKey, input.Values)
	if err != nil {
		return nil, nil, fmt.Errorf("canot parse values: %v", err)
	}

	config, err := parseAuth(rootAuthKey, input.ConfigValues)
	if err != nil {
		return nil, nil, fmt.Errorf("canot parse config values: %v", err)
	}
	return values, config, nil
}

func parseAuth(rootAuthKey string, values *go_hook.PatchableValues) (*authValues, error) {
	var data authValues
	if s, ok := values.GetOk(rootAuthKey); ok {
		b := []byte(s.String())
		err := json.Unmarshal(b, &data)
		if err != nil {
			return nil, err
		}
	}
	return &data, nil
}

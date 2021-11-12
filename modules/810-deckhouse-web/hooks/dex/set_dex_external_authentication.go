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

	"github.com/deckhouse/deckhouse/go_lib/module"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 9},
}, chooseDexExternalAuth)

// Module settings templates
const (
	externalAuthenticationTemplate = `
{
  "authURL": "https://deckhouse-web-dex-authenticator.d8-system.svc.%s/dex-authenticator/auth",
  "authSignInURL": "https://$host/dex-authenticator/sign_in"
}`
)

// Values keys
const (
	moduleName                = "deckhouseWeb"
	externalAuthenticationKey = moduleName + ".auth.externalAuthentication"
	deployDexAuthenticatorKey = moduleName + ".internal.deployDexAuthenticator"
	clusterDomainKey          = "global.discovery.clusterDomain"
)

func setExternalAuthenticationFromConfig(input *go_hook.HookInput) {
	if !input.ConfigValues.Exists(externalAuthenticationKey) {
		input.Values.Remove(externalAuthenticationKey)
	} else {
		input.Values.Set(externalAuthenticationKey, input.ConfigValues.Get(externalAuthenticationKey))
	}
	input.Values.Remove(deployDexAuthenticatorKey)
}

func chooseDexExternalAuth(input *go_hook.HookInput) error {

	if !input.Values.Get("global.clusterIsBootstrapped").Bool() {
		return nil
	}

	if !set.NewFromValues(input.Values, "global.enabledModules").Has("user-authn") {
		return nil
	}

	if module.GetHTTPSMode("deckhouseWeb", input) != "Disabled" {
		if !input.ConfigValues.Exists(externalAuthenticationKey) {
			// Use dex authenticator by default if cluster bootstrapped and has module user-authn
			rawExternalAuthentication := make(map[string]string)
			externalAuthentication := fmt.Sprintf(externalAuthenticationTemplate, input.Values.Get(clusterDomainKey))
			err := json.Unmarshal([]byte(externalAuthentication), &rawExternalAuthentication)
			if err != nil {
				return err
			}
			input.Values.Set(externalAuthenticationKey, rawExternalAuthentication)
			input.Values.Set(deployDexAuthenticatorKey, true)
		} else {
			setExternalAuthenticationFromConfig(input)
		}
	} else {
		setExternalAuthenticationFromConfig(input)
	}

	return nil
}

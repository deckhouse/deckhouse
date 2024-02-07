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
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/hooks/external_auth"
)

var _ = external_auth.RegisterHook(external_auth.Settings{
	ExternalAuthPath:            "dashboard.auth.externalAuthentication",
	DexAuthenticatorEnabledPath: "dashboard.internal.deployDexAuthenticator",
	DexExternalAuth: external_auth.ExternalAuth{
		AuthURL:         "https://dashboard-dex-authenticator.d8-dashboard.svc.%CLUSTER_DOMAIN%/dex-authenticator/auth",
		AuthSignInURL:   "https://$host/dex-authenticator/sign_in",
		UseBearerTokens: pointer.Bool(true),
	},
})

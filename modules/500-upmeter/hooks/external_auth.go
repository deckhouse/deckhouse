package hooks

import (
	"github.com/deckhouse/deckhouse/go_lib/hooks/external_auth"
)

var _ = external_auth.RegisterHook(external_auth.Settings{
	ExternalAuthPath:            "upmeter.webui.auth.externalAuthentication",
	DexAuthenticatorEnabledPath: "upmeter.internal.deployDexAuthenticator",
	DexExternalAuth: external_auth.ExternalAuth{
		AuthURL:       "upmeter-dex-authenticator.d8-upmeter.svc.$CLUSTER_DOMAIN%/dex-authenticator/auth",
		AuthSignInURL: "https://$host/dex-authenticator/sign_in",
	},
})

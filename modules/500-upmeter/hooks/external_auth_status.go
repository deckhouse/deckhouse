package hooks

import (
	"github.com/deckhouse/deckhouse/go_lib/hooks/external_auth"
)

var _ = external_auth.RegisterHook(external_auth.Settings{
	ExternalAuthPath:            "upmeter.status.auth.externalAuthentication",
	DexAuthenticatorEnabledPath: "upmeter.internal.deployStatusDexAuthenticator",
	DexExternalAuth: external_auth.ExternalAuth{
		AuthURL:       "status-dex-authenticator.d8-upmeter.svc.$CLUSTER_DOMAIN%/dex-authenticator/auth",
		AuthSignInURL: "https://$host/dex-authenticator/sign_in",
	},
})

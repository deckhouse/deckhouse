package hooks

import (
	"github.com/deckhouse/deckhouse/go_lib/hooks/external_auth"
)

var _ = external_auth.RegisterHook(external_auth.Settings{
	ExternalAuthPath:            "istio.auth.externalAuthentication",
	DexAuthenticatorEnabledPath: "istio.internal.deployDexAuthenticator",
	DexExternalAuth: external_auth.ExternalAuth{
		AuthURL:       "https://istio-dex-authenticator.d8-dashboard.svc.$CLUSTER_DOMAIN%/dex-authenticator/auth",
		AuthSignInURL: "https://$host/dex-authenticator/sign_in",
	},
})

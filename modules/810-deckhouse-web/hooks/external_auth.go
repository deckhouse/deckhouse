package hooks

import (
	"github.com/deckhouse/deckhouse/go_lib/hooks/external_auth"
)

var _ = external_auth.RegisterHook(external_auth.Settings{
	ExternalAuthPath:            "deckhouseWeb.auth.externalAuthentication",
	DexAuthenticatorEnabledPath: "deckhouseWeb.internal.deployDexAuthenticator",
	DexExternalAuth: external_auth.ExternalAuth{
		AuthURL:       "https://deckhouse-web-dex-authenticator.d8-system.svc.$CLUSTER_DOMAIN%/dex-authenticator/auth",
		AuthSignInURL: "https://$host/dex-authenticator/sign_in",
	},
})

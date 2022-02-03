package hooks

import (
	"github.com/deckhouse/deckhouse/go_lib/hooks/external_auth"
)

var _ = external_auth.RegisterHook(external_auth.Settings{
	ExternalAuthPath:            "openvpn.auth.externalAuthentication",
	DexAuthenticatorEnabledPath: "openvpn.internal.deployDexAuthenticator",
	DexExternalAuth: external_auth.ExternalAuth{
		AuthURL:       "https://openvpn-dex-authenticator.d8-openvpn.svc.$CLUSTER_DOMAIN%/dex-authenticator/auth",
		AuthSignInURL: "https://$host/dex-authenticator/sign_in",
	},
})

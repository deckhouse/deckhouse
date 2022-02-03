package hooks

import (
	"github.com/deckhouse/deckhouse/go_lib/hooks/external_auth"
	"k8s.io/utils/pointer"
)

var _ = external_auth.RegisterHook(external_auth.Settings{
	ExternalAuthPath:            "dashboard.auth.externalAuthentication",
	DexAuthenticatorEnabledPath: "dashboard.internal.deployDexAuthenticator",
	DexExternalAuth: external_auth.ExternalAuth{
		AuthURL:         "https://dashboard-dex-authenticator.d8-dashboard.svc.$CLUSTER_DOMAIN%/dex-authenticator/auth",
		AuthSignInURL:   "https://$host/dex-authenticator/sign_in",
		UseBearerTokens: pointer.BoolPtr(true),
	},
})

package hooks

import "github.com/deckhouse/deckhouse/go_lib/hooks/external_auth"

var _ = external_auth.RegisterHook(external_auth.Settings{
	ExternalAuthPath:            "prometheus.auth.externalAuthentication",
	DexAuthenticatorEnabledPath: "prometheus.internal.deployDexAuthenticator",
	DexExternalAuth: external_auth.ExternalAuth{
		AuthURL:       "https://grafana-dex-authenticator.d8-monitoring.svc.$CLUSTER_DOMAIN%/dex-authenticator/auth",
		AuthSignInURL: "https://$host/dex-authenticator/sign_in",
	},
})

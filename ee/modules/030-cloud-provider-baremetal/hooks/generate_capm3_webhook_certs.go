/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import "github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"

var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	SANs: tls_certificate.DefaultSANs([]string{
		"capm3-webhook-service." + webhookProviderNamespace,
		"capm3-webhook-service." + webhookProviderNamespace + ".svc",
		tls_certificate.ClusterDomainSAN("capm3-webhook-service." + webhookProviderNamespace),
		tls_certificate.ClusterDomainSAN("capm3-webhook-service." + webhookProviderNamespace + ".svc"),
	}),

	CN: "capm3-webhook",

	Namespace:            webhookProviderNamespace,
	TLSSecretName:        "capm3-webhook-service-cert",
	SnapshotName:         "capm3WebhookCertificate",
	FullValuesPathPrefix: "cloudProviderBaremetal.internal.capm3WebhookCert",
})

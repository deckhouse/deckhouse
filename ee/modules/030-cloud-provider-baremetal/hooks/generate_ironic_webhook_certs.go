/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import "github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"

var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	SANs: tls_certificate.DefaultSANs([]string{
		"ironic-standalone-operator-webhook-service." + webhookProviderNamespace,
		"ironic-standalone-operator-webhook-service." + webhookProviderNamespace + ".svc",
		tls_certificate.ClusterDomainSAN("ironic-standalone-operator-webhook-service." + webhookProviderNamespace),
		tls_certificate.ClusterDomainSAN("ironic-standalone-operator-webhook-service." + webhookProviderNamespace + ".svc"),
	}),

	CN: "ironic-standalone-operator-webhook",

	Namespace:            webhookProviderNamespace,
	TLSSecretName:        "irso-webhook-server-cert",
	SnapshotName:         "ironicStandaloneOperatorWebhookCertificate",
	FullValuesPathPrefix: "cloudProviderBaremetal.internal.ironicStandaloneOperatorWebhookCert",
})

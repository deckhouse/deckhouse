/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import "github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"

const webhookProviderNamespace = "d8-cloud-provider-baremetal"

var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	SANs: tls_certificate.DefaultSANs([]string{
		"baremetal-operator-webhook-service." + webhookProviderNamespace,
		"baremetal-operator-webhook-service." + webhookProviderNamespace + ".svc",
		tls_certificate.ClusterDomainSAN("baremetal-operator-webhook-service." + webhookProviderNamespace),
		tls_certificate.ClusterDomainSAN("baremetal-operator-webhook-service." + webhookProviderNamespace + ".svc"),
	}),

	CN: "baremetal-operator-webhook",

	Namespace:            webhookProviderNamespace,
	TLSSecretName:        "bmo-webhook-server-cert",
	SnapshotName:         "baremetalOperatorWebhookCertificate",
	FullValuesPathPrefix: "cloudProviderBaremetal.internal.baremetalOperatorWebhookCert",
})

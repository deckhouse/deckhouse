/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import "github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"

var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	SANs: tls_certificate.DefaultSANs([]string{
		"validation-webhook.d8-cloud-provider-zvirt",
		"validation-webhook.d8-cloud-provider-zvirt.svc",
		tls_certificate.ClusterDomainSAN("validation-webhook.d8-cloud-provider-zvirt"),
		tls_certificate.ClusterDomainSAN("validation-webhook.d8-cloud-provider-zvirt.svc"),
	}),

	CN: "cloud-provider-zvirt-validation-webhook",

	Namespace:            "d8-cloud-provider-zvirt",
	TLSSecretName:        "validation-webhook-tls",
	FullValuesPathPrefix: "cloudProviderZvirt.internal.validationWebhookCert",
})

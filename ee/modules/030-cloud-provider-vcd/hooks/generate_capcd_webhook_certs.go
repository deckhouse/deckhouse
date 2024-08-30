/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
)

var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	SANs: tls_certificate.DefaultSANs([]string{
		"capcd-controller-manager-webhook-service.d8-cloud-provider-vcd",
		"capcd-controller-manager-webhook-service.d8-cloud-provider-vcd.svc",
		tls_certificate.ClusterDomainSAN("capcd-controller-manager-webhook-service.d8-cloud-provider-vcd"),
		tls_certificate.ClusterDomainSAN("capcd-controller-manager-webhook-service.d8-cloud-provider-vcd.svc"),
	}),

	CN: "capcd-controller-manager-webhook",

	Namespace:            "d8-cloud-provider-vcd",
	TLSSecretName:        "capcd-controller-manager-webhook-tls",
	FullValuesPathPrefix: "cloudProviderVcd.internal.capcdControllerManagerWebhookCert",
})

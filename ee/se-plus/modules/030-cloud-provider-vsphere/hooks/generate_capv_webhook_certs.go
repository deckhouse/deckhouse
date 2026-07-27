/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
)

var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	SANs: tls_certificate.DefaultSANs([]string{
		"capv-controller-manager-webhook-service.d8-cloud-provider-vsphere",
		"capv-controller-manager-webhook-service.d8-cloud-provider-vsphere.svc",
		tls_certificate.ClusterDomainSAN("capv-controller-manager-webhook-service.d8-cloud-provider-vsphere"),
		tls_certificate.ClusterDomainSAN("capv-controller-manager-webhook-service.d8-cloud-provider-vsphere.svc"),
	}),

	CN: "capv-controller-manager-webhook",

	Namespace:            "d8-cloud-provider-vsphere",
	TLSSecretName:        "capv-controller-manager-webhook-tls",
	FullValuesPathPrefix: "cloudProviderVsphere.internal.capvControllerManagerWebhookCert",
})

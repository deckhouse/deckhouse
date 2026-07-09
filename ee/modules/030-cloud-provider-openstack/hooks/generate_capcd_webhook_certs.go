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
		"capo-controller-manager-webhook-service.d8-cloud-provider-openstack",
		"capo-controller-manager-webhook-service.d8-cloud-provider-openstack.svc",
		tls_certificate.ClusterDomainSAN("capo-controller-manager-webhook-service.d8-cloud-provider-openstack"),
		tls_certificate.ClusterDomainSAN("capo-controller-manager-webhook-service.d8-cloud-provider-openstack.svc"),
	}),

	CN: "capo-controller-manager-webhook",

	Namespace:            "d8-cloud-provider-openstack",
	TLSSecretName:        "capo-controller-manager-webhook-tls",
	FullValuesPathPrefix: "cloudProviderOpenstack.internal.capoControllerManagerWebhookCert",
})

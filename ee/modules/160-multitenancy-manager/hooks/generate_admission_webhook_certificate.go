/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
)

const (
	multitenancyManagerService = "multitenancy-manager.d8-multitenancy-manager.svc"
)

var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	SANs: tls_certificate.DefaultSANs([]string{
		multitenancyManagerService,
		tls_certificate.ClusterDomainSAN(multitenancyManagerService),
	}),

	CN: multitenancyManagerService,

	Namespace:            "d8-multitenancy-manager",
	TLSSecretName:        "admission-webhook-certs",
	FullValuesPathPrefix: "multitenancyManager.internal.admissionWebhookCert",
})

package hooks

import (
	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
)

var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	SANs: tls_certificate.DefaultSANs([]string{
		"capyandex-controller-manager.d8-cloud-provider-yandex",
		"capyandex-controller-manager.d8-cloud-provider-yandex.svc",
		tls_certificate.ClusterDomainSAN("capyandex-controller-manager.d8-cloud-provider-yandex"),
		tls_certificate.ClusterDomainSAN("capyandex-controller-manager.d8-cloud-provider-yandex.svc"),
	}),

	CN: "capyandex-controller-manager-webhook",

	Namespace:            "d8-cloud-provider-yandex",
	TLSSecretName:        "capyandex-controller-manager-webhook-tls",
	FullValuesPathPrefix: "cloudProviderYandex.internal.capyandexControllerManagerWebhookCert",
})

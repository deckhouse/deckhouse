/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package pricing

import (
	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
)

var _ = tls_certificate.RegisterOrderCertificateHook(
	[]tls_certificate.OrderCertificateRequest{
		{
			Namespace:  "d8-flant-integration",
			SecretName: "pricing-prometheus-api-client-tls",
			CommonName: "d8-flant-integration:flant-integration:prometheus-api-client",
			ValueName:  "internal.prometheusAPIClientTLS",
			Groups:     []string{"prometheus:auth"},
			ModuleName: "flantIntegration",
		},
	},
)

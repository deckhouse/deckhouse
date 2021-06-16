package hooks

import (
	"github.com/deckhouse/deckhouse/go_lib/hooks/order_certificate"
)

var _ = order_certificate.RegisterOrderCertificateHook(
	[]order_certificate.OrderCertificateRequest{
		{
			Namespace:  "d8-flant-pricing",
			SecretName: "flant-pricing-prometheus-api-client-tls",
			CommonName: "d8-flant-pricing:flant-pricing:prometheus-api-client",
			ValueName:  "internal.prometheusAPIClientTLS",
			Group:      "prometheus:auth",
			ModuleName: "flantPricing",
		},
	},
)

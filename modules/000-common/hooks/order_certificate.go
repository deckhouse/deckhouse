package hooks

import (
	"time"

	"github.com/deckhouse/deckhouse/go_lib/hooks/order_certificate"
)

var _ = order_certificate.RegisterOrderCertificateHook(
	[]order_certificate.OrderCertificateRequest{
		{
			Namespace:   "d8-module-name",
			SecretName:  "module-name-auth-tls",
			CommonName:  "d8-module-name:module-name:auth",
			ValueName:   "internal.moduleAuthTLS",
			Group:       "prometheus:auth",
			ModuleName:  "moduleName",
			WaitTimeout: 1 * time.Millisecond,
		},
		{
			Namespace:   "d8-module-name",
			SecretName:  "module-name-access-tls",
			CommonName:  "d8-module-name:module-name:access",
			ValueName:   "internal.moduleAccessTLS",
			Group:       "prometheus:access",
			ModuleName:  "moduleName",
			WaitTimeout: 1 * time.Millisecond,
		},
	},
)

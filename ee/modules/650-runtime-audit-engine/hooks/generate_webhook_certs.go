/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
)

var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	SANs: tls_certificate.DefaultSANs([]string{
		"127.0.0.1",
		"runtime-audit-engine-webhook",
		"runtime-audit-engine-webhook.d8-runtime-audit-engine.svc",
	}),
	CN: "127.0.0.1",

	Namespace:            "d8-runtime-audit-engine",
	TLSSecretName:        "runtime-audit-engine-webhook-tls",
	FullValuesPathPrefix: "runtimeAuditEngine.internal.webhookCertificate",
})

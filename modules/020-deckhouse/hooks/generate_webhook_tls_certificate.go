/*
Copyright 2022 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
)

const (
	webhookServiceHost = "webhook-handler.d8-system.svc"
)

var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	SANs: tls_certificate.DefaultSANs([]string{
		"webhook-handler.d8-system.svc",
		"validating-webhook-handler.d8-system.svc",
		"conversion-webhook-handler.d8-system.svc",
		tls_certificate.ClusterDomainSAN("webhook-handler.d8-system.svc"),
		tls_certificate.ClusterDomainSAN("validating-webhook-handler.d8-system.svc"),
		tls_certificate.ClusterDomainSAN("conversion-webhook-handler.d8-system.svc"),
	}),

	CN: "webhook-handler.d8-system.svc",

	Namespace:            "d8-system",
	TLSSecretName:        "webhook-handler-certs",
	FullValuesPathPrefix: "deckhouse.internal.webhookHandlerCert",
})

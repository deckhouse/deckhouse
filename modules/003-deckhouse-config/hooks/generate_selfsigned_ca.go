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
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
)

const (
	webhookServiceHost      = "deckhouse-config-webhook"
	webhookServiceNamespace = "d8-system"
)

var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	SANs: tls_certificate.DefaultSANs([]string{
		webhookServiceHost,
		fmt.Sprintf(
			"%s.%s.svc",
			webhookServiceHost,
			webhookServiceNamespace,
		),
	}),
	CN: webhookServiceHost,

	Namespace:            webhookServiceNamespace,
	TLSSecretName:        "deckhouse-config-webhook-tls",
	FullValuesPathPrefix: "deckhouseConfig.internal.webhookCert",
})

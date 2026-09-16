/*
Copyright 2026 Flant JSC

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

import "github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"

const webhookProviderNamespace = "d8-cloud-provider-baremetal"

var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	SANs: tls_certificate.DefaultSANs([]string{
		"baremetal-operator-webhook-service." + webhookProviderNamespace,
		"baremetal-operator-webhook-service." + webhookProviderNamespace + ".svc",
		tls_certificate.ClusterDomainSAN("baremetal-operator-webhook-service." + webhookProviderNamespace),
		tls_certificate.ClusterDomainSAN("baremetal-operator-webhook-service." + webhookProviderNamespace + ".svc"),
	}),

	CN: "baremetal-operator-webhook",

	Namespace:            webhookProviderNamespace,
	TLSSecretName:        "bmo-webhook-server-cert",
	FullValuesPathPrefix: "cloudProviderBaremetal.internal.baremetalOperatorWebhookCert",
})

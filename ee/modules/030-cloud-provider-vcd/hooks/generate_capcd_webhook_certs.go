/*
Copyright 2023 Flant JSC

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

var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	SANs: tls_certificate.DefaultSANs([]string{
		"capcd-controller-manager-webhook-service.d8-cloud-provider-vcd",
		"capcd-controller-manager-webhook-service.d8-cloud-provider-vcd.svc",
		tls_certificate.ClusterDomainSAN("capcd-controller-manager-webhook-service.d8-cloud-provider-vcd"),
		tls_certificate.ClusterDomainSAN("capcd-controller-manager-webhook-service.d8-cloud-provider-vcd.svc"),
	}),

	CN: "capcd-controller-manager-webhook",

	Namespace:            "d8-cloud-provider-vcd",
	TLSSecretName:        "capcd-controller-manager-webhook-tls",
	FullValuesPathPrefix: "cloudProviderVcd.internal.capcdControllerManagerWebhookCert",
})

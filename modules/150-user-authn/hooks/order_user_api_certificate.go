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

import (
	certificatesv1 "k8s.io/api/certificates/v1"

	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
)

// The certificate of user-api. Its CA goes to the ConfigMap user-api-ca, so that the console can
// verify user-api before it sends the ID token of a user there.
var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	SANs: tls_certificate.DefaultSANs([]string{
		"user-api.d8-user-authn",
		"user-api.d8-user-authn.svc",
		tls_certificate.ClusterDomainSAN("user-api.d8-user-authn.svc"),
	}),
	CN: "user-api.d8-user-authn",
	Usages: []certificatesv1.KeyUsage{
		certificatesv1.UsageDigitalSignature,
		certificatesv1.UsageKeyEncipherment,
		certificatesv1.UsageServerAuth,
	},
	Namespace:            "d8-user-authn",
	TLSSecretName:        "user-api-tls",
	FullValuesPathPrefix: "userAuthn.internal.userAPITLS",
})

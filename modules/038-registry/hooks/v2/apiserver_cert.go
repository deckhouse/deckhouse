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

package v2

import (
	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
)

// The serving certificate of registry-controller's aggregated API, registry.deckhouse.io: the
// APIService trusts its authority, and the controller serves it from the Secret below. Issued by an
// authority of its own rather than the storage's, which the kube-apiserver has no reason to trust for
// anything else.
var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	SANs: tls_certificate.DefaultSANs([]string{
		"registry-controller.d8-system",
		"registry-controller.d8-system.svc",
		tls_certificate.ClusterDomainSAN("registry-controller.d8-system"),
		tls_certificate.ClusterDomainSAN("registry-controller.d8-system.svc"),
	}),

	CN: "registry-controller",

	Namespace:            "d8-system",
	TLSSecretName:        "registry-controller-apiserver-tls",
	FullValuesPathPrefix: "registry.internal.v2.apiserverCert",
})

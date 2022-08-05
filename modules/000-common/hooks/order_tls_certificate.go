/*
Copyright 2021 Flant JSC

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
	"time"

	certificatesv1 "k8s.io/api/certificates/v1"

	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
)

var _ = tls_certificate.RegisterOrderCertificateHook(
	[]tls_certificate.OrderCertificateRequest{
		{
			Namespace:  "d8-module-name",
			SecretName: "module-name-tls",
			CommonName: "system:node:module-name.d8-module-name",
			SANs: []string{
				"module-name.d8-module-name",
				"module-name.d8-module-name.svc",
			},
			Usages: []certificatesv1.KeyUsage{
				certificatesv1.UsageDigitalSignature,
				certificatesv1.UsageKeyEncipherment,
				certificatesv1.UsageServerAuth,
			},
			Groups:      []string{"system:nodes"},
			SignerName:  certificatesv1.KubeletServingSignerName,
			ValueName:   "internal.moduleTLS",
			ModuleName:  "moduleName",
			WaitTimeout: 1 * time.Millisecond,
		},
	},
)

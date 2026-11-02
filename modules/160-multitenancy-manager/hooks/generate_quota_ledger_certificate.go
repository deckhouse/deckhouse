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
	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
)

const (
	quotaLedgerService = "multitenancy-manager-quota-ledger.d8-multitenancy-manager.svc"
)

// The quota ledger has a certificate of its own. Adding its Service to the SANs of the certificate of
// the other webhooks would make the hook issue that certificate and its CA anew, and every webhook of
// the module would fail closed until the new CA reaches its configuration.
var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	SANs: tls_certificate.DefaultSANs([]string{
		quotaLedgerService,
		tls_certificate.ClusterDomainSAN(quotaLedgerService),
	}),

	CN: quotaLedgerService,

	Namespace:            "d8-multitenancy-manager",
	TLSSecretName:        "quota-ledger-certs",
	FullValuesPathPrefix: "multitenancyManager.internal.quotaLedgerCert",
})

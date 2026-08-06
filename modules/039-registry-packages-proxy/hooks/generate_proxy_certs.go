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
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	certificatesv1 "k8s.io/api/certificates/v1"

	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
)

const (
	proxyCertCN         = "registry-packages-proxy"
	proxyCertSecretName = "registry-packages-proxy-tls"
	proxyCertValuesPath = "registryPackagesProxy.internal.proxyCert"
)

// Issue the certificate kube-rbac-proxy serves on port 4219. Without it the
// sidecar generates its own certificate on every pod start, signed by a
// throwaway CA and holding no addresses, so no client can verify the connection.
//
// The SAN list carries the master addresses, which lets a client connect by IP.
// The library reissues the certificate whenever that list changes.
var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	SANs:          proxyCertSANs,
	CN:            proxyCertCN,
	Namespace:     proxyNamespace,
	TLSSecretName: proxyCertSecretName,

	// Declare "server auth" explicitly. The library's default usage set is meant
	// for client certificates and leaves the extended key usage empty, so the
	// certificate would pass only because verifiers treat an absent extension as
	// "any usage".
	Usages: []certificatesv1.KeyUsage{
		certificatesv1.UsageSigning,
		certificatesv1.UsageKeyEncipherment,
		certificatesv1.UsageServerAuth,
	},

	FullValuesPathPrefix: proxyCertValuesPath,
})

// proxyCertSANs returns every address a client may connect to: the master
// addresses, localhost for node-local calls, and the service names for callers
// inside the cluster.
func proxyCertSANs(ctx context.Context, input *go_hook.HookInput) []string {
	service := proxyAppLabel + "." + proxyNamespace

	sans := []string{
		"127.0.0.1",
		service,
		service + ".svc",
		tls_certificate.ClusterDomainSAN(service),
		tls_certificate.ClusterDomainSAN(service + ".svc"),
	}

	for _, address := range input.Values.Get(proxyAddressesValuesPath).Array() {
		sans = append(sans, address.String())
	}

	return tls_certificate.DefaultSANs(sans)(ctx, input)
}

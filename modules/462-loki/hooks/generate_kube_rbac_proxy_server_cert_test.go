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
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: loki :: hooks :: generate_kube_rbac_proxy_server_cert ::", func() {
	f := HookExecutionConfigInit(`{"loki": {"internal":{"kubeRbacProxyTLS":{}}}, "global": {"discovery": {"clusterDomain": "cluster.local"}, "internal": {"modules": {}}}}`, `{}`)

	logger := log.NewNop()

	It("must generate kube-rbac-proxy server certificate with required DNS SANs", func() {
		selfSignedCA, err := certificate.GenerateCA(logger, "kube-rbac-proxy-ca-key-pair")
		Expect(err).ToNot(HaveOccurred())

		// Put CA into global values as the real global hook does.
		// Keep YAML indentation stable for heredoc.
		kubeRBACProxyCA := fmt.Sprintf(`
kubeRBACProxyCA:
  cert: |
%s
  key: |
%s
`, indentMultiline(selfSignedCA.Cert, 4), indentMultiline(selfSignedCA.Key, 4))
		f.ValuesSetFromYaml("global.internal.modules", []byte(kubeRBACProxyCA))

		f.BindingContexts.Set(f.GenerateBeforeHelmContext())
		f.RunHook()

		Expect(f).To(ExecuteSuccessfully())

		crt := f.ValuesGet("loki.internal.kubeRbacProxyTLS.cert").String()
		Expect(crt).ToNot(BeEmpty())

		block, _ := pem.Decode([]byte(crt))
		Expect(block).ToNot(BeNil())

		x509cert, err := x509.ParseCertificate(block.Bytes)
		Expect(err).ToNot(HaveOccurred())

		expectedSANs := []string{
			"loki",
			"loki.d8-monitoring",
			"loki.d8-monitoring.svc",
			"loki.d8-monitoring.svc.cluster.local",
		}

		Expect(stringSlicesSetEqual(x509cert.DNSNames, expectedSANs)).To(BeTrue(), "unexpected DNS SANs: %v", x509cert.DNSNames)
	})
})

func indentMultiline(s string, spaces int) string {
	indent := strings.Repeat(" ", spaces)
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for i := range lines {
		lines[i] = indent + lines[i]
	}
	return strings.Join(lines, "\n")
}

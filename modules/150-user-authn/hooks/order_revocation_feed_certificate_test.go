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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: order revocation feed certificate ::", func() {
	const (
		valuesPath = "userAuthn.internal.revocationFeedTLS"
		secret     = `
apiVersion: v1
kind: Secret
metadata:
  name: revocation-feed-tls
  namespace: d8-user-authn
data:
  ca.crt: YQo= # a
  tls.crt: Ygo= # b
  tls.key: Ywo= # c
`
	)

	f := HookExecutionConfigInit(`{"global":{"discovery":{"clusterDomain":"cluster.local"}},"userAuthn":{"internal":{}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Generates a certificate of the revocation-feed Service signed by its own CA", func() {
			Expect(f).To(ExecuteSuccessfully())

			pool := x509.NewCertPool()
			Expect(pool.AppendCertsFromPEM([]byte(f.ValuesGet(valuesPath + ".ca").String()))).To(BeTrue())
			Expect(f.ValuesGet(valuesPath + ".key").String()).NotTo(BeEmpty())

			block, _ := pem.Decode([]byte(f.ValuesGet(valuesPath + ".crt").String()))
			Expect(block).NotTo(BeNil())
			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).NotTo(HaveOccurred())

			Expect(cert.Subject.CommonName).To(Equal("revocation-feed.d8-user-authn"))
			Expect(cert.DNSNames).To(ConsistOf(
				"revocation-feed.d8-user-authn",
				"revocation-feed.d8-user-authn.svc",
				"revocation-feed.d8-user-authn.svc.cluster.local",
			))
			Expect(cert.ExtKeyUsage).To(ContainElement(x509.ExtKeyUsageServerAuth))
			for _, name := range cert.DNSNames {
				_, err := cert.Verify(x509.VerifyOptions{DNSName: name, Roots: pool})
				Expect(err).NotTo(HaveOccurred())
			}
			// Not the CA of Dex: dex-authenticator trusts this one for the feed only.
			Expect(f.ValuesGet(valuesPath + ".ca").String()).NotTo(Equal(f.ValuesGet("userAuthn.internal.dexTLS.ca").String()))
		})
	})

	Context("Cluster with the secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(secret))
			f.RunHook()
		})

		It("Keeps the certificate of the secret in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(valuesPath + ".ca").String()).To(Equal("a\n"))
			Expect(f.ValuesGet(valuesPath + ".crt").String()).To(Equal("b\n"))
			Expect(f.ValuesGet(valuesPath + ".key").String()).To(Equal("c\n"))
		})
	})
})

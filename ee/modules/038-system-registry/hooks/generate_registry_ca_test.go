/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"crypto/x509"
	"encoding/pem"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("System Registry hooks :: generate registry ca ::", func() {
	f := HookExecutionConfigInit(`{"systemRegistry":{"internal":{"registryCA":{}}}}`, "")

	Context("Without registry CA secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should generate and add CA certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("systemRegistry.internal.registryCA.key").Exists()).To(BeTrue())

			block, _ := pem.Decode([]byte(f.ValuesGet("systemRegistry.internal.registryCA.cert").String()))
			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).To(BeNil())
			Expect(cert.IsCA).To(BeTrue())
			Expect(cert.Subject.CommonName).To(Equal("embedded-registry-ca"))
		})
	})

	Context("With existing registry CA secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Secret
metadata:
  name: registry-pki
  namespace: d8-system
data:
  registry-ca.crt: dGVzdA==
  registry-ca.key: dGVzdA==
`))
			f.RunHook()
		})

		It("Should add existing CA certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("systemRegistry.internal.registryCA.cert").String()).To(Equal("test"))
			Expect(f.ValuesGet("systemRegistry.internal.registryCA.key").String()).To(Equal("test"))
		})
	})
})

package hooks

import (
	"crypto/x509"
	"encoding/pem"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: generate selfsigned ca ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal":{"selfSignedCA":{}}}}`, "")

	Context("Without secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(BeforeHelmContext)
			f.ValuesSet("userAuthn.publishAPI.enable", true)
			f.ValuesSet("userAuthn.publishAPI.https.mode", "SelfSigned")
			f.RunHook()
		})

		It("Should add ca certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.selfSignedCA.key").Exists()).To(BeTrue())

			block, _ := pem.Decode([]byte(f.ValuesGet("userAuthn.internal.selfSignedCA.cert").String()))
			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).To(BeNil())
			Expect(cert.IsCA).To(BeTrue())
			Expect(cert.Subject.CommonName).To(Equal("kubernetes-api-selfsigned-ca"))
		})
	})
	Context("Without secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Secret
metadata:
  name: kubernetes-api-ca-key-pair
  namespace: d8-user-authn
data:
  tls.crt: dGVzdA==
  tls.key: dGVzdA==
`))
			f.ValuesSet("userAuthn.publishAPI.enable", true)
			f.ValuesSet("userAuthn.publishAPI.https.mode", "SelfSigned")
			f.RunHook()
		})
		It("Should add existing ca certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.selfSignedCA.cert").String()).To(Equal("test"))
			Expect(f.ValuesGet("userAuthn.internal.selfSignedCA.key").String()).To(Equal("test"))
		})

	})
})

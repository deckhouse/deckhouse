package hooks

import (
	"crypto/x509"
	"encoding/pem"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Cert Manager hooks :: generate selfsigned ca ::", func() {
	f := HookExecutionConfigInit(`{"certManager":{"internal":{"selfSignedCA":{}}}}`, "")

	Context("Without secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(BeforeHelmContext)
			f.RunHook()
		})

		It("Should add ca certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("certManager.internal.selfSignedCA.key").Exists()).To(BeTrue())

			block, _ := pem.Decode([]byte(f.ValuesGet("certManager.internal.selfSignedCA.cert").String()))
			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).To(BeNil())
			Expect(cert.IsCA).To(BeTrue())
			Expect(cert.Subject.CommonName).To(Equal("cluster-selfsigned-ca"))
		})
	})
	Context("Without secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Secret
metadata:
  name: selfsigned-ca-key-pair
  namespace: d8-cert-manager
data:
  tls.crt: dGVzdA==
  tls.key: dGVzdA==
`))
			f.RunHook()
		})
		It("Should add existing ca certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("certManager.internal.selfSignedCA.cert").String()).To(Equal("test"))
			Expect(f.ValuesGet("certManager.internal.selfSignedCA.key").String()).To(Equal("test"))
		})

	})
})

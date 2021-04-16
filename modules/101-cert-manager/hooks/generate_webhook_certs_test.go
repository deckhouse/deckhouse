package hooks

import (
	"crypto/x509"
	"encoding/pem"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Cert Manager hooks :: generate_webhook_certs ::", func() {
	f := HookExecutionConfigInit(`{"certManager":{"internal":{}}}`, "")

	Context("Without secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should add ca and certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("certManager.internal.webhookCACrt").Exists()).To(BeTrue())
			Expect(f.ValuesGet("certManager.internal.webhookCAKey").Exists()).To(BeTrue())
			Expect(f.ValuesGet("certManager.internal.webhookCrt").Exists()).To(BeTrue())
			Expect(f.ValuesGet("certManager.internal.webhookKey").Exists()).To(BeTrue())

			blockCA, _ := pem.Decode([]byte(f.ValuesGet("certManager.internal.webhookCACrt").String()))
			certCA, err := x509.ParseCertificate(blockCA.Bytes)
			Expect(err).To(BeNil())
			Expect(certCA.IsCA).To(BeTrue())
			Expect(certCA.Subject.CommonName).To(Equal("cert-manager.webhook.ca"))

			block, _ := pem.Decode([]byte(f.ValuesGet("certManager.internal.webhookCrt").String()))
			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).To(BeNil())
			Expect(cert.IsCA).To(BeFalse())
			Expect(cert.Subject.CommonName).To(Equal("cert-manager-webhook"))
		})
	})
	Context("With secrets", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  name: cert-manager-webhook-ca
  namespace: d8-cert-manager
data:
  ca.crt: dGVzdA==
  tls.crt: dGVzdA==
  tls.key: dGVzdA==
---
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  name: cert-manager-webhook-tls
  namespace: d8-cert-manager
data:
  ca.crt: dGVzdA==
  tls.crt: dGVzdA==
  tls.key: dGVzdA==
`))
			f.RunHook()
		})
		It("Should add existing ca certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("certManager.internal.webhookCACrt").String()).To(Equal("test"))
			Expect(f.ValuesGet("certManager.internal.webhookCAKey").String()).To(Equal("test"))
			Expect(f.ValuesGet("certManager.internal.webhookCrt").String()).To(Equal("test"))
			Expect(f.ValuesGet("certManager.internal.webhookKey").String()).To(Equal("test"))
		})

	})
})

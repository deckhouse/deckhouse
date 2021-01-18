package hooks

/*

User-stories:
1. Webhook mechanism requires a pair of certificates. This hook generates them and stores in values.

*/

import (
	"crypto/x509"
	"encoding/pem"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	initValuesString       = `{"deckhouse":{"internal":{"validatingWebhookHandlerCert":{}}},"global":{"discovery":{"clusterDomain":"mycluster.local"}}}`
	initConfigValuesString = `{}`
)

const (
	stateSecretCreated = `
apiVersion: v1
kind: Secret
metadata:
  name: validating-webhook-handler-certs
  namespace: d8-system
data:
  cert.crt: YQo= # a
  cert.key: Ygo= # b
  ca.crt:   Ywo= # c
`
)

var _ = Describe("Deckhouse hooks :: generate_selfsigned_ca ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("New cert data must be generated and stored to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("deckhouse.internal.validatingWebhookHandlerCert.crt").Exists()).To(BeTrue())
			Expect(f.ValuesGet("deckhouse.internal.validatingWebhookHandlerCert.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("deckhouse.internal.validatingWebhookHandlerCert.ca").Exists()).To(BeTrue())

			certPool := x509.NewCertPool()
			ok := certPool.AppendCertsFromPEM([]byte(f.ValuesGet("deckhouse.internal.validatingWebhookHandlerCert.ca").String()))
			Expect(ok).To(BeTrue())

			block, _ := pem.Decode([]byte(f.ValuesGet("deckhouse.internal.validatingWebhookHandlerCert.crt").String()))
			Expect(block).ShouldNot(BeNil())

			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())

			opts := x509.VerifyOptions{
				DNSName: "validating-webhook-handler.d8-system.svc.mycluster.local",
				Roots:   certPool,
			}
			_, err = cert.Verify(opts)
			Expect(err).ShouldNot(HaveOccurred())
		})

		Context("Secret Created", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateSecretCreated))
				f.RunHook()
			})

			It("Cert data must be stored in values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("deckhouse.internal.validatingWebhookHandlerCert.crt").String()).To(Equal("a"))
				Expect(f.ValuesGet("deckhouse.internal.validatingWebhookHandlerCert.key").String()).To(Equal("b"))
				Expect(f.ValuesGet("deckhouse.internal.validatingWebhookHandlerCert.ca").String()).To(Equal("c"))
			})
		})
	})
})

package hooks

/*

User-stories:
1. Webhook mechanism requires a pair of certificates. This hook generates them and stores in cluster as Secret resource.

*/

import (
	"crypto/x509"
	"encoding/pem"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	initValuesString       = `{"userAuthz":{"internal":{}}}`
	initConfigValuesString = `{}`
)

const (
	stateEmpty = ``

	stateSecretCreated = `
apiVersion: v1
kind: Secret
metadata:
  name: user-authz-webhook
  namespace: d8-user-authz
data:
  ca.crt: YQo=
  webhook-server.crt: Ygo=
  webhook-server.key: Ywo=
`

	stateSecretChanged = `
apiVersion: v1
kind: Secret
metadata:
  name: user-authz-webhook
  namespace: d8-user-authz
data:
  ca.crt: eAo=
  webhook-server.crt: eQo=
  webhook-server.key: ego=
`
)

var _ = Describe("User Authz hooks :: gen webhook certs ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateEmpty))
			f.RunHook()
		})

		It("", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("Secret Created", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateSecretCreated))
				f.RunHook()
			})

			It("Cert data must be stored in values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("userAuthz.internal.webhookCA").String()).To(Equal("a"))
				Expect(f.ValuesGet("userAuthz.internal.webhookServerCrt").String()).To(Equal("b"))
				Expect(f.ValuesGet("userAuthz.internal.webhookServerKey").String()).To(Equal("c"))
			})

			Context("Secret Changed", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(stateSecretChanged))
					f.RunHook()
				})

				It("New cert data must be stored in values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("userAuthz.internal.webhookCA").String()).To(Equal("x"))
					Expect(f.ValuesGet("userAuthz.internal.webhookServerCrt").String()).To(Equal("y"))
					Expect(f.ValuesGet("userAuthz.internal.webhookServerKey").String()).To(Equal("z"))
				})
			})
		})
	})

	Context("Cluster with secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateSecretCreated))
			f.RunHook()
		})

		It("Cert data must be stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.webhookCA").String()).To(Equal("a"))
			Expect(f.ValuesGet("userAuthz.internal.webhookServerCrt").String()).To(Equal("b"))
			Expect(f.ValuesGet("userAuthz.internal.webhookServerKey").String()).To(Equal("c"))
		})
	})

	Context("Empty cluster, onBeforeHelm", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(BeforeHelmContext)
			f.ValuesSet("userAuthz.enableMultiTenancy", true)
			f.RunHook()
		})

		It("New cert data must be generated and stored to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.webhookCA").Exists()).To(BeTrue())
			Expect(f.ValuesGet("userAuthz.internal.webhookServerCrt").Exists()).To(BeTrue())
			Expect(f.ValuesGet("userAuthz.internal.webhookServerKey").Exists()).To(BeTrue())

			certPool := x509.NewCertPool()
			ok := certPool.AppendCertsFromPEM([]byte(f.ValuesGet("userAuthz.internal.webhookCA").String()))
			Expect(ok).To(BeTrue())

			block, _ := pem.Decode([]byte(f.ValuesGet("userAuthz.internal.webhookServerCrt").String()))
			Expect(block).ShouldNot(BeNil())

			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())

			opts := x509.VerifyOptions{
				DNSName: "127.0.0.1",
				Roots:   certPool,
			}

			_, err = cert.Verify(opts)
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})

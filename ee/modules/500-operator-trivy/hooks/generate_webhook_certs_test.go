/*
Copyright 2023 Flant JSC
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

const (
	initValuesString       = `{"operatorTrivy":{"linkCVEtoBDU": false, "internal":{"reportUpdater":{}}}}`
	initConfigValuesString = `{}`
)

const (
	stateSecretCreated = `
apiVersion: v1
kind: Secret
metadata:
  name: report-updater-webhook-ssl
  namespace: d8-operator-trivy
data:
  ca.crt: YQo= # a
  tls.crt: Ygo= # b
  tls.key: Ywo= # c
`

	stateSecretChanged = `
apiVersion: v1
kind: Secret
metadata:
  name: report-updater-webhook-ssl
  namespace: d8-operator-trivy
data:
  ca.crt: eAo= # x
  tls.crt: eQo= # y
  tls.key: ego= # z
`
)

var _ = Describe("Operator trivy hooks :: gen webhook certs ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
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
				Expect(f.ValuesGet("operatorTrivy.internal.reportUpdater.webhookCertificate.ca").String()).To(Equal("a\n"))
				Expect(f.ValuesGet("operatorTrivy.internal.reportUpdater.webhookCertificate.crt").String()).To(Equal("b\n"))
				Expect(f.ValuesGet("operatorTrivy.internal.reportUpdater.webhookCertificate.key").String()).To(Equal("c\n"))
			})

			Context("Secret Changed", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(stateSecretChanged))
					f.RunHook()
				})

				It("New cert data must be stored in values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("operatorTrivy.internal.reportUpdater.webhookCertificate.ca").String()).To(Equal("x\n"))
					Expect(f.ValuesGet("operatorTrivy.internal.reportUpdater.webhookCertificate.crt").String()).To(Equal("y\n"))
					Expect(f.ValuesGet("operatorTrivy.internal.reportUpdater.webhookCertificate.key").String()).To(Equal("z\n"))
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
			Expect(f.ValuesGet("operatorTrivy.internal.reportUpdater.webhookCertificate.ca").String()).To(Equal("a\n"))
			Expect(f.ValuesGet("operatorTrivy.internal.reportUpdater.webhookCertificate.crt").String()).To(Equal("b\n"))
			Expect(f.ValuesGet("operatorTrivy.internal.reportUpdater.webhookCertificate.key").String()).To(Equal("c\n"))
		})
	})

	Context("Empty cluster with linkCVEtoBDU, onBeforeHelm", func() {
		BeforeEach(func() {
			// TODO we need to unset cluster state between contexts.
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("operatorTrivy.linkCVEtoBDU", true)
			f.RunHook()
		})

		It("New cert data must be generated and stored to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("operatorTrivy.internal.reportUpdater.webhookCertificate.ca").Exists()).To(BeTrue())
			Expect(f.ValuesGet("operatorTrivy.internal.reportUpdater.webhookCertificate.crt").Exists()).To(BeTrue())
			Expect(f.ValuesGet("operatorTrivy.internal.reportUpdater.webhookCertificate.key").Exists()).To(BeTrue())

			certPool := x509.NewCertPool()
			ok := certPool.AppendCertsFromPEM([]byte(f.ValuesGet("operatorTrivy.internal.reportUpdater.webhookCertificate.ca").String()))
			Expect(ok).To(BeTrue())

			block, _ := pem.Decode([]byte(f.ValuesGet("operatorTrivy.internal.reportUpdater.webhookCertificate.crt").String()))
			Expect(block).ShouldNot(BeNil())

			_, err := x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})

/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

/*

User-stories:
Webhook mechanism requires a pair of certificates. This hook generates them and stores in cluster as Secret resource.

*/

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	initValuesString       = `{"runtimeAuditEngine":{"internal":{}}}`
	initConfigValuesString = `{}`
)

const (
	stateSecretCreated = `
apiVersion: v1
kind: Secret
metadata:
  name: runtime-audit-engine-webhook-tls
  namespace: d8-runtime-audit-engine
data:
  ca.crt: YQo= # a
  tls.crt: Ygo= # b
  tls.key: Ywo= # c
`

	stateSecretChanged = `
apiVersion: v1
kind: Secret
metadata:
  name: runtime-audit-engine-webhook-tls
  namespace: d8-runtime-audit-engine
data:
  ca.crt: eAo= # x
  tls.crt: eQo= # y
  tls.key: ego= # z
`
)

var _ = Describe("Runtime Audit Engine hooks :: gen webhook certs ::", func() {
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
				Expect(f.ValuesGet("runtimeAuditEngine.internal.webhookCertificate.ca").String()).To(Equal("a\n"))
				Expect(f.ValuesGet("runtimeAuditEngine.internal.webhookCertificate.crt").String()).To(Equal("b\n"))
				Expect(f.ValuesGet("runtimeAuditEngine.internal.webhookCertificate.key").String()).To(Equal("c\n"))
			})

			Context("Secret Changed", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(stateSecretChanged))
					f.RunHook()
				})

				It("New cert data must be stored in values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("runtimeAuditEngine.internal.webhookCertificate.ca").String()).To(Equal("x\n"))
					Expect(f.ValuesGet("runtimeAuditEngine.internal.webhookCertificate.crt").String()).To(Equal("y\n"))
					Expect(f.ValuesGet("runtimeAuditEngine.internal.webhookCertificate.key").String()).To(Equal("z\n"))
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
			Expect(f.ValuesGet("runtimeAuditEngine.internal.webhookCertificate.ca").String()).To(Equal("a\n"))
			Expect(f.ValuesGet("runtimeAuditEngine.internal.webhookCertificate.crt").String()).To(Equal("b\n"))
			Expect(f.ValuesGet("runtimeAuditEngine.internal.webhookCertificate.key").String()).To(Equal("c\n"))
		})
	})
})

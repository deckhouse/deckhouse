/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const webhook = `
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: d8-runtime-audit-engine.deckhouse.io
webhooks: {}
`

var _ = Describe("Runtime Audit Engine hooks :: cleanup validating webhook ::", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)
	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})
	Context("valdiating webhook exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(webhook))
			f.RunHook()
		})
		It("must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "d8-runtime-audit-engine.deckhouse.io").Exists()).To(BeFalse())
		})
	})
})

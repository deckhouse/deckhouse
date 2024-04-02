/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package pricing

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Flant integration :: hooks :: envs_from_deckhouse_secret ", func() {
	f := HookExecutionConfigInit(`{"flantIntegration":{"internal":{}}}`, `{}`)

	Context("Without deckhouse-discovery secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(``, 0))
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("flantIntegration.internal.bundle").Exists()).To(BeFalse())
		})
	})

	Context("With deckhouse-discovery secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
apiVersion: v1
data:
  bundle: RGVmYXVsdA==
  releaseChannel: QWxwaGE=
kind: Secret
metadata:
  labels:
    heritage: deckhouse
  name: deckhouse-discovery
  namespace: d8-system
type: Opaque
`, 0))
			f.RunHook()
		})

		It("Should fill flantIntegration internal", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("flantIntegration.internal.bundle").String()).To(Equal(`Default`))
			Expect(f.ValuesGet("flantIntegration.internal.releaseChannel").String()).To(Equal(`Alpha`))
		})
	})
})

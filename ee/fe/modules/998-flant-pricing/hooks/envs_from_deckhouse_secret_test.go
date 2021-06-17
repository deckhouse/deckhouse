package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: flant-pricing :: hooks :: envs_from_deckhouse_secret ", func() {
	f := HookExecutionConfigInit(`{"flantPricing":{"internal":{}}}`, `{}`)

	Context("Without d8-deckhouse-flant-pricing secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(``, 0))
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("flantPricing.internal.bundle").Exists()).To(BeFalse())
		})
	})

	Context("With d8-deckhouse-flant-pricing secret", func() {
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
  name: d8-deckhouse-flant-pricing
  namespace: d8-system
type: Opaque
`, 0))
			f.RunHook()
		})

		It("Should fill flantPricing internal", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("flantPricing.internal.bundle").String()).To(Equal(`Default`))
			Expect(f.ValuesGet("flantPricing.internal.releaseChannel").String()).To(Equal(`Alpha`))
		})
	})
})

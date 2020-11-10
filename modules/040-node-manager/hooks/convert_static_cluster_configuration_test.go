package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: static cluster configuration ", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{"static":{}}}}`, `{}`)

	Context("Without configuration", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(``, 0))
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.static.internalNetworkCIDRs").Exists()).To(BeFalse())
		})
	})

	Context("With configuration", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
apiVersion: v1
data:
  static-cluster-configuration.yaml: YXBpVmVyc2lvbjogZGVja2hvdXNlLmlvL3YxYWxwaGExCmtpbmQ6IFN0YXRpY0NsdXN0ZXJDb25maWd1cmF0aW9uCmludGVybmFsTmV0d29ya0NJRFJzOgotIDE5Mi4xNjguMTk5LjAvMjQK
kind: Secret
metadata:
  labels:
    heritage: deckhouse
  name: d8-static-configuration
  namespace: kube-system
type: Opaque
`, 0))
			f.RunHook()
		})

		It("Should fill internalNetworkCIDRs", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.static.internalNetworkCIDRs").String()).To(MatchJSON(`["192.168.199.0/24"]`))
		})
	})
})

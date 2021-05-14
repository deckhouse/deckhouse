package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Prometheus hooks :: detect max vpa ::", func() {
	f := HookExecutionConfigInit(`
prometheus:
  internal:
    vpa: {}
`, ``)

	Context("1 node clustaer", func() {
		BeforeEach(func() {

			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Node
metadata:
  name: test-master-0
spec:
  podCIDR: 10.111.0.0/24
status:
  capacity:
    pods: "110"
`))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))
			f.RunHook()
		})

		It("should fill internal vpa values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.vpa.maxCPU").String()).Should(BeEquivalentTo("2200m"))
			Expect(f.ValuesGet("prometheus.internal.vpa.maxMemory").String()).Should(BeEquivalentTo("1650Mi"))
			Expect(f.ValuesGet("prometheus.internal.vpa.longtermMaxCPU").String()).Should(BeEquivalentTo("733m"))
			Expect(f.ValuesGet("prometheus.internal.vpa.longtermMaxMemory").String()).Should(BeEquivalentTo("550Mi"))
		})
	})
})

package hooks

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Modules :: upmeter :: hooks :: smoke_mini_rescheduler ::", func() {
	f := HookExecutionConfigInit(`{"upmeter":{"internal":{"smokeMini":{"sts":{"a":{},"b":{},"c":{}}}}}}`, `{}`)

	Context("One node", func() {
		state := `
---
apiVersion: v1
kind: Node
metadata:
  labels:
    kubernetes.io/hostname: node-a-1
  name: node-a-1
status:
  conditions:
  - status: "True"
    type: Ready
`
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})
})

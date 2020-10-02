package hooks

import (
	. "github.com/deckhouse/deckhouse/testing/hooks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Global hooks :: migrate/domain ::", func() {

	const (
		initValuesString       = `{}`
		initConfigValuesString = `{}`
	)

	const (
		stateNodeWithOldLabels = `
---
apiVersion: v1
kind: Node
metadata:
  labels:
    node-role.flant.com/system: ""
    node-role.flant.com/frontend: ""
    node-role.flant.com/whatever: ""
  name: node-0
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with Node having flant.com labels", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNodeWithOldLabels))
			f.RunHook()
		})

		It("Hook must not fail; missing labels must be added", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Node", "node-0").Field(`metadata.labels.node-role\.deckhouse\.io/system`).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("Node", "node-0").Field(`metadata.labels.node-role\.deckhouse\.io/frontend`).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("Node", "node-0").Field(`metadata.labels.node-role\.deckhouse\.io/whatever`).Exists()).To(BeFalse())
		})
	})

})

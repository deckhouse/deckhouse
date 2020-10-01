package migrate

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Global hooks :: resources/node_role ::", func() {

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
		stateNodeGroupWithOldLabels = `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: ng
spec:
  nodeTemplate:
    labels:
      node-role.flant.com/system: ""
      node-role.flant.com/frontend: ""
      node-role.flant.com/whatever: ""
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "NodeGroup", false)

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

	Context("Cluster with NodeGroup having flant.com labels", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNodeGroupWithOldLabels))
			f.RunHook()
		})

		It("Hook must not fail; missing labels must be added", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("NodeGroup", "ng").Field(`spec.nodeTemplate.labels.node-role\.deckhouse\.io/system`).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("NodeGroup", "ng").Field(`spec.nodeTemplate.labels.node-role\.deckhouse\.io/frontend`).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("NodeGroup", "ng").Field(`spec.nodeTemplate.labels.node-role\.deckhouse\.io/whatever`).Exists()).To(BeFalse())
		})
	})
})

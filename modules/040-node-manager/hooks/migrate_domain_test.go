package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: migrate_domain ::", func() {

	const (
		initValuesString       = `{}`
		initConfigValuesString = `{}`
	)

	const (
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

})

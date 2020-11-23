package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: flant-pricing :: hooks :: envs_from_ngs", func() {
	f := HookExecutionConfigInit(`{"flantPricing":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "NodeGroup", false)

	Context("Not all managed nodes are up to date", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: test-1
spec:
  nodeType: Static
status:
  nodes: 1
  ready: 1
  upToDate: 0
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: test-2
spec:
  nodeType: Static
status:
  nodes: 3
  ready: 3
  upToDate: 3
`))
			f.RunHook()
		})

		It("flantPricing.internal.allManagedNodesUpToDate is false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("flantPricing.internal.allManagedNodesUpToDate").String()).To(Equal(`false`))
		})
	})

	Context("All managed nodes are up to date", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: test-1
spec:
  nodeType: Static
status:
  nodes: 1
  ready: 1
  upToDate: 1
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: test-2
spec:
  nodeType: Static
status:
  nodes: 3
  ready: 3
  upToDate: 3
`))
			f.RunHook()
		})

		It("flantPricing.internal.allManagedNodesUpToDate is true", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("flantPricing.internal.allManagedNodesUpToDate").String()).To(Equal(`true`))
		})
	})
})

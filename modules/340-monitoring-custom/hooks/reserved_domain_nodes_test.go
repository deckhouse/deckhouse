package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: monitoring-custom :: hooks :: reserved_domain_nodes ::", func() {
	const (
		properResources = `
---
apiVersion: v1
kind: Node
metadata:
  name: system
  labels:
    node-role.deckhouse.io/system: ""
spec:
  taints:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    value: system
`
		resourcesWithReservedLabels = `
---
apiVersion: v1
kind: Node
metadata:
  name: stateful
  labels:
    node-role.deckhouse.io/stateful: ""
`
		resourcesWithReservedTaints = `
---
apiVersion: v1
kind: Node
metadata:
  name: stateful
spec:
  taints:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    value: stateful
`
	)
	f := HookExecutionConfigInit(
		`{"monitoringKubernetes":{"internal":{}},"global":{"enabledModules":[]}}`,
		`{}`,
	)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster containing proper Node", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(properResources))
			f.RunHook()
		})

		It("Hook must not fail, no metrics should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.nodes.0.filterResult.labels").Exists()).To(BeFalse())
		})
	})

	Context("Cluster with Node having reserved `metadata.labels`", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(resourcesWithReservedLabels))
			f.RunHook()
		})

		It("Hook must not fail, metrics must render", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.nodes.0.filterResult.labels").String()).To(MatchJSON(`{"name":"stateful"}`))
		})
	})

	Context("Cluster with Node having reserved `spec.taints`", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(resourcesWithReservedTaints))
			f.RunHook()
		})

		It("Hook must not fail, metrics must render", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.nodes.0.filterResult.labels").String()).To(MatchJSON(`{"name":"stateful"}`))
		})
	})

})

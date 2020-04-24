package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: remove_csi_taints ::", func() {
	const (
		stateNodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-1
spec:
  taints:
  - key: somekey-1
  - effect: NoSchedule
    key: node.deckhouse.io/csi-not-bootstrapped
    value: ""
---
apiVersion: v1
kind: Node
metadata:
  name: node-2
spec:
  taints:
  - key: somekey-2
  - effect: NoSchedule
    key: node.deckhouse.io/csi-not-bootstrapped
    value: ""
---
apiVersion: v1
kind: Node
metadata:
  name: node-3
spec:
  taints:
  - key: somekey-3
---
apiVersion: v1
kind: Node
metadata:
  name: node-4
spec: {}
---
apiVersion: v1
kind: Node
metadata:
  name: node-5
spec: {}
`
		stateCSINode1 = `
---
apiVersion: storage.k8s.io/v1beta1
kind: CSINode
metadata:
  name: node-1
`
		stateCSINode2 = `
---
apiVersion: storage.k8s.io/v1beta1
kind: CSINode
metadata:
  name: node-2
`
		stateCSINode4 = `
---
apiVersion: storage.k8s.io/v1beta1
kind: CSINode
metadata:
  name: node-4
`
	)

	f := HookExecutionConfigInit(`{"nodeManager":{"internal": {}}}`, `{}`)

	Context("Cluster is empty", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster has five nodes and single CSINode", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNodes + stateCSINode1))
			f.RunHook()
		})

		It("node-1 must lose taint 'node.deckhouse.io/csi-not-bootstrapped'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Node", "", "node-1").Field("spec.taints").String()).To(MatchJSON(`[{"key":"somekey-1"}]`))
			Expect(f.KubernetesResource("Node", "", "node-2").Field("spec.taints").String()).To(MatchJSON(`[{"key":"somekey-2"},{"effect":"NoSchedule","key":"node.deckhouse.io/csi-not-bootstrapped","value":""}]`))
			Expect(f.KubernetesResource("Node", "", "node-3").Field("spec.taints").String()).To(MatchJSON(`[{"key":"somekey-3"}]`))
		})

		Context("CSINode for node-2 added", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateNodes + stateCSINode1 + stateCSINode2))
				f.RunHook()
			})

			It("node-2 must lose taint 'node.deckhouse.io/csi-not-bootstrapped'", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.KubernetesResource("Node", "", "node-1").Field("spec.taints").String()).To(MatchJSON(`[{"key":"somekey-1"}]`))
				Expect(f.KubernetesResource("Node", "", "node-2").Field("spec.taints").String()).To(MatchJSON(`[{"key":"somekey-2"}]`))
				Expect(f.KubernetesResource("Node", "", "node-3").Field("spec.taints").String()).To(MatchJSON(`[{"key":"somekey-3"}]`))
			})
		})

		Context("CSINode for node-4 added", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateNodes + stateCSINode1 + stateCSINode4))
				f.RunHook()
			})

			It("node-4 must not get spec.taints", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.KubernetesResource("Node", "", "node-1").Field("spec.taints").String()).To(MatchJSON(`[{"key":"somekey-1"}]`))
				Expect(f.KubernetesResource("Node", "", "node-2").Field("spec.taints").String()).To(MatchJSON(`[{"key": "somekey-2"},{"effect": "NoSchedule","key": "node.deckhouse.io/csi-not-bootstrapped","value": ""}]`))
				Expect(f.KubernetesResource("Node", "", "node-3").Field("spec.taints").String()).To(MatchJSON(`[{"key":"somekey-3"}]`))
				Expect(f.KubernetesResource("Node", "", "node-4").Field("spec.taints").Exists()).To(BeFalse())

			})
		})

	})
})

/*
Copyright 2021 Flant JSC
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: nodeManager :: hooks :: remove_bashible_completed_labels_and_taints ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should execute successfully with no nodes", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Node", "node1").Exists()).To(BeFalse())
		})
	})

	Context("Node with bashible label but no taint", func() {
		BeforeEach(func() {
			state := `
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    node.deckhouse.io/bashible-first-run-finished: ""
spec: {}
`
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("Should remove bashible label but not modify taints", func() {
			Expect(f).To(ExecuteSuccessfully())
			node := f.KubernetesGlobalResource("Node", "node1")
			Expect(node.Field("metadata.labels").Map()).NotTo(HaveKey(BashibleFirstRunFinishedLabel))
			Expect(node.Field("spec.taints").Array()).To(BeEmpty())
		})
	})

	Context("Node with bashible taint but no label", func() {
		BeforeEach(func() {
			state := `
---
apiVersion: v1
kind: Node
metadata:
  name: node1
spec:
  taints:
  - key: node.deckhouse.io/bashible-uninitialized
    effect: NoSchedule
`
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("Should not modify the node", func() {
			Expect(f).To(ExecuteSuccessfully())
			node := f.KubernetesGlobalResource("Node", "node1")
			Expect(node.Field("spec.taints").Array()).To(HaveLen(1))
			Expect(node.Field("spec.taints.0.key").String()).To(Equal(BashibleUninitializedTaintKey))
		})
	})

	Context("Node with both bashible label and taint", func() {
		BeforeEach(func() {
			state := `
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    node.deckhouse.io/bashible-first-run-finished: ""
    example-label: "value"
spec:
  taints:
  - key: node.deckhouse.io/bashible-uninitialized
    effect: NoSchedule
  - key: example-taint
    effect: NoExecute
`
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("Should remove both bashible label and taint", func() {
			Expect(f).To(ExecuteSuccessfully())
			node := f.KubernetesGlobalResource("Node", "node1")

			// Verify label removal
			labels := node.Field("metadata.labels").Map()
			Expect(labels).NotTo(HaveKey(BashibleFirstRunFinishedLabel))
			Expect(labels["example-label"].String()).To(Equal("value"))

			// Verify taint removal
			taints := node.Field("spec.taints").Array()
			Expect(taints).To(HaveLen(1))
			Expect(taints[0].Get("key").String()).To(Equal("example-taint"))
		})
	})

	Context("Multiple nodes with different bashible states", func() {
		BeforeEach(func() {
			state := `
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    node.deckhouse.io/bashible-first-run-finished: ""
spec:
  taints:
  - key: node.deckhouse.io/bashible-uninitialized
    effect: NoSchedule
---
apiVersion: v1
kind: Node
metadata:
  name: node2
  labels:
    node.deckhouse.io/bashible-first-run-finished: ""
spec: {}
---
apiVersion: v1
kind: Node
metadata:
  name: node3
spec:
  taints:
  - key: node.deckhouse.io/bashible-uninitialized
    effect: NoSchedule
`
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("Should process each node correctly", func() {
			Expect(f).To(ExecuteSuccessfully())

			// Node1 - both label and taint should be removed
			node1 := f.KubernetesGlobalResource("Node", "node1")
			Expect(node1.Field("metadata.labels").Map()).NotTo(HaveKey(BashibleFirstRunFinishedLabel))
			Expect(node1.Field("spec.taints").Array()).To(BeEmpty())

			// Node2 - only label should be removed
			node2 := f.KubernetesGlobalResource("Node", "node2")
			Expect(node2.Field("metadata.labels").Map()).NotTo(HaveKey(BashibleFirstRunFinishedLabel))
			Expect(node2.Field("spec.taints").Array()).To(BeEmpty())

			// Node3 - no changes expected
			node3 := f.KubernetesGlobalResource("Node", "node3")
			Expect(node3.Field("metadata.labels").Map()).NotTo(HaveKey(BashibleFirstRunFinishedLabel))
			Expect(node3.Field("spec.taints").Array()).To(HaveLen(1))
		})
	})
})

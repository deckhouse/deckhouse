/*
Copyright 2021 Flant CJSC

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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: nodeManager :: hooks :: update_approval_draining ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(``, 1))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster node is draining", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: v1
kind: Node
metadata:
  name: wor-ker
  labels:
    node.deckhouse.io/group: "master"
  annotations:
    update.node.deckhouse.io/draining: ""
`, 1))
			f.RunHook()
		})

		It("Must be drained", func() {
			Expect(f).To(ExecuteSuccessfully())
			node := f.KubernetesGlobalResource("Node", "wor-ker")
			Expect(node.Field("metadata.annotations.\"update.node.deckhouse.io/drained\"").String()).To(BeEmpty())
			Expect(node.Field("metadata.annotations.\"update.node.deckhouse.io/draining\"").Exists()).To(BeFalse())
		})
	})

	Context("draining_nodes", func() {
		var initialState = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
status:
  desired: 1
  ready: 1
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: undisruptable-worker
spec:
  nodeType: Static
  disruptions:
    approvalMode: Manual
status:
  desired: 1
  ready: 1
---
apiVersion: v1
kind: Secret
metadata:
  name: configuration-checksums
  namespace: d8-cloud-instance-manager
data:
  worker: dXBkYXRlZA== # updated
  undisruptable-worker: dXBkYXRlZA== # updated
`
		var nodeNames = []string{"worker-1", "worker-2", "worker-3"}
		for _, gDraining := range []bool{true, false} {
			for _, gUnschedulable := range []bool{true, false} {
				Context(fmt.Sprintf("Draining: %t, Unschedulable: %t", gDraining, gUnschedulable), func() {
					draining := gDraining
					unschedulable := gUnschedulable
					BeforeEach(func() {
						f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(initialState+generateStateToTestDrainingNodes(nodeNames, draining, unschedulable), 4))
						f.RunHook()
					})

					It("Works as expected", func() {
						Expect(f).To(ExecuteSuccessfully())
						for _, nodeName := range nodeNames {
							if draining {
								By(fmt.Sprintf("%s must have /drained", nodeName), func() {
									Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`metadata.annotations.update\.node\.deckhouse\.io/drained`).Exists()).To(BeTrue())
								})

								By(fmt.Sprintf("%s must not have /draining", nodeName), func() {
									Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`metadata.annotations.update\.node\.deckhouse\.io/draining`).Exists()).To(BeFalse())
								})
							} else {
								By(fmt.Sprintf("%s must not have /drained", nodeName), func() {
									Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`metadata.annotations.update\.node\.deckhouse\.io/drained`).Exists()).To(BeFalse())
								})

								if unschedulable {
									By(fmt.Sprintf("%s must be unschedulable", nodeName), func() {
										Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`spec.unschedulable`).Exists()).To(BeTrue())
										Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`spec.unschedulable`).String()).To(Equal("true"))
									})
								} else {
									By(fmt.Sprintf("%s must not be unschedulable", nodeName), func() {
										Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`spec.unschedulable`).Exists()).To(BeFalse())
									})
								}
							}
						}
					})
				})
			}
		}
	})
})

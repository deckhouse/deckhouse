/*
Copyright 2023 Flant JSC

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
	"bytes"
	"fmt"
	"os"
	"text/template"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: nodeManager :: hooks :: update_approval ::", func() {
	var (
		initialState = `
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
	)
	nodeNames := []string{"worker-1", "worker-2", "worker-3"}

	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "Instance", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("approve_updates", func() {
		for _, gOneIsApproved := range []bool{true, false} {
			for _, gWaitingForApproval := range []bool{true, false} {
				for _, gNodeReady := range []bool{true, false} {
					for _, gNgReady := range []bool{true, false} {
						for _, gNodeType := range []string{"CloudEphemeral", "CloudPermanent", "Static"} {
							Context(fmt.Sprintf("Approved: %t, AproveRequired: %v, NodeReady: %t, NgReady: %t, NodeType: %s", gOneIsApproved, gWaitingForApproval, gNodeReady, gNgReady, gNodeType), func() {
								oneIsApproved := gOneIsApproved
								waitingForApproval := gWaitingForApproval
								nodeReady := gNodeReady
								ngReady := gNgReady
								nodeType := gNodeType

								BeforeEach(func() {
									f.BindingContexts.Set(f.KubeStateSet(initialState + generateStateToTestApproveUpdates(nodeNames, oneIsApproved, waitingForApproval, nodeReady, ngReady, nodeType)))
									f.RunHook()
								})

								It("Works as expected", func() {
									Expect(f).To(ExecuteSuccessfully())

									approvedReadyCount := 0
									approvedNotReadyCount := 0
									waitingForApprovalCount := 0
									for i := 1; i <= len(nodeNames); i++ {
										if f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists() {
											if f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`status.conditions.0.status`).String() == "True" {
												approvedReadyCount++
											} else {
												approvedNotReadyCount++
											}
										}
										if f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/waiting-for-approval`).Exists() {
											waitingForApprovalCount++
										}
									}

									if oneIsApproved {
										By("If 1 node was approved – it should stay approved", func() {
											Expect(approvedReadyCount + approvedNotReadyCount).To(Equal(1))
										})
									} else if waitingForApproval {
										if ngReady && nodeReady {
											By("If ng desired==ready and all nodes ready – 1 node should be approved", func() {
												Expect(approvedReadyCount).To(Equal(1))
												Expect(approvedNotReadyCount).To(Equal(0))
												Expect(waitingForApprovalCount).To(Equal(len(nodeNames) - 1))
											})
										} else if !ngReady && nodeReady && nodeType != "CloudEphemeral" {
											By("If nodeType is not Cloud, ng will not have desired field, so if all existing nodes are ready – 1 node should be approved", func() {
												Expect(approvedReadyCount).To(Equal(1))
												Expect(approvedNotReadyCount).To(Equal(0))
												Expect(waitingForApprovalCount).To(Equal(len(nodeNames) - 1))
											})
										} else if !ngReady && nodeReady && nodeType == "CloudEphemeral" {
											By("If ng desired != ready, but all existing nodes are ready – there should be no approved nodes", func() {
												Expect(approvedReadyCount + approvedNotReadyCount).To(Equal(0))
												Expect(waitingForApprovalCount).To(Equal(len(nodeNames)))
											})
										} else if !nodeReady {
											By("If there are not ready nodes – one of them should be approved", func() {
												Expect(approvedReadyCount).To(Equal(0))
												Expect(approvedNotReadyCount).To(Equal(1))
												Expect(waitingForApprovalCount).To(Equal(len(nodeNames) - 1))
											})
										} else {
											By("Something went wrong!", func() {
												Expect(true).To(BeFalse())
											})
										}

									} else {
										By("If there was no approved before and no nodes are waiting for approval – there should be no approved nodes", func() {
											Expect(approvedReadyCount + approvedNotReadyCount).To(Equal(0))
										})
									}
								})
							})
						}
					}
				}
			}
		}
	})

	Context("approve_disruptions", func() {
		for _, gDisruptionRequired := range []bool{true, false} {
			for _, gDisruptionsApprovalMode := range []string{"Manual", "Automatic", "RollingUpdate"} {
				for _, gDisruptionsDrainBeforeApproval := range []string{"false", "true", ""} {
					for _, gUnschedulable := range []bool{true, false} {
						Context(fmt.Sprintf("DisruptionRequired: %t, DisruptionsApprovalMode: %v, DisruptionsDrainBeforeApproval: %v, Unschedulable: %t", gDisruptionRequired, gDisruptionsApprovalMode, gDisruptionsDrainBeforeApproval, gUnschedulable), func() {
							disruptionRequired := gDisruptionRequired
							disruptionsApprovalMode := gDisruptionsApprovalMode
							disruptionsDrainBeforeApproval := gDisruptionsDrainBeforeApproval
							unschedulable := gUnschedulable
							BeforeEach(func() {
								f.BindingContexts.Set(f.KubeStateSet(initialState + generateStateToTestApproveDisruptions(nodeNames, disruptionRequired, disruptionsApprovalMode, disruptionsDrainBeforeApproval, unschedulable)))
								f.RunHook()
							})

							It("Works as expected", func() {
								Expect(f).To(ExecuteSuccessfully())
								for _, nodeName := range nodeNames {
									if disruptionRequired && disruptionsApprovalMode != "Manual" {
										if disruptionsDrainBeforeApproval != "false" {
											if unschedulable {
												By(fmt.Sprintf("%s must not have /disruption-required", nodeName), func() {
													Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-required`).Exists()).To(BeFalse())
												})
												By(fmt.Sprintf("%s must have /disruption-approved", nodeName), func() {
													Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-approved`).Exists()).To(BeTrue())
												})
											} else {
												By(fmt.Sprintf("%s must have /disruption-required", nodeName), func() {
													Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-required`).Exists()).To(BeTrue())
												})
												By(fmt.Sprintf("%s must have /draining", nodeName), func() {
													Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`metadata.annotations.update\.node\.deckhouse\.io/draining`).Exists()).To(BeTrue())
												})
											}
										} else {
											By(fmt.Sprintf("%s must not have /disruption-required", nodeName), func() {
												Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-required`).Exists()).To(BeFalse())
											})
											By(fmt.Sprintf("%s must have /disruption-approved", nodeName), func() {
												Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-approved`).Exists()).To(BeTrue())
											})
										}
									} else {
										if disruptionRequired {
											By(fmt.Sprintf("%s must have /disruption-required", nodeName), func() {
												Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-required`).Exists()).To(BeTrue())
											})
										}
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
			}
		}
	})

	Context("Skipping drain before approve in automatic mode with enabled draining before approve", func() {
		assertNodeApproved := func(f *HookExecutionConfig, nodeName string) {
			n := f.KubernetesGlobalResource("Node", nodeName)
			approveAnnotate := n.Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-approved`)
			Expect(approveAnnotate.Exists()).To(BeTrue())

			disruptionReqAnnotate := n.Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-required`)
			Expect(disruptionReqAnnotate.Exists()).To(BeFalse())
		}

		assertNodeWillDrain := func(f *HookExecutionConfig, nodeName string) {
			n := f.KubernetesGlobalResource("Node", nodeName)

			drainAnnotate := n.Field(`metadata.annotations.update\.node\.deckhouse\.io/draining`)
			Expect(drainAnnotate.Exists()).To(BeTrue())

			disruptionReqAnnotate := n.Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-required`)
			Expect(disruptionReqAnnotate.Exists()).To(BeTrue())
		}

		Context("when have single-master control-plane", func() {
			const masterNodeName = "kube-master-0"
			Context("need disruptive operation on master", func() {
				Context("deckhouse locates on this node", func() {
					BeforeEach(func() {
						s := skipDrainingState{
							masters:          []string{masterNodeName},
							deckhousePodNode: masterNodeName,
							distruptionNode:  masterNodeName,
							workers:          []string{"worker-0", "worker-1"},
							workersReady:     2,
						}
						f.BindingContexts.Set(f.KubeStateSet(s.generate()))
						f.RunHook()
					})

					It("should approve node master", func() {
						Expect(f).To(ExecuteSuccessfully())

						assertNodeApproved(f, masterNodeName)
					})
				})

				Context("deckhouse locates on worker node", func() {
					BeforeEach(func() {
						s := skipDrainingState{
							masters:          []string{masterNodeName},
							deckhousePodNode: "worker-0",
							distruptionNode:  masterNodeName,
							workers:          []string{"worker-0", "worker-1"},
							workersReady:     2,
						}
						f.BindingContexts.Set(f.KubeStateSet(s.generate()))
						f.RunHook()
					})

					It("should approve node", func() {
						Expect(f).To(ExecuteSuccessfully())

						assertNodeApproved(f, masterNodeName)
					})
				})
			})

			Context("need disruptive operation on worker with deckhouse", func() {
				const workerWithDeckhouseName = "worker-0"
				Context("one ready worker node", func() {
					BeforeEach(func() {
						s := skipDrainingState{
							masters:          []string{masterNodeName},
							deckhousePodNode: workerWithDeckhouseName,
							distruptionNode:  workerWithDeckhouseName,
							workers:          []string{workerWithDeckhouseName, "worker-1"},
							workersReady:     1,
						}
						f.BindingContexts.Set(f.KubeStateSet(s.generate()))
						f.RunHook()
					})

					It("should approve node", func() {
						Expect(f).To(ExecuteSuccessfully())

						assertNodeApproved(f, workerWithDeckhouseName)
					})
				})

				Context("two ready worker nodes", func() {
					BeforeEach(func() {
						s := skipDrainingState{
							masters:          []string{masterNodeName},
							deckhousePodNode: workerWithDeckhouseName,
							distruptionNode:  workerWithDeckhouseName,
							workers:          []string{workerWithDeckhouseName, "worker-1"},
							workersReady:     2,
						}
						f.BindingContexts.Set(f.KubeStateSet(s.generate()))
						f.RunHook()
					})

					It("should drain node", func() {
						Expect(f).To(ExecuteSuccessfully())

						assertNodeWillDrain(f, workerWithDeckhouseName)
					})
				})
			})

			Context("need disruptive operation on worker node without deckhouse", func() {
				Context("two ready worker nodes", func() {
					const workerForDistruptive = "worker-1"
					BeforeEach(func() {
						s := skipDrainingState{
							masters:          []string{masterNodeName},
							deckhousePodNode: "worker-0",
							distruptionNode:  workerForDistruptive,
							workers:          []string{"worker-0", workerForDistruptive},
							workersReady:     2,
						}
						f.BindingContexts.Set(f.KubeStateSet(s.generate()))
						f.RunHook()
					})

					It("should drain node", func() {
						Expect(f).To(ExecuteSuccessfully())

						assertNodeWillDrain(f, workerForDistruptive)
					})
				})
			})

		})

		Context("when have multi-master", func() {
			Context("need disruptive operation on one of them", func() {
				const masterNodeForDisruptive = "kube-master-0"
				Context("deckhouse locates on this node", func() {
					BeforeEach(func() {
						s := skipDrainingState{
							masters:          []string{masterNodeForDisruptive, "kube-master-1", "kube-master-2"},
							deckhousePodNode: masterNodeForDisruptive,
							distruptionNode:  masterNodeForDisruptive,
							workers:          []string{"worker-0", "worker-1"},
							workersReady:     2,
						}
						f.BindingContexts.Set(f.KubeStateSet(s.generate()))
						f.RunHook()
					})

					It("should drain node", func() {
						Expect(f).To(ExecuteSuccessfully())

						assertNodeWillDrain(f, masterNodeForDisruptive)
					})
				})

				Context("deckhouse locates on another master node", func() {
					BeforeEach(func() {
						s := skipDrainingState{
							masters:          []string{masterNodeForDisruptive, "kube-master-1", "kube-master-2"},
							deckhousePodNode: "kube-master-1",
							distruptionNode:  masterNodeForDisruptive,
							workers:          []string{"worker-0", "worker-1"},
							workersReady:     2,
						}
						f.BindingContexts.Set(f.KubeStateSet(s.generate()))
						f.RunHook()
					})

					It("should drain node", func() {
						Expect(f).To(ExecuteSuccessfully())

						assertNodeWillDrain(f, masterNodeForDisruptive)
					})
				})

				Context("deckhouse locates on worker node", func() {
					BeforeEach(func() {
						s := skipDrainingState{
							masters:          []string{masterNodeForDisruptive, "kube-master-1", "kube-master-2"},
							deckhousePodNode: "worker-1",
							distruptionNode:  masterNodeForDisruptive,
							workers:          []string{"worker-0", "worker-1"},
							workersReady:     2,
						}
						f.BindingContexts.Set(f.KubeStateSet(s.generate()))
						f.RunHook()
					})

					It("should drain node", func() {
						Expect(f).To(ExecuteSuccessfully())

						assertNodeWillDrain(f, masterNodeForDisruptive)
					})
				})
			})

			Context("need disruptive operation on worker node with deckhouse", func() {
				const workerWithDeckhouse = "worker-1"
				Context("one ready worker node", func() {
					BeforeEach(func() {
						s := skipDrainingState{
							masters:          []string{"kube-master-0", "kube-master-1", "kube-master-2"},
							deckhousePodNode: workerWithDeckhouse,
							distruptionNode:  workerWithDeckhouse,
							workers:          []string{"worker-0", workerWithDeckhouse},
							workersReady:     1,
						}
						f.BindingContexts.Set(f.KubeStateSet(s.generate()))
						f.RunHook()
					})

					It("should approve node", func() {
						Expect(f).To(ExecuteSuccessfully())

						assertNodeApproved(f, workerWithDeckhouse)
					})
				})

				Context("two ready worker nodes", func() {
					BeforeEach(func() {
						s := skipDrainingState{
							masters:          []string{"kube-master-0", "kube-master-1", "kube-master-2"},
							deckhousePodNode: workerWithDeckhouse,
							distruptionNode:  workerWithDeckhouse,
							workers:          []string{"worker-0", workerWithDeckhouse},
							workersReady:     2,
						}
						f.BindingContexts.Set(f.KubeStateSet(s.generate()))
						f.RunHook()
					})

					It("should drain node", func() {
						Expect(f).To(ExecuteSuccessfully())

						assertNodeWillDrain(f, workerWithDeckhouse)
					})
				})
			})

			Context("need disruptive operation on worker node without deckhouse", func() {
				Context("two ready worker nodes", func() {
					const workerForDistruptive = "worker-1"
					BeforeEach(func() {
						s := skipDrainingState{
							masters:          []string{"kube-master-0", "kube-master-1", "kube-master-2"},
							deckhousePodNode: "worker-0",
							distruptionNode:  workerForDistruptive,
							workers:          []string{"worker-0", workerForDistruptive},
							workersReady:     2,
						}
						f.BindingContexts.Set(f.KubeStateSet(s.generate()))
						f.RunHook()
					})

					It("should drain node", func() {
						Expect(f).To(ExecuteSuccessfully())

						assertNodeWillDrain(f, workerForDistruptive)
					})
				})
			})
		})

	})

	Context("process_updated_nodes :: ", func() {
		for _, gUpdated := range []bool{true, false} {
			for _, gReady := range []bool{true, false} {
				for _, gDisruptionsApprovalMode := range []bool{true, false} {
					for _, gDisruption := range []bool{true, false} {
						for _, gDrained := range []bool{true, false} {
							Context(fmt.Sprintf("Updated: %t, Ready: %t, DisruptionsApprovalMode: %t, Disruption: %t, Drained: %t :: ", gUpdated, gReady, gDisruptionsApprovalMode, gDisruption, gDrained), func() {
								updated := gUpdated
								ready := gReady
								disruptionsApprovalMode := gDisruptionsApprovalMode
								disruption := gDisruption
								drained := gDrained
								BeforeEach(func() {
									f.BindingContexts.Set(f.KubeStateSet(initialState + generateStateToTestProcessUpdatedNodes(nodeNames, updated, ready, disruptionsApprovalMode, disruption, drained)))
									f.RunHook()
								})

								It("Works as expected", func() {
									Expect(f).To(ExecuteSuccessfully())
									for _, nodeName := range nodeNames {
										if updated && ready {
											By(fmt.Sprintf("%s must not have /approved", nodeName), func() {
												Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists()).To(BeFalse())
											})
											By(fmt.Sprintf("%s must not have /drained", nodeName), func() {
												Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`metadata.annotations.update\.node\.deckhouse\.io/drained`).Exists()).To(BeFalse())
											})
											By(fmt.Sprintf("%s must not have /disruption-approved", nodeName), func() {
												Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-approved`).Exists()).To(BeFalse())
											})

											By(fmt.Sprintf("%s must not have /disruption-required, which might be left, because node was approved manually", nodeName), func() {
												Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-required`).Exists()).To(BeFalse())
											})

											if drained {
												By(fmt.Sprintf("%s must not be unschedulable", nodeName), func() {
													Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`spec.unschedulable`).Exists()).To(BeFalse())
												})
											} else {
												By(fmt.Sprintf("%s must be unschedulable", nodeName), func() {
													Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`spec.unschedulable`).Exists()).To(BeTrue())
													Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`spec.unschedulable`).String()).To(Equal("true"))
												})
											}
										} else {
											By(fmt.Sprintf("%s must have /approved", nodeName), func() {
												Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists()).To(BeTrue())
											})

											if drained {
												By(fmt.Sprintf("%s must have /drained", nodeName), func() {
													Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`metadata.annotations.update\.node\.deckhouse\.io/drained`).Exists()).To(BeTrue())
												})
											}
											By(fmt.Sprintf("%s must be unschedulable", nodeName), func() {
												Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`spec.unschedulable`).Exists()).To(BeTrue())
												Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`spec.unschedulable`).String()).To(Equal("true"))
											})

											if disruption {
												By(fmt.Sprintf("%s must have /disruption-approved", nodeName), func() {
													Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-approved`).Exists()).To(BeTrue())
												})
											}
										}

									}
								})

							})
						}
					}
				}
			}
		}
	})

	Context("approve_updates :: all flow for one node", func() {
		state := `
---
apiVersion: v1
kind: Secret
metadata:
  name: configuration-checksums
  namespace: d8-cloud-instance-manager
data:
  test: dXBkYXRlZA== # updated
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: Static
status:
  desired: 3
  ready: 3
---
apiVersion: v1
kind: Node
metadata:
  name: worker-1
  labels:
    node.deckhouse.io/group: test
  annotations:
    update.node.deckhouse.io/waiting-for-approval: ""
    stub: worker-1
    stub2: worker-1
status:
  conditions:
  - type: Ready
    status: 'True'
---
apiVersion: v1
kind: Node
metadata:
  name: worker-2
  labels:
    node.deckhouse.io/group: test
  annotations:
    update.node.deckhouse.io/waiting-for-approval: ""
    stub: worker-2
    stub2: worker-2
status:
  conditions:
  - type: Ready
    status: 'True'
---
apiVersion: v1
kind: Node
metadata:
  name: worker-3
  labels:
    node.deckhouse.io/group: test
  annotations:
    update.node.deckhouse.io/waiting-for-approval: ""
    stub: worker-3
    stub2: worker-3
status:
  conditions:
  - type: Ready
    status: 'True'
`

		It("Works as expected", func() {
			approvedNodeIndex := -1

			By("one of nodes must be approved", func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(state, 2))
				f.RunHook()
				Expect(f).To(ExecuteSuccessfully())

				approvedCount := 0
				waitingForApprovalCount := 0
				for i := 1; i <= len(nodeNames); i++ {
					if f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists() {
						approvedCount++
						approvedNodeIndex = i
					}
					if f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/waiting-for-approval`).Exists() {
						waitingForApprovalCount++
					}
				}

				Expect(approvedNodeIndex).To(Not(Equal(-1)))
				Expect(approvedCount).To(Equal(1))
				Expect(waitingForApprovalCount).To(Equal(len(nodeNames) - 1))

				Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists()).To(BeTrue())

				Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.update\.node\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeFalse())

				for i := 1; i <= len(nodeNames); i++ {
					if i == approvedNodeIndex {
						continue
					}
					Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeTrue())
					Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists()).To(BeFalse())
				}
			})

			// TODO: no ObjectStore anymore
			// By(fmt.Sprintf("worker-%d must be marked for draining", approvedNodeIndex), func() {
			// 	newState := strings.Replace(f.ObjectStore.ToYaml(), fmt.Sprintf("stub: worker-%d", approvedNodeIndex), `update.node.deckhouse.io/disruption-required: ""`, 1)
			// 	f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(newState, 1))
			// 	f.RunHook()
			//
			// 	Expect(f).To(ExecuteSuccessfully())
			//
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists()).To(BeTrue())
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-required`).Exists()).To(BeTrue())
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.update\.node\.deckhouse\.io/draining`).Exists()).To(BeTrue())
			//
			// 	for i := 1; i <= len(nodeNames); i++ {
			// 		if i == approvedNodeIndex {
			// 			continue
			// 		}
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeTrue())
			//
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists()).To(BeFalse())
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-required`).Exists()).To(BeFalse())
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/draining`).Exists()).To(BeFalse())
			// 	}
			// })

			// By(fmt.Sprintf("worker-%d must be drained", approvedNodeIndex), func() {
			// 	f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(f.ObjectStore.ToYaml(), 2))
			// 	f.RunHook()
			//
			// 	Expect(f).To(ExecuteSuccessfully())
			//
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists()).To(BeTrue())
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.update\.node\.deckhouse\.io/drained`).Exists()).To(BeTrue())
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`spec.unschedulable`).Exists()).To(BeTrue())
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`spec.unschedulable`).String()).To(Equal("true"))
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-required`).Exists()).To(BeTrue())
			//
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.update\.node\.deckhouse\.io/draining`).Exists()).To(BeFalse())
			//
			// 	for i := 1; i <= len(nodeNames); i++ {
			// 		if i == approvedNodeIndex {
			// 			continue
			// 		}
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeTrue())
			//
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists()).To(BeFalse())
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-required`).Exists()).To(BeFalse())
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/draining`).Exists()).To(BeFalse())
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/drained`).Exists()).To(BeFalse())
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`spec.unschedulable`).Exists()).To(BeFalse())
			// 	}
			// })
			//
			// By(fmt.Sprintf("disruption of worker-%d should be approved", approvedNodeIndex), func() {
			// 	f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(f.ObjectStore.ToYaml(), 2))
			// 	f.RunHook()
			//
			// 	Expect(f).To(ExecuteSuccessfully())
			//
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists()).To(BeTrue())
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.update\.node\.deckhouse\.io/drained`).Exists()).To(BeTrue())
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`spec.unschedulable`).Exists()).To(BeTrue())
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`spec.unschedulable`).String()).To(Equal("true"))
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-approved`).Exists()).To(BeTrue())
			//
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-required`).Exists()).To(BeFalse())
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.update\.node\.deckhouse\.io/draining`).Exists()).To(BeFalse())
			//
			// 	for i := 1; i <= len(nodeNames); i++ {
			// 		if i == approvedNodeIndex {
			// 			continue
			// 		}
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeTrue())
			//
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists()).To(BeFalse())
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-required`).Exists()).To(BeFalse())
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/draining`).Exists()).To(BeFalse())
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/drained`).Exists()).To(BeFalse())
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`spec.unschedulable`).Exists()).To(BeFalse())
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-approved`).Exists()).To(BeFalse())
			// 	}
			// })
			//
			// By(fmt.Sprintf("worker-%d must be processed after it becomes updated", approvedNodeIndex), func() {
			// 	newState := strings.Replace(f.ObjectStore.ToYaml(), fmt.Sprintf("stub2: worker-%d", approvedNodeIndex), `node.deckhouse.io/configuration-checksum: "updated"`, 1)
			// 	f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(newState, 1))
			// 	f.RunHook()
			//
			// 	Expect(f).To(ExecuteSuccessfully())
			//
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists()).To(BeFalse())
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.update\.node\.deckhouse\.io/drained`).Exists()).To(BeFalse())
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`spec.unschedulable`).Exists()).To(BeFalse())
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-approved`).Exists()).To(BeFalse())
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-required`).Exists()).To(BeFalse())
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.update\.node\.deckhouse\.io/draining`).Exists()).To(BeFalse())
			// 	Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.update\.node\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeFalse())
			//
			// 	for i := 1; i <= len(nodeNames); i++ {
			// 		if i == approvedNodeIndex {
			// 			continue
			// 		}
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeTrue())
			//
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists()).To(BeFalse())
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-required`).Exists()).To(BeFalse())
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/draining`).Exists()).To(BeFalse())
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/drained`).Exists()).To(BeFalse())
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`spec.unschedulable`).Exists()).To(BeFalse())
			// 		Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-approved`).Exists()).To(BeFalse())
			// 	}
			// })
			//
			// By("next node must be approved", func() {
			// 	f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(f.ObjectStore.ToYaml(), 1))
			// 	f.RunHook()
			//
			// 	Expect(f).To(ExecuteSuccessfully())
			//
			// 	approvedCount := 0
			// 	waitingForApprovalCount := 0
			// 	for i := 1; i <= len(nodeNames); i++ {
			// 		if f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists() {
			// 			approvedCount++
			// 		}
			// 		if f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.update\.node\.deckhouse\.io/waiting-for-approval`).Exists() {
			// 			waitingForApprovalCount++
			// 		}
			// 	}
			//
			// 	Expect(approvedCount).To(Equal(1))
			// 	Expect(waitingForApprovalCount).To(Equal(len(nodeNames) - 2))
			// })
		})
	})

	Context("Update windows", func() {
		Context("out of update windows", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: configuration-checksums
  namespace: d8-cloud-instance-manager
data:
  test: dXBkYXRlZA== # updated
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: Static
  disruptions:
    approvalMode: Automatic
    automatic:
      windows:
        - from: "18:00"
          to: "21:00"
      drainBeforeApproval: true
---
apiVersion: v1
kind: Node
metadata:
  name: worker-1
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    update.node.deckhouse.io/approved: ""
    update.node.deckhouse.io/disruption-required: ""
spec:
  unschedulable: true
`))
				f.RunHook()
			})

			It("Should not be approved", func() {
				Expect(f).To(ExecuteSuccessfully())

				n := f.KubernetesGlobalResource("Node", "worker-1")
				Expect(n.Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-approved`).String()).To(Equal(""))
				Expect(n.Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-required`).Exists()).To(BeTrue())
			})
		})

		Context("inside update windows", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: configuration-checksums
  namespace: d8-cloud-instance-manager
data:
  test: dXBkYXRlZA== # updated
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng2
spec:
  nodeType: Static
  disruptions:
    approvalMode: Automatic
    automatic:
      windows:
        - from: "8:00"
          to: "18:00"
      drainBeforeApproval: true
---
apiVersion: v1
kind: Node
metadata:
  name: worker-2
  labels:
    node.deckhouse.io/group: ng2
  annotations:
    update.node.deckhouse.io/approved: ""
    update.node.deckhouse.io/disruption-required: ""
    update.node.deckhouse.io/drained: "bashible"
spec:
  unschedulable: true

`))
				f.RunHook()
			})

			It("Should be approved", func() {
				Expect(f).To(ExecuteSuccessfully())

				n := f.KubernetesGlobalResource("Node", "worker-2")
				Expect(n.Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-approved`).String()).To(Equal(""))
				Expect(n.Field(`metadata.annotations.update\.node\.deckhouse\.io/disruption-required`).Exists()).To(BeFalse())
			})
		})

		Context("With maxConcurrent update set", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: configuration-checksums
  namespace: d8-cloud-instance-manager
data:
  test: dXBkYXRlZA== # updated
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng2
spec:
  nodeType: Static
  disruptions:
    approvalMode: Automatic
    automatic:
      drainBeforeApproval: true
  update:
    maxConcurrent: 3
---
apiVersion: v1
kind: Node
metadata:
  name: worker-1
  labels:
    node.deckhouse.io/group: ng2
  annotations:
    update.node.deckhouse.io/waiting-for-approval: ""
spec:
  unschedulable: true
---
apiVersion: v1
kind: Node
metadata:
  name: worker-2
  labels:
    node.deckhouse.io/group: ng2
  annotations:
    update.node.deckhouse.io/waiting-for-approval: ""
spec:
  unschedulable: true
---
apiVersion: v1
kind: Node
metadata:
  name: worker-3
  labels:
    node.deckhouse.io/group: ng2
  annotations:
    update.node.deckhouse.io/waiting-for-approval: ""
spec:
  unschedulable: true

`))
				f.RunHook()
			})

			It("Should be approved", func() {
				Expect(f).To(ExecuteSuccessfully())

				n1 := f.KubernetesGlobalResource("Node", "worker-1")
				n2 := f.KubernetesGlobalResource("Node", "worker-2")
				n3 := f.KubernetesGlobalResource("Node", "worker-3")
				Expect(n1.Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists()).To(BeTrue())
				Expect(n1.Field(`metadata.annotations.update\.node\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeFalse())
				Expect(n2.Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists()).To(BeTrue())
				Expect(n2.Field(`metadata.annotations.update\.node\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeFalse())
				Expect(n3.Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists()).To(BeTrue())
				Expect(n3.Field(`metadata.annotations.update\.node\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeFalse())
			})
		})

		Context("With maxConcurrent update set to half of nodes", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: configuration-checksums
  namespace: d8-cloud-instance-manager
data:
  test: dXBkYXRlZA== # updated
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng2
spec:
  nodeType: Static
  disruptions:
    approvalMode: Automatic
    automatic:
      drainBeforeApproval: true
  update:
    maxConcurrent: "50%"
---
apiVersion: v1
kind: Node
metadata:
  name: worker-1
  labels:
    node.deckhouse.io/group: ng2
  annotations:
    update.node.deckhouse.io/waiting-for-approval: ""
spec:
  unschedulable: true
---
apiVersion: v1
kind: Node
metadata:
  name: worker-2
  labels:
    node.deckhouse.io/group: ng2
  annotations:
    update.node.deckhouse.io/waiting-for-approval: ""
spec:
  unschedulable: true

`))
				f.RunHook()
			})

			It("Should be approved", func() {
				Expect(f).To(ExecuteSuccessfully())

				n1 := f.KubernetesGlobalResource("Node", "worker-1")
				n2 := f.KubernetesGlobalResource("Node", "worker-2")
				n1Approved := n1.Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists()
				if n1Approved {
					Expect(n1.Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists()).To(BeTrue())
					Expect(n1.Field(`metadata.annotations.update\.node\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeFalse())
					Expect(n2.Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists()).To(BeFalse())
					Expect(n2.Field(`metadata.annotations.update\.node\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeTrue())
				} else {
					Expect(n1.Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists()).To(BeFalse())
					Expect(n1.Field(`metadata.annotations.update\.node\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeTrue())
					Expect(n2.Field(`metadata.annotations.update\.node\.deckhouse\.io/approved`).Exists()).To(BeTrue())
					Expect(n2.Field(`metadata.annotations.update\.node\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeFalse())
				}

			})
		})
	})

	Context("Rolling Update", func() {
		Context("without update windows", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: configuration-checksums
  namespace: d8-cloud-instance-manager
data:
  test: dXBkYXRlZA== # updated
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  disruptions:
    approvalMode: RollingUpdate
---
apiVersion: v1
kind: Node
metadata:
  name: worker-1
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    update.node.deckhouse.io/approved: ""
    update.node.deckhouse.io/rolling-update: ""
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  name: worker-1
  labels:
    node.deckhouse.io/group: ng1
`))
				f.RunHook()
			})

			It("Instance should be deleted", func() {
				Expect(f).To(ExecuteSuccessfully())

				m := f.KubernetesResource("Instance", "", "worker-1")
				Expect(m.Exists()).To(BeFalse())
			})
		})
		Context("inside update windows", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: configuration-checksums
  namespace: d8-cloud-instance-manager
data:
  test: dXBkYXRlZA== # updated
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  disruptions:
    approvalMode: RollingUpdate
    rollingUpdate:
      windows:
        - from: "8:00"
          to: "18:00"
---
apiVersion: v1
kind: Node
metadata:
  name: worker-1
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    update.node.deckhouse.io/approved: ""
    update.node.deckhouse.io/rolling-update: ""
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  name: worker-1
  labels:
    node.deckhouse.io/group: ng1
`))
				f.RunHook()
			})

			It("Instance should be deleted", func() {
				Expect(f).To(ExecuteSuccessfully())

				m := f.KubernetesResource("Instance", "", "worker-1")
				Expect(m.Exists()).To(BeFalse())
			})
		})
		Context("out of update windows", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: configuration-checksums
  namespace: d8-cloud-instance-manager
data:
  test: dXBkYXRlZA== # updated
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  disruptions:
    approvalMode: RollingUpdate
    rollingUpdate:
      windows:
        - from: "18:00"
          to: "21:00"
---
apiVersion: v1
kind: Node
metadata:
  name: worker-1
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    update.node.deckhouse.io/approved: ""
    update.node.deckhouse.io/rolling-update: ""
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  name: worker-1
  labels:
    node.deckhouse.io/group: ng1
`))
				f.RunHook()
			})

			It("Instance should be deleted", func() {
				Expect(f).To(ExecuteSuccessfully())

				m := f.KubernetesResource("Instance", "", "worker-1")
				Expect(m.Exists()).To(BeTrue())
			})
		})
	})
})

type skipDrainingState struct {
	deckhousePodNode string
	distruptionNode  string

	masters      []string
	workers      []string
	workersReady int
}

func (s *skipDrainingState) generate() string {
	var t = `
---
apiVersion: v1
kind: Secret
metadata:
  name: configuration-checksums
  namespace: d8-cloud-instance-manager
data:
  worker: dXBkYXRlZA== # updated
  undisruptable-worker: dXBkYXRlZA== # updated
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: Static
  disruptions:
    approvalMode: Automatic
    automatic:
      drainBeforeApproval: true
status:
  nodes: {{ len .MasterNodes }}
  ready: {{ len .MasterNodes }}
{{- range $nodeName := .MasterNodes }}
---
apiVersion: v1
kind: Node
metadata:
  name: {{ $nodeName }}
  labels:
    node.deckhouse.io/group: master
{{- if eq $nodeName $.DisruptionNode }}
  annotations:
    update.node.deckhouse.io/approved: ""
    update.node.deckhouse.io/disruption-required: ""
{{- end }}
{{- end }}
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  disruptions:
    approvalMode: Automatic
    automatic:
      drainBeforeApproval: true
status:
  nodes: {{ len .WorkerNodes }}
  ready: {{ $.WorkerNodesReadyCount }}
{{- range $nodeName := .WorkerNodes }}
---
apiVersion: v1
kind: Node
metadata:
  name: {{ $nodeName }}
  labels:
    node.deckhouse.io/group: worker
{{ if eq $nodeName $.DisruptionNode }}
  annotations:
    update.node.deckhouse.io/approved: ""
    update.node.deckhouse.io/disruption-required: ""
{{- end }}
{{- end }}

`

	os.Setenv("DECKHOUSE_NODE_NAME", s.deckhousePodNode)

	tmpl, _ := template.New("state").Parse(t)
	var state bytes.Buffer
	err := tmpl.Execute(&state, struct {
		MasterNodes           []string
		WorkerNodes           []string
		DisruptionNode        string
		WorkerNodesReadyCount int
	}{s.masters, s.workers, s.distruptionNode, s.workersReady})
	if err != nil {
		panic(err)
	}
	return state.String()
}

func generateStateToTestApproveUpdates(nodeNames []string, oneIsApproved, waitingForApproval, nodeReady, ngReady bool, nodeType string) string {
	const tpl = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker-2
spec:
  nodeType: {{ $.NodeType }}
status:
{{- if $.NgReady }}
  {{- if eq $.NodeType "CloudEphemeral" }}
  desired: 3
  {{- end }}
  ready: 3
{{- else }}
  {{- if eq $.NodeType "CloudEphemeral" }}
  desired: 3
  {{- end }}
  ready: 2
{{- end }}

{{- range $i, $nodeName := .NodeNames }}
---
apiVersion: v1
kind: Node
metadata:
  name: {{ $nodeName }}
  labels:
    node.deckhouse.io/group: worker-2
  annotations:
{{- if and $.OneIsApproved (eq $i 0) }}
    update.node.deckhouse.io/approved: ""
{{- end }}
{{- if $.AproveRequired }}
    update.node.deckhouse.io/waiting-for-approval: ""
{{- end }}
{{- if $.NodeReady }}
status:
  conditions:
  - type: Ready
    status: 'True'
{{- else }}
status:
  conditions:
  - type: Ready
    status: 'False'
{{- end }}
{{- end }}
`
	tmpl, _ := template.New("state").Parse(tpl)
	var state bytes.Buffer
	err := tmpl.Execute(&state, struct {
		NodeNames      []string
		OneIsApproved  bool
		AproveRequired bool
		NodeReady      bool
		NgReady        bool
		NodeType       string
	}{nodeNames, oneIsApproved, waitingForApproval, nodeReady, ngReady, nodeType})
	if err != nil {
		panic(err)
	}
	return state.String()
}

const tpl = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker-2
spec:
  nodeType: Static
{{- if eq .DisruptionsApprovalMode "Manual" }}
  disruptions:
    approvalMode: Manual
{{- else if eq .DisruptionsApprovalMode "Automatic" }}
  disruptions:
    approvalMode: Automatic
    {{- if eq .DisruptionsDrainBeforeApproval "false" }}
    automatic:
      drainBeforeApproval: false
    {{- else if eq .DisruptionsDrainBeforeApproval "true" }}
    automatic:
      drainBeforeApproval: true
    {{- end }}
{{- else if eq .DisruptionsApprovalMode "RollingUpdate" }}
  {{- if eq .DisruptionsDrainBeforeApproval "false" }}
  disruptions:
    automatic:
      drainBeforeApproval: false
    {{- else if eq .DisruptionsDrainBeforeApproval "true" }}
  disruptions:
    automatic:
      drainBeforeApproval: true
  {{- end }}
{{- end }}
{{- range $nodeName := .NodeNames }}
---
apiVersion: v1
kind: Node
metadata:
  name: {{ $nodeName }}
  labels:
    node.deckhouse.io/group: worker-2
  annotations:
    update.node.deckhouse.io/approved: ""
{{- if $.DisruptionRequired }}
    update.node.deckhouse.io/disruption-required: ""
{{- end }}
{{- if $.Unschedulable }}
    update.node.deckhouse.io/drained: "bashible"
spec:
  unschedulable: true
{{- end }}
{{- end }}
`

func generateStateToTestApproveDisruptions(nodeNames []string, disruptionRequired bool, disruptionsApprovalMode, disruptionsDrainBeforeApproval string, unschedulable bool) string {
	tmpl, _ := template.New("state").Parse(tpl)
	var state bytes.Buffer
	err := tmpl.Execute(&state, struct {
		NodeNames                      []string
		DisruptionRequired             bool
		DisruptionsApprovalMode        string
		DisruptionsDrainBeforeApproval string
		Unschedulable                  bool
	}{nodeNames, disruptionRequired, disruptionsApprovalMode, disruptionsDrainBeforeApproval, unschedulable})
	if err != nil {
		panic(fmt.Errorf("execute template: %v", err))
	}
	return state.String()
}

func generateStateToTestDrainingNodes(nodeNames []string, draining, unschedulable bool) string {
	state := ``

	for _, nodeName := range nodeNames {
		state += fmt.Sprintf(`
---
apiVersion: v1
kind: Node
metadata:
  name: %s
  labels:
    node.deckhouse.io/group: worker
  annotations:
    update.node.deckhouse.io/approved: ""
`, nodeName)

		if draining {
			state += `
    update.node.deckhouse.io/draining: "bashible"
`
		}
		if unschedulable {
			state += `
spec:
  unschedulable: true`
		}
	}

	return state
}

func generateStateToTestProcessUpdatedNodes(nodeNames []string, updated, ready, disruptionsApprovalMode, disruption, drained bool) string {
	state := ``
	ngName := "worker"
	if !disruptionsApprovalMode {
		ngName = "undisruptable-worker"
	}

	for _, nodeName := range nodeNames {
		state += fmt.Sprintf(`
---
apiVersion: v1
kind: Node
metadata:
  name: %s
  labels:
    node.deckhouse.io/group: %s
  annotations:
    update.node.deckhouse.io/approved: ""
`, nodeName, ngName)
		if updated {
			state += `
    node.deckhouse.io/configuration-checksum: updated
`
		} else {
			state += `
    node.deckhouse.io/configuration-checksum: notupdated
`
		}

		if !disruptionsApprovalMode {
			state += `
    update.node.deckhouse.io/disruption-required: ""
`
		}

		if disruption {
			state += `
    update.node.deckhouse.io/disruption-approved: ""
`
		}

		if drained {
			state += `
    update.node.deckhouse.io/drained: "bashible"
`
		}
		state += `
spec:
  unschedulable: true`

		if ready {
			state += `
status:
  conditions:
  - type: Ready
    status: 'True'
`
		} else {
			state += `
status:
  conditions:
  - type: Ready
    status: 'False'
`
		}
	}

	return state
}

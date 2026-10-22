/*
Copyright 2026 Flant JSC

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

package machinedrain

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// keepFinalizer stands in for CAPI's own finalizer: it keeps a deleted Machine
// in the API while the test looks at what happens to it.
const keepFinalizer = "test.deckhouse.io/keep"

// User story: As a cluster operator, I want the eviction of a deleted machine to
// run as a Drain operation, so that I see how it goes and why it is slow.
var _ = Describe("MachineDrain controller", func() {
	BeforeEach(func(ctx context.Context) {
		testenv.EnsureObject(ctx, k8sClient, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nodecommon.MachineNamespace}})
	})

	It("creates one Drain for a deleted machine whose node is Ready", func(ctx context.Context) {
		group := createNodeGroup(ctx)
		node := createNode(ctx, group, corev1.ConditionTrue)
		machine := createMachine(ctx, group, node.Name, true)
		deleteMachine(ctx, machine)

		var drain v1alpha1.NodeOperation
		Eventually(func(g Gomega) {
			drains := drainsOf(ctx, g, machine)
			g.Expect(drains).To(HaveLen(1))
			drain = drains[0]
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Expect(drain.Name).To(HavePrefix("drain-" + node.Name + "-"))
		Expect(drain.Spec.Type).To(Equal(v1alpha1.NodeOperationTypeDrain))
		Expect(drain.Spec.NodeName).To(Equal(node.Name))
		Expect(drain.Spec.Cause).To(Equal(fmt.Sprintf("Machine %s of NodeGroup %s is deleted", machine.Name, group)))
		Expect(drain.Labels).To(Equal(map[string]string{
			v1alpha1.NodeOperationMachineUIDLabel: string(machine.UID),
			managedByLabel:                        managedByValue,
			nodecommon.NodeGroupLabel:             group,
		}))
		Expect(drain.Annotations).To(HaveKeyWithValue(v1alpha1.NodeOperationMachineAnnotation, machine.Name))
		Expect(drain.OwnerReferences).To(ConsistOf(SatisfyAll(
			HaveField("Kind", "Node"), HaveField("Name", node.Name), HaveField("UID", node.UID))))

		By("passing over the machine again")
		for range 3 {
			nudgeMachine(ctx, machine)
		}

		Consistently(func(g Gomega) {
			g.Expect(drainsOf(ctx, g, machine)).To(HaveLen(1))
			g.Expect(getMachine(ctx, g, machine).Annotations).To(HaveKey(nodecommon.MachineDrainHookAnnotation),
				"the machine waits until the Drain ends")
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// The MachineDeployment of a static group sets no drain timeout unless the
	// group does, so Cluster API drained its machines without a time limit.
	DescribeTable("gives the Drain the time limit Cluster API had",
		func(ctx context.Context, spec deckhousev1.NodeGroupSpec, unlimited bool) {
			group := createNodeGroupOf(ctx, spec)
			node := createNode(ctx, group, corev1.ConditionTrue)
			machine := createMachine(ctx, group, node.Name, true)
			deleteMachine(ctx, machine)

			Eventually(func(g Gomega) {
				drains := drainsOf(ctx, g, machine)
				g.Expect(drains).To(HaveLen(1))
				if unlimited {
					g.Expect(drains[0].Annotations).To(HaveKeyWithValue(v1alpha1.NodeOperationUnlimitedDrainAnnotation, "true"))
				} else {
					g.Expect(drains[0].Annotations).NotTo(HaveKey(v1alpha1.NodeOperationUnlimitedDrainAnnotation))
				}
			}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		},
		Entry("a static group without nodeDrainTimeoutSecond: no limit",
			deckhousev1.NodeGroupSpec{NodeType: deckhousev1.NodeTypeStatic}, true),
		Entry("a CloudStatic group without nodeDrainTimeoutSecond: no limit",
			deckhousev1.NodeGroupSpec{NodeType: deckhousev1.NodeTypeCloudStatic}, true),
		Entry("a static group with nodeDrainTimeoutSecond: that value",
			deckhousev1.NodeGroupSpec{NodeType: deckhousev1.NodeTypeStatic, NodeDrainTimeoutSecond: ptr.To(120)}, false),
		Entry("a cloud group without nodeDrainTimeoutSecond: the default",
			deckhousev1.NodeGroupSpec{
				NodeType: deckhousev1.NodeTypeCloudEphemeral,
				CloudInstances: &deckhousev1.CloudInstancesSpec{
					MinPerZone: 1, MaxPerZone: 1,
					ClassReference: deckhousev1.ClassReference{Kind: "DVPInstanceClass", Name: "worker"},
				},
			}, false),
	)

	DescribeTable("removes the hook once the Drain ends",
		func(ctx context.Context, phase v1alpha1.NodeOperationPhase, reason string) {
			group := createNodeGroup(ctx)
			node := createNode(ctx, group, corev1.ConditionTrue)
			machine := createMachine(ctx, group, node.Name, true)
			deleteMachine(ctx, machine)

			var drain v1alpha1.NodeOperation
			Eventually(func(g Gomega) {
				drains := drainsOf(ctx, g, machine)
				g.Expect(drains).To(HaveLen(1))
				drain = drains[0]
			}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

			By("the nodeoperation controller ending the Drain")
			endOperation(ctx, drain.Name, phase, reason)

			Eventually(func(g Gomega) {
				annotations := getMachine(ctx, g, machine).Annotations
				g.Expect(annotations).NotTo(HaveKey(nodecommon.MachineDrainHookAnnotation))
				g.Expect(annotations).To(HaveKeyWithValue(capiv1beta2.ExcludeNodeDrainingAnnotation, ""),
					"only the hook is removed")
			}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
			Expect(drainsOf(ctx, Default, machine)).To(ConsistOf(HaveField("Name", drain.Name)), "an ended Drain is not replaced")
		},
		Entry("completed", v1alpha1.NodeOperationPhaseCompleted, "Drained"),
		Entry("failed", v1alpha1.NodeOperationPhaseFailed, "PreparationTimedOut"),
	)

	// A machine of a group that is gone keeps the limit Cluster API gave it:
	// spec.deletion.nodeDrainTimeoutSeconds, which a static MachineDeployment never sets.
	It("reads the time limit off the machine when its NodeGroup is gone", func(ctx context.Context) {
		group := testenv.UniqueName("gone")
		static := createMachine(ctx, group, createNode(ctx, group, corev1.ConditionTrue).Name, true)
		cloud := createMachine(ctx, group, createNode(ctx, group, corev1.ConditionTrue).Name, true)
		setDrainTimeout(ctx, cloud, 600)
		deleteMachine(ctx, static)
		deleteMachine(ctx, cloud)

		Eventually(func(g Gomega) {
			drains := drainsOf(ctx, g, static)
			g.Expect(drains).To(HaveLen(1))
			g.Expect(drains[0].Annotations).To(HaveKeyWithValue(v1alpha1.NodeOperationUnlimitedDrainAnnotation, "true"))
			drains = drainsOf(ctx, g, cloud)
			g.Expect(drains).To(HaveLen(1))
			g.Expect(drains[0].Annotations).NotTo(HaveKey(v1alpha1.NodeOperationUnlimitedDrainAnnotation))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	DescribeTable("releases the machine at once when there is no node to drain",
		func(ctx context.Context, nodeOf func(ctx context.Context, group string) string, unreachable bool) {
			group := createNodeGroup(ctx)
			nodeName := nodeOf(ctx, group)
			machine := createMachine(ctx, group, nodeName, true)
			deleteMachine(ctx, machine)

			why := "it has no node"
			if unreachable {
				why = fmt.Sprintf("node %s is unreachable: Ready is Unknown", nodeName)
			}
			Eventually(func(g Gomega) {
				g.Expect(getMachine(ctx, g, machine).Annotations).NotTo(HaveKey(nodecommon.MachineDrainHookAnnotation))
				g.Expect(eventsOfNodeGroup(ctx, g, group)).To(ContainElement(SatisfyAll(
					HaveField("Type", corev1.EventTypeWarning),
					HaveField("Reason", eventDeletedWithoutDrain),
					HaveField("Message", fmt.Sprintf("Machine %s is deleted without a drain: %s", machine.Name, why)))))
			}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

			Consistently(func(g Gomega) {
				g.Expect(drainsOf(ctx, g, machine)).To(BeEmpty())
			}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
		},
		Entry("the machine has no node yet",
			func(context.Context, string) string { return "" }, false),
		Entry("its node does not exist",
			func(context.Context, string) string { return testenv.UniqueName("gone") }, false),
		Entry("its node is unreachable, as when MachineHealthCheck deletes it",
			func(ctx context.Context, group string) string {
				return createNode(ctx, group, corev1.ConditionUnknown).Name
			}, true),
	)

	It("releases the machine when its node becomes unreachable during the Drain", func(ctx context.Context) {
		group := createNodeGroup(ctx)
		node := createNode(ctx, group, corev1.ConditionTrue)
		machine := createMachine(ctx, group, node.Name, true)
		deleteMachine(ctx, machine)

		Eventually(func(g Gomega) {
			g.Expect(drainsOf(ctx, g, machine)).To(HaveLen(1))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		setReady(ctx, node.Name, corev1.ConditionUnknown)

		Eventually(func(g Gomega) {
			g.Expect(getMachine(ctx, g, machine).Annotations).NotTo(HaveKey(nodecommon.MachineDrainHookAnnotation))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// Ready=False is reported by a kubelet that is up: it ends the pods, so the
	// node is drained like a Ready one. Only Unknown means nobody is there to.
	It("drains a deleted machine whose node reports Ready=False", func(ctx context.Context) {
		group := createNodeGroup(ctx)
		node := createNode(ctx, group, corev1.ConditionFalse)
		machine := createMachine(ctx, group, node.Name, true)
		deleteMachine(ctx, machine)

		Eventually(func(g Gomega) {
			g.Expect(drainsOf(ctx, g, machine)).To(HaveLen(1))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(getMachine(ctx, g, machine).Annotations).To(HaveKey(nodecommon.MachineDrainHookAnnotation))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	It("creates the Drain again when it is deleted while the machine waits", func(ctx context.Context) {
		group := createNodeGroup(ctx)
		node := createNode(ctx, group, corev1.ConditionTrue)
		machine := createMachine(ctx, group, node.Name, true)
		deleteMachine(ctx, machine)

		var first string
		Eventually(func(g Gomega) {
			drains := drainsOf(ctx, g, machine)
			g.Expect(drains).To(HaveLen(1))
			first = drains[0].Name
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Expect(k8sClient.Delete(ctx, &v1alpha1.NodeOperation{ObjectMeta: metav1.ObjectMeta{Name: first}})).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(drainsOf(ctx, g, machine)).To(ConsistOf(HaveField("Name", Not(Equal(first)))))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// A machine deleted before the upgrade has no hook, and CAPI drains it as it
	// always did. The machine with the hook next to it shows the controller runs.
	It("leaves a deleted machine without the hook alone", func(ctx context.Context) {
		group := createNodeGroup(ctx)
		hooked := createMachine(ctx, group, createNode(ctx, group, corev1.ConditionTrue).Name, true)
		plain := createMachine(ctx, group, createNode(ctx, group, corev1.ConditionTrue).Name, false)
		deleteMachine(ctx, hooked)
		deleteMachine(ctx, plain)
		before := getMachine(ctx, Default, plain)

		Eventually(func(g Gomega) {
			g.Expect(drainsOf(ctx, g, hooked)).To(HaveLen(1))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(drainsOf(ctx, g, plain)).To(BeEmpty())
			g.Expect(getMachine(ctx, g, plain).ResourceVersion).To(Equal(before.ResourceVersion))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})
})

func createNodeGroup(ctx context.Context) string {
	GinkgoHelper()

	return createNodeGroupOf(ctx, deckhousev1.NodeGroupSpec{NodeType: deckhousev1.NodeTypeStatic})
}

func createNodeGroupOf(ctx context.Context, spec deckhousev1.NodeGroupSpec) string {
	GinkgoHelper()

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: testenv.UniqueName("ng")},
		Spec:       spec,
	}
	Expect(k8sClient.Create(ctx, ng)).To(Succeed())
	DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, ng) })
	return ng.Name
}

// createNode creates a node of the group with the given Ready status. The
// machine annotation is what CAPI writes onto the node of a Machine.
func createNode(ctx context.Context, group string, ready corev1.ConditionStatus) *corev1.Node {
	GinkgoHelper()

	name := testenv.UniqueName("node")
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:        name,
		Labels:      map[string]string{nodecommon.NodeGroupLabel: group},
		Annotations: map[string]string{nodecommon.CAPIMachineAnnotation: machineNameOf(name)},
	}}
	Expect(k8sClient.Create(ctx, node)).To(Succeed())
	DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, node) })
	setReady(ctx, name, ready)

	fresh := &corev1.Node{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name}, fresh)).To(Succeed())
	return fresh
}

// machineNameOf ties a machine to its node by name, so either can be created first.
func machineNameOf(nodeName string) string {
	return "machine-" + nodeName
}

func setReady(ctx context.Context, name string, ready corev1.ConditionStatus) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		node := &corev1.Node{}
		g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name}, node)).To(Succeed())
		node.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: ready}}
		g.Expect(k8sClient.Status().Update(ctx, node)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

// createMachine creates the Machine a MachineDeployment of the group makes, with
// or without node-manager's hook. An empty nodeName leaves nodeRef unset.
func createMachine(ctx context.Context, group, nodeName string, hooked bool) *capiv1beta2.Machine {
	GinkgoHelper()

	name := testenv.UniqueName("machine")
	if nodeName != "" {
		name = machineNameOf(nodeName)
	}
	annotations := map[string]string{capiv1beta2.ExcludeNodeDrainingAnnotation: ""}
	if hooked {
		annotations[nodecommon.MachineDrainHookAnnotation] = nodecommon.MachineDrainHookOwner
	}
	machine := &capiv1beta2.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   nodecommon.MachineNamespace,
			Labels:      map[string]string{machineNodeGroupLabel: group},
			Annotations: annotations,
			Finalizers:  []string{keepFinalizer},
		},
		Spec: capiv1beta2.MachineSpec{
			ClusterName: "e2e",
			Bootstrap:   capiv1beta2.Bootstrap{DataSecretName: ptr.To("e2e-bootstrap")},
			InfrastructureRef: capiv1beta2.ContractVersionedObjectReference{
				APIGroup: "infrastructure.cluster.x-k8s.io",
				Kind:     "DVPMachine",
				Name:     name,
			},
		},
	}
	Expect(k8sClient.Create(ctx, machine)).To(Succeed())
	DeferCleanup(func(ctx context.Context) {
		testenv.RemoveFinalizers(ctx, k8sClient, machine)
		_ = k8sClient.Delete(ctx, machine)
	})

	if nodeName != "" {
		machine.Status.NodeRef = capiv1beta2.MachineNodeReference{Name: nodeName}
		Expect(k8sClient.Status().Update(ctx, machine)).To(Succeed())
	}
	return machine
}

// setDrainTimeout gives the machine the drain timeout a cloud MachineDeployment sets.
func setDrainTimeout(ctx context.Context, machine *capiv1beta2.Machine, seconds int32) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		fresh := getMachine(ctx, g, machine)
		fresh.Spec.Deletion.NodeDrainTimeoutSeconds = ptr.To(seconds)
		g.Expect(k8sClient.Update(ctx, fresh)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

// deleteMachine is what a MachineSet does when its group is scaled down: the
// Machine gets its deletionTimestamp and stays while finalizers hold it.
func deleteMachine(ctx context.Context, machine *capiv1beta2.Machine) {
	GinkgoHelper()

	Expect(k8sClient.Delete(ctx, machine)).To(Succeed())
	Eventually(func(g Gomega) {
		g.Expect(getMachine(ctx, g, machine).DeletionTimestamp).NotTo(BeNil())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

func getMachine(ctx context.Context, g Gomega, machine *capiv1beta2.Machine) *capiv1beta2.Machine {
	fresh := &capiv1beta2.Machine{}
	g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(machine), fresh)).To(Succeed())
	return fresh
}

func drainsOf(ctx context.Context, g Gomega, machine *capiv1beta2.Machine) []v1alpha1.NodeOperation {
	list := &v1alpha1.NodeOperationList{}
	g.Expect(k8sClient.List(ctx, list, client.MatchingLabels{v1alpha1.NodeOperationMachineUIDLabel: string(machine.UID)})).To(Succeed())
	return list.Items
}

// endOperation writes what the nodeoperation controller writes when an
// operation ends. That controller does not run in this suite.
func endOperation(ctx context.Context, name string, phase v1alpha1.NodeOperationPhase, reason string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		op := &v1alpha1.NodeOperation{}
		g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name}, op)).To(Succeed())
		op.Status.Phase = phase
		op.Status.Reason = reason
		op.Status.Message = "ended by the test"
		g.Expect(k8sClient.Status().Update(ctx, op)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

// eventsOfNodeGroup returns the events recorded on a NodeGroup. Listed across all
// namespaces: the recorder files a cluster-scoped object's events under "default".
func eventsOfNodeGroup(ctx context.Context, g Gomega, name string) []corev1.Event {
	events := &corev1.EventList{}
	g.Expect(k8sClient.List(ctx, events, client.InNamespace(""))).To(Succeed())

	var ofGroup []corev1.Event
	for _, event := range events.Items {
		if event.InvolvedObject.Kind == "NodeGroup" && event.InvolvedObject.Name == name {
			ofGroup = append(ofGroup, event)
		}
	}
	return ofGroup
}

func nudgeMachine(ctx context.Context, machine *capiv1beta2.Machine) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		fresh := getMachine(ctx, g, machine)
		patch := client.MergeFrom(fresh.DeepCopy())
		fresh.Annotations["test.deckhouse.io/nudge"] = testenv.UniqueName("n")
		g.Expect(k8sClient.Patch(ctx, fresh, patch)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

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

package nodeoperation

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// User story: As a cluster operator, I want a deleted machine to go after the
// group's drain timeout, as it did when Cluster API drained it, and not wait for
// the other operations of its node.
var _ = Describe("Drain of a deleted machine", func() {
	It("fails at its drainDeadline, while an ordinary Drain keeps the margin", func(ctx context.Context) {
		machineNode := createNode(ctx, testenv.UniqueName("machine"), false)
		plainNode := createNode(ctx, testenv.UniqueName("plain"), false)
		machineDrain := createMachineDrain(ctx, machineNode.Name, false)
		plain := createOperation(ctx, plainNode.Name, v1alpha1.NodeOperationTypeDrain, nil)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, machineDrain.Name)
			g.Expect(fresh.Status.DrainDeadline).NotTo(BeNil())
			g.Expect(fresh.Status.Deadline).To(HaveValue(HaveField("Time", BeTemporally("==", fresh.Status.DrainDeadline.Time))))
			fresh = getOperation(ctx, g, plain.Name)
			g.Expect(fresh.Status.DrainDeadline).NotTo(BeNil())
			g.Expect(fresh.Status.Deadline).To(HaveValue(HaveField("Time", BeTemporally("==", fresh.Status.DrainDeadline.Add(operationTimeout)))))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the draining controller cordoning the node of the machine")
		Eventually(func(g Gomega) {
			node := getNode(ctx, g, machineNode.Name)
			patch := client.MergeFrom(node.DeepCopy())
			node.Spec.Unschedulable = true
			g.Expect(k8sClient.Patch(ctx, node, patch)).To(Succeed())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("both evictions running a minute past the group's drain timeout")
		passed := metav1.NewTime(time.Now().Add(-time.Minute).Truncate(time.Second))
		setDrainDeadline(ctx, machineDrain.Name, passed)
		setDrainDeadline(ctx, plain.Name, passed)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, machineDrain.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseFailed))
			g.Expect(fresh.Status.Reason).To(Equal("PreparationTimedOut"))
			g.Expect(fresh.Status.Message).To(Equal(fmt.Sprintf(
				"node %s was not drained by %s. The machine is deleted with the pods that did not leave",
				machineNode.Name, passed.UTC().Format(time.RFC3339))))
			g.Expect(getNode(ctx, g, machineNode.Name).Annotations).NotTo(HaveKey(nodecommon.DrainingAnnotation),
				"the eviction stops, and the machine goes with the pods left")
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(getNode(ctx, g, machineNode.Name).Spec.Unschedulable).To(BeTrue(),
				"the node of a deleted machine is not given back to the scheduler")
			fresh := getOperation(ctx, g, plain.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseInProgress))
			g.Expect(fresh.Status.Reason).To(Equal("Draining"))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// Cluster API drained a machine of a static group without
	// nodeDrainTimeoutSecond without a time limit, and the Drain keeps that.
	It("never times out the Drain of a static machine without a drain timeout", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("static"), false)
		drain := createMachineDrain(ctx, node.Name, true)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, drain.Name)
			g.Expect(fresh.Status.Reason).To(Equal("Draining"))
			g.Expect(fresh.Status.StartedAt).NotTo(BeNil())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the eviction running for longer than any stretch of an operation may")
		expireDeadline(ctx, drain.Name)
		nudge(ctx, drain.Name)

		Consistently(func(g Gomega) {
			fresh := getOperation(ctx, g, drain.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseInProgress))
			g.Expect(fresh.Status.DrainDeadline).To(BeNil())
			g.Expect(fresh.Status.Deadline).To(BeNil(), "no deadline is published")
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	It("ends the other live operations of its node and leaves the ended ones alone", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("deleted"), false)

		By("a Reboot that already ran to its end")
		done := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeReboot, &v1alpha1.NodeOperationDrainSpec{Skip: true})
		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, done.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseInProgress))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		completeByNode(ctx, done.Name)
		doneBefore := getOperation(ctx, Default, done.Name).Status

		By("a Disruption waiting for an operator")
		pending := createApproval(ctx, node.Name, false)
		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, pending.Name).Status.Reason).To(Equal("WaitingForApproval"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("a Disruption the node is carrying out")
		atWork := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeDisruption, &v1alpha1.NodeOperationDrainSpec{Skip: true})
		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, atWork.Name).Status.Reason).To(Equal("WaitingForNode"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("a Reboot evicting the workload, and one queued behind its eviction")
		evicting := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeReboot, nil)
		Eventually(func(g Gomega) {
			g.Expect(drainRequestOn(ctx, g, node.Name)).To(Equal(drainingSource + "/" + string(evicting.UID)))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		queued := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeReboot, nil)
		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, queued.Name).Status.Reason).To(Equal("WaitingForOperation"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		drain := createMachineDrain(ctx, node.Name, false)

		message := "Machine " + machineOfDrain + " is deleted. node-manager ended this operation so that the machine can be removed"
		Eventually(func(g Gomega) {
			for _, name := range []string{pending.Name, atWork.Name, evicting.Name, queued.Name} {
				fresh := getOperation(ctx, g, name)
				g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseFailed), name)
				g.Expect(fresh.Status.Reason).To(Equal("MachineDeleted"), name)
				g.Expect(fresh.Status.Message).To(Equal(message), name)
				g.Expect(eventsOf(ctx, g, name)).To(ContainElement(SatisfyAll(
					HaveField("Type", corev1.EventTypeWarning), HaveField("Reason", "MachineDeleted"), HaveField("Message", message))))
			}
			g.Expect(drainRequestOn(ctx, g, node.Name)).To(Equal(drainingSource+"/"+string(drain.UID)),
				"the machine's Drain evicts at once")
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(getOperation(ctx, g, done.Name).Status).To(Equal(doneBefore))
			g.Expect(getOperation(ctx, g, drain.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseInProgress))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})
})

// bashible's drain request is no operation to end. The Drain of a deleted machine
// takes it over, and bashible asks again once it is free. An ordinary Drain keeps
// waiting for bashible's eviction.
func TestAMachineDrainTakesTheEvictionBashibleAskedFor(t *testing.T) {
	node := groupedNode("worker")
	node.Annotations = map[string]string{nodecommon.DrainingAnnotation: "bashible"}
	drain := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "drain-worker-abcde", UID: "drain-uid",
			Labels:      map[string]string{v1alpha1.NodeOperationMachineUIDLabel: "machine-uid"},
			Annotations: map[string]string{v1alpha1.NodeOperationMachineAnnotation: machineOfDrain},
		},
		Spec: v1alpha1.NodeOperationSpec{Type: v1alpha1.NodeOperationTypeDrain, NodeName: node.Name},
	}
	plain := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "drain-worker-plain", UID: "plain-uid"},
		Spec:       v1alpha1.NodeOperationSpec{Type: v1alpha1.NodeOperationTypeDrain, NodeName: node.Name},
	}
	r, c, _ := phaseRecordingReconciler(t, node, drain, plain)

	reconcileOperation(t, r, plain.Name)
	require.Equal(t, "WaitingForOperation", getOp(t, c, plain.Name).Status.Reason)
	require.Equal(t, "bashible", getNodeOf(t, c, node.Name).Annotations[nodecommon.DrainingAnnotation])

	reconcileOperation(t, r, drain.Name)

	fresh := getOp(t, c, drain.Name)
	require.Equal(t, "Draining", fresh.Status.Reason)
	require.NotNil(t, fresh.Status.DrainDeadline, "the time limit starts with the eviction")
	require.Equal(t, drainMarker(fresh), getNodeOf(t, c, node.Name).Annotations[nodecommon.DrainingAnnotation])
}

// The node has one drain request. The Drain of a deleted machine takes it on the
// pass that ends its holder, and never says it waits for an operation it ended.
func TestAMachineDrainTakesTheEvictionOfTheOperationItEnds(t *testing.T) {
	node := groupedNode("worker")
	reboot := rebootOf(node)
	drain := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "drain-worker-abcde", UID: "drain-uid",
			Labels:      map[string]string{v1alpha1.NodeOperationMachineUIDLabel: "machine-uid"},
			Annotations: map[string]string{v1alpha1.NodeOperationMachineAnnotation: machineOfDrain},
		},
		Spec: v1alpha1.NodeOperationSpec{Type: v1alpha1.NodeOperationTypeDrain, NodeName: node.Name},
	}
	// Written before the type was renamed: the API server refuses every write to it.
	unknown := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "approve-worker-abcde", UID: "old-uid"},
		Spec:       v1alpha1.NodeOperationSpec{Type: "ApproveDisruption", NodeName: node.Name},
	}
	r, c, _ := phaseRecordingReconciler(t, node, reboot, drain, unknown)

	reconcileOperation(t, r, reboot.Name)
	require.Equal(t, drainMarker(reboot), getNodeOf(t, c, node.Name).Annotations[nodecommon.DrainingAnnotation])
	unknownBefore := getOp(t, c, unknown.Name).ResourceVersion

	reconcileOperation(t, r, drain.Name)

	require.Equal(t, unknownBefore, getOp(t, c, unknown.Name).ResourceVersion, "an operation of an unknown type is not written")

	ended := getOp(t, c, reboot.Name)
	require.Equal(t, v1alpha1.NodeOperationPhaseFailed, ended.Status.Phase)
	require.Equal(t, "MachineDeleted", ended.Status.Reason)
	fresh := getOp(t, c, drain.Name)
	require.Equal(t, "Draining", fresh.Status.Reason)
	require.Equal(t, drainMarker(fresh), getNodeOf(t, c, node.Name).Annotations[nodecommon.DrainingAnnotation])
}

// machineOfDrain is the Machine the Drains of these tests evict for.
const machineOfDrain = "worker-cnvbj"

// createMachineDrain creates the Drain the machinedrain controller creates for a
// deleted Machine, unlimited for a static group without a drain timeout.
func createMachineDrain(ctx context.Context, nodeName string, unlimited bool) *v1alpha1.NodeOperation {
	GinkgoHelper()

	op := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name:        testenv.UniqueName("drain"),
			Labels:      map[string]string{v1alpha1.NodeOperationMachineUIDLabel: string(uuid.NewUUID())},
			Annotations: map[string]string{v1alpha1.NodeOperationMachineAnnotation: machineOfDrain},
		},
		Spec: v1alpha1.NodeOperationSpec{
			Type:     v1alpha1.NodeOperationTypeDrain,
			NodeName: nodeName,
			Cause:    "Machine " + machineOfDrain + " of NodeGroup worker is deleted",
		},
	}
	if unlimited {
		op.Annotations[v1alpha1.NodeOperationUnlimitedDrainAnnotation] = "true"
	}
	Expect(k8sClient.Create(ctx, op)).To(Succeed())
	DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, op) })
	return op
}

func setDrainDeadline(ctx context.Context, name string, deadline metav1.Time) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		op := getOperation(ctx, g, name)
		op.Status.DrainDeadline = &deadline
		g.Expect(k8sClient.Status().Update(ctx, op)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

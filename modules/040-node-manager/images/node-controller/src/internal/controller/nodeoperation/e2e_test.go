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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// User story: As a cluster operator, I want every interruption of a node to
// finish in bounded time and leave the node as found, so one stuck eviction
// cannot hold a node out of the scheduler or silently freeze a rollout.
var _ = Describe("NodeOperation controller", func() {
	It("completes a Drain once the workload has left", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("drain"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeDrain, nil)

		By("asking the draining controller to empty the node")
		waitForDrainRequest(ctx, node.Name)

		By("the draining controller reporting the workload gone")
		markDrained(ctx, node.Name)

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseCompleted))
			g.Expect(eventsOf(ctx, g, op.Name)).To(ContainElement(SatisfyAll(
				HaveField("Type", corev1.EventTypeNormal), HaveField("Reason", "Drained"))))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// A Drain hands the node to the draining controller rather than to the
	// node's agent, but it is a hand-over all the same: without it the record
	// never says when the eviction began, and nothing measures the wait.
	It("marks a Drain InProgress from the moment the eviction is asked for", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("started"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeDrain, nil)

		var startedAt metav1.Time
		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseInProgress))
			g.Expect(fresh.Status.StartedAt).NotTo(BeNil())
			startedAt = *fresh.Status.StartedAt
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("its markers being cleared, so it has to ask again")
		clearDrainAnnotations(ctx, node.Name)
		waitForDrainRequest(ctx, node.Name)

		Consistently(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.StartedAt).To(HaveValue(Equal(startedAt)))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())

		By("the workload leaving")
		markDrained(ctx, node.Name)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseCompleted))
			g.Expect(fresh.Status.StartedAt).To(HaveValue(Equal(startedAt)))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// The status the controller writes is only as real as the CRD lets through:
	// a property missing from the schema is pruned and the write is lost in
	// silence. An operator reads the stage, its cause and the time left here.
	It("says what a Drain is doing and until when", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("status"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeDrain, nil)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseInProgress))
			g.Expect(fresh.Status.Reason).To(Equal("Draining"))
			g.Expect(fresh.Status.Message).NotTo(BeEmpty())
			g.Expect(fresh.Status.Deadline).NotTo(BeNil())
			g.Expect(fresh.Status.DrainDeadline).NotTo(BeNil())
			g.Expect(fresh.Status.Deadline.Time).To(BeTemporally("==", fresh.Status.DrainDeadline.Add(operationTimeout)))
			g.Expect(fresh.Status.Conditions).To(ContainElement(SatisfyAll(
				HaveField("Type", conditionDrained), HaveField("Status", metav1.ConditionFalse), HaveField("Reason", "Draining"))))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		markDrained(ctx, node.Name)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseCompleted))
			g.Expect(fresh.Status.Reason).To(Equal("Drained"))
			g.Expect(fresh.Status.Deadline).To(BeNil(), "a finished operation has no deadline left")
			g.Expect(fresh.Status.Conditions).To(ContainElement(SatisfyAll(
				HaveField("Type", conditionDrained), HaveField("Status", metav1.ConditionTrue), HaveField("Reason", "Drained"))))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// The operation says how its eviction went, so nobody has to look anywhere
	// else to know whether the workload is still on the node.
	It("says on the operation how its eviction went", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("parent"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeReboot, nil)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseApproved), "the node must not act before the workload left")
			g.Expect(fresh.Status.Reason).To(Equal("Draining"))
			g.Expect(fresh.Status.Conditions).To(ContainElement(SatisfyAll(
				HaveField("Type", conditionDrained), HaveField("Status", metav1.ConditionFalse))))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		handOver(ctx, node.Name, op.Name)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Reason).To(Equal("WaitingForNode"))
			g.Expect(fresh.Status.Conditions).To(ContainElement(SatisfyAll(
				HaveField("Type", conditionDrained), HaveField("Status", metav1.ConditionTrue), HaveField("Reason", "Drained"))))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		// The eviction is recorded on the operation alone: no second record next
		// to it promises anything about the node.
		Consistently(func(g Gomega) {
			g.Expect(operationsOn(ctx, g, node.Name)).To(ConsistOf(HaveField("Name", op.Name)))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())

		skipped := createOperation(ctx, createNode(ctx, testenv.UniqueName("skipped"), false).Name,
			v1alpha1.NodeOperationTypeReboot, &v1alpha1.NodeOperationDrainSpec{Skip: true})
		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, skipped.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseInProgress))
			g.Expect(fresh.Status.Conditions).To(ContainElement(SatisfyAll(
				HaveField("Type", conditionDrained), HaveField("Status", metav1.ConditionTrue), HaveField("Reason", "DrainSkipped"))))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// NOP-1: the node writes only its report, and the outcome is decided here.
	DescribeTable("ends the operation the way the node reported it",
		func(ctx context.Context, reported string, phase v1alpha1.NodeOperationPhase, reason, eventType string) {
			node := createNode(ctx, testenv.UniqueName("report"), false)
			op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeReboot, &v1alpha1.NodeOperationDrainSpec{Skip: true})
			Eventually(func(g Gomega) {
				g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseInProgress))
			}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

			reportByNode(ctx, op.Name, reported, "the node says so")

			Eventually(func(g Gomega) {
				fresh := getOperation(ctx, g, op.Name)
				g.Expect(fresh.Status.Phase).To(Equal(phase))
				g.Expect(fresh.Status.Reason).To(Equal(reason))
				g.Expect(fresh.Status.Message).To(Equal("the node says so"))
				g.Expect(fresh.Status.FinishedAt).NotTo(BeNil())
				g.Expect(fresh.Status.Deadline).To(BeNil())
				g.Expect(eventsOf(ctx, g, op.Name)).To(ContainElement(SatisfyAll(
					HaveField("Type", eventType), HaveField("Reason", reason), HaveField("Message", "the node says so"))))
			}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		},
		Entry("applied", "Succeeded", v1alpha1.NodeOperationPhaseCompleted, "Succeeded", corev1.EventTypeNormal),
		Entry("failed", "Failed", v1alpha1.NodeOperationPhaseFailed, "NodeFailed", corev1.EventTypeWarning),
		Entry("refused", "Refused", v1alpha1.NodeOperationPhaseFailed, "NodeRefused", corev1.EventTypeWarning),
	)

	// The node gives the window back when it gives up on the revision. The interruption
	// itself went through, so the operation completes and the node goes back to the scheduler.
	It("completes a Disruption the node released without applying it", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("released"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeDisruption, nil)
		handOver(ctx, node.Name, op.Name)
		Expect(getNode(ctx, Default, node.Name).Spec.Unschedulable).To(BeTrue())

		reportByNode(ctx, op.Name, "Released", "The node did not apply generation 1 and runs generation 0. Its NodeConfig says why")

		message := fmt.Sprintf("Node %s is released without generation 1 applied. Its NodeConfig says why", node.Name)
		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseCompleted))
			g.Expect(fresh.Status.Reason).To(Equal("Released"))
			g.Expect(fresh.Status.Message).To(Equal(message))
			g.Expect(eventsOf(ctx, g, op.Name)).To(ContainElement(SatisfyAll(
				HaveField("Type", corev1.EventTypeNormal), HaveField("Reason", "Released"), HaveField("Message", message))))
			g.Expect(getNode(ctx, g, node.Name).Spec.Unschedulable).To(BeFalse())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// A reboot is over when the node is back, not when it goes down: until then the
	// node stays cordoned and the operation is the one thing that says why.
	DescribeTable("holds an operation whose node reboots until the node is back",
		func(ctx context.Context, opType v1alpha1.NodeOperationType) {
			node := createNode(ctx, testenv.UniqueName("rebooting"), false)
			bootNode(ctx, node.Name, "boot-1", corev1.ConditionTrue)
			op := createOperation(ctx, node.Name, opType, nil)
			handOver(ctx, node.Name, op.Name)

			reportByNode(ctx, op.Name, "RebootStarted", "The node is rebooting")

			Eventually(func(g Gomega) {
				fresh := getOperation(ctx, g, op.Name)
				g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseInProgress))
				g.Expect(fresh.Status.Reason).To(Equal("Rebooting"))
				g.Expect(fresh.Status.BootID).To(Equal("boot-1"))
				g.Expect(fresh.Status.Deadline).NotTo(BeNil())
				g.Expect(fresh.Status.Message).To(Equal(fmt.Sprintf("Node %s is rebooting. Waiting for it to come back, until %s",
					node.Name, fresh.Status.Deadline.UTC().Format(time.RFC3339))))
			}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

			By("the node booting again, not Ready yet")
			bootNode(ctx, node.Name, "boot-2", corev1.ConditionFalse)
			Consistently(func(g Gomega) {
				g.Expect(getOperation(ctx, g, op.Name).Status.Reason).To(Equal("Rebooting"))
				g.Expect(getNode(ctx, g, node.Name).Spec.Unschedulable).To(BeTrue(), "the node stays cordoned until it is back")
			}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())

			By("the node becoming Ready")
			bootNode(ctx, node.Name, "boot-2", corev1.ConditionTrue)
			Eventually(func(g Gomega) {
				fresh := getOperation(ctx, g, op.Name)
				g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseCompleted))
				g.Expect(fresh.Status.Reason).To(Equal("Succeeded"))
				g.Expect(fresh.Status.Message).To(Equal(fmt.Sprintf("Node %s rebooted and is Ready", node.Name)))
				g.Expect(getNode(ctx, g, node.Name).Spec.Unschedulable).To(BeFalse())
			}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		},
		Entry("a Reboot", v1alpha1.NodeOperationTypeReboot),
		Entry("a Disruption that switches the system image", v1alpha1.NodeOperationTypeDisruption),
	)

	It("fails an operation whose node does not come back from the reboot", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("lost"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeReboot, &v1alpha1.NodeOperationDrainSpec{Skip: true})
		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseInProgress))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		reportByNode(ctx, op.Name, "RebootStarted", "The node is rebooting")
		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Reason).To(Equal("Rebooting"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		expireDeadline(ctx, op.Name)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseFailed))
			g.Expect(fresh.Status.Reason).To(Equal("NodeTimedOut"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// A late write, by anyone, must not turn a failed operation back into a
	// live one: the node was already given back to the scheduler.
	It("refuses to reopen a finished operation", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("reopen"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeReboot, &v1alpha1.NodeOperationDrainSpec{Skip: true})
		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseInProgress))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		reportByNode(ctx, op.Name, "Refused", "not on this node")
		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseFailed))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Expect(k8sClient.Status().Patch(ctx, op, mergePatch(`{"status":{"phase":"InProgress"}}`))).
			To(MatchError(ContainSubstring("a finished operation cannot be reopened")))
	})

	// NOP-3: the request is in the API from the moment the node asks, and
	// nothing happens to the node until somebody approves it.
	It("waits for an approval before it touches the node", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("approval"), false)
		op := createApproval(ctx, node.Name, false)

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Reason).To(Equal("WaitingForApproval"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		Consistently(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhasePending))
			g.Expect(fresh.Status.Message).To(Equal(waitingForApprovalMessage), "a first attempt has no earlier one to name")
			g.Expect(fresh.Status.Deadline).To(BeNil(), "an approval takes as long as it takes")
			g.Expect(fresh.Status.NodeWasUnschedulable).To(BeNil(), "the cordon is recorded when the operation may go ahead")
			g.Expect(getNode(ctx, g, node.Name).Annotations).NotTo(HaveKey(nodecommon.DrainingAnnotation))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())

		By("an operator approving it")
		approve(ctx, op.Name)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseApproved), "approved, and no longer waiting for anybody")
			g.Expect(fresh.Status.Reason).To(Equal("Draining"))
			g.Expect(fresh.Status.Deadline).NotTo(BeNil())
			g.Expect(fresh.Status.Deadline.Time).To(BeTemporally(">", time.Now()), "the clock starts at the approval")
			g.Expect(drainRequestOn(ctx, g, node.Name)).To(Equal(drainingSource + "/" + string(op.UID)))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// The operator approving a retry learns from the request itself that it is
	// one, and why the attempt before it failed. A failure of another revision
	// is not an attempt at this one.
	It("names the failed attempt in the request that retries it", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("retry"), false)
		failDisruption(ctx, node.Name, "the previous revision failed")
		first := failDisruption(ctx, node.Name, "module example: no example.ko found")

		retry := createRetry(ctx, first)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, retry.Name)
			g.Expect(fresh.Status.Reason).To(Equal("WaitingForApproval"))
			g.Expect(fresh.Status.Message).To(Equal("Attempt 2 of 3. The previous attempt " + first.Name +
				" failed: module example: no example.ko found. Waiting for an operator to set spec.approved to true"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// Once handed to the node, the operation says what was done to the node,
	// what the node is to do, and by when it must answer.
	DescribeTable("says what the node was handed and until when",
		func(ctx context.Context, opType v1alpha1.NodeOperationType, skip bool, format string) {
			node := createNode(ctx, testenv.UniqueName("handed"), false)
			op := createOperation(ctx, node.Name, opType, &v1alpha1.NodeOperationDrainSpec{Skip: skip})
			if !skip {
				handOver(ctx, node.Name, op.Name)
			}

			Eventually(func(g Gomega) {
				fresh := getOperation(ctx, g, op.Name)
				g.Expect(fresh.Status.Reason).To(Equal("WaitingForNode"))
				g.Expect(fresh.Status.Deadline).NotTo(BeNil())
				g.Expect(fresh.Status.Message).To(Equal(fmt.Sprintf(format, node.Name, fresh.Status.Deadline.UTC().Format(time.RFC3339))))
			}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		},
		Entry("an approved Disruption after the drain", v1alpha1.NodeOperationTypeDisruption, false,
			"Approved. Node %s is drained and cordoned. Waiting for it to apply generation 1 and report, until %s"),
		Entry("an approved Disruption without a drain", v1alpha1.NodeOperationTypeDisruption, true,
			"Approved. Waiting for node %s to apply generation 1 and report, until %s"),
		Entry("a Reboot after the drain", v1alpha1.NodeOperationTypeReboot, false,
			"Node %s is drained and cordoned. Waiting for it to reboot and report, until %s"),
		Entry("a Reboot without a drain", v1alpha1.NodeOperationTypeReboot, true,
			"Waiting for node %s to reboot and report, until %s"),
	)

	// approved is the one field of the spec that moves, and only one way: an
	// approval taken back mid-drain would leave nobody owning the node.
	It("lets the approval be given and nothing else in the spec change", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("spec"), false)
		op := createApproval(ctx, node.Name, false)

		approve(ctx, op.Name)

		// Merge patches, as kubectl sends them: no resourceVersion, so the answer
		// is the schema's and not a conflict with the controller's own writes.
		Expect(k8sClient.Patch(ctx, op, mergePatch(`{"spec":{"approved":false}}`))).
			To(MatchError(ContainSubstring("an approval cannot be withdrawn")))
		Expect(k8sClient.Patch(ctx, op, mergePatch(`{"spec":{"configGeneration":2}}`))).
			To(MatchError(ContainSubstring("immutable except .spec.approved")))
		Expect(k8sClient.Patch(ctx, op, mergePatch(`{"spec":{"nodeConfigUID":"`+string(uuid.NewUUID())+`"}}`))).
			To(MatchError(ContainSubstring("immutable except .spec.approved")))
		Expect(k8sClient.Patch(ctx, op, mergePatch(`{"spec":{"drain":{"skip":true}}}`))).
			To(MatchError(ContainSubstring("immutable except .spec.approved")))
		Expect(k8sClient.Patch(ctx, op, mergePatch(`{"spec":{"cause":"another cause"}}`))).
			To(MatchError(ContainSubstring("immutable except .spec.approved")))

		By("an operator giving a Reboot its cause")
		reboot := &v1alpha1.NodeOperation{
			ObjectMeta: metav1.ObjectMeta{Name: testenv.UniqueName("cause")},
			Spec:       v1alpha1.NodeOperationSpec{Type: v1alpha1.NodeOperationTypeReboot, NodeName: node.Name, Cause: "kernel update"},
		}
		Expect(k8sClient.Create(ctx, reboot)).To(Succeed())
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, reboot) })
		Expect(getOperation(ctx, Default, reboot.Name).Spec.Cause).To(Equal("kernel update"))
		Expect(k8sClient.Patch(ctx, reboot, mergePatch(`{"spec":{"cause":null}}`))).
			To(MatchError(ContainSubstring("immutable except .spec.approved")))

		incomplete := &v1alpha1.NodeOperation{
			ObjectMeta: metav1.ObjectMeta{Name: testenv.UniqueName("nouid")},
			Spec: v1alpha1.NodeOperationSpec{Type: v1alpha1.NodeOperationTypeDisruption, NodeName: node.Name,
				ConfigGeneration: ptr.To(int64(1))},
		}
		Expect(k8sClient.Create(ctx, incomplete)).To(MatchError(ContainSubstring("requires .spec.configGeneration and .spec.nodeConfigUID")))
	})

	// An approved operation evicts the workload itself. A second operation next
	// to it in kubectl get would look like something else waiting for the operator.
	It("evicts under its own marker and creates no other operation", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("once"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeDisruption, nil)

		Eventually(func(g Gomega) {
			g.Expect(drainRequestOn(ctx, g, node.Name)).To(Equal(drainingSource + "/" + string(op.UID)))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("reconciling the operation again several times")
		for range 3 {
			nudge(ctx, op.Name)
		}

		Consistently(func(g Gomega) {
			g.Expect(operationsOn(ctx, g, node.Name)).To(ConsistOf(HaveField("Name", op.Name)))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// A drained marker outlives the operation that asked for it: a drain still
	// running after its operation gave up writes the marker back. Reading that
	// as "workload left" interrupts the node with its pods still on it.
	It("does not take another operation's drained marker for its own", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("stale"), false)
		stale := foreignMarker()
		annotate(ctx, node.Name, nodecommon.DrainedAnnotation, stale)

		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeDrain, nil)

		By("asking for an eviction of its own and leaving the other one's answer alone")
		Eventually(func(g Gomega) {
			fresh := getNode(ctx, g, node.Name)
			g.Expect(fresh.Annotations[nodecommon.DrainingAnnotation]).To(HavePrefix(drainingSource + "/"))
			g.Expect(fresh.Annotations[nodecommon.DrainingAnnotation]).NotTo(Equal(stale))
			g.Expect(fresh.Annotations).To(HaveKeyWithValue(nodecommon.DrainedAnnotation, stale))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).
				To(Equal(v1alpha1.NodeOperationPhaseInProgress), "somebody else's answer does not finish this eviction")
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())

		By("the workload actually leaving this time")
		markDrained(ctx, node.Name)

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseCompleted))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// releaseNode runs on every reconcile of a terminal operation. If it
	// cannot tell its own markers from a live sibling's, a finished operation
	// wipes an ongoing eviction's request and uncordons the node under it.
	It("leaves a live operation's drain alone when a finished one is released", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("sibling"), false)

		By("a Reboot that runs to the end and is kept as the record")
		finished := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeReboot, nil)
		handOver(ctx, node.Name, finished.Name)
		completeByNode(ctx, finished.Name)

		By("a Drain that is still evicting")
		live := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeDrain, nil)
		waitForDrainRequest(ctx, node.Name)

		var request string
		Eventually(func(g Gomega) {
			request = drainRequestOn(ctx, g, node.Name)
			g.Expect(request).To(HavePrefix(drainingSource + "/"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		// The finished operation keeps being reconciled the whole time.
		for range 3 {
			nudge(ctx, finished.Name)
		}

		Consistently(func(g Gomega) {
			g.Expect(getNode(ctx, g, node.Name).Annotations).
				To(HaveKeyWithValue(nodecommon.DrainingAnnotation, request),
					"the eviction still owed must keep the request it is waiting on")
			g.Expect(getOperation(ctx, g, live.Name).Status.Phase).NotTo(Equal(v1alpha1.NodeOperationPhaseFailed))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// The mutable path writes into the same two annotations. Nothing this
	// controller does may touch bashible's, or update-approval waits forever for
	// a drain nobody is doing.
	It("leaves the mutable path's drain annotations alone", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("bashible"), false)

		finished := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeReboot, nil)
		handOver(ctx, node.Name, finished.Name)
		completeByNode(ctx, finished.Name)

		By("bashible asking for a drain of its own")
		annotate(ctx, node.Name, nodecommon.DrainingAnnotation, "bashible")

		for range 3 {
			nudge(ctx, finished.Name)
		}

		Consistently(func(g Gomega) {
			g.Expect(getNode(ctx, g, node.Name).Annotations).
				To(HaveKeyWithValue(nodecommon.DrainingAnnotation, "bashible"))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// The two drain annotations are one slot, so two evictions on one node
	// cannot both be written there — but both need the node out of the
	// scheduler; a release must not put pods back onto an emptying node.
	It("does not uncordon a node another live Drain is holding", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("held"), false)

		By("a Reboot whose own eviction has finished, so it holds the node's marker")
		reboot := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeReboot, nil)
		handOver(ctx, node.Name, reboot.Name)

		By("a Drain of its own arriving and asking while the Reboot is still open")
		drain := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeDrain, nil)
		Eventually(func(g Gomega) {
			g.Expect(drainRequestOn(ctx, g, node.Name)).To(HavePrefix(drainingSource + "/"))
			g.Expect(getOperation(ctx, g, drain.Name).Status.Conditions).
				To(ContainElement(HaveField("Type", conditionDrained)))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the Reboot finishing and releasing what is its own")
		completeByNode(ctx, reboot.Name)

		Consistently(func(g Gomega) {
			g.Expect(getNode(ctx, g, node.Name).Spec.Unschedulable).
				To(BeTrue(), "an eviction is still running on this node")
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// Asking again is not a reason to be given another full drain: a deadline
	// renewed on every re-issue is no deadline at all, and the wait for an
	// eviction is unbounded again.
	It("does not extend its deadline by asking again", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("nopush"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeDrain, nil)

		var pinned metav1.Time
		Eventually(func(g Gomega) {
			deadline := getOperation(ctx, g, op.Name).Status.DrainDeadline
			g.Expect(deadline).NotTo(BeNil())
			pinned = *deadline
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("its markers being cleared repeatedly, so it has to ask again each time")
		for range 3 {
			clearDrainAnnotations(ctx, node.Name)
			waitForDrainRequest(ctx, node.Name)
		}

		Consistently(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.DrainDeadline).To(HaveValue(Equal(pinned)))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// Two operations can hold the same node at once. If the second records the
	// first one's cordon as the operator's, it puts that cordon back after the
	// first has lifted it, and every operation after that does the same.
	It("does not record a cordon another open operation is holding", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("shared"), false)

		first := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeReboot, nil)
		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, first.Name).Status.NodeWasUnschedulable).To(HaveValue(BeFalse()))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the first operation's eviction taking the node out of the scheduler")
		markDrained(ctx, node.Name)

		second := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeReboot, nil)
		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, second.Name).Status.NodeWasUnschedulable).To(HaveValue(BeFalse()))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// The other half of the same question: a cordon the operator placed is not
	// this controller's to lift, and an operation arriving while another one is
	// open must not answer "no cordon" merely because someone else is here.
	It("inherits an operator's cordon from the operation that recorded it", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("inherit"), true)

		first := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeReboot, nil)
		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, first.Name).Status.NodeWasUnschedulable).To(HaveValue(BeTrue()))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		second := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeReboot, nil)
		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, second.Name).Status.NodeWasUnschedulable).To(HaveValue(BeTrue()))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// An operator's hand-written operation arrives with no owner reference and
	// nothing but its spec. Its siblings find it by spec.nodeName, which the
	// CRD makes selectable; without that, the next one reads its cordon as the
	// operator's — the sticky cordon.
	It("finds a hand-written operation by its node and adopts it", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("byhand"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeReboot, nil)

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).OwnerReferences).To(ContainElement(HaveField("Kind", "Node")))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		list := &v1alpha1.NodeOperationList{}
		Expect(k8sClient.List(ctx, list, client.MatchingFields{v1alpha1.NodeOperationNodeNameField: node.Name})).To(Succeed())
		Expect(list.Items).To(ContainElement(HaveField("Name", op.Name)))
		Expect(list.Items).To(HaveEach(HaveField("Spec.NodeName", node.Name)))
	})

	// releaseNode clears both markers for whichever operation finishes first,
	// so a waiting sibling can have its request wiped. Asking once and trusting
	// the answer leaves it waiting on an eviction nobody is doing any more.
	It("asks again when its drain request is wiped out from under it", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("wiped"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeDrain, nil)

		waitForDrainRequest(ctx, node.Name)

		By("something removing both markers while the eviction is still owed")
		clearDrainAnnotations(ctx, node.Name)

		waitForDrainRequest(ctx, node.Name)

		By("the re-issued eviction finishing")
		markDrained(ctx, node.Name)

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseCompleted))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// The two drain annotations are one slot. Two operations that both write it
	// wake each other through the node event and take it back turn after turn,
	// while the draining controller drains for whoever wrote last.
	It("waits for the drain request instead of taking it from another operation", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("turns"), false)
		first := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeDrain, nil)

		waitForDrainRequest(ctx, node.Name)
		var held string
		Eventually(func(g Gomega) {
			held = drainRequestOn(ctx, g, node.Name)
			g.Expect(held).To(HavePrefix(drainingSource + "/"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("an operator interrupting the same node while the eviction runs")
		second := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeReboot, nil)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, second.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseApproved))
			g.Expect(fresh.Status.Reason).To(Equal("WaitingForOperation"), "the waiting operation says what it waits for")
			g.Expect(fresh.Status.Message).To(Equal(
				"the workload of node " + node.Name + " is already being evicted for NodeOperation " + first.Name + ". Waiting for that eviction to finish"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(drainRequestOn(ctx, g, node.Name)).
				To(Equal(held), "the eviction under way keeps the request it is waiting on")
			g.Expect(getOperation(ctx, g, second.Name).Status.Phase).NotTo(Equal(v1alpha1.NodeOperationPhaseFailed))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())

		By("the first eviction finishing, which frees the slot")
		markDrained(ctx, node.Name)

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, first.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseCompleted))
			g.Expect(drainRequestOn(ctx, g, node.Name)).
				To(SatisfyAll(HavePrefix(drainingSource+"/"), Not(Equal(held))), "the one that waited gets its turn")
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// The draining controller acts on the node alone. A marker left behind by an
	// operation that gave up outlives it, and evicts the workload of a node
	// whose operation reported failure long ago.
	It("takes its drain request back when it fails", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("giveup"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeDrain, nil)

		waitForDrainRequest(ctx, node.Name)

		By("the eviction never arriving, so the operation runs out of time")
		expireDrainDeadline(ctx, op.Name)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseFailed))
			g.Expect(fresh.Status.Reason).To(Equal("PreparationTimedOut"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Eventually(func(g Gomega) {
			fresh := getNode(ctx, g, node.Name)
			g.Expect(fresh.Annotations).NotTo(HaveKey(nodecommon.DrainingAnnotation))
			g.Expect(fresh.Annotations).NotTo(HaveKey(nodecommon.DrainedAnnotation))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// Deleting a running Drain is how an operator cancels it. A request left on the
	// node outlives the operation, and the draining controller evicts for nobody.
	It("withdraws its drain request when it is deleted mid-eviction, and leaves the node cordoned", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("cancel"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeDrain, nil)

		waitForDrainRequest(ctx, node.Name)

		By("the draining controller cordoning the node for the eviction")
		Expect(k8sClient.Patch(ctx, getNode(ctx, Default, node.Name), mergePatch(`{"spec":{"unschedulable":true}}`))).To(Succeed())

		By("an operator deleting the operation")
		Expect(k8sClient.Delete(ctx, op)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: op.Name}, &v1alpha1.NodeOperation{}))).
				To(BeTrue(), "the operation goes once it has let the node go")
			fresh := getNode(ctx, g, node.Name)
			g.Expect(fresh.Annotations).NotTo(HaveKey(nodecommon.DrainingAnnotation))
			g.Expect(fresh.Spec.Unschedulable).To(BeTrue(), "the cordon stays, as when a hand drain is withdrawn")
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(getNode(ctx, g, node.Name).Annotations).NotTo(HaveKey(nodecommon.DrainingAnnotation))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// A finished operation is deleted too, when it is collected. A completed Drain
	// asked for the node to stay out, and its collection must not undo that.
	It("keeps the node out when a completed Drain is deleted", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("keptout"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeDrain, nil)
		markDrained(ctx, node.Name)

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseCompleted))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Expect(k8sClient.Delete(ctx, op)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: op.Name}, &v1alpha1.NodeOperation{}))).To(BeTrue())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(getNode(ctx, g, node.Name).Spec.Unschedulable).To(BeTrue())
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// A node outside every group is invisible to the draining controller, so a
	// request written onto it is never picked up — and fires whenever someone
	// puts the node into a group, with no operation left to answer to.
	It("refuses to drain a node that belongs to no group", func(ctx context.Context) {
		node := createUngroupedNode(ctx, testenv.UniqueName("nogroup"))
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeDrain, nil)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseFailed))
			g.Expect(fresh.Status.Reason).To(Equal("NodeGroupMissing"))
			g.Expect(eventsOf(ctx, g, op.Name)).To(ContainElement(SatisfyAll(
				HaveField("Type", corev1.EventTypeWarning), HaveField("Reason", "NodeGroupMissing"), HaveField("Message", fresh.Status.Message))))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(getNode(ctx, g, node.Name).Annotations).NotTo(HaveKey(nodecommon.DrainingAnnotation))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// The mutable path asks for its drains with the bare word, not with an
	// operation's identity: the waiting operation names the update instead.
	It("names a bashible update holding the eviction it waits for", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("bbhold"), false)
		annotate(ctx, node.Name, nodecommon.DrainingAnnotation, "bashible")

		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeDrain, nil)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Reason).To(Equal("WaitingForOperation"))
			g.Expect(fresh.Status.Message).To(Equal(
				"the workload of node " + node.Name + " is already being evicted for a bashible update. Waiting for that eviction to finish"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// The draining controller retries a failing eviction until the deadline and
	// writes the error only onto the node; the operation is where it is read.
	It("says why its eviction keeps failing, to the end", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("evictfail"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeDrain, nil)
		waitForDrainRequest(ctx, node.Name)

		By("the draining controller recording a failed attempt")
		annotate(ctx, node.Name, nodecommon.DrainFailedAnnotation, evictionError)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Reason).To(Equal("Draining"))
			g.Expect(fresh.Status.Message).To(Equal("the eviction of node " + node.Name + " is failing: " + evictionError))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the eviction running out of time")
		expireDrainDeadline(ctx, op.Name)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseFailed))
			g.Expect(fresh.Status.Reason).To(Equal("PreparationTimedOut"))
			g.Expect(fresh.Status.Message).To(SatisfyAll(ContainSubstring(node.Name), ContainSubstring(evictionError)))
			g.Expect(eventsOf(ctx, g, op.Name)).To(ContainElement(SatisfyAll(
				HaveField("Type", corev1.EventTypeWarning), HaveField("Reason", "PreparationTimedOut"), HaveField("Message", fresh.Status.Message))))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// A Disruption or a Reboot is what node-manager and the operator
	// watch while the node is prepared: the error has to reach it.
	It("says on an interruption why its eviction keeps failing, to the end", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("parentfail"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeReboot, nil)
		waitForDrainRequest(ctx, node.Name)

		annotate(ctx, node.Name, nodecommon.DrainFailedAnnotation, evictionError)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseApproved))
			g.Expect(fresh.Status.Reason).To(Equal("Draining"))
			g.Expect(fresh.Status.Message).To(Equal("the eviction of node " + node.Name + " is failing: " + evictionError))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the eviction running out of time")
		expireDrainDeadline(ctx, op.Name)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseFailed))
			g.Expect(fresh.Status.Reason).To(Equal("PreparationTimedOut"))
			g.Expect(fresh.Status.Message).To(SatisfyAll(ContainSubstring(node.Name), ContainSubstring(evictionError)))
			g.Expect(eventsOf(ctx, g, op.Name)).To(ContainElement(SatisfyAll(
				HaveField("Type", corev1.EventTypeWarning), HaveField("Reason", "PreparationTimedOut"), HaveField("Message", fresh.Status.Message))))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	It("fails an operation whose node is gone", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("vanish"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeReboot, &v1alpha1.NodeOperationDrainSpec{Skip: true})

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).NotTo(BeEmpty())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Expect(k8sClient.Delete(ctx, node)).To(Succeed())

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseFailed))
			g.Expect(fresh.Status.Reason).To(Equal("NodeNotFound"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// A node that says nothing must not keep its group's rollout waiting
	// forever, and the node has to come back to the scheduler when it does.
	It("times a silent node out and gives the node back", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("silent"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeReboot, nil)

		handOver(ctx, node.Name, op.Name)
		expireDeadline(ctx, op.Name)

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseFailed))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Eventually(func(g Gomega) {
			fresh := getNode(ctx, g, node.Name)
			g.Expect(fresh.Spec.Unschedulable).To(BeFalse())
			g.Expect(fresh.Annotations).NotTo(HaveKey(nodecommon.DrainingAnnotation))
			g.Expect(fresh.Annotations).NotTo(HaveKey(nodecommon.DrainedAnnotation))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// The operation took the node out of the scheduler; an operator who had
	// already taken it out for their own reasons must get that back, not a node
	// silently put into service.
	It("restores a cordon the operator set by hand", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("cordoned"), true)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeReboot, nil)

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.NodeWasUnschedulable).To(HaveValue(BeTrue()))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		handOver(ctx, node.Name, op.Name)
		expireDeadline(ctx, op.Name)

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseFailed))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(getNode(ctx, g, node.Name).Spec.Unschedulable).To(BeTrue())
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// The report is what the deadline waits for: a report that lands as the
	// deadline falls due ends the operation as reported, not as timed out.
	// The stale-read half of the race is closed by the optimistic lock.
	It("keeps the node's report when the deadline is already due", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("report"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationTypeReboot, &v1alpha1.NodeOperationDrainSpec{Skip: true})

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseInProgress))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the node reporting success in the same write that makes the deadline due")
		fresh := &v1alpha1.NodeOperation{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: op.Name}, fresh)).To(Succeed())
		fresh.Status.StartedAt = &metav1.Time{Time: time.Now().Add(-2 * operationTimeout)}
		meta.SetStatusCondition(&fresh.Status.Conditions, nodeReport("Succeeded", "The node applied the configuration"))
		Expect(k8sClient.Status().Update(ctx, fresh)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseCompleted))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseCompleted))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})
})

// createNode creates the node every other spec works with: one that belongs to
// a group, which is what the draining controller requires of a node before it
// will evict anything on it.
func createNode(ctx context.Context, name string, unschedulable bool) *corev1.Node {
	GinkgoHelper()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{nodecommon.NodeGroupLabel: "worker"},
		},
		Spec: corev1.NodeSpec{Unschedulable: unschedulable},
	}
	Expect(k8sClient.Create(ctx, node)).To(Succeed())
	DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, node) })

	fresh := &corev1.Node{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name}, fresh)).To(Succeed())
	return fresh
}

func createOperation(ctx context.Context, nodeName string, opType v1alpha1.NodeOperationType, drain *v1alpha1.NodeOperationDrainSpec) *v1alpha1.NodeOperation {
	GinkgoHelper()

	op := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{Name: testenv.UniqueName("op")},
		Spec:       v1alpha1.NodeOperationSpec{Type: opType, NodeName: nodeName, Drain: drain},
	}
	// The CRD ties the covered revision to the operation that asks for it.
	if opType == v1alpha1.NodeOperationTypeDisruption {
		op.Spec.ConfigGeneration = ptr.To(int64(1))
		op.Spec.NodeConfigUID = uuid.NewUUID()
		op.Spec.Approved = ptr.To(true)
	}
	Expect(k8sClient.Create(ctx, op)).To(Succeed())
	DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, op) })
	return op
}

func getOperation(ctx context.Context, g Gomega, name string) *v1alpha1.NodeOperation {
	op := &v1alpha1.NodeOperation{}
	g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name}, op)).To(Succeed())
	return op
}

// evictionError is what the draining controller writes into drain-failed for an
// eviction a PodDisruptionBudget keeps refusing.
const evictionError = "Cannot evict pod as it would violate the pod disruption budget"

// eventsOf returns the events recorded on an operation. Listed across all
// namespaces: the recorder files a cluster-scoped object's events under "default".
func eventsOf(ctx context.Context, g Gomega, name string) []corev1.Event {
	events := &corev1.EventList{}
	g.Expect(k8sClient.List(ctx, events, client.InNamespace(""))).To(Succeed())

	var ofOperation []corev1.Event
	for _, event := range events.Items {
		if event.InvolvedObject.Kind == nodeOperationKind && event.InvolvedObject.Name == name {
			ofOperation = append(ofOperation, event)
		}
	}
	return ofOperation
}

func getNode(ctx context.Context, g Gomega, name string) *corev1.Node {
	node := &corev1.Node{}
	g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name}, node)).To(Succeed())
	return node
}

// createApproval creates the Disruption node-manager creates for a node
// that asked, approved or waiting for an operator.
func createApproval(ctx context.Context, nodeName string, approved bool) *v1alpha1.NodeOperation {
	GinkgoHelper()

	op := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{Name: testenv.UniqueName("disruption")},
		Spec: v1alpha1.NodeOperationSpec{
			Type:             v1alpha1.NodeOperationTypeDisruption,
			NodeName:         nodeName,
			ConfigGeneration: ptr.To(int64(1)),
			NodeConfigUID:    uuid.NewUUID(),
			Approved:         ptr.To(approved),
		},
	}
	Expect(k8sClient.Create(ctx, op)).To(Succeed())
	DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, op) })
	return op
}

// failDisruption runs an approved Disruption of a revision of its own to the
// node's report of failure, as an attempt that failed on the node.
func failDisruption(ctx context.Context, nodeName, message string) *v1alpha1.NodeOperation {
	GinkgoHelper()

	op := createOperation(ctx, nodeName, v1alpha1.NodeOperationTypeDisruption, &v1alpha1.NodeOperationDrainSpec{Skip: true})
	Eventually(func(g Gomega) {
		g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseInProgress))
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	reportByNode(ctx, op.Name, "Failed", message)
	Eventually(func(g Gomega) {
		g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseFailed))
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	return op
}

// createRetry creates what node-manager creates after a failed attempt: an
// unapproved Disruption of the same revision.
func createRetry(ctx context.Context, failed *v1alpha1.NodeOperation) *v1alpha1.NodeOperation {
	GinkgoHelper()

	op := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{Name: testenv.UniqueName("retry")},
		Spec: v1alpha1.NodeOperationSpec{
			Type:             v1alpha1.NodeOperationTypeDisruption,
			NodeName:         failed.Spec.NodeName,
			ConfigGeneration: failed.Spec.ConfigGeneration,
			NodeConfigUID:    failed.Spec.NodeConfigUID,
			Approved:         ptr.To(false),
		},
	}
	Expect(k8sClient.Create(ctx, op)).To(Succeed())
	DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, op) })
	return op
}

// approve is what the operator runs: kubectl patch --type=merge on spec.approved.
func approve(ctx context.Context, name string) {
	GinkgoHelper()

	op := &v1alpha1.NodeOperation{ObjectMeta: metav1.ObjectMeta{Name: name}}
	Expect(k8sClient.Patch(ctx, op, mergePatch(`{"spec":{"approved":true}}`))).To(Succeed())
}

func mergePatch(body string) client.Patch {
	return client.RawPatch(types.MergePatchType, []byte(body))
}

func operationsOn(ctx context.Context, g Gomega, nodeName string) []v1alpha1.NodeOperation {
	list := &v1alpha1.NodeOperationList{}
	g.Expect(k8sClient.List(ctx, list, client.MatchingFields{v1alpha1.NodeOperationNodeNameField: nodeName})).To(Succeed())
	return list.Items
}

// markDrained plays the draining controller: waits to be asked, cordons, then
// reports the workload gone by clearing the request and echoing its value into
// the drained marker — carrying the asking operation's identity back.
func markDrained(ctx context.Context, name string) {
	GinkgoHelper()

	waitForDrainRequest(ctx, name)

	Eventually(func(g Gomega) {
		node := getNode(ctx, g, name)
		if node.Annotations == nil {
			node.Annotations = map[string]string{}
		}
		requested := node.Annotations[nodecommon.DrainingAnnotation]
		g.Expect(requested).NotTo(BeEmpty(), "the draining controller only answers a request")

		patch := client.MergeFrom(node.DeepCopy())
		delete(node.Annotations, nodecommon.DrainingAnnotation)
		node.Annotations[nodecommon.DrainedAnnotation] = requested
		node.Spec.Unschedulable = true
		g.Expect(k8sClient.Patch(ctx, node, patch)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

// completeByNode plays the part of the on-node agent reporting that it carried
// the operation out, and waits for the controller to end it.
func completeByNode(ctx context.Context, name string) {
	GinkgoHelper()

	reportByNode(ctx, name, "Succeeded", "The node applied the configuration")
	Eventually(func(g Gomega) {
		g.Expect(getOperation(ctx, g, name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseCompleted))
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

// reportByNode writes what the node writes: its NodeReported condition only,
// by an update carrying the resourceVersion it read.
func reportByNode(ctx context.Context, name, reason, message string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		op := getOperation(ctx, g, name)
		meta.SetStatusCondition(&op.Status.Conditions, nodeReport(reason, message))
		g.Expect(k8sClient.Status().Update(ctx, op)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

func nodeReport(reason, message string) metav1.Condition {
	return metav1.Condition{Type: conditionNodeReported, Status: metav1.ConditionTrue, Reason: reason, Message: message}
}

// bootNode writes what the kubelet reports about the node's boot: its boot ID and
// whether the node is Ready.
func bootNode(ctx context.Context, name, bootID string, ready corev1.ConditionStatus) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		node := getNode(ctx, g, name)
		node.Status.NodeInfo.BootID = bootID
		node.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: ready}}
		g.Expect(k8sClient.Status().Update(ctx, node)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

// createUngroupedNode creates a node the draining controller does not watch:
// one with no group label, as a node has between joining and being labelled.
func createUngroupedNode(ctx context.Context, name string) *corev1.Node {
	GinkgoHelper()

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}}
	Expect(k8sClient.Create(ctx, node)).To(Succeed())
	DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, node) })
	return node
}

// clearDrainAnnotations is what releaseNode does to the node when any operation
// on it reaches a terminal phase — including one that is not this one.
func clearDrainAnnotations(ctx context.Context, name string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		node := getNode(ctx, g, name)
		patch := client.MergeFrom(node.DeepCopy())
		delete(node.Annotations, nodecommon.DrainingAnnotation)
		delete(node.Annotations, nodecommon.DrainedAnnotation)
		g.Expect(k8sClient.Patch(ctx, node, patch)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

// annotate writes one of the drain annotations directly, standing in for
// whoever else touches the node: an operation that is over, the mutable path, or
// an operator.
func annotate(ctx context.Context, name, key, value string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		node := getNode(ctx, g, name)
		patch := client.MergeFrom(node.DeepCopy())
		if node.Annotations == nil {
			node.Annotations = map[string]string{}
		}
		node.Annotations[key] = value
		g.Expect(k8sClient.Patch(ctx, node, patch)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

// foreignMarker is a well-formed marker belonging to some other operation.
func foreignMarker() string {
	return drainingSource + "/" + string(uuid.NewUUID())
}

func waitForDrainRequest(ctx context.Context, name string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		g.Expect(getNode(ctx, g, name).Annotations).
			To(HaveKeyWithValue(nodecommon.DrainingAnnotation, HavePrefix(drainingSource+"/")))
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

func drainRequestOn(ctx context.Context, g Gomega, name string) string {
	return getNode(ctx, g, name).Annotations[nodecommon.DrainingAnnotation]
}

// handOver walks the operation to the point where the node owns it: workload
// gone, operation InProgress. The cordon is recorded
// before the drain places one, so the snapshot cannot pick up our own cordon.
func handOver(ctx context.Context, nodeName, opName string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		g.Expect(getOperation(ctx, g, opName).Status.NodeWasUnschedulable).NotTo(BeNil())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

	markDrained(ctx, nodeName)
	Eventually(func(g Gomega) {
		g.Expect(getOperation(ctx, g, opName).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseInProgress))
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

// expireDeadline backdates StartedAt, the clock hardDeadline measures from.
// envtest cannot backdate metadata.creationTimestamp, so that branch of
// hardDeadline is covered by its unit tests instead.
func expireDeadline(ctx context.Context, name string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		op := getOperation(ctx, g, name)
		op.Status.StartedAt = &metav1.Time{Time: time.Now().Add(-2 * operationTimeout)}
		g.Expect(k8sClient.Status().Update(ctx, op)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

// expireDrainDeadline backdates the eviction's own deadline, the clock an
// operation still waiting to be drained is measured against.
func expireDrainDeadline(ctx context.Context, name string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		op := getOperation(ctx, g, name)
		op.Status.DrainDeadline = &metav1.Time{Time: time.Now().Add(-2 * operationTimeout)}
		g.Expect(k8sClient.Status().Update(ctx, op)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

func nudge(ctx context.Context, name string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		op := getOperation(ctx, g, name)
		patch := client.MergeFrom(op.DeepCopy())
		if op.Annotations == nil {
			op.Annotations = map[string]string{}
		}
		op.Annotations["test.deckhouse.io/nudge"] = testenv.UniqueName("n")
		g.Expect(k8sClient.Patch(ctx, op, patch)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

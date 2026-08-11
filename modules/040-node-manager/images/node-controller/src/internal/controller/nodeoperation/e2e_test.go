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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
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
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationDrain, nil)

		By("asking the draining controller to empty the node")
		waitForDrainRequest(ctx, node.Name)

		By("the draining controller reporting the workload gone")
		markDrained(ctx, node.Name)

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationCompleted))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// The eviction is a step of its own so that it can be watched; creating a
	// second one would drain the same node twice.
	It("spawns exactly one eviction and stays idempotent", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("once"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationApproveDisruption, nil)

		Eventually(func(g Gomega) {
			g.Expect(drainChildrenOf(ctx, g, op)).To(HaveLen(1))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("reconciling the parent again several times")
		for range 3 {
			nudge(ctx, op.Name)
		}

		Consistently(func(g Gomega) {
			g.Expect(drainChildrenOf(ctx, g, op)).To(HaveLen(1))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// A drained marker outlives the operation that asked for it: a drain still
	// running after its operation gave up writes the marker back. Reading that
	// as "workload left" interrupts the node with its pods still on it.
	It("does not take another operation's drained marker for its own", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("stale"), false)
		stale := foreignMarker()
		annotate(ctx, node.Name, nodecommon.DrainedAnnotation, stale)

		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationDrain, nil)

		By("asking for an eviction of its own and leaving the other one's answer alone")
		Eventually(func(g Gomega) {
			fresh := getNode(ctx, g, node.Name)
			g.Expect(fresh.Annotations[nodecommon.DrainingAnnotation]).To(HavePrefix(drainingSource + "/"))
			g.Expect(fresh.Annotations[nodecommon.DrainingAnnotation]).NotTo(Equal(stale))
			g.Expect(fresh.Annotations).To(HaveKeyWithValue(nodecommon.DrainedAnnotation, stale))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPending))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())

		By("the workload actually leaving this time")
		markDrained(ctx, node.Name)

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationCompleted))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// releaseNode runs on every reconcile of a terminal operation. If it
	// cannot tell its own markers from a live sibling's, a finished operation
	// wipes an ongoing eviction's request and uncordons the node under it.
	It("leaves a live operation's drain alone when a finished one is released", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("sibling"), false)

		By("a Reboot that runs to the end and is kept as the record")
		finished := createOperation(ctx, node.Name, v1alpha1.NodeOperationReboot, nil)
		handOver(ctx, node.Name, finished.Name)
		completeByNode(ctx, finished.Name)

		By("a Drain that is still evicting")
		live := createOperation(ctx, node.Name, v1alpha1.NodeOperationDrain, nil)
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
			g.Expect(getOperation(ctx, g, live.Name).Status.Phase).NotTo(Equal(v1alpha1.NodeOperationFailed))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// The mutable path writes into the same two annotations. Nothing this
	// controller does may touch bashible's, or update-approval waits forever for
	// a drain nobody is doing.
	It("leaves the mutable path's drain annotations alone", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("bashible"), false)

		finished := createOperation(ctx, node.Name, v1alpha1.NodeOperationReboot, nil)
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
		reboot := createOperation(ctx, node.Name, v1alpha1.NodeOperationReboot, nil)
		handOver(ctx, node.Name, reboot.Name)

		By("a Drain of its own arriving and asking while the Reboot is still open")
		drain := createOperation(ctx, node.Name, v1alpha1.NodeOperationDrain, nil)
		Eventually(func(g Gomega) {
			g.Expect(drainRequestOn(ctx, g, node.Name)).To(HavePrefix(drainingSource + "/"))
			g.Expect(getOperation(ctx, g, drain.Name).Status.Conditions).
				To(ContainElement(HaveField("Type", conditionDrainRequested)))
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
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationDrain, nil)

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

		first := createOperation(ctx, node.Name, v1alpha1.NodeOperationReboot, nil)
		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, first.Name).Status.NodeWasUnschedulable).To(HaveValue(BeFalse()))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the first operation's eviction taking the node out of the scheduler")
		markDrained(ctx, node.Name)

		second := createOperation(ctx, node.Name, v1alpha1.NodeOperationReboot, nil)
		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, second.Name).Status.NodeWasUnschedulable).To(HaveValue(BeFalse()))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// The other half of the same question: a cordon the operator placed is not
	// this controller's to lift, and an operation arriving while another one is
	// open must not answer "no cordon" merely because someone else is here.
	It("inherits an operator's cordon from the operation that recorded it", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("inherit"), true)

		first := createOperation(ctx, node.Name, v1alpha1.NodeOperationReboot, nil)
		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, first.Name).Status.NodeWasUnschedulable).To(HaveValue(BeTrue()))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		second := createOperation(ctx, node.Name, v1alpha1.NodeOperationReboot, nil)
		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, second.Name).Status.NodeWasUnschedulable).To(HaveValue(BeTrue()))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// An operator's hand-written operation arrives with neither owner
	// reference nor node label; unlabelled it is invisible to siblings, and
	// the next one reads its cordon as the operator's — the sticky cordon.
	It("labels a hand-written operation so its siblings can see it", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("byhand"), false)
		op := createUnlabelledOperation(ctx, node.Name)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Labels).To(HaveKeyWithValue(v1alpha1.NodeOperationNodeLabel, node.Name))
			g.Expect(fresh.OwnerReferences).To(ContainElement(HaveField("Kind", "Node")))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// releaseNode clears both markers for whichever operation finishes first,
	// so a waiting sibling can have its request wiped. Asking once and trusting
	// the answer leaves it waiting on an eviction nobody is doing any more.
	It("asks again when its drain request is wiped out from under it", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("wiped"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationDrain, nil)

		waitForDrainRequest(ctx, node.Name)

		By("something removing both markers while the eviction is still owed")
		clearDrainAnnotations(ctx, node.Name)

		waitForDrainRequest(ctx, node.Name)

		By("the re-issued eviction finishing")
		markDrained(ctx, node.Name)

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationCompleted))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// The two drain annotations are one slot. Two operations that both write it
	// wake each other through the node event and take it back turn after turn,
	// while the draining controller drains for whoever wrote last.
	It("waits for the drain request instead of taking it from another operation", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("turns"), false)
		first := createOperation(ctx, node.Name, v1alpha1.NodeOperationDrain, nil)

		waitForDrainRequest(ctx, node.Name)
		var held string
		Eventually(func(g Gomega) {
			held = drainRequestOn(ctx, g, node.Name)
			g.Expect(held).To(HavePrefix(drainingSource + "/"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("an operator interrupting the same node while the eviction runs")
		second := createOperation(ctx, node.Name, v1alpha1.NodeOperationReboot, nil)

		Consistently(func(g Gomega) {
			g.Expect(drainRequestOn(ctx, g, node.Name)).
				To(Equal(held), "the eviction under way keeps the request it is waiting on")
			g.Expect(getOperation(ctx, g, second.Name).Status.Phase).NotTo(Equal(v1alpha1.NodeOperationFailed))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())

		By("the first eviction finishing, which frees the slot")
		markDrained(ctx, node.Name)

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, first.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationCompleted))
			g.Expect(drainRequestOn(ctx, g, node.Name)).
				To(SatisfyAll(HavePrefix(drainingSource+"/"), Not(Equal(held))), "the one that waited gets its turn")
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// The draining controller acts on the node alone. A marker left behind by an
	// operation that gave up outlives it, and evicts the workload of a node
	// whose operation reported failure long ago.
	It("takes its drain request back when it fails", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("giveup"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationDrain, nil)

		waitForDrainRequest(ctx, node.Name)

		By("the eviction never arriving, so the operation runs out of time")
		expireDrainDeadline(ctx, op.Name)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationFailed))
			g.Expect(fresh.Status.Conditions).To(ContainElement(HaveField("Reason", "PreparationTimedOut")))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Eventually(func(g Gomega) {
			fresh := getNode(ctx, g, node.Name)
			g.Expect(fresh.Annotations).NotTo(HaveKey(nodecommon.DrainingAnnotation))
			g.Expect(fresh.Annotations).NotTo(HaveKey(nodecommon.DrainedAnnotation))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// A node outside every group is invisible to the draining controller, so a
	// request written onto it is never picked up — and fires whenever someone
	// puts the node into a group, with no operation left to answer to.
	It("refuses to drain a node that belongs to no group", func(ctx context.Context) {
		node := createUngroupedNode(ctx, testenv.UniqueName("nogroup"))
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationDrain, nil)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationFailed))
			g.Expect(fresh.Status.Conditions).To(ContainElement(HaveField("Reason", "NodeGroupMissing")))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(getNode(ctx, g, node.Name).Annotations).NotTo(HaveKey(nodecommon.DrainingAnnotation))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	It("fails an operation whose node is gone", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("vanish"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationReboot, &v1alpha1.NodeOperationDrainSpec{Skip: true})

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).NotTo(BeEmpty())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Expect(k8sClient.Delete(ctx, node)).To(Succeed())

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationFailed))
			g.Expect(fresh.Status.Conditions).To(ContainElement(HaveField("Reason", "NodeNotFound")))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// A node that says nothing must not keep its group's rollout waiting
	// forever, and the node has to come back to the scheduler when it does.
	It("times a silent node out and gives the node back", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("silent"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationReboot, nil)

		handOver(ctx, node.Name, op.Name)
		expireDeadline(ctx, op.Name)

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationFailed))
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
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationReboot, nil)

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.NodeWasUnschedulable).To(HaveValue(BeTrue()))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		handOver(ctx, node.Name, op.Name)
		expireDeadline(ctx, op.Name)

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationFailed))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(getNode(ctx, g, node.Name).Spec.Unschedulable).To(BeTrue())
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// The node and this controller both write the phase; this holds the end
	// state — a due deadline does not reopen what the node reported. The
	// stale-cache half of the race is closed by the optimistic lock in setPhase.
	It("keeps the node's report when the deadline is already due", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("report"), false)
		op := createOperation(ctx, node.Name, v1alpha1.NodeOperationReboot, &v1alpha1.NodeOperationDrainSpec{Skip: true})

		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationInProgress))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the node reporting success in the same write that makes the deadline due")
		fresh := &v1alpha1.NodeOperation{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: op.Name}, fresh)).To(Succeed())
		fresh.Status.StartedAt = &metav1.Time{Time: time.Now().Add(-2 * operationTimeout)}
		fresh.Status.Phase = v1alpha1.NodeOperationCompleted
		Expect(k8sClient.Status().Update(ctx, fresh)).To(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationCompleted))
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
		ObjectMeta: metav1.ObjectMeta{
			Name:   testenv.UniqueName("op"),
			Labels: map[string]string{v1alpha1.NodeOperationNodeLabel: nodeName},
		},
		Spec: v1alpha1.NodeOperationSpec{Type: opType, NodeName: nodeName, Drain: drain},
	}
	// The CRD ties the covered revision to the operation that asks for it.
	if opType == v1alpha1.NodeOperationApproveDisruption {
		op.Spec.ConfigGeneration = ptr.To(int64(1))
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

func getNode(ctx context.Context, g Gomega, name string) *corev1.Node {
	node := &corev1.Node{}
	g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name}, node)).To(Succeed())
	return node
}

func drainChildrenOf(ctx context.Context, g Gomega, parent *v1alpha1.NodeOperation) []v1alpha1.NodeOperation {
	list := &v1alpha1.NodeOperationList{}
	g.Expect(k8sClient.List(ctx, list, client.MatchingLabels{v1alpha1.NodeOperationNodeLabel: parent.Spec.NodeName})).To(Succeed())

	var children []v1alpha1.NodeOperation
	for i := range list.Items {
		child := list.Items[i]
		if child.Spec.Type != v1alpha1.NodeOperationDrain || child.Name == parent.Name {
			continue
		}
		for _, owner := range child.OwnerReferences {
			if owner.Name == parent.Name {
				children = append(children, child)
				break
			}
		}
	}
	return children
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
// the operation out.
func completeByNode(ctx context.Context, name string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		op := getOperation(ctx, g, name)
		op.Status.Phase = v1alpha1.NodeOperationCompleted
		g.Expect(k8sClient.Status().Update(ctx, op)).To(Succeed())
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

// createUnlabelledOperation writes the operation an operator writes: no owner
// reference, no node label, just the intent.
func createUnlabelledOperation(ctx context.Context, nodeName string) *v1alpha1.NodeOperation {
	GinkgoHelper()

	op := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{Name: testenv.UniqueName("byhand")},
		Spec:       v1alpha1.NodeOperationSpec{Type: v1alpha1.NodeOperationReboot, NodeName: nodeName},
	}
	Expect(k8sClient.Create(ctx, op)).To(Succeed())
	DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, op) })
	return op
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
// gone, child eviction finished, parent InProgress. The cordon is recorded
// before the drain places one, so the snapshot cannot pick up our own cordon.
func handOver(ctx context.Context, nodeName, opName string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		g.Expect(getOperation(ctx, g, opName).Status.NodeWasUnschedulable).NotTo(BeNil())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

	markDrained(ctx, nodeName)
	Eventually(func(g Gomega) {
		g.Expect(getOperation(ctx, g, opName).Status.Phase).To(Equal(v1alpha1.NodeOperationInProgress))
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

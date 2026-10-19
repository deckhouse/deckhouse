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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// User story: As a cluster operator, I want a failed Disruption to say whether another attempt
// follows and when, so that I do not wait for an approval that node-manager is never going to
// ask for.
var _ = Describe("Retry of a Disruption", func() {
	It("names the attempt that follows a failed one", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("retried"), false)
		nc := createNodeConfig(ctx, node.Name)
		first := runAttempt(ctx, nc, "Failed", "module example: no example.ko found")

		waitForNextSecond()
		retry := createRetry(ctx, first)

		Eventually(func(g Gomega) {
			g.Expect(retryOf(ctx, g, first.Name)).To(beRetry(metav1.ConditionTrue, "Created",
				"Attempt 2 of 3 is NodeOperation "+retry.Name))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	It("says when the next attempt is requested after a failed one", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("scheduled"), false)
		nc := createNodeConfig(ctx, node.Name)
		askToBeInterrupted(ctx, node.Name, time.Now())
		first := runAttempt(ctx, nc, "Failed", "module example: no example.ko found")

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, first.Name)
			g.Expect(fresh.Status.FinishedAt).NotTo(BeNil())
			g.Expect(retryOf(ctx, g, first.Name)).To(beRetry(metav1.ConditionTrue, "Scheduled",
				"Attempt 2 of 3 is requested at "+fresh.Status.FinishedAt.Add(time.Minute).UTC().Format(time.RFC3339)))
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseFailed))
			g.Expect(fresh.Status.Reason).To(Equal("NodeFailed"))
			g.Expect(fresh.Status.Message).To(Equal("module example: no example.ko found"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	It("says that no attempt follows once the NodeConfig moves on", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("moved"), false)
		nc := createNodeConfig(ctx, node.Name)
		askToBeInterrupted(ctx, node.Name, time.Now())
		first := runAttempt(ctx, nc, "Failed", "module example: no example.ko found")
		Eventually(func(g Gomega) {
			g.Expect(retryOf(ctx, g, first.Name)).To(HaveField("Reason", "Scheduled"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the NodeConfig moving to generation 2")
		Eventually(func(g Gomega) {
			fresh := getNodeConfig(ctx, g, node.Name)
			fresh.Spec.FeatureGates = map[string]bool{"example": true}
			g.Expect(k8sClient.Update(ctx, fresh)).To(Succeed())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(retryOf(ctx, g, first.Name)).To(beRetry(metav1.ConditionFalse, "Superseded",
				"The NodeConfig moved to generation 2. node-manager makes no new attempt for generation 1"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the NodeConfig being deleted")
		Expect(k8sClient.Delete(ctx, nc)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(retryOf(ctx, g, first.Name)).To(beRetry(metav1.ConditionFalse, "Superseded",
				fmt.Sprintf("The NodeConfig of node %s is no longer the one this operation was for", node.Name)))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// The count drops below the limit when the oldest attempt is collected, not the newest:
	// collection goes by finishedAt.
	It("says until when no attempt follows after the last one", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("exhausted"), false)
		nc := createNodeConfig(ctx, node.Name)
		askToBeInterrupted(ctx, node.Name, time.Now())
		first := runAttempt(ctx, nc, "Failed", "first")
		oldest := backdateFinished(ctx, first.Name, 3*time.Hour)
		second := runAttempt(ctx, nc, "Failed", "second")
		backdateFinished(ctx, second.Name, 2*time.Hour)
		third := runAttempt(ctx, nc, "Failed", "third")

		Eventually(func(g Gomega) {
			g.Expect(retryOf(ctx, g, third.Name)).To(beRetry(metav1.ConditionFalse, "AttemptsExhausted",
				"3 of 3 attempts for generation 1 failed. node-manager makes no new attempt until the NodeConfig changes, or until "+
					oldest.Add(24*time.Hour).UTC().Format(time.RFC3339)+", when NodeOperation "+first.Name+" is deleted"))
			g.Expect(retryOf(ctx, g, first.Name)).To(beRetry(metav1.ConditionTrue, "Created",
				"Attempt 2 of 3 is NodeOperation "+second.Name))
			g.Expect(retryOf(ctx, g, second.Name)).To(beRetry(metav1.ConditionTrue, "Created",
				"Attempt 3 of 3 is NodeOperation "+third.Name))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// A completed attempt is a window the node released, with the revision applied or not.
	// It is never retried, and it is not counted.
	DescribeTable("writes no Retry on a completed attempt the node asks again after",
		func(ctx context.Context, reported, message string) {
			node := createNode(ctx, testenv.UniqueName("released"), false)
			nc := createNodeConfig(ctx, node.Name)
			done := runAttempt(ctx, nc, reported, message)

			askToBeInterrupted(ctx, node.Name, finishedAtOf(ctx, done.Name).Add(time.Second))

			Consistently(func(g Gomega) {
				g.Expect(retryOf(ctx, g, done.Name)).To(BeNil())
				g.Expect(getOperation(ctx, g, done.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseCompleted))
			}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
		},
		Entry("applied", "Succeeded", "The node applied the configuration"),
		Entry("released", "Released", "The node did not apply generation 1 and runs generation 0. Its NodeConfig says why"),
	)

	It("counts the failed attempts only", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("counted"), false)
		nc := createNodeConfig(ctx, node.Name)
		done := runAttempt(ctx, nc, "Succeeded", "The node applied the configuration")
		askToBeInterrupted(ctx, node.Name, finishedAtOf(ctx, done.Name).Add(time.Second))
		failed := runAttempt(ctx, nc, "Failed", "module example: no example.ko found")
		finished := finishedAtOf(ctx, failed.Name)

		Eventually(func(g Gomega) {
			g.Expect(retryOf(ctx, g, failed.Name)).To(beRetry(metav1.ConditionTrue, "Scheduled",
				"Attempt 2 of 3 is requested at "+finished.Add(time.Minute).UTC().Format(time.RFC3339)))
			g.Expect(retryOf(ctx, g, done.Name)).To(BeNil())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// The new window after a completed one is not a retry, so its request reads as a first one.
	It("names no completed attempt in the request that follows it", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("next-window"), false)
		nc := createNodeConfig(ctx, node.Name)
		done := runAttempt(ctx, nc, "Succeeded", "The node applied the configuration")
		askToBeInterrupted(ctx, node.Name, finishedAtOf(ctx, done.Name).Add(time.Second))

		next := createRetry(ctx, done)

		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, next.Name)
			g.Expect(fresh.Status.Reason).To(Equal("WaitingForApproval"))
			g.Expect(fresh.Status.Message).To(Equal(waitingForApprovalMessage))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// A node that quarantined the revision stops asking, and node-manager makes no attempt for it.
	It("says that no attempt follows once the node stops asking", func(ctx context.Context) {
		node := createNode(ctx, testenv.UniqueName("not-asked"), false)
		nc := createNodeConfig(ctx, node.Name)
		first := runAttempt(ctx, nc, "Failed", "module example: no example.ko found")
		askToBeInterrupted(ctx, node.Name, finishedAtOf(ctx, first.Name).Add(time.Second))
		Eventually(func(g Gomega) {
			g.Expect(retryOf(ctx, g, first.Name)).To(HaveField("Reason", "Scheduled"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the node quarantining the revision at the same generation")
		Eventually(func(g Gomega) {
			fresh := getNodeConfig(ctx, g, node.Name)
			fresh.Status.Conditions = []metav1.Condition{{
				Type: "DisruptionApproved", Status: metav1.ConditionTrue, Reason: "NoDisruptionPending",
				Message: "no disruption is pending", ObservedGeneration: fresh.Generation, LastTransitionTime: metav1.Now(),
			}}
			g.Expect(k8sClient.Status().Update(ctx, fresh)).To(Succeed())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(retryOf(ctx, g, first.Name)).To(beRetry(metav1.ConditionFalse, "NotRequested",
				"The node no longer asks to be interrupted for generation 1. node-manager makes no new attempt"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})
})

// createNodeConfig creates the NodeConfig of a node at generation 1, the revision runAttempt
// asks to interrupt the node for.
func createNodeConfig(ctx context.Context, nodeName string) *internalv1alpha1.NodeConfig {
	GinkgoHelper()

	nc := &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName},
		Spec: internalv1alpha1.NodeSpec{
			SystemType: internalv1alpha1.SystemTypeImmutable,
			NodeName:   nodeName,
			OSImage:    internalv1alpha1.OSImage{Digest: testenv.TestOSImageDigest},
		},
	}
	Expect(k8sClient.Create(ctx, nc)).To(Succeed())
	DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, nc) })
	Expect(nc.Generation).To(Equal(int64(1)))
	return nc
}

func getNodeConfig(ctx context.Context, g Gomega, name string) *internalv1alpha1.NodeConfig {
	nc := &internalv1alpha1.NodeConfig{}
	g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name}, nc)).To(Succeed())
	return nc
}

// waitForNextSecond keeps the next attempt out of the second the previous one was created in.
// Creation timestamps are whole seconds, and the order of attempts is read off them: the
// attempts node-manager creates are minutes apart.
func waitForNextSecond() {
	time.Sleep(time.Until(time.Now().Truncate(time.Second).Add(time.Second)))
}

// runAttempt carries an approved Disruption of the NodeConfig's revision to the node's report.
func runAttempt(ctx context.Context, nc *internalv1alpha1.NodeConfig, reported, message string) *v1alpha1.NodeOperation {
	GinkgoHelper()

	waitForNextSecond()
	op := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{Name: testenv.UniqueName("attempt")},
		Spec: v1alpha1.NodeOperationSpec{
			Type:             v1alpha1.NodeOperationTypeDisruption,
			NodeName:         nc.Name,
			ConfigGeneration: ptr.To(nc.Generation),
			NodeConfigUID:    nc.UID,
			Approved:         ptr.To(true),
			Drain:            &v1alpha1.NodeOperationDrainSpec{Skip: true},
		},
	}
	Expect(k8sClient.Create(ctx, op)).To(Succeed())
	DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, op) })

	Eventually(func(g Gomega) {
		g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseInProgress))
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	reportByNode(ctx, op.Name, reported, message)
	Eventually(func(g Gomega) {
		fresh := getOperation(ctx, g, op.Name)
		g.Expect(terminal(fresh)).To(BeTrue())
		g.Expect(fresh.Status.FinishedAt).NotTo(BeNil())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	return op
}

// backdateFinished moves the end of a finished attempt into the past and returns it.
func backdateFinished(ctx context.Context, name string, ago time.Duration) time.Time {
	GinkgoHelper()

	finished := metav1.NewTime(time.Now().Add(-ago).Truncate(time.Second))
	Eventually(func(g Gomega) {
		op := getOperation(ctx, g, name)
		op.Status.FinishedAt = &finished
		g.Expect(k8sClient.Status().Update(ctx, op)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	return finished.Time
}

func finishedAtOf(ctx context.Context, name string) time.Time {
	GinkgoHelper()

	op := getOperation(ctx, Default, name)
	Expect(op.Status.FinishedAt).NotTo(BeNil())
	return op.Status.FinishedAt.Time
}

// askToBeInterrupted is what the node writes when it waits for permission to apply the
// revision it holds, the request dated as given.
func askToBeInterrupted(ctx context.Context, name string, at time.Time) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		nc := getNodeConfig(ctx, g, name)
		nc.Status.Conditions = []metav1.Condition{{
			Type:               "DisruptionApproved",
			Status:             metav1.ConditionFalse,
			Reason:             "ApprovalPending",
			Message:            "waiting for approval to interrupt this node: applying this config restarts kubelet",
			ObservedGeneration: nc.Generation,
			LastTransitionTime: metav1.NewTime(at),
		}}
		g.Expect(k8sClient.Status().Update(ctx, nc)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

func retryOf(ctx context.Context, g Gomega, name string) *metav1.Condition {
	return meta.FindStatusCondition(getOperation(ctx, g, name).Status.Conditions, "Retry")
}

func beRetry(status metav1.ConditionStatus, reason, message string) OmegaMatcher {
	return HaveValue(SatisfyAll(HaveField("Status", status), HaveField("Reason", reason), HaveField("Message", message)))
}

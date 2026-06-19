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

package bashiblecleanup

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/node-controller/internal/testenv"
)

func getEnvNode(name string) *corev1.Node {
	node := &corev1.Node{}
	Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: name}, node)).To(Succeed())
	return node
}

func hasBashibleTaint(node *corev1.Node) bool {
	for _, t := range node.Spec.Taints {
		if t.Key == bashibleUninitializedTaintKey {
			return true
		}
	}
	return false
}

func hasTaintKey(node *corev1.Node, key string) bool {
	for _, t := range node.Spec.Taints {
		if t.Key == key {
			return true
		}
	}
	return false
}

// User story: As a cluster user, I want a node to become schedulable as soon as bashible finishes
// its first run — its bootstrap-finished label and the "uninitialized" taint removed — so that my
// workloads can land on freshly bootstrapped nodes without manual intervention.
var _ = Describe("Bashible cleanup (Node watch-triggered)", func() {
	It("removes the bashible-first-run-finished label and the bashible-uninitialized taint", func() {
		name := testenv.UniqueName("cleanup")
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					bashibleFirstRunFinishedLabel:    "",
					"node-role.kubernetes.io/worker": "",
				},
			},
			Spec: corev1.NodeSpec{
				Taints: []corev1.Taint{
					{Key: bashibleUninitializedTaintKey, Effect: corev1.TaintEffectNoSchedule},
					{Key: "other-taint", Effect: corev1.TaintEffectNoExecute},
				},
			},
		}
		Expect(k8sClient.Create(suiteCtx, node)).To(Succeed())

		Eventually(func(g Gomega) {
			got := getEnvNode(name)
			_, hasLabel := got.Labels[bashibleFirstRunFinishedLabel]
			g.Expect(hasLabel).To(BeFalse(), "bashible-first-run-finished label must be removed")
			g.Expect(hasBashibleTaint(got)).To(BeFalse(), "bashible-uninitialized taint must be removed")

			// Unrelated label and taint must survive the cleanup.
			g.Expect(got.Labels).To(HaveKey("node-role.kubernetes.io/worker"))
			g.Expect(hasTaintKey(got, "other-taint")).To(BeTrue())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	It("removes the label even when the node carries no taints", func() {
		name := testenv.UniqueName("cleanup-notaint")
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{bashibleFirstRunFinishedLabel: ""},
			},
		}
		Expect(k8sClient.Create(suiteCtx, node)).To(Succeed())

		Eventually(func(g Gomega) {
			got := getEnvNode(name)
			_, hasLabel := got.Labels[bashibleFirstRunFinishedLabel]
			g.Expect(hasLabel).To(BeFalse())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	It("leaves the bashible taint untouched when the label is absent", func() {
		name := testenv.UniqueName("cleanup-nolabel")
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{"some-label": "value"},
			},
			Spec: corev1.NodeSpec{
				Taints: []corev1.Taint{
					{Key: bashibleUninitializedTaintKey, Effect: corev1.TaintEffectNoSchedule},
				},
			},
		}
		Expect(k8sClient.Create(suiteCtx, node)).To(Succeed())

		// Positive control: a node WITH the label, created at the same time, gets cleaned up.
		// Once the control is reconciled we know the controller has processed node events, so the
		// taint still being present on the no-label node is a real "not touched", not a not-yet.
		control := testenv.UniqueName("cleanup-nolabel-control")
		controlNode := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   control,
				Labels: map[string]string{bashibleFirstRunFinishedLabel: ""},
			},
			Spec: corev1.NodeSpec{
				Taints: []corev1.Taint{
					{Key: bashibleUninitializedTaintKey, Effect: corev1.TaintEffectNoSchedule},
				},
			},
		}
		Expect(k8sClient.Create(suiteCtx, controlNode)).To(Succeed())
		Eventually(func() bool {
			return hasBashibleTaint(getEnvNode(control))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(BeFalse())

		Consistently(func() bool {
			got := getEnvNode(name)
			_, hasLabel := got.Labels["some-label"]
			return hasBashibleTaint(got) && hasLabel
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(BeTrue())
	})
})

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

package staticproviderid

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/node-controller/internal/testenv"
)

const (
	eventuallyTimeout     = testenv.EventuallyTimeout
	eventuallyPoll        = testenv.EventuallyPoll
	negativeCheckDuration = testenv.NegativeCheckDuration
)

// createNode creates a node with the given type label value, providerID and taints. An empty
// typeLabel value means the node carries no type label at all; an empty providerID means none is
// set. The providerID is set at creation time so there is no window for the controller to race a
// later Update on it.
func createNode(name, typeLabel, providerID string, taints []corev1.Taint) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       corev1.NodeSpec{ProviderID: providerID, Taints: taints},
	}
	if typeLabel != "" {
		node.Labels = map[string]string{nodeTypeLabel: typeLabel}
	}
	Expect(k8sClient.Create(suiteCtx, node)).To(Succeed())
}

func providerIDOf(name string) string {
	node := &corev1.Node{}
	Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: name}, node)).To(Succeed())
	return node.Spec.ProviderID
}

var _ = AfterEach(func() {
	list := &corev1.NodeList{}
	Expect(k8sClient.List(suiteCtx, list)).To(Succeed())
	for i := range list.Items {
		Expect(k8sClient.Delete(suiteCtx, &list.Items[i])).To(Succeed())
	}
})

// User story: As a user running static (bare-metal/VM) nodes, I want each initialized Static node to
// receive a `static://` providerID, so that Kubernetes and Deckhouse treat it as a properly
// provisioned node.
var _ = Describe("StaticProviderID controller", func() {
	It("sets providerID=static:// on a Static node without one", func() {
		name := testenv.UniqueName("static")
		createNode(name, nodeTypeStatic, "", nil)

		Eventually(func() string {
			return providerIDOf(name)
		}, eventuallyTimeout, eventuallyPoll).Should(Equal(staticProviderIDValue))
	})

	It("leaves an existing providerID on a Static node untouched", func() {
		name := testenv.UniqueName("preset")
		createNode(name, nodeTypeStatic, "aws://existing", nil)

		Consistently(func() string {
			return providerIDOf(name)
		}, negativeCheckDuration, eventuallyPoll).Should(Equal("aws://existing"))
	})

	It("does not set providerID on a Static node carrying the uninitialized taint", func() {
		name := testenv.UniqueName("tainted")
		createNode(name, nodeTypeStatic, "", []corev1.Taint{
			{Key: uninitializedTaintKey, Effect: corev1.TaintEffectNoSchedule},
		})

		// Positive control: a plain Static node created alongside DOES get its providerID set,
		// so once that converges we know the controller has processed node events and the
		// tainted node's empty providerID is a real decision, not a not-yet-reconciled state.
		control := testenv.UniqueName("tainted-control")
		createNode(control, nodeTypeStatic, "", nil)
		Eventually(func() string {
			return providerIDOf(control)
		}, eventuallyTimeout, eventuallyPoll).Should(Equal(staticProviderIDValue))

		Consistently(func() string {
			return providerIDOf(name)
		}, negativeCheckDuration, eventuallyPoll).Should(BeEmpty())
	})

	It("does not set providerID on a non-Static node", func() {
		name := testenv.UniqueName("cloud")
		createNode(name, "CloudEphemeral", "", nil)

		control := testenv.UniqueName("cloud-control")
		createNode(control, nodeTypeStatic, "", nil)
		Eventually(func() string {
			return providerIDOf(control)
		}, eventuallyTimeout, eventuallyPoll).Should(Equal(staticProviderIDValue))

		Consistently(func() string {
			return providerIDOf(name)
		}, negativeCheckDuration, eventuallyPoll).Should(BeEmpty())
	})

	It("does not set providerID on a CloudStatic node (type is not exactly Static)", func() {
		name := testenv.UniqueName("cloudstatic")
		createNode(name, "CloudStatic", "", nil)

		control := testenv.UniqueName("cloudstatic-control")
		createNode(control, nodeTypeStatic, "", nil)
		Eventually(func() string {
			return providerIDOf(control)
		}, eventuallyTimeout, eventuallyPoll).Should(Equal(staticProviderIDValue))

		Consistently(func() string {
			return providerIDOf(name)
		}, negativeCheckDuration, eventuallyPoll).Should(BeEmpty())
	})

	It("does not set providerID on a node with no type label", func() {
		name := testenv.UniqueName("nolabel")
		createNode(name, "", "", nil)

		control := testenv.UniqueName("nolabel-control")
		createNode(control, nodeTypeStatic, "", nil)
		Eventually(func() string {
			return providerIDOf(control)
		}, eventuallyTimeout, eventuallyPoll).Should(Equal(staticProviderIDValue))

		Consistently(func() string {
			return providerIDOf(name)
		}, negativeCheckDuration, eventuallyPoll).Should(BeEmpty())
	})
})

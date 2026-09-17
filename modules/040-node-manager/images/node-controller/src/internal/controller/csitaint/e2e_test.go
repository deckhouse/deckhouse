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

package csitaint

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/node-controller/internal/testenv"
)

const (
	eventuallyTimeout     = testenv.EventuallyTimeout
	eventuallyPoll        = testenv.EventuallyPoll
	negativeCheckDuration = testenv.NegativeCheckDuration
)

const csiDriverName = "cinder.csi.openstack.org"

var otherTaint = corev1.Taint{Key: "somekey", Effect: corev1.TaintEffectPreferNoSchedule}

var csiTaint = corev1.Taint{Key: csiNotBootstrappedTaintKey, Effect: corev1.TaintEffectNoSchedule}

// createNode creates a node with the given taints. The apiserver's TaintNodesByCondition
// admission adds `node.kubernetes.io/not-ready` on top, so specs assert on individual taint keys
// rather than on the whole list.
func createNode(name string, taints ...corev1.Taint) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       corev1.NodeSpec{Taints: taints},
	}
	Expect(k8sClient.Create(suiteCtx, node)).To(Succeed())
}

// createCSINode creates a CSINode for the node of the same name. Passing no driver names creates
// one with an empty driver list — the shape a node has before its CSI driver registers.
func createCSINode(name string, driverNames ...string) {
	drivers := make([]storagev1.CSINodeDriver, 0, len(driverNames))
	for _, driverName := range driverNames {
		drivers = append(drivers, storagev1.CSINodeDriver{Name: driverName, NodeID: name})
	}

	csiNode := &storagev1.CSINode{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       storagev1.CSINodeSpec{Drivers: drivers},
	}
	Expect(k8sClient.Create(suiteCtx, csiNode)).To(Succeed())
}

func taintKeysOf(name string) []string {
	node := &corev1.Node{}
	Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: name}, node)).To(Succeed())

	keys := make([]string, 0, len(node.Spec.Taints))
	for _, taint := range node.Spec.Taints {
		keys = append(keys, taint.Key)
	}
	return keys
}

var _ = AfterEach(func() {
	csiNodes := &storagev1.CSINodeList{}
	Expect(k8sClient.List(suiteCtx, csiNodes)).To(Succeed())
	for i := range csiNodes.Items {
		Expect(k8sClient.Delete(suiteCtx, &csiNodes.Items[i])).To(Succeed())
	}

	nodes := &corev1.NodeList{}
	Expect(k8sClient.List(suiteCtx, nodes)).To(Succeed())
	for i := range nodes.Items {
		Expect(k8sClient.Delete(suiteCtx, &nodes.Items[i])).To(Succeed())
	}
})

// User story: As a user adding a node to a cloud cluster, I want its
// `node.deckhouse.io/csi-not-bootstrapped` taint removed as soon as the CSI driver has registered
// on that node, so that pods needing cloud volumes are scheduled there and no earlier.
var _ = Describe("CSITaint controller", func() {
	It("removes the taint once the CSINode registers a driver, keeping other taints", func() {
		name := testenv.UniqueName("registered")
		createNode(name, otherTaint, csiTaint)
		createCSINode(name, csiDriverName)

		Eventually(func() []string {
			return taintKeysOf(name)
		}, eventuallyTimeout, eventuallyPoll).ShouldNot(ContainElement(csiNotBootstrappedTaintKey))

		Expect(taintKeysOf(name)).To(ContainElement(otherTaint.Key))
	})

	It("keeps the taint while no CSINode exists for the node", func() {
		name := testenv.UniqueName("nocsinode")
		createNode(name, otherTaint, csiTaint)

		// Positive control: a node whose CSINode does have a driver converges, which proves the
		// controller has processed events and the tainted node above is a real decision.
		control := testenv.UniqueName("nocsinode-control")
		createNode(control, csiTaint)
		createCSINode(control, csiDriverName)
		Eventually(func() []string {
			return taintKeysOf(control)
		}, eventuallyTimeout, eventuallyPoll).ShouldNot(ContainElement(csiNotBootstrappedTaintKey))

		Consistently(func() []string {
			return taintKeysOf(name)
		}, negativeCheckDuration, eventuallyPoll).Should(ContainElement(csiNotBootstrappedTaintKey))
	})

	It("keeps the taint while the CSINode has no drivers, and removes it when a driver appears", func() {
		name := testenv.UniqueName("nodrivers")
		createNode(name, csiTaint)
		createCSINode(name)

		Consistently(func() []string {
			return taintKeysOf(name)
		}, negativeCheckDuration, eventuallyPoll).Should(ContainElement(csiNotBootstrappedTaintKey))

		csiNode := &storagev1.CSINode{}
		Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: name}, csiNode)).To(Succeed())
		csiNode.Spec.Drivers = []storagev1.CSINodeDriver{{Name: csiDriverName, NodeID: name}}
		Expect(k8sClient.Update(suiteCtx, csiNode)).To(Succeed())

		Eventually(func() []string {
			return taintKeysOf(name)
		}, eventuallyTimeout, eventuallyPoll).ShouldNot(ContainElement(csiNotBootstrappedTaintKey))
	})

	It("leaves the taints of a node without the CSI taint untouched", func() {
		name := testenv.UniqueName("untainted")
		createNode(name, otherTaint)
		createCSINode(name, csiDriverName)

		before := taintKeysOf(name)
		Consistently(func() []string {
			return taintKeysOf(name)
		}, negativeCheckDuration, eventuallyPoll).Should(ConsistOf(before))
	})
})

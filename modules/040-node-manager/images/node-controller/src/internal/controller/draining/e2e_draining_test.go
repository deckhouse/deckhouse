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

package draining

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

const (
	eventuallyTimeout     = testenv.EventuallyTimeout
	eventuallyPoll        = testenv.EventuallyPoll
	negativeCheckDuration = testenv.NegativeCheckDuration
)

func getNodeState(name string) *corev1.Node {
	node := &corev1.Node{}
	Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: name}, node)).To(Succeed())
	return node
}

// createGroupNode creates a node carrying the NodeGroup label (so the controller's event filter
// admits it) plus the given annotations. The group is also a real NodeGroup name when ngName is
// non-empty; callers that need a NodeGroup create it separately.
func createGroupNode(name, ngName string, annotations map[string]string) *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      map[string]string{nodecommon.NodeGroupLabel: ngName},
			Annotations: annotations,
		},
	}
	Expect(k8sClient.Create(suiteCtx, node)).To(Succeed())
	return node
}

func createNodeGroup(name string, drainTimeoutSecond *int) *deckhousev1.NodeGroup {
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType:               deckhousev1.NodeTypeStatic,
			NodeDrainTimeoutSecond: drainTimeoutSecond,
		},
	}
	Expect(k8sClient.Create(suiteCtx, ng)).To(Succeed())
	return ng
}

// createBoundPod creates a pod already bound to a node, with a zero grace period so that an
// eviction (delete) takes effect immediately under envtest, where no kubelet exists to finalize
// termination. ownerDS, when non-empty, marks the pod as owned by that DaemonSet.
func createBoundPod(name, nodeName, ownerDS string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: corev1.PodSpec{
			NodeName:                      nodeName,
			TerminationGracePeriodSeconds: ptr.To(int64(0)),
			Containers: []corev1.Container{
				{Name: "c", Image: "busybox"},
			},
		},
	}
	if ownerDS != "" {
		pod.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "DaemonSet",
			Name:       ownerDS,
			UID:        types.UID(ownerDS + "-uid"),
			Controller: ptr.To(true),
		}}
	}
	Expect(k8sClient.Create(suiteCtx, pod)).To(Succeed())
	return pod
}

func createDaemonSet(name string) *appsv1.DaemonSet {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: metav1.NamespaceDefault},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"ds": name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"ds": name}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "c", Image: "busybox"}},
				},
			},
		},
	}
	Expect(k8sClient.Create(suiteCtx, ds)).To(Succeed())
	return ds
}

func podExists(name string) bool {
	pod := &corev1.Pod{}
	err := k8sClient.Get(suiteCtx, types.NamespacedName{Namespace: metav1.NamespaceDefault, Name: name}, pod)
	if err == nil {
		return pod.DeletionTimestamp.IsZero()
	}
	Expect(apierrors.IsNotFound(err)).To(BeTrue())
	return false
}

var _ = AfterEach(func() {
	cleanupAll()
})

func cleanupAll() {
	Eventually(func(g Gomega) {
		podList := &corev1.PodList{}
		g.Expect(k8sClient.List(suiteCtx, podList, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())
		for i := range podList.Items {
			_ = client.IgnoreNotFound(k8sClient.Delete(suiteCtx, &podList.Items[i], client.GracePeriodSeconds(0)))
		}

		dsList := &appsv1.DaemonSetList{}
		g.Expect(k8sClient.List(suiteCtx, dsList, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())
		for i := range dsList.Items {
			_ = client.IgnoreNotFound(k8sClient.Delete(suiteCtx, &dsList.Items[i]))
		}

		nodeList := &corev1.NodeList{}
		g.Expect(k8sClient.List(suiteCtx, nodeList)).To(Succeed())
		for i := range nodeList.Items {
			_ = client.IgnoreNotFound(k8sClient.Delete(suiteCtx, &nodeList.Items[i]))
		}

		ngList := &deckhousev1.NodeGroupList{}
		g.Expect(k8sClient.List(suiteCtx, ngList)).To(Succeed())
		for i := range ngList.Items {
			_ = client.IgnoreNotFound(k8sClient.Delete(suiteCtx, &ngList.Items[i]))
		}
	}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
}

var _ = Describe("Draining a node on the draining annotation", func() {
	It("cordons, evicts non-DaemonSet pods, and flips draining->drained", func() {
		name := testenv.UniqueName("drain")
		createNodeGroup(name, nil)
		createDaemonSet("ds-" + name)
		createGroupNode(name, name, map[string]string{nodecommon.DrainingAnnotation: "bashible"})

		appPod := createBoundPod("app-"+name, name, "")
		dsPod := createBoundPod("ds-pod-"+name, name, "ds-"+name)

		Eventually(func(g Gomega) {
			node := getNodeState(name)
			g.Expect(node.Spec.Unschedulable).To(BeTrue())
			g.Expect(node.Annotations).NotTo(HaveKey(nodecommon.DrainingAnnotation))
			g.Expect(node.Annotations[nodecommon.DrainedAnnotation]).To(Equal("bashible"))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		Eventually(func() bool { return podExists(appPod.Name) }, eventuallyTimeout, eventuallyPoll).
			Should(BeFalse(), "non-DaemonSet pod should be evicted")

		Consistently(func() bool { return podExists(dsPod.Name) }, negativeCheckDuration, eventuallyPoll).
			Should(BeTrue(), "DaemonSet pod should survive the drain")
	})

	It("preserves a custom draining source into the drained annotation", func() {
		name := testenv.UniqueName("drain-custom")
		createGroupNode(name, name, map[string]string{nodecommon.DrainingAnnotation: "machine-controller"})

		Eventually(func(g Gomega) {
			node := getNodeState(name)
			g.Expect(node.Spec.Unschedulable).To(BeTrue())
			g.Expect(node.Annotations[nodecommon.DrainedAnnotation]).To(Equal("machine-controller"))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("drains a node whose NodeGroup sets a custom drain timeout", func() {
		name := testenv.UniqueName("drain-timeout")
		createNodeGroup(name, ptr.To(300))
		createGroupNode(name, name, map[string]string{nodecommon.DrainingAnnotation: "bashible"})

		Eventually(func(g Gomega) {
			node := getNodeState(name)
			g.Expect(node.Spec.Unschedulable).To(BeTrue())
			g.Expect(node.Annotations).NotTo(HaveKey(nodecommon.DrainingAnnotation))
			g.Expect(node.Annotations[nodecommon.DrainedAnnotation]).To(Equal("bashible"))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("removes a stale drained=user annotation from a schedulable node", func() {
		name := testenv.UniqueName("stale-user")
		createGroupNode(name, name, map[string]string{nodecommon.DrainedAnnotation: "user"})

		Eventually(func(g Gomega) {
			node := getNodeState(name)
			g.Expect(node.Annotations).NotTo(HaveKey(nodecommon.DrainedAnnotation))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("does not cordon a node that has no draining annotation", func() {
		name := testenv.UniqueName("no-draining")
		createGroupNode(name, name, nil)

		// Positive control: a sibling node with the draining annotation gets cordoned, proving the
		// controller is processing node events; only then is the absence of cordon meaningful.
		control := testenv.UniqueName("no-draining-control")
		createGroupNode(control, control, map[string]string{nodecommon.DrainingAnnotation: "bashible"})
		Eventually(func() bool {
			return getNodeState(control).Spec.Unschedulable
		}, eventuallyTimeout, eventuallyPoll).Should(BeTrue())

		Consistently(func() bool {
			return getNodeState(name).Spec.Unschedulable
		}, negativeCheckDuration, eventuallyPoll).Should(BeFalse())
	})

	It("keeps a non-user drained annotation on a schedulable node", func() {
		name := testenv.UniqueName("drained-bashible")
		createGroupNode(name, name, map[string]string{nodecommon.DrainedAnnotation: "bashible"})

		control := testenv.UniqueName("drained-bashible-control")
		createGroupNode(control, control, map[string]string{nodecommon.DrainingAnnotation: "bashible"})
		Eventually(func() bool {
			return getNodeState(control).Spec.Unschedulable
		}, eventuallyTimeout, eventuallyPoll).Should(BeTrue())

		Consistently(func() bool {
			return getNodeState(name).Annotations[nodecommon.DrainedAnnotation] == "bashible"
		}, negativeCheckDuration, eventuallyPoll).Should(BeTrue())
	})
})

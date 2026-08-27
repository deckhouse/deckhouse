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
	return createGroupNodeWithSpec(name, ngName, annotations, corev1.NodeSpec{})
}

// createGroupNodeWithSpec is createGroupNode for the cases that need the node to
// already be in a given state — cordoned, typically — before the controller ever
// sees it, rather than patched into it afterwards in a race with the reconcile.
func createGroupNodeWithSpec(name, ngName string, annotations map[string]string, spec corev1.NodeSpec) *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      map[string]string{nodecommon.NodeGroupLabel: ngName},
			Annotations: annotations,
		},
		Spec: spec,
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

// createStuckPod creates a pod on the node that can be evicted but never goes
// away: the finalizer keeps the object alive after its deletion is requested,
// and the drain waits once a second for it to disappear, indefinitely. That is
// the whole of a hard-stuck drain, without depending on a disruption controller
// envtest does not run.
//
// The finalizer is dropped again when the spec ends, or the pod outlives the
// suite.
func createStuckPod(name, nodeName string) *corev1.Pod {
	pod := createBoundPod(name, nodeName, "")
	pod.Finalizers = []string{"node-controller.test/hold"}
	Expect(k8sClient.Update(suiteCtx, pod)).To(Succeed())

	DeferCleanup(func() {
		Eventually(func(g Gomega) {
			fresh := &corev1.Pod{}
			err := k8sClient.Get(suiteCtx, client.ObjectKeyFromObject(pod), fresh)
			if apierrors.IsNotFound(err) {
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			fresh.Finalizers = nil
			g.Expect(client.IgnoreNotFound(k8sClient.Update(suiteCtx, fresh))).To(Succeed())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	return pod
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

// User story: As a cluster operator, I want a node marked for draining to be cordoned and have its
// non-DaemonSet pods evicted (honoring the NodeGroup's drain timeout) so that I can update or remove
// the node without disrupting my workloads.
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

	// A hand drain is closed out by an operator: the stale marker goes once the
	// node is back in service, and stays while it is still cordoned.
	It("removes a stale drained=user annotation only from a schedulable node", func() {
		schedulable := testenv.UniqueName("user-schedulable")
		createGroupNode(schedulable, schedulable, map[string]string{nodecommon.DrainedAnnotation: "user"})

		cordoned := testenv.UniqueName("user-cordoned")
		createGroupNodeWithSpec(cordoned, cordoned,
			map[string]string{nodecommon.DrainedAnnotation: "user"},
			corev1.NodeSpec{Unschedulable: true})

		Eventually(func(g Gomega) {
			g.Expect(getNodeState(schedulable).Annotations).NotTo(HaveKey(nodecommon.DrainedAnnotation))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		Consistently(func(g Gomega) {
			node := getNodeState(cordoned)
			g.Expect(node.Annotations).To(HaveKeyWithValue(nodecommon.DrainedAnnotation, "user"))
			g.Expect(node.Spec.Unschedulable).To(BeTrue())
		}, negativeCheckDuration, eventuallyPoll).Should(Succeed())
	})

	// User story: as a cluster operator who changed my mind, I want removing the
	// draining annotation to stop the eviction and give me my node back, instead
	// of having to wait the drain out and uncordon by hand.
	It("cancels an in-flight drain and uncordons the node when the request is withdrawn", func() {
		name := testenv.UniqueName("drain-cancel")

		// The pod comes first: a drain that starts before it exists has nothing
		// to evict and simply succeeds.
		createStuckPod("stuck-"+name, name)
		createGroupNode(name, name, map[string]string{nodecommon.DrainingAnnotation: "bashible"})

		// The pod going into termination is proof the eviction is under way — the
		// cordon alone is not, since it is written a pass earlier. The finalizer
		// then guarantees the eviction cannot finish on its own.
		Eventually(func() bool {
			return podExists("stuck-" + name)
		}, eventuallyTimeout, eventuallyPoll).Should(BeFalse())
		Expect(getNodeState(name).Spec.Unschedulable).To(BeTrue())

		Expect(k8sClient.Patch(suiteCtx, getNodeState(name), client.RawPatch(types.MergePatchType,
			[]byte(`{"metadata":{"annotations":{"`+nodecommon.DrainingAnnotation+`":null}}}`)))).To(Succeed())

		Eventually(func(g Gomega) {
			node := getNodeState(name)
			g.Expect(node.Spec.Unschedulable).To(BeFalse(), "a cancelled drain must uncordon the node")
			g.Expect(node.Annotations).NotTo(HaveKey(nodecommon.DrainedAnnotation), "a cancelled drain must not be recorded as done")
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

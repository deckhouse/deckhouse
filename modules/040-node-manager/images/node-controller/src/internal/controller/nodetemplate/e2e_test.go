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

package nodetemplate

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/testenv"
)

const (
	scaleDownDisabledAnnotation = "cluster-autoscaler.kubernetes.io/scale-down-disabled"
	nodeTypeLabel               = "node.deckhouse.io/type"
)

// createNodeGroup creates a NodeGroup with the given node type and optional template. For
// CloudEphemeral groups the CRD's oneOf schema requires spec.cloudInstances, so a minimal valid
// block is added in that case.
func createNodeGroup(name string, nodeType v1.NodeType, template *v1.NodeTemplate) *v1.NodeGroup {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1.NodeGroupSpec{NodeType: nodeType, NodeTemplate: template},
	}
	if nodeType == v1.NodeTypeCloudEphemeral {
		ng.Spec.CloudInstances = &v1.CloudInstancesSpec{
			MinPerZone:     1,
			MaxPerZone:     1,
			ClassReference: v1.ClassReference{Kind: "DVPInstanceClass", Name: name},
		}
	}
	Expect(k8sClient.Create(suiteCtx, ng)).To(Succeed())
	return ng
}

// createNode creates a Node carrying the given NodeGroup label, plus optional extra labels,
// annotations and taints.
func createNode(name, nodeGroupName string, labels, annotations map[string]string, taints []corev1.Taint) *corev1.Node {
	allLabels := map[string]string{}
	for k, v := range labels {
		allLabels[k] = v
	}
	if nodeGroupName != "" {
		allLabels[nodeGroupNameLabel] = nodeGroupName
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: allLabels, Annotations: annotations},
		Spec:       corev1.NodeSpec{Taints: taints},
	}
	Expect(k8sClient.Create(suiteCtx, node)).To(Succeed())
	return node
}

func getNodeFromAPI(name string) *corev1.Node {
	node := &corev1.Node{}
	Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: name}, node)).To(Succeed())
	return node
}

func uninitializedTaint() corev1.Taint {
	return corev1.Taint{Key: nodeUninitializedTaintKey, Effect: corev1.TaintEffectNoSchedule}
}

var _ = AfterEach(func() {
	Eventually(func(g Gomega) {
		nodeList := &corev1.NodeList{}
		g.Expect(k8sClient.List(suiteCtx, nodeList)).To(Succeed())
		for i := range nodeList.Items {
			g.Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, &nodeList.Items[i]))).To(Succeed())
		}
		ngList := &v1.NodeGroupList{}
		g.Expect(k8sClient.List(suiteCtx, ngList)).To(Succeed())
		for i := range ngList.Items {
			g.Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, &ngList.Items[i]))).To(Succeed())
		}
		g.Expect(nodeList.Items).To(BeEmpty())
		g.Expect(ngList.Items).To(BeEmpty())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
})

var _ = Describe("NodeTemplate controller applies the NodeGroup template to its nodes", func() {
	It("applies template labels/annotations/taints, system labels and removes the uninitialized taint", func() {
		ngName := testenv.UniqueName("worker")
		createNodeGroup(ngName, v1.NodeTypeStatic, &v1.NodeTemplate{
			Labels:      map[string]string{"template-label": "yes"},
			Annotations: map[string]string{"template-annotation": "yes"},
			Taints: []corev1.Taint{
				{Key: "dedicated", Value: "workload", Effect: corev1.TaintEffectNoSchedule},
			},
		})

		nodeName := testenv.UniqueName("worker-node")
		createNode(nodeName, ngName, nil, nil, []corev1.Taint{uninitializedTaint()})

		Eventually(func(g Gomega) {
			node := getNodeFromAPI(nodeName)

			// Template labels/annotations applied.
			g.Expect(node.Labels).To(HaveKeyWithValue("template-label", "yes"))
			g.Expect(node.Annotations).To(HaveKeyWithValue("template-annotation", "yes"))

			// System labels set: type, role and last-applied annotation.
			g.Expect(node.Labels).To(HaveKeyWithValue(nodeTypeLabel, string(v1.NodeTypeStatic)))
			g.Expect(node.Labels).To(HaveKey("node-role.kubernetes.io/" + ngName))
			g.Expect(node.Annotations).To(HaveKey(lastAppliedNodeTemplateAnnotation))

			// Static type => scale-down disabled.
			g.Expect(node.Annotations).To(HaveKeyWithValue(scaleDownDisabledAnnotation, "true"))

			// Template taint kept, uninitialized taint removed.
			g.Expect(taintSliceHasKey(node.Spec.Taints, "dedicated")).To(BeTrue())
			g.Expect(taintSliceHasKey(node.Spec.Taints, nodeUninitializedTaintKey)).To(BeFalse())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	It("applies master role labels and removes the legacy master taint not present in the template", func() {
		createNodeGroup("master", v1.NodeTypeCloudPermanent, nil)

		nodeName := testenv.UniqueName("master-node")
		createNode(nodeName, "master", nil, nil, []corev1.Taint{
			{Key: masterNodeRoleKey, Effect: corev1.TaintEffectNoSchedule},
		})

		Eventually(func(g Gomega) {
			node := getNodeFromAPI(nodeName)
			g.Expect(node.Labels).To(HaveKey(controlPlaneTaintKey))
			g.Expect(node.Labels).To(HaveKey(masterNodeRoleKey))
			g.Expect(node.Labels).To(HaveKeyWithValue(nodeTypeLabel, string(v1.NodeTypeCloudPermanent)))
			g.Expect(taintSliceHasKey(node.Spec.Taints, masterNodeRoleKey)).To(BeFalse())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	It("only fixes taints (no full template) on a non-CAPI cloud-ephemeral node", func() {
		ngName := testenv.UniqueName("cloud")
		createNodeGroup(ngName, v1.NodeTypeCloudEphemeral, &v1.NodeTemplate{
			Labels: map[string]string{"template-label": "yes"},
			Taints: []corev1.Taint{
				{Key: "dedicated", Value: "monitoring", Effect: corev1.TaintEffectNoSchedule},
			},
		})

		nodeName := testenv.UniqueName("cloud-node")
		createNode(nodeName, ngName, nil, nil, []corev1.Taint{
			{Key: "dedicated", Value: "monitoring", Effect: corev1.TaintEffectNoSchedule},
			uninitializedTaint(),
		})

		Eventually(func(g Gomega) {
			node := getNodeFromAPI(nodeName)
			g.Expect(taintSliceHasKey(node.Spec.Taints, nodeUninitializedTaintKey)).To(BeFalse())
			g.Expect(taintSliceHasKey(node.Spec.Taints, "dedicated")).To(BeTrue())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		// The full template (its label) must NOT be applied to a non-CAPI cloud-ephemeral node.
		Consistently(func(g Gomega) {
			node := getNodeFromAPI(nodeName)
			g.Expect(node.Labels).NotTo(HaveKey("template-label"))
			g.Expect(node.Labels).NotTo(HaveKey(nodeTypeLabel))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	It("applies the full template to a CAPI cloud-ephemeral node", func() {
		ngName := testenv.UniqueName("capi-cloud")
		createNodeGroup(ngName, v1.NodeTypeCloudEphemeral, &v1.NodeTemplate{
			Labels:      map[string]string{"template-label": "yes"},
			Annotations: map[string]string{"template-annotation": "yes"},
		})

		nodeName := testenv.UniqueName("capi-cloud-node")
		createNode(nodeName, ngName, nil, map[string]string{clusterAPIAnnotationKey: "machine-1"}, nil)

		Eventually(func(g Gomega) {
			node := getNodeFromAPI(nodeName)
			g.Expect(node.Labels).To(HaveKeyWithValue("template-label", "yes"))
			g.Expect(node.Labels).To(HaveKeyWithValue(nodeTypeLabel, string(v1.NodeTypeCloudEphemeral)))
			g.Expect(node.Annotations).To(HaveKeyWithValue("template-annotation", "yes"))
			g.Expect(node.Annotations).To(HaveKey(lastAppliedNodeTemplateAnnotation))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	It("does not modify an unmanaged node that carries no NodeGroup label", func() {
		// Positive control: a managed node created at the same time DOES get the template,
		// proving the controller is reconciling, so the unmanaged node's lack of change is real.
		ngName := testenv.UniqueName("ctrl")
		createNodeGroup(ngName, v1.NodeTypeStatic, nil)
		controlName := testenv.UniqueName("managed-node")
		createNode(controlName, ngName, nil, nil, nil)

		unmanagedName := testenv.UniqueName("unmanaged-node")
		createNode(unmanagedName, "", nil, nil, nil)

		Eventually(func(g Gomega) {
			g.Expect(getNodeFromAPI(controlName).Labels).To(HaveKey(nodeTypeLabel))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Consistently(func(g Gomega) {
			node := getNodeFromAPI(unmanagedName)
			g.Expect(node.Labels).NotTo(HaveKey(nodeTypeLabel))
			g.Expect(node.Annotations).NotTo(HaveKey(lastAppliedNodeTemplateAnnotation))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	It("does not modify an orphan node whose NodeGroup does not exist", func() {
		ngName := testenv.UniqueName("ctrl")
		createNodeGroup(ngName, v1.NodeTypeStatic, nil)
		controlName := testenv.UniqueName("managed-node")
		createNode(controlName, ngName, nil, nil, nil)

		orphanName := testenv.UniqueName("orphan-node")
		createNode(orphanName, testenv.UniqueName("ghost-ng"), nil, nil, nil)

		Eventually(func(g Gomega) {
			g.Expect(getNodeFromAPI(controlName).Labels).To(HaveKey(nodeTypeLabel))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Consistently(func(g Gomega) {
			node := getNodeFromAPI(orphanName)
			g.Expect(node.Labels).NotTo(HaveKey(nodeTypeLabel))
			g.Expect(node.Annotations).NotTo(HaveKey(lastAppliedNodeTemplateAnnotation))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})
})

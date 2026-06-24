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

package updateapproval

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	ua "github.com/deckhouse/node-controller/internal/controller/updateapproval/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

func intOrString(i int32) intstr.IntOrString { return intstr.FromInt32(i) }

// cleanupAll runs after every spec. NodeGroups and Nodes are cluster-scoped and envtest does not
// truly delete namespaces, so specs use unique names; this just removes the objects a spec created
// (and strips any finalizers) so the cluster does not accumulate state across specs.
var _ = AfterEach(func() {
	Eventually(func(g Gomega) {
		ngList := &v1.NodeGroupList{}
		g.Expect(k8sClient.List(suiteCtx, ngList)).To(Succeed())
		for i := range ngList.Items {
			_ = client.IgnoreNotFound(k8sClient.Delete(suiteCtx, &ngList.Items[i]))
			testenv.RemoveFinalizers(suiteCtx, k8sClient, &ngList.Items[i])
		}

		nodeList := &corev1.NodeList{}
		g.Expect(k8sClient.List(suiteCtx, nodeList)).To(Succeed())
		for i := range nodeList.Items {
			_ = client.IgnoreNotFound(k8sClient.Delete(suiteCtx, &nodeList.Items[i]))
			testenv.RemoveFinalizers(suiteCtx, k8sClient, &nodeList.Items[i])
		}

		g.Expect(ngList.Items).To(BeEmpty())
		g.Expect(nodeList.Items).To(BeEmpty())
	}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
})

const (
	eventuallyTimeout     = testenv.EventuallyTimeout
	eventuallyPoll        = testenv.EventuallyPoll
	negativeCheckDuration = testenv.NegativeCheckDuration
)

func uniqueName(base string) string { return testenv.UniqueName(base) }

// createNodeGroup creates a NodeGroup and applies the given mutator before creating it, so a
// spec can set spec.disruptions / spec.update etc. nodeType is Static (the simplest CRD branch:
// it forbids cloudInstances and keeps the ApproveUpdates ready-batch path active).
func createNodeGroup(name string, mutate func(*v1.NodeGroup)) *v1.NodeGroup {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}
	if mutate != nil {
		mutate(ng)
	}
	Expect(k8sClient.Create(suiteCtx, ng)).To(Succeed())
	return ng
}

// setReadyStatus sets the NodeGroup status Ready/Nodes counters via the status subresource, which
// NeedDrainNode consults (it refuses to drain when the group has fewer than two ready nodes).
func setReadyStatus(name string, ready, nodes int32) {
	ng := &v1.NodeGroup{}
	Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: name}, ng)).To(Succeed())
	ng.Status.Ready = ready
	ng.Status.Nodes = nodes
	Expect(k8sClient.Status().Update(suiteCtx, ng)).To(Succeed())
}

// setChecksum writes the NodeGroup's configuration checksum into the shared configuration-checksums
// secret the controller reads. Keying by NodeGroup name keeps specs isolated despite the shared
// secret object.
func setChecksum(ngName, checksum string) {
	secret := &corev1.Secret{}
	Expect(k8sClient.Get(suiteCtx, types.NamespacedName{
		Namespace: ua.MachineNamespace,
		Name:      ua.ConfigurationChecksumsSecretName,
	}, secret)).To(Succeed())
	patch := client.MergeFrom(secret.DeepCopy())
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secret.Data[ngName] = []byte(checksum)
	Expect(k8sClient.Patch(suiteCtx, secret, patch)).To(Succeed())
}

// createReadyNode creates a Ready node belonging to ngName, carrying the given annotations.
// The Ready condition lives on the status subresource, so it is applied after create; because
// the controller starts reconciling the node the moment it is created (the watch fires on
// create) and mutates its annotations, the status update is retried on the resulting conflict
// by re-getting the latest version.
func createReadyNode(name, ngName string, annotations map[string]string) *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      map[string]string{ua.NodeGroupLabel: ngName},
			Annotations: annotations,
		},
	}
	Expect(k8sClient.Create(suiteCtx, node)).To(Succeed())

	Eventually(func() error {
		latest := &corev1.Node{}
		if err := k8sClient.Get(suiteCtx, types.NamespacedName{Name: name}, latest); err != nil {
			return err
		}
		latest.Status.Conditions = []corev1.NodeCondition{
			{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
		}
		return k8sClient.Status().Update(suiteCtx, latest)
	}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	return node
}

func nodeState(name string) *corev1.Node {
	node := &corev1.Node{}
	Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: name}, node)).To(Succeed())
	return node
}

func hasAnnotation(node *corev1.Node, key string) bool {
	_, ok := node.Annotations[key]
	return ok
}

// approvedCount counts how many of the named nodes carry the approved annotation.
func approvedCount(names ...string) int {
	count := 0
	for _, name := range names {
		if hasAnnotation(nodeState(name), ua.ApprovedAnnotation) {
			count++
		}
	}
	return count
}

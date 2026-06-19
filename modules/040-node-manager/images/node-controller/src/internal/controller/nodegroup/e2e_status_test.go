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

package nodegroup

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// uniqueNG yields a unique cluster-scoped NodeGroup name per spec. NodeGroups are cluster-scoped
// and envtest namespaces never truly delete, so each spec uses its own name for isolation.
func uniqueNG(base string) string { return testenv.UniqueName(base) }

func createNodeGroup(ng *v1.NodeGroup) *v1.NodeGroup {
	Expect(k8sClient.Create(suiteCtx, ng)).To(Succeed())
	return ng
}

func staticNodeGroup(name string) *v1.NodeGroup {
	return &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}
}

func cloudEphemeralNodeGroup(name string, zones []string, minPerZone, maxPerZone int32) *v1.NodeGroup {
	return &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			CloudInstances: &v1.CloudInstancesSpec{
				Zones:          zones,
				MinPerZone:     minPerZone,
				MaxPerZone:     maxPerZone,
				ClassReference: v1.ClassReference{Kind: "DVPInstanceClass", Name: name + "-class"},
			},
		},
	}
}

// createGroupNode creates a Node labelled into ngName. ready controls the NodeReady condition;
// checksum (when non-empty) sets the configuration-checksum annotation; extraAnnotations are
// merged in (e.g. disruption-required).
func createGroupNode(name, ngName string, ready bool, checksum string, extraAnnotations map[string]string) *corev1.Node {
	annotations := map[string]string{}
	for k, v := range extraAnnotations {
		annotations[k] = v
	}
	if checksum != "" {
		annotations[nodecommon.ConfigurationChecksumAnnotation] = checksum
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      map[string]string{nodecommon.NodeGroupLabel: ngName},
			Annotations: annotations,
		},
	}
	Expect(k8sClient.Create(suiteCtx, node)).To(Succeed())

	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	node.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: status}}
	Expect(k8sClient.Status().Update(suiteCtx, node)).To(Succeed())
	return node
}

func createChecksumSecret(ngName, checksum string) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ConfigurationChecksumsSecretName,
			Namespace: common.MachineNamespace,
		},
		Data: map[string][]byte{ngName: []byte(checksum)},
	}
	Expect(k8sClient.Create(suiteCtx, secret)).To(Succeed())
	return secret
}

// createMCMMachine creates an MCM Machine carrying the node-group label on its node template, the
// source the controller counts as an instance of ngName.
func createMCMMachine(name, ngName string) *mcmv1alpha1.Machine {
	m := &mcmv1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: common.MachineNamespace},
		Spec: mcmv1alpha1.MachineSpec{
			NodeTemplateSpec: mcmv1alpha1.NodeTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{nodecommon.NodeGroupLabel: ngName}},
			},
		},
	}
	Expect(k8sClient.Create(suiteCtx, m)).To(Succeed())
	return m
}

// createMCMMachineDeployment creates an MCM MachineDeployment (unstructured) for ngName with the
// given replicas; failureMsg, when set, adds a status.failedMachines entry the controller turns
// into a LastMachineFailure and an Error condition.
func createMCMMachineDeployment(name, ngName string, replicas int64, failureMsg string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(common.MCMMachineDeploymentGVK)
	u.SetName(name)
	u.SetNamespace(common.MachineNamespace)
	u.SetLabels(map[string]string{"node-group": ngName})
	Expect(unstructured.SetNestedField(u.Object, replicas, "spec", "replicas")).To(Succeed())
	Expect(k8sClient.Create(suiteCtx, u)).To(Succeed())

	if failureMsg != "" {
		Expect(unstructured.SetNestedSlice(u.Object, []interface{}{
			map[string]interface{}{
				"name": name + "-broken",
				"lastOperation": map[string]interface{}{
					"description":    failureMsg,
					"lastUpdateTime": "2025-06-01T00:00:00Z",
				},
			},
		}, "status", "failedMachines")).To(Succeed())
		Expect(k8sClient.Status().Update(suiteCtx, u)).To(Succeed())
	}
	return u
}

func getNG(name string) *v1.NodeGroup {
	ng := &v1.NodeGroup{}
	Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: name}, ng)).To(Succeed())
	return ng
}

func conditionStatus(ng *v1.NodeGroup, condType string) metav1.ConditionStatus {
	for i := range ng.Status.Conditions {
		if ng.Status.Conditions[i].Type == condType {
			return ng.Status.Conditions[i].Status
		}
	}
	return ""
}

// processedDeckhouse reads the unstructured status.deckhouse the processed-status service writes
// (synced flag and processed.checkSum); these fields are not on the typed NodeGroupStatus.
func processedDeckhouse(name string) (synced, processedCheckSum string) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(v1.GroupVersion.WithKind("NodeGroup"))
	if err := k8sClient.Get(suiteCtx, types.NamespacedName{Name: name}, u); err != nil {
		return "", ""
	}
	synced, _, _ = unstructured.NestedString(u.Object, "status", "deckhouse", "synced")
	processedCheckSum, _, _ = unstructured.NestedString(u.Object, "status", "deckhouse", "processed", "checkSum")
	return synced, processedCheckSum
}

// User story: As a platform user, I want my NodeGroup's status to accurately report node/ready/
// up-to-date counters, cloud desired/min/max, machine failures and an overall readiness summary, so
// that I can monitor the health and scaling of my node pool at a glance.
var _ = Describe("NodeGroup status controller", func() {
	Context("Static NodeGroup", func() {
		It("reports zero counters and a Ready summary when there are no nodes", func() {
			name := uniqueNG("static-empty")
			createNodeGroup(staticNodeGroup(name))

			Eventually(func(g Gomega) {
				ng := getNG(name)
				g.Expect(ng.Status.Nodes).To(Equal(int32(0)))
				g.Expect(ng.Status.Ready).To(Equal(int32(0)))
				g.Expect(ng.Status.UpToDate).To(Equal(int32(0)))
				g.Expect(ng.Status.Desired).To(Equal(int32(0)))
				g.Expect(ng.Status.ConditionSummary).NotTo(BeNil())
				g.Expect(ng.Status.ConditionSummary.Ready).To(Equal("True"))
				g.Expect(conditionStatus(ng, common.ConditionTypeReady)).To(Equal(metav1.ConditionTrue))
			}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		})

		It("counts nodes and ready nodes, and marks up-to-date by matching checksum", func() {
			name := uniqueNG("static-counts")
			createNodeGroup(staticNodeGroup(name))
			createChecksumSecret(name, "good")
			createGroupNode(name+"-n1", name, true, "good", nil)
			createGroupNode(name+"-n2", name, true, "good", nil)
			createGroupNode(name+"-n3", name, false, "stale", nil)

			Eventually(func(g Gomega) {
				ng := getNG(name)
				g.Expect(ng.Status.Nodes).To(Equal(int32(3)))
				g.Expect(ng.Status.Ready).To(Equal(int32(2)))
				g.Expect(ng.Status.UpToDate).To(Equal(int32(2)))
			}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		})

		It("sets Updating and WaitingForDisruptiveApproval when a node needs disruptive approval", func() {
			name := uniqueNG("static-disrupt")
			createNodeGroup(staticNodeGroup(name))
			createGroupNode(name+"-n1", name, true, "", map[string]string{
				nodecommon.DisruptionRequiredAnnotation: "",
			})

			Eventually(func(g Gomega) {
				ng := getNG(name)
				g.Expect(ng.Status.Nodes).To(Equal(int32(1)))
				g.Expect(conditionStatus(ng, common.ConditionTypeUpdating)).To(Equal(metav1.ConditionTrue))
				g.Expect(conditionStatus(ng, common.ConditionTypeWaitingForDisruptiveApproval)).To(Equal(metav1.ConditionTrue))
			}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		})

		It("does not populate cloud counters for a Static NodeGroup", func() {
			name := uniqueNG("static-nocloud")
			createNodeGroup(staticNodeGroup(name))

			// Positive control: wait until the controller has populated the summary, so the
			// zero cloud counters below are a real result and not a not-yet-reconciled state.
			Eventually(func(g Gomega) {
				g.Expect(getNG(name).Status.ConditionSummary).NotTo(BeNil())
			}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

			Consistently(func(g Gomega) {
				ng := getNG(name)
				g.Expect(ng.Status.Desired).To(Equal(int32(0)))
				g.Expect(ng.Status.Min).To(Equal(int32(0)))
				g.Expect(ng.Status.Max).To(Equal(int32(0)))
				g.Expect(ng.Status.Instances).To(Equal(int32(0)))
				g.Expect(conditionStatus(ng, common.ConditionTypeScaling)).To(BeEmpty())
			}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
		})
	})

	Context("CloudEphemeral NodeGroup", func() {
		It("computes desired/min/max/instances from the MachineDeployment and machines", func() {
			name := uniqueNG("cloud-counts")
			createNodeGroup(cloudEphemeralNodeGroup(name, []string{"a", "b"}, 1, 4))
			createMCMMachineDeployment(name+"-md", name, 3, "")
			createMCMMachine(name+"-m1", name)
			createMCMMachine(name+"-m2", name)

			Eventually(func(g Gomega) {
				ng := getNG(name)
				g.Expect(ng.Status.Min).To(Equal(int32(2)))     // minPerZone 1 * 2 zones
				g.Expect(ng.Status.Max).To(Equal(int32(8)))     // maxPerZone 4 * 2 zones
				g.Expect(ng.Status.Desired).To(Equal(int32(3))) // MachineDeployment replicas
				g.Expect(ng.Status.Instances).To(Equal(int32(2)))
				// Desired (3) > existing nodes (0): controller reports Scaling.
				g.Expect(conditionStatus(ng, common.ConditionTypeScaling)).To(Equal(metav1.ConditionTrue))
			}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		})

		It("surfaces a failed machine as a LastMachineFailure, Error condition and non-ready summary", func() {
			name := uniqueNG("cloud-failed")
			createNodeGroup(cloudEphemeralNodeGroup(name, []string{"a"}, 1, 2))
			createMCMMachineDeployment(name+"-md", name, 1, "provider quota exceeded")

			Eventually(func(g Gomega) {
				ng := getNG(name)
				g.Expect(ng.Status.LastMachineFailures).To(HaveLen(1))
				g.Expect(ng.Status.LastMachineFailures[0].LastOperation).NotTo(BeNil())
				g.Expect(ng.Status.LastMachineFailures[0].LastOperation.Description).To(Equal("provider quota exceeded"))
				g.Expect(conditionStatus(ng, common.ConditionTypeError)).To(Equal(metav1.ConditionTrue))
				g.Expect(ng.Status.ConditionSummary).NotTo(BeNil())
				g.Expect(ng.Status.ConditionSummary.Ready).To(Equal("False"))
			}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		})
	})

	Context("processed status", func() {
		It("writes status.deckhouse.processed and a synced flag after reconciliation", func() {
			name := uniqueNG("processed")
			createNodeGroup(staticNodeGroup(name))

			// The controller computes a checksum of the filtered NodeGroup into
			// status.deckhouse.processed.checkSum and sets synced=False until an external
			// component records a matching status.deckhouse.observed.checkSum (not present in
			// envtest). So the controller-observable result is: a non-empty processed checksum
			// and synced=False.
			Eventually(func(g Gomega) {
				synced, processedCheckSum := processedDeckhouse(name)
				g.Expect(processedCheckSum).NotTo(BeEmpty())
				g.Expect(synced).To(Equal("False"))
			}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
		})
	})
})

var _ = AfterEach(func() {
	cleanupNodeGroupEnv()
})

// cleanupNodeGroupEnv removes every spec-created object. NodeGroups, Nodes and Machines are the
// inputs; the checksum Secret is shared (named configuration-checksums) so it is deleted too.
// Names are unique per spec, so this fully isolates specs despite envtest never truly deleting.
func cleanupNodeGroupEnv() {
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
		}

		mcmList := &mcmv1alpha1.MachineList{}
		g.Expect(k8sClient.List(suiteCtx, mcmList)).To(Succeed())
		for i := range mcmList.Items {
			_ = client.IgnoreNotFound(k8sClient.Delete(suiteCtx, &mcmList.Items[i]))
		}

		mdList := &unstructured.UnstructuredList{}
		mdList.SetGroupVersionKind(common.MCMMachineDeploymentGVK.GroupVersion().WithKind("MachineDeploymentList"))
		g.Expect(k8sClient.List(suiteCtx, mdList)).To(Succeed())
		for i := range mdList.Items {
			_ = client.IgnoreNotFound(k8sClient.Delete(suiteCtx, &mdList.Items[i]))
		}

		secret := &corev1.Secret{}
		err := k8sClient.Get(suiteCtx, types.NamespacedName{
			Namespace: common.MachineNamespace, Name: common.ConfigurationChecksumsSecretName}, secret)
		if err == nil {
			_ = client.IgnoreNotFound(k8sClient.Delete(suiteCtx, secret))
		}

		g.Expect(ngList.Items).To(BeEmpty())
		g.Expect(nodeList.Items).To(BeEmpty())
		g.Expect(mcmList.Items).To(BeEmpty())
		g.Expect(mdList.Items).To(BeEmpty())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}

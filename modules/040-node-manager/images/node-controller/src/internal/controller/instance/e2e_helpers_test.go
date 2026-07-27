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

package instance_controller

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	machinepkg "github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
	"github.com/deckhouse/node-controller/internal/testenv"
)

const (
	machineNamespace = machinepkg.MachineNamespace

	// Field managers used by the real bashible agent on the node (observed in production).
	bashibleReadyManager      = "bashible-bashibleready"
	bashibleApprovalManager   = "bashible-waitingapproval"
	bashibleDisruptionManager = "bashible-waitingdisruptionapproval"

	// Field managers used by the controller (for ownership assertions).
	controllerMachineStatusManager     = "node-controller-instancestatus"
	controllerBashibleStatusManager    = "node-controller-instance-bashible-status"
	controllerBashibleHeartbeatManager = "node-controller-instance-bashible-heartbeat"

	// Aliases of the shared testenv timeouts, kept short for the specs in this package.
	eventuallyTimeout     = testenv.EventuallyTimeout
	eventuallyPoll        = testenv.EventuallyPoll
	negativeCheckDuration = testenv.NegativeCheckDuration
)

func uniqueName(base string) string { return testenv.UniqueName(base) }

// cleanupAll runs after every spec. The controller keeps re-creating Instances from live
// sources, so sources (machines, nodes) are deleted first, then Instances are force-deleted
// until none remain. Names are unique per spec, so this fully isolates specs despite the
// cluster-scoped Instance objects.
// Under ENVTEST_DEBUG, dump the end-of-spec state before cleanup: the sources the test created
// and the Instance(s) the controller produced from them. (The pre-spec state is always empty —
// cleanupAll isolates specs — so there is nothing useful to dump before a spec runs.)
var _ = AfterEach(func() {
	if testenv.DebugEnabled() {
		testenv.KubectlDumpNodeObjects(GinkgoWriter, testEnv, cfg, CurrentSpecReport().LeafNodeText)
	}
	cleanupAll()
})

// createCAPIMachine creates a CAPI machine with the required spec fields and, when given,
// applies the status via the status subresource.
func createCAPIMachine(name, nodeName string, phase capiv1beta2.MachinePhase, conditions []metav1.Condition) *capiv1beta2.Machine {
	m := &capiv1beta2.Machine{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: machineNamespace},
		Spec: capiv1beta2.MachineSpec{
			ClusterName: "e2e",
			Bootstrap:   capiv1beta2.Bootstrap{DataSecretName: ptr.To("e2e-bootstrap")},
			InfrastructureRef: capiv1beta2.ContractVersionedObjectReference{
				APIGroup: "infrastructure.cluster.x-k8s.io",
				Kind:     "DVPMachine",
				Name:     name,
			},
		},
	}
	Expect(k8sClient.Create(suiteCtx, m)).To(Succeed())

	if phase != "" || nodeName != "" || len(conditions) > 0 {
		m.Status = capiv1beta2.MachineStatus{Phase: string(phase), Conditions: conditions}
		if nodeName != "" {
			m.Status.NodeRef = capiv1beta2.MachineNodeReference{Name: nodeName}
		}
		Expect(k8sClient.Status().Update(suiteCtx, m)).To(Succeed())
	}
	return m
}

func createMCMMachine(name, nodeName string, status mcmv1alpha1.MachineStatus) *mcmv1alpha1.Machine {
	m := &mcmv1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: machineNamespace},
	}
	Expect(k8sClient.Create(suiteCtx, m)).To(Succeed())

	if nodeName != "" {
		status.Node = nodeName
	}
	m.Status = status
	Expect(k8sClient.Status().Update(suiteCtx, m)).To(Succeed())
	return m
}

func createStaticNode(name string) *corev1.Node {
	return createNode(name, map[string]string{nodecommon.NodeTypeLabel: "Static"}, nil, "")
}

func createNodeWithCAPIAnnotation(name, machineName string) *corev1.Node {
	return createNode(name,
		map[string]string{nodecommon.NodeTypeLabel: "Static"},
		map[string]string{nodecommon.CAPIMachineAnnotation: machineName}, "")
}

func createNodeWithCAPSProviderID(name, providerID string) *corev1.Node {
	return createNode(name, map[string]string{nodecommon.NodeTypeLabel: "Static"}, nil, providerID)
}

func createNode(name string, labels, annotations map[string]string, providerID string) *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels, Annotations: annotations},
		Spec:       corev1.NodeSpec{ProviderID: providerID},
	}
	Expect(k8sClient.Create(suiteCtx, node)).To(Succeed())
	return node
}

// applyBashibleCondition emulates the on-node bashible agent: it Server-Side-Applies a
// single status condition under the given field manager, exactly as bashible does in
// production (separate managers per condition, lastHeartbeatTime set).
func applyBashibleCondition(name, manager string, cond deckhousev1alpha2.InstanceCondition) {
	applyObj := &deckhousev1alpha2.Instance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: deckhousev1alpha2.GroupVersion.String(),
			Kind:       "Instance",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status:     deckhousev1alpha2.InstanceStatus{Conditions: []deckhousev1alpha2.InstanceCondition{cond}},
	}
	Expect(k8sClient.Status().Patch(suiteCtx, applyObj, client.Apply,
		client.FieldOwner(manager), client.ForceOwnership)).To(Succeed())
}

func bashibleReady(status metav1.ConditionStatus, reason, message string, heartbeat time.Time) deckhousev1alpha2.InstanceCondition {
	t := metav1.NewTime(heartbeat)
	return deckhousev1alpha2.InstanceCondition{
		Type:              deckhousev1alpha2.InstanceConditionTypeBashibleReady,
		Status:            status,
		Reason:            reason,
		Message:           message,
		LastHeartbeatTime: &t,
	}
}

func approvalCondition(conditionType string, status metav1.ConditionStatus, reason, message string) deckhousev1alpha2.InstanceCondition {
	now := metav1.Now()
	return deckhousev1alpha2.InstanceCondition{
		Type:              conditionType,
		Status:            status,
		Reason:            reason,
		Message:           message,
		LastHeartbeatTime: &now,
	}
}

func getInstance(name string) *deckhousev1alpha2.Instance {
	instance := &deckhousev1alpha2.Instance{}
	Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: name}, instance)).To(Succeed())
	return instance
}

func conditionOf(instance *deckhousev1alpha2.Instance, conditionType string) *deckhousev1alpha2.InstanceCondition {
	for i := range instance.Status.Conditions {
		if instance.Status.Conditions[i].Type == conditionType {
			return &instance.Status.Conditions[i]
		}
	}
	return nil
}

// statusFieldManagers returns the field managers that touched the Instance status subresource.
func statusFieldManagers(instance *deckhousev1alpha2.Instance) []string {
	var managers []string
	for _, mf := range instance.ManagedFields {
		if mf.Subresource == "status" {
			managers = append(managers, mf.Manager)
		}
	}
	return managers
}

func instanceExists(name string) bool {
	instance := &deckhousev1alpha2.Instance{}
	err := k8sClient.Get(suiteCtx, types.NamespacedName{Name: name}, instance)
	if err == nil {
		return true
	}
	Expect(apierrors.IsNotFound(err)).To(BeTrue())
	return false
}

func capiMachineDeletionTimestamp(name string) *metav1.Time {
	m := &capiv1beta2.Machine{}
	if err := k8sClient.Get(suiteCtx, types.NamespacedName{Namespace: machineNamespace, Name: name}, m); err != nil {
		return nil
	}
	return m.DeletionTimestamp
}

// cleanupAll runs after every spec. The controller re-creates Instances from live sources,
// so all sources (machines, nodes) are deleted and confirmed gone BEFORE deleting Instances;
// otherwise a delete would race the controller's create-from-source and ping-pong.
// ENVTEST_DEBUG=1 dumps cluster state after every spec via testenv.KubectlDumpNodeObjects (real
// `kubectl get … -o wide` against the envtest apiserver); see the AfterEach above.

func cleanupAll() {
	// delete all sources and wait until they are gone, then force-delete the Instances
	Eventually(func(g Gomega) {
		capiList := &capiv1beta2.MachineList{}
		g.Expect(k8sClient.List(suiteCtx, capiList)).To(Succeed())
		for i := range capiList.Items {
			_ = client.IgnoreNotFound(k8sClient.Delete(suiteCtx, &capiList.Items[i]))
			testenv.RemoveFinalizers(suiteCtx, k8sClient, &capiList.Items[i])
		}

		mcmList := &mcmv1alpha1.MachineList{}
		g.Expect(k8sClient.List(suiteCtx, mcmList)).To(Succeed())
		for i := range mcmList.Items {
			_ = client.IgnoreNotFound(k8sClient.Delete(suiteCtx, &mcmList.Items[i]))
			testenv.RemoveFinalizers(suiteCtx, k8sClient, &mcmList.Items[i])
		}

		nodeList := &corev1.NodeList{}
		g.Expect(k8sClient.List(suiteCtx, nodeList)).To(Succeed())
		for i := range nodeList.Items {
			_ = client.IgnoreNotFound(k8sClient.Delete(suiteCtx, &nodeList.Items[i]))
			testenv.RemoveFinalizers(suiteCtx, k8sClient, &nodeList.Items[i])
		}

		g.Expect(capiList.Items).To(BeEmpty())
		g.Expect(mcmList.Items).To(BeEmpty())
		g.Expect(nodeList.Items).To(BeEmpty())
	}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

	Eventually(func(g Gomega) {
		list := &deckhousev1alpha2.InstanceList{}
		g.Expect(k8sClient.List(suiteCtx, list)).To(Succeed())
		for i := range list.Items {
			_ = client.IgnoreNotFound(k8sClient.Delete(suiteCtx, &list.Items[i]))
			testenv.RemoveFinalizers(suiteCtx, k8sClient, &list.Items[i])
		}
		g.Expect(list.Items).To(BeEmpty())
	}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
}

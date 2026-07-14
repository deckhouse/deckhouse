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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	machinepkg "github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
)

var _ = Describe("Machine status sync and deletion lifecycle", func() {
	It("re-syncs Instance status when the backing machine status changes (watch-triggered)", func() {
		name := uniqueName("machine-resync")
		machine := createCAPIMachine(name, name+"-node", capiv1beta2.MachinePhaseRunning, nil)

		By("converging to Running/Ready from the running machine")
		Eventually(func(g Gomega) {
			instance := getInstance(name)
			g.Expect(instance.Status.Phase).To(Equal(deckhousev1alpha2.InstancePhaseRunning))
			g.Expect(instance.Status.MachineStatus).To(Equal(string("Ready")))
			machineReady := conditionOf(instance, deckhousev1alpha2.InstanceConditionTypeMachineReady)
			g.Expect(machineReady).NotTo(BeNil())
			g.Expect(machineReady.Status).To(Equal(metav1.ConditionTrue))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		By("moving the machine to a drain-blocked Deleting state")
		Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Namespace: machineNamespace, Name: name}, machine)).To(Succeed())
		machine.Status.Phase = string(capiv1beta2.MachinePhaseDeleting)
		machine.Status.Conditions = []metav1.Condition{{
			Type:               capiv1beta2.DeletingCondition,
			Status:             metav1.ConditionTrue,
			Reason:             capiv1beta2.MachineDeletingDrainingNodeReason,
			Message:            "cannot evict pod because disruption budget",
			LastTransitionTime: metav1.Now(),
		}}
		Expect(k8sClient.Status().Update(suiteCtx, machine)).To(Succeed())

		By("re-syncing the Instance to Terminating/Blocked")
		Eventually(func(g Gomega) {
			instance := getInstance(name)
			g.Expect(instance.Status.Phase).To(Equal(deckhousev1alpha2.InstancePhaseTerminating))
			g.Expect(instance.Status.MachineStatus).To(Equal(string("Blocked")))
			machineReady := conditionOf(instance, deckhousev1alpha2.InstanceConditionTypeMachineReady)
			g.Expect(machineReady).NotTo(BeNil())
			g.Expect(machineReady.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(machineReady.Reason).To(Equal(capiv1beta2.MachineDeletingDrainingNodeReason))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("re-syncs Instance status from an MCM machine (Running/Ready then Terminating/Blocked)", func() {
		name := uniqueName("mcm-resync")
		createMCMMachine(name, name+"-node", mcmv1alpha1.MachineStatus{
			CurrentStatus: mcmv1alpha1.CurrentStatus{Phase: mcmv1alpha1.MachineRunning},
			LastOperation: mcmv1alpha1.LastOperation{State: mcmv1alpha1.MachineStateSuccessful, Description: "machine is ready"},
		})

		By("converging to Running/Ready from the running MCM machine")
		Eventually(func(g Gomega) {
			instance := getInstance(name)
			g.Expect(instance.Status.Phase).To(Equal(deckhousev1alpha2.InstancePhaseRunning))
			g.Expect(instance.Status.MachineStatus).To(Equal(string(machinepkg.StatusReady)))
			machineReady := conditionOf(instance, deckhousev1alpha2.InstanceConditionTypeMachineReady)
			g.Expect(machineReady).NotTo(BeNil())
			g.Expect(machineReady.Status).To(Equal(metav1.ConditionTrue))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		By("moving the MCM machine to a drain-blocked Terminating state")
		mcm := &mcmv1alpha1.Machine{}
		Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Namespace: machineNamespace, Name: name}, mcm)).To(Succeed())
		mcm.Status.CurrentStatus.Phase = mcmv1alpha1.MachineTerminating
		mcm.Status.LastOperation = mcmv1alpha1.LastOperation{
			State:          mcmv1alpha1.MachineStateFailed,
			Description:    "drain failed due to disruption budget",
			LastUpdateTime: metav1.Now(),
		}
		Expect(k8sClient.Status().Update(suiteCtx, mcm)).To(Succeed())

		By("re-syncing the Instance to Terminating/Blocked")
		Eventually(func(g Gomega) {
			instance := getInstance(name)
			g.Expect(instance.Status.Phase).To(Equal(deckhousev1alpha2.InstancePhaseTerminating))
			g.Expect(instance.Status.MachineStatus).To(Equal(string(machinepkg.StatusBlocked)))
			machineReady := conditionOf(instance, deckhousev1alpha2.InstanceConditionTypeMachineReady)
			g.Expect(machineReady).NotTo(BeNil())
			g.Expect(machineReady.Status).To(Equal(metav1.ConditionFalse))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("keeps the finalizer until the machine is gone, then deletes the Instance", func() {
		name := uniqueName("delete-lifecycle")
		machine := createCAPIMachine(name, name+"-node", capiv1beta2.MachinePhaseRunning, nil)

		By("giving the machine a finalizer so it lingers after the controller deletes it")
		Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Namespace: machineNamespace, Name: name}, machine)).To(Succeed())
		machine.Finalizers = append(machine.Finalizers, "e2e.test/keep")
		Expect(k8sClient.Update(suiteCtx, machine)).To(Succeed())

		By("waiting until the controller adopted the Instance")
		Eventually(func(g Gomega) {
			g.Expect(getInstance(name).Finalizers).NotTo(BeEmpty())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		By("deleting the Instance")
		instance := getInstance(name)
		Expect(k8sClient.Delete(suiteCtx, instance)).To(Succeed())

		By("the controller asks the machine to delete and keeps the Instance finalizer")
		Eventually(func(g Gomega) {
			g.Expect(instanceExists(name)).To(BeTrue())
			g.Expect(capiMachineDeletionTimestamp(name)).NotTo(BeNil())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		By("removing the machine finalizer so it is garbage-collected")
		Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Namespace: machineNamespace, Name: name}, machine)).To(Succeed())
		machine.Finalizers = nil
		Expect(k8sClient.Update(suiteCtx, machine)).To(Succeed())

		By("the controller removes its finalizer and the Instance disappears")
		Eventually(func() bool {
			return instanceExists(name)
		}, eventuallyTimeout, eventuallyPoll).Should(BeFalse())
	})
})

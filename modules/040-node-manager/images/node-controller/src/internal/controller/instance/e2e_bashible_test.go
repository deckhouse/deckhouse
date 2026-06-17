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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
)

// newNodeBackedInstance creates a static node and waits until the controller has created
// and adopted the node-backed Instance (finalizer present). bashible conditions can then be
// applied to a stable Instance that the controller will not garbage-collect.
func newNodeBackedInstance(name string) {
	createStaticNode(name)
	Eventually(func(g Gomega) {
		g.Expect(instanceExists(name)).To(BeTrue())
		g.Expect(getInstance(name).Finalizers).NotTo(BeEmpty())
	}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
}

var _ = Describe("Bashible status aggregation (real SSA field ownership)", func() {
	It("maps BashibleReady=True to bashibleStatus Ready with a bashible message", func() {
		name := uniqueName("bashible-ready")
		newNodeBackedInstance(name)
		applyBashibleCondition(name, bashibleReadyManager,
			bashibleReady(metav1.ConditionTrue, "StepsCompleted", "converge cycle finished", time.Now()))

		Eventually(func(g Gomega) {
			instance := getInstance(name)
			g.Expect(instance.Status.BashibleStatus).To(Equal(deckhousev1alpha2.BashibleStatusReady))
			g.Expect(instance.Status.Message).To(Equal("bashible: converge cycle finished"))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("maps BashibleReady=False to bashibleStatus Error", func() {
		name := uniqueName("bashible-error")
		newNodeBackedInstance(name)
		applyBashibleCondition(name, bashibleReadyManager,
			bashibleReady(metav1.ConditionFalse, "StepFailed", "apt-get failed", time.Now()))

		Eventually(func(g Gomega) {
			instance := getInstance(name)
			g.Expect(instance.Status.BashibleStatus).To(Equal(deckhousev1alpha2.BashibleStatusError))
			g.Expect(instance.Status.Message).To(Equal("bashible: apt-get failed"))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("maps WaitingApproval=True with reason UpdateApprovalTimeout to bashibleStatus WaitingApproval", func() {
		name := uniqueName("bashible-approval")
		newNodeBackedInstance(name)
		applyBashibleCondition(name, bashibleReadyManager,
			bashibleReady(metav1.ConditionTrue, "StepsCompleted", "converge cycle finished", time.Now()))
		applyBashibleCondition(name, bashibleApprovalManager,
			approvalCondition(deckhousev1alpha2.InstanceConditionTypeWaitingApproval, metav1.ConditionTrue,
				deckhousev1alpha2.InstanceConditionReasonUpdateApprovalTimeout, "waiting for approval"))

		Eventually(func(g Gomega) {
			instance := getInstance(name)
			g.Expect(instance.Status.BashibleStatus).To(Equal(deckhousev1alpha2.BashibleStatusWaitingApproval))
			g.Expect(instance.Status.Message).To(Equal("bashible: waiting for approval"))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("maps WaitingDisruptionApproval=True to bashibleStatus WaitingApproval", func() {
		name := uniqueName("bashible-disruption")
		newNodeBackedInstance(name)
		applyBashibleCondition(name, bashibleReadyManager,
			bashibleReady(metav1.ConditionTrue, "StepsCompleted", "converge cycle finished", time.Now()))
		applyBashibleCondition(name, bashibleDisruptionManager,
			approvalCondition(deckhousev1alpha2.InstanceConditionTypeWaitingDisruptionApproval, metav1.ConditionTrue,
				"DisruptionApprovalRequired", "waiting for disruption approval"))

		Eventually(func(g Gomega) {
			instance := getInstance(name)
			g.Expect(instance.Status.BashibleStatus).To(Equal(deckhousev1alpha2.BashibleStatusWaitingApproval))
			g.Expect(instance.Status.Message).To(Equal("bashible: waiting for disruption approval"))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("prefers a machine problem over a bashible message in Status.Message", func() {
		name := uniqueName("bashible-machine-priority")
		createCAPIMachine(name, name+"-node", capiv1beta2.MachinePhasePending, []metav1.Condition{{
			Type:               capiv1beta2.InfrastructureReadyCondition,
			Status:             metav1.ConditionFalse,
			Reason:             "WaitingForInfrastructure",
			LastTransitionTime: metav1.Now(),
		}})
		Eventually(func(g Gomega) {
			g.Expect(instanceExists(name)).To(BeTrue())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
		applyBashibleCondition(name, bashibleReadyManager,
			bashibleReady(metav1.ConditionFalse, "StepFailed", "bashible failed", time.Now()))

		Eventually(func(g Gomega) {
			instance := getInstance(name)
			g.Expect(instance.Status.BashibleStatus).To(Equal(deckhousev1alpha2.BashibleStatusError))
			g.Expect(instance.Status.Message).To(Equal("machine: Waiting for infrastructure"))
			machineReady := conditionOf(instance, deckhousev1alpha2.InstanceConditionTypeMachineReady)
			g.Expect(machineReady).NotTo(BeNil())
			g.Expect(machineReady.Status).To(Equal(metav1.ConditionFalse))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("clears Status.BootstrapStatus once bashible reports a BashibleReady condition", func() {
		name := uniqueName("bootstrap-clear")
		newNodeBackedInstance(name)

		// An external bootstrap component records a BootstrapStatus.
		bootstrapObj := &deckhousev1alpha2.Instance{
			TypeMeta:   metav1.TypeMeta{APIVersion: deckhousev1alpha2.GroupVersion.String(), Kind: "Instance"},
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Status:     deckhousev1alpha2.InstanceStatus{BootstrapStatus: &deckhousev1alpha2.BootstrapStatus{Description: "bootstrapping"}},
		}
		Expect(k8sClient.Status().Patch(suiteCtx, bootstrapObj, client.Apply,
			client.FieldOwner("bootstrap"), client.ForceOwnership)).To(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(getInstance(name).Status.BootstrapStatus).NotTo(BeNil())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		// Once bashible reports readiness, the controller clears the bootstrap status.
		applyBashibleCondition(name, bashibleReadyManager,
			bashibleReady(metav1.ConditionTrue, "StepsCompleted", "converge cycle finished", time.Now()))

		Eventually(func(g Gomega) {
			g.Expect(getInstance(name).Status.BootstrapStatus).To(BeNil())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("does not clobber the controller-owned MachineReady condition when bashible applies BashibleReady", func() {
		name := uniqueName("bashible-ownership")
		// A machine-backed Instance, so node-controller-instancestatus owns a MachineReady entry.
		createCAPIMachine(name, name+"-node", capiv1beta2.MachinePhaseRunning, nil)
		Eventually(func(g Gomega) {
			g.Expect(instanceExists(name)).To(BeTrue())
			g.Expect(conditionOf(getInstance(name), deckhousev1alpha2.InstanceConditionTypeMachineReady)).NotTo(BeNil())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		applyBashibleCondition(name, bashibleReadyManager,
			bashibleReady(metav1.ConditionTrue, "StepsCompleted", "converge cycle finished", time.Now()))

		Eventually(func(g Gomega) {
			instance := getInstance(name)
			g.Expect(instance.Status.BashibleStatus).To(Equal(deckhousev1alpha2.BashibleStatusReady))

			// SSA list-map merge by condition type keeps every owner's entry intact.
			machineReady := conditionOf(instance, deckhousev1alpha2.InstanceConditionTypeMachineReady)
			g.Expect(machineReady).NotTo(BeNil())
			g.Expect(machineReady.Status).To(Equal(metav1.ConditionTrue))
			bashibleReadyCond := conditionOf(instance, deckhousev1alpha2.InstanceConditionTypeBashibleReady)
			g.Expect(bashibleReadyCond).NotTo(BeNil())
			g.Expect(bashibleReadyCond.Status).To(Equal(metav1.ConditionTrue))

			managers := statusFieldManagers(instance)
			g.Expect(managers).To(ContainElement(controllerMachineStatusManager))
			g.Expect(managers).To(ContainElement(controllerBashibleStatusManager))
			g.Expect(managers).To(ContainElement(bashibleReadyManager))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})
})

// These heartbeat specs rely on the bashible status patch re-triggering Reconcile (the
// Instance For() has no generation predicate, so status-only updates fire events). They do
// not depend on the 1m periodic requeue, so the 20s timeout is sufficient.
var _ = Describe("Bashible heartbeat", func() {
	It("forces BashibleReady to Unknown when the heartbeat is stale (>5m)", func() {
		name := uniqueName("heartbeat-stale")
		newNodeBackedInstance(name)
		applyBashibleCondition(name, bashibleReadyManager,
			bashibleReady(metav1.ConditionTrue, "StepsCompleted", "converge cycle finished", time.Now().Add(-6*time.Minute)))

		Eventually(func(g Gomega) {
			instance := getInstance(name)
			br := conditionOf(instance, deckhousev1alpha2.InstanceConditionTypeBashibleReady)
			g.Expect(br).NotTo(BeNil())
			g.Expect(br.Status).To(Equal(metav1.ConditionUnknown))
			g.Expect(br.Reason).To(Equal("HeartBeat"))
			g.Expect(br.Message).To(Equal("No Bashible reconciliation for 5m"))
			g.Expect(instance.Status.BashibleStatus).To(Equal(deckhousev1alpha2.BashibleStatusUnknown))
			g.Expect(statusFieldManagers(instance)).To(ContainElement(controllerBashibleHeartbeatManager))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("forces BashibleReady to Unknown with the disruption reason when WaitingDisruptionApproval is stale (>20m)", func() {
		name := uniqueName("heartbeat-disruption")
		newNodeBackedInstance(name)
		applyBashibleCondition(name, bashibleDisruptionManager,
			approvalCondition(deckhousev1alpha2.InstanceConditionTypeWaitingDisruptionApproval, metav1.ConditionTrue,
				"DisruptionApprovalRequired", "waiting for disruption approval"))
		applyBashibleCondition(name, bashibleReadyManager,
			bashibleReady(metav1.ConditionTrue, "StepsCompleted", "converge cycle finished", time.Now().Add(-21*time.Minute)))

		Eventually(func(g Gomega) {
			br := conditionOf(getInstance(name), deckhousev1alpha2.InstanceConditionTypeBashibleReady)
			g.Expect(br).NotTo(BeNil())
			g.Expect(br.Status).To(Equal(metav1.ConditionUnknown))
			g.Expect(br.Reason).To(Equal(deckhousev1alpha2.InstanceConditionTypeWaitingDisruptionApproval))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("does not force a stale BashibleReady that is below the 20m approval timeout", func() {
		name := uniqueName("heartbeat-approval-young")
		newNodeBackedInstance(name)
		applyBashibleCondition(name, bashibleApprovalManager,
			approvalCondition(deckhousev1alpha2.InstanceConditionTypeWaitingApproval, metav1.ConditionTrue,
				deckhousev1alpha2.InstanceConditionReasonUpdateApprovalTimeout, "waiting for approval"))
		// Stale by 6m: past the 5m normal timeout but below the 20m approval timeout.
		applyBashibleCondition(name, bashibleReadyManager,
			bashibleReady(metav1.ConditionTrue, "StepsCompleted", "converge cycle finished", time.Now().Add(-6*time.Minute)))

		Consistently(func(g Gomega) {
			br := conditionOf(getInstance(name), deckhousev1alpha2.InstanceConditionTypeBashibleReady)
			g.Expect(br).NotTo(BeNil())
			g.Expect(br.Status).To(Equal(metav1.ConditionTrue))
		}, negativeCheckDuration, eventuallyPoll).Should(Succeed())
	})

	It("preserves an explicit BashibleReady=False even when the heartbeat is stale", func() {
		name := uniqueName("heartbeat-false")
		newNodeBackedInstance(name)
		applyBashibleCondition(name, bashibleReadyManager,
			bashibleReady(metav1.ConditionFalse, "StepFailed", "apt-get failed", time.Now().Add(-6*time.Minute)))

		// The controller must NOT take ownership of the condition nor flip it to Unknown.
		Consistently(func(g Gomega) {
			instance := getInstance(name)
			br := conditionOf(instance, deckhousev1alpha2.InstanceConditionTypeBashibleReady)
			g.Expect(br).NotTo(BeNil())
			g.Expect(br.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(br.Reason).To(Equal("StepFailed"))
			g.Expect(br.Message).To(Equal("apt-get failed"))
			g.Expect(statusFieldManagers(instance)).NotTo(ContainElement(controllerBashibleHeartbeatManager))
		}, negativeCheckDuration, eventuallyPoll).Should(Succeed())
	})
})

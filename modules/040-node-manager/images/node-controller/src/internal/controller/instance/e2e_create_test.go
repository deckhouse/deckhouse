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

	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	instancecommon "github.com/deckhouse/node-controller/internal/controller/instance/common"
)

var _ = Describe("Instance creation from a source (watch-triggered)", func() {
	It("creates an Instance from a CAPI machine with machineRef, nodeRef and finalizer", func() {
		name := uniqueName("capi-create")
		createCAPIMachine(name, name+"-node", capiv1beta2.MachinePhaseRunning, nil)

		Eventually(func(g Gomega) {
			g.Expect(instanceExists(name)).To(BeTrue())
			instance := getInstance(name)
			g.Expect(instance.Spec.MachineRef).NotTo(BeNil())
			g.Expect(instance.Spec.MachineRef.APIVersion).To(Equal(capiv1beta2.GroupVersion.String()))
			g.Expect(instance.Spec.MachineRef.Name).To(Equal(name))
			g.Expect(instance.Spec.NodeRef.Name).To(Equal(name + "-node"))
			g.Expect(instance.Finalizers).To(ContainElement(instancecommon.InstanceControllerFinalizer))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("creates an Instance from an MCM machine when no CAPI machine exists", func() {
		name := uniqueName("mcm-create")
		createMCMMachine(name, name+"-node", mcmv1alpha1.MachineStatus{
			CurrentStatus: mcmv1alpha1.CurrentStatus{Phase: mcmv1alpha1.MachineRunning},
			LastOperation: mcmv1alpha1.LastOperation{State: mcmv1alpha1.MachineStateSuccessful, Description: "ready"},
		})

		Eventually(func(g Gomega) {
			g.Expect(instanceExists(name)).To(BeTrue())
			instance := getInstance(name)
			g.Expect(instance.Spec.MachineRef).NotTo(BeNil())
			g.Expect(instance.Spec.MachineRef.APIVersion).To(Equal(mcmv1alpha1.SchemeGroupVersion.String()))
			g.Expect(instance.Spec.NodeRef.Name).To(Equal(name + "-node"))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("creates a node-backed Instance in phase Running from a static node", func() {
		name := uniqueName("static-create")
		createStaticNode(name)

		Eventually(func(g Gomega) {
			g.Expect(instanceExists(name)).To(BeTrue())
			instance := getInstance(name)
			g.Expect(instance.Spec.MachineRef).To(BeNil())
			g.Expect(instance.Spec.NodeRef.Name).To(Equal(name))
			g.Expect(instance.Status.Phase).To(Equal(deckhousev1alpha2.InstancePhaseRunning))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("does not create an Instance for a static node carrying a CAPI machine annotation", func() {
		name := uniqueName("capi-annotated")
		createNodeWithCAPIAnnotation(name, "some-machine")

		// Positive control: a plain static node created at the same time DOES get an Instance.
		// Once that appears we know the controller has processed node events, so the absence
		// of an Instance for the annotated node is meaningful rather than a not-yet-reconciled.
		control := uniqueName("capi-annotated-control")
		createStaticNode(control)
		Eventually(func() bool { return instanceExists(control) }, eventuallyTimeout, eventuallyPoll).Should(BeTrue())

		Consistently(func() bool {
			return instanceExists(name)
		}, negativeCheckDuration, eventuallyPoll).Should(BeFalse())
	})

	It("does not create an Instance for a node with a CAPS (static:///) provider id", func() {
		name := uniqueName("caps-providerid")
		createNodeWithCAPSProviderID(name, "static:///"+name)

		control := uniqueName("caps-providerid-control")
		createStaticNode(control)
		Eventually(func() bool { return instanceExists(control) }, eventuallyTimeout, eventuallyPoll).Should(BeTrue())

		Consistently(func() bool {
			return instanceExists(name)
		}, negativeCheckDuration, eventuallyPoll).Should(BeFalse())
	})
})

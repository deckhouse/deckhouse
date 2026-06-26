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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
)

var _ = Describe("Instance self-heal and source-existence GC", func() {
	It("self-heals a missing machineRef when a matching CAPI machine appears", func() {
		name := uniqueName("heal-machineref")

		// A node-backed Instance is created from a static node (no machineRef yet).
		createStaticNode(name)
		Eventually(func(g Gomega) {
			g.Expect(instanceExists(name)).To(BeTrue())
			g.Expect(getInstance(name).Spec.MachineRef).To(BeNil())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		// A CAPI machine with the same name appears; the controller binds the machineRef.
		createCAPIMachine(name, name+"-node", capiv1beta2.MachinePhaseRunning, nil)
		Eventually(func(g Gomega) {
			instance := getInstance(name)
			g.Expect(instance.Spec.MachineRef).NotTo(BeNil())
			g.Expect(instance.Spec.MachineRef.APIVersion).To(Equal(capiv1beta2.GroupVersion.String()))
			g.Expect(instance.Spec.MachineRef.Name).To(Equal(name))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("self-heals a missing nodeRef when the backing machine reports its node", func() {
		name := uniqueName("heal-noderef")

		// A machine without a node yields a machine-backed Instance with an empty nodeRef.
		createCAPIMachine(name, "", capiv1beta2.MachinePhaseRunning, nil)
		Eventually(func(g Gomega) {
			g.Expect(instanceExists(name)).To(BeTrue())
			instance := getInstance(name)
			g.Expect(instance.Spec.MachineRef).NotTo(BeNil())
			g.Expect(instance.Spec.NodeRef.Name).To(BeEmpty())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		// The machine starts reporting a node; the controller binds the nodeRef.
		machine := &capiv1beta2.Machine{}
		Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Namespace: machineNamespace, Name: name}, machine)).To(Succeed())
		machine.Status.NodeRef = capiv1beta2.MachineNodeReference{Name: name + "-node"}
		Expect(k8sClient.Status().Update(suiteCtx, machine)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(getInstance(name).Spec.NodeRef.Name).To(Equal(name + "-node"))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("garbage-collects a node-backed Instance after its node is deleted", func() {
		name := uniqueName("gc-node")
		createStaticNode(name)
		Eventually(func(g Gomega) {
			g.Expect(instanceExists(name)).To(BeTrue())
			g.Expect(getInstance(name).Finalizers).NotTo(BeEmpty())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		node := &corev1.Node{}
		Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: name}, node)).To(Succeed())
		Expect(k8sClient.Delete(suiteCtx, node)).To(Succeed())

		Eventually(func() bool {
			return instanceExists(name)
		}, eventuallyTimeout, eventuallyPoll).Should(BeFalse())
	})

	It("garbage-collects a machine-backed Instance after its machine is deleted", func() {
		name := uniqueName("gc-machine")
		createCAPIMachine(name, name+"-node", capiv1beta2.MachinePhaseRunning, nil)
		Eventually(func(g Gomega) {
			g.Expect(instanceExists(name)).To(BeTrue())
			instance := getInstance(name)
			g.Expect(instance.Spec.MachineRef).NotTo(BeNil())
			g.Expect(instance.Finalizers).NotTo(BeEmpty())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		machine := &capiv1beta2.Machine{}
		Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Namespace: machineNamespace, Name: name}, machine)).To(Succeed())
		Expect(k8sClient.Delete(suiteCtx, machine)).To(Succeed())

		Eventually(func() bool {
			return instanceExists(name)
		}, eventuallyTimeout, eventuallyPoll).Should(BeFalse())
	})
})

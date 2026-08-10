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

package capi

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// User story: As an operator removing a NodeGroup or one of its zones, I want the MCM
// MachineClass to stay until the MachineDeployment it belongs to is really gone, so that
// machine-controller-manager can still read the cloud credentials it deletes the VMs with —
// and to be deleted afterwards, so no orphan class is left behind.
//
// The suite's cloud provider is a CAPI one, so these specs drive the MCM teardown helpers
// directly against the envtest apiserver instead of flipping the shared discovery Secret to MCM
// mid-suite, which would make every other spec's controller reconcile take the wrong path.
var _ = Describe("MCM MachineDeployment and MachineClass teardown", func() {
	// mcmFinalizer stands in for machine-controller-manager, which holds its MachineDeployment
	// until every Machine is deleted. envtest runs no MCM, so without it a deleted object would
	// vanish instantly and the terminating window these specs are about would not exist.
	const mcmFinalizer = "machine.sapcloud.io/machine-controller-manager"
	const machineClassKind = "YandexMachineClass"

	newReconciler := func() *MachineDeploymentReconciler {
		r := &MachineDeploymentReconciler{}
		r.Client = k8sClient
		r.APIReader = k8sClient
		return r
	}

	createMachineClass := func(name string, labels map[string]string) *unstructured.Unstructured {
		mc := newUnstructured("machine.sapcloud.io", "v1alpha1", machineClassKind)
		mc.SetName(name)
		mc.SetNamespace(common.MachineNamespace)
		mc.SetLabels(labels)
		Expect(k8sClient.Create(suiteCtx, mc)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, mc))).To(Succeed())
		})
		return mc
	}

	createMachineDeployment := func(name, ngName, className string) *unstructured.Unstructured {
		md := newUnstructured("machine.sapcloud.io", "v1alpha1", "MachineDeployment")
		md.SetName(name)
		md.SetNamespace(common.MachineNamespace)
		md.SetLabels(map[string]string{"node-group": ngName})
		md.SetFinalizers([]string{mcmFinalizer})
		Expect(unstructured.SetNestedMap(md.Object, map[string]interface{}{
			"kind": machineClassKind,
			"name": className,
		}, "spec", "template", "spec", "class")).To(Succeed())
		Expect(k8sClient.Create(suiteCtx, md)).To(Succeed())
		DeferCleanup(func() {
			testenv.RemoveFinalizers(suiteCtx, k8sClient, md)
			Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, md))).To(Succeed())
		})
		return md
	}

	finishTermination := func(md *unstructured.Unstructured) {
		testenv.RemoveFinalizers(suiteCtx, k8sClient, md)
		Eventually(func() bool {
			got := newUnstructured("machine.sapcloud.io", "v1alpha1", "MachineDeployment")
			err := k8sClient.Get(suiteCtx, client.ObjectKeyFromObject(md), got)
			return errors.IsNotFound(err)
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(BeTrue())
	}

	getObject := func(kind, name string) error {
		got := newUnstructured("machine.sapcloud.io", "v1alpha1", kind)
		return k8sClient.Get(suiteCtx, types.NamespacedName{
			Name: name, Namespace: common.MachineNamespace,
		}, got)
	}

	It("keeps the NodeGroup finalizer while a MachineDeployment is still terminating", func() {
		ngName := testenv.UniqueName("mcm-teardown")
		createMachineClass(ngName+"-zone-a", nil)
		md := createMachineDeployment(ngName+"-zone-a", ngName, ngName+"-zone-a")

		r := newReconciler()

		done, err := r.cleanupMachineDeployments(suiteCtx, ngName)
		Expect(err).NotTo(HaveOccurred())
		Expect(done).To(BeFalse(), "the NodeGroup must stay finalized while the MachineDeployment terminates")

		terminating := newUnstructured("machine.sapcloud.io", "v1alpha1", "MachineDeployment")
		Expect(k8sClient.Get(suiteCtx, client.ObjectKeyFromObject(md), terminating)).To(Succeed())
		Expect(terminating.GetDeletionTimestamp()).NotTo(BeNil())

		// The class was rendered by helm before the migration and carries no node-group label;
		// cleanup must stamp it while the MachineDeployment reference is still readable.
		labelled := newUnstructured("machine.sapcloud.io", "v1alpha1", machineClassKind)
		Expect(k8sClient.Get(suiteCtx, types.NamespacedName{
			Name: ngName + "-zone-a", Namespace: common.MachineNamespace,
		}, labelled)).To(Succeed())
		Expect(labelled.GetLabels()).To(HaveKeyWithValue("node-group", ngName))

		finishTermination(md)

		done, err = r.cleanupMachineDeployments(suiteCtx, ngName)
		Expect(err).NotTo(HaveOccurred())
		Expect(done).To(BeTrue(), "cleanup is finished once no MachineDeployment is left")
	})

	It("deletes a stale MachineClass only after its MachineDeployment is gone", func() {
		ngName := testenv.UniqueName("mcm-prune")
		keptName := ngName + "-zone-a"
		staleName := ngName + "-zone-b"
		ownedByNG := map[string]string{"node-group": ngName}

		createMachineClass(keptName, ownedByNG)
		createMachineClass(staleName, ownedByNG)
		createMachineDeployment(keptName, ngName, keptName)
		staleMD := createMachineDeployment(staleName, ngName, staleName)

		r := newReconciler()
		desiredMDs := map[string]struct{}{keptName: {}}
		desiredClasses := map[string]struct{}{keptName: {}}

		stale, err := r.pruneStaleMCMs(suiteCtx, k8sClient, ngName, machineClassKind, desiredMDs, desiredClasses)
		Expect(err).NotTo(HaveOccurred())
		Expect(stale).To(Equal(1))
		Expect(getObject(machineClassKind, staleName)).To(Succeed(),
			"the class must outlive the MachineDeployment that is still deleting its Machines")

		finishTermination(staleMD)

		stale, err = r.pruneStaleMCMs(suiteCtx, k8sClient, ngName, machineClassKind, desiredMDs, desiredClasses)
		Expect(err).NotTo(HaveOccurred())
		Expect(stale).To(Equal(0))
		Eventually(func() bool {
			return errors.IsNotFound(getObject(machineClassKind, staleName))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(BeTrue())

		Expect(getObject(machineClassKind, keptName)).To(Succeed(), "the desired class must be untouched")
		Expect(getObject("MachineDeployment", keptName)).To(Succeed(), "the desired MachineDeployment must be untouched")
	})
})

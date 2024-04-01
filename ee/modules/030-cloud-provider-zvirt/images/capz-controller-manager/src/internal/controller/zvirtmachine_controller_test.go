/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ovirt "github.com/ovirt/go-ovirt-client/v3"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrastructurev1 "github.com/deckhouse/deckhouse/api/v1"
)

var _ = Describe("ZvirtMachine Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-machine"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "d8-cloud-provider-zvirt",
		}
		zvirtmachine := &infrastructurev1.ZvirtMachine{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind ZvirtMachine")
			err := k8sClient.Get(ctx, typeNamespacedName, zvirtmachine)
			if err != nil && errors.IsNotFound(err) {
				resource := &infrastructurev1.ZvirtMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "d8-cloud-provider-zvirt",
					},
					Spec: infrastructurev1.ZvirtMachineSpec{
						TemplateName:  "astra-175",
						VNICProfileID: "411768ee-9d95-4d03-90b5-18bf4e4a78a0",
						CPU:           infrastructurev1.CPU{Sockets: 4, Cores: 1, Threads: 1},
						Memory:        8192,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &infrastructurev1.ZvirtMachine{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ZvirtMachine")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &ZvirtMachineReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				Zvirt:  ovirt.NewMock(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(controllerReconciler.Zvirt.GetVMByName(resourceName)).To(Succeed())
		})
	})
})

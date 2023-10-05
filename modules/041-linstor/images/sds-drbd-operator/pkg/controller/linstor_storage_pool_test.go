package controller_test

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sds-drbd-operator/api/v1alpha1"
	"sds-drbd-operator/pkg/controller"
)

var _ = Describe(controller.LinstorStoragePoolControllerName, func() {
	const (
		testNameSpace = "test_namespace"
		testName      = "test_name"
	)

	var (
		ctx     = context.Background()
		cl      = NewFakeClient()
		testLsp = &v1alpha1.LinstorStoragePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testName,
				Namespace: testNameSpace,
			},
		}
	)

	It("GetLinstorStoragePool", func() {
		err := cl.Create(ctx, testLsp)
		Expect(err).NotTo(HaveOccurred())

		lsp, err := controller.GetLinstorStoragePool(ctx, cl, testNameSpace, testName)
		Expect(err).NotTo(HaveOccurred())
		Expect(lsp.Name).To(Equal(testName))
		Expect(lsp.Namespace).To(Equal(testNameSpace))
	})

	It("UpdateLinstorStoragePool", func() {
		const (
			testLblKey   = "test_label_key"
			testLblValue = "test_label_value"
		)

		Expect(testLsp.Labels[testLblKey]).To(Equal(""))

		lspLabs := map[string]string{testLblKey: testLblValue}
		testLsp.Labels = lspLabs

		err := controller.UpdateLinstorStoragePool(ctx, cl, testLsp)
		Expect(err).NotTo(HaveOccurred())

		updatedLsp, _ := controller.GetLinstorStoragePool(ctx, cl, testNameSpace, testName)
		Expect(updatedLsp.Labels[testLblKey]).To(Equal(testLblValue))
	})

	It("HasDuplicates", func() {
		unique := []string{"a", "b", "c", "d"}

		hasDuplicates := controller.HasDuplicates(unique)
		Expect(hasDuplicates).To(BeFalse())

		duplicates := []string{"a", "a", "b", "c"}

		hasDuplicates = controller.HasDuplicates(duplicates)
		Expect(hasDuplicates).To(BeTrue())
	})

	It("GetLvmVolumeGroup", func() {
		testLvm := &v1alpha1.LvmVolumeGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testName,
				Namespace: testNameSpace,
			},
		}

		err := cl.Create(ctx, testLvm)
		Expect(err).NotTo(HaveOccurred())

		lvm, err := controller.GetLvmVolumeGroup(ctx, cl, testNameSpace, testName)
		Expect(err).NotTo(HaveOccurred())
		Expect(lvm.Name).To(Equal(testName))
		Expect(lvm.Namespace).To(Equal(testNameSpace))
	})

	It("ValidateVolumeGroup", func() {
		const (
			uniqueNodeName     = "uniqueNodeName"
			duplicatedNodeName = "duplicatedNodeName"
		)
		uniqueNodesLvm := &v1alpha1.LvmVolumeGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      uniqueNodeName,
				Namespace: testNameSpace,
			},
			Status: v1alpha1.LvmVGStatus{Nodes: []v1alpha1.LvmVGNode{
				{
					Name: "first_node",
				},
				{
					Name: "second_node",
				},
			}},
		}

		err := cl.Create(ctx, uniqueNodesLvm)
		Expect(err).NotTo(HaveOccurred())

		duplicatedNodesLvm := &v1alpha1.LvmVolumeGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      duplicatedNodeName,
				Namespace: testNameSpace,
			},
			Status: v1alpha1.LvmVGStatus{Nodes: []v1alpha1.LvmVGNode{
				{
					Name: "first_node",
				},
				{
					Name: "first_node",
				},
			}},
		}

		err = cl.Create(ctx, duplicatedNodesLvm)
		Expect(err).NotTo(HaveOccurred())

		testLsp.Spec.LvmVolumeGroups = []v1alpha1.LSPLvmVolumeGroups{
			{
				Name:         uniqueNodeName,
				ThinPoolName: "",
			},
		}

		noDuplicates, err := controller.ValidateVolumeGroup(ctx, cl, testLsp)
		Expect(err).NotTo(HaveOccurred())
		Expect(noDuplicates).To(Equal(0))

		testLsp.Spec.LvmVolumeGroups = []v1alpha1.LSPLvmVolumeGroups{
			{
				Name:         duplicatedNodeName,
				ThinPoolName: "",
			},
		}

		hasDuplicates, err := controller.ValidateVolumeGroup(ctx, cl, testLsp)
		Expect(err).NotTo(HaveOccurred())
		Expect(hasDuplicates).To(Equal(1))

	})
})

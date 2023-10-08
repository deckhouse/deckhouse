/*
Copyright 2023 Flant JSC

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

package controller_test

import (
	"context"
	"fmt"
	"sds-drbd-operator/api/v1alpha1"
	"sds-drbd-operator/pkg/controller"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	It("UpdateMapValue", func() {
		m := make(map[string]string)

		// Test adding a new key-value pair
		controller.UpdateMapValue(m, "key1", "value1")
		Expect(m["key1"]).To(Equal("value1"))

		// Test updating an existing key-value pair
		controller.UpdateMapValue(m, "key1", "value2")
		Expect(m["key1"]).To(Equal("value1. Also: value2"))

		// Test another updating an existing key-value pair
		controller.UpdateMapValue(m, "key1", "value3")
		Expect(m["key1"]).To(Equal("value1. Also: value2. Also: value3"))

		// Test adding another new key-value pair
		controller.UpdateMapValue(m, "key2", "value2")
		Expect(m["key2"]).To(Equal("value2"))

		// Test updating an existing key-value pair with an empty value
		controller.UpdateMapValue(m, "key2", "")
		Expect(m["key2"]).To(Equal("value2. Also: "))

		// Test adding a new key-value pair with an empty key
		controller.UpdateMapValue(m, "", "value3")
		Expect(m[""]).To(Equal("value3"))
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
			lvmVGOneOnFirstNodeName  = "lvmVG-1-on-FirstNode"
			lvmVGTwoOnFirstNodeName  = "lvmVG-2-on-FirstNode"
			lvmVGOneOnSecondNodeName = "lvmVG-1-on-SecondNode"
			notExistedlvnVGName      = "not_existed_lvmVG"
			firstNodeName            = "first_node"
			secondNodeName           = "second_node"
			thirdNodeName            = "third_node"
		)

		err := CreateLVMVolumeGroup(ctx, cl, lvmVGOneOnFirstNodeName, testNameSpace, []string{firstNodeName})
		Expect(err).NotTo(HaveOccurred())

		err = CreateLVMVolumeGroup(ctx, cl, lvmVGTwoOnFirstNodeName, testNameSpace, []string{firstNodeName})
		Expect(err).NotTo(HaveOccurred())

		err = CreateLVMVolumeGroup(ctx, cl, lvmVGOneOnSecondNodeName, testNameSpace, []string{secondNodeName})
		Expect(err).NotTo(HaveOccurred())

		GoodListorStoragePool := GetListorStoragePool(testLsp, []string{lvmVGOneOnFirstNodeName, lvmVGOneOnSecondNodeName})

		ok, msg := controller.ValidateVolumeGroup(ctx, cl, GoodListorStoragePool)
		Expect(ok).To(BeTrue())
		Expect(msg).To(BeNil())

		BadListorStoragePool := GetListorStoragePool(testLsp, []string{lvmVGOneOnFirstNodeName, notExistedlvnVGName, lvmVGOneOnSecondNodeName, lvmVGTwoOnFirstNodeName, lvmVGOneOnSecondNodeName})
		expectedMsg := map[string]string{
			"lvmVG-2-on-FirstNode":  "This LvmVolumeGroup have same node first_node as LvmVolumeGroup with name: lvmVG-1-on-FirstNode. This is forbidden",
			"lvmVG-2-on-SecondNode": "LvmVolumeGroup name is not unique",
			"sdasd":                 "Error getting LVMVolumeGroup: lvmvolumegroups.storage.deckhouse.io \"sdasd\" not found",
		}
		ok, msg = controller.ValidateVolumeGroup(ctx, cl, BadListorStoragePool)
		Expect(ok).To(BeFalse())
		Expect(msg).To(HaveLen(len(expectedMsg)))
		Expect(msg).To(HaveKeyWithValue(lvmVGTwoOnFirstNodeName, fmt.Sprintf("This LvmVolumeGroup have same node %s as LvmVolumeGroup with name: %s. This is forbidden", firstNodeName, lvmVGOneOnFirstNodeName)))
		Expect(msg).To(HaveKeyWithValue(lvmVGOneOnSecondNodeName, "LvmVolumeGroup name is not unique"))
		Expect(msg).To(HaveKeyWithValue(notExistedlvnVGName, fmt.Sprintf("Error getting LVMVolumeGroup: lvmvolumegroups.storage.deckhouse.io \"%s\" not found", notExistedlvnVGName)))
	})
})

func CreateLVMVolumeGroup(ctx context.Context, cl client.WithWatch, name string, namespace string, nodes []string) error {
	vgNodes := make([]v1alpha1.LvmVGNode, len(nodes))
	for i, node := range nodes {
		vgNodes[i] = v1alpha1.LvmVGNode{Name: node}
	}
	lvmVolumeGroup := &v1alpha1.LvmVolumeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: v1alpha1.LvmVGStatus{Nodes: vgNodes},
	}

	err := cl.Create(ctx, lvmVolumeGroup)
	return err
}

func GetListorStoragePool(lsp *v1alpha1.LinstorStoragePool, vgNames []string) *v1alpha1.LinstorStoragePool {

	volumeGroups := make([]v1alpha1.LSPLvmVolumeGroups, len(vgNames))

	for i, vgName := range vgNames {
		volumeGroups[i] = v1alpha1.LSPLvmVolumeGroups{
			Name:         vgName,
			ThinPoolName: "",
		}
	}

	lsp.Spec.LvmVolumeGroups = volumeGroups
	return lsp
}

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
	"sds-drbd-operator/api/v1alpha1"
	"sds-drbd-operator/pkg/controller"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe(controller.DRBDOperatorStoragePoolControllerName, func() {
	const (
		testNameSpace = "test_namespace"
		testName      = "test_name"
	)

	var (
		ctx        = context.Background()
		cl         = NewFakeClient()
		testDRBDSP = &v1alpha1.DRBDOperatorStoragePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testName,
				Namespace: testNameSpace,
			},
		}
	)

	It("GetDRBDOperatorStoragePool", func() {
		err := cl.Create(ctx, testDRBDSP)
		Expect(err).NotTo(HaveOccurred())

		drbdsp, err := controller.GetDRBDOperatorStoragePool(ctx, cl, testNameSpace, testName)
		Expect(err).NotTo(HaveOccurred())
		Expect(drbdsp.Name).To(Equal(testName))
		Expect(drbdsp.Namespace).To(Equal(testNameSpace))
	})

	It("UpdateDRBDOperatorStoragePool", func() {
		const (
			testLblKey   = "test_label_key"
			testLblValue = "test_label_value"
		)

		Expect(testDRBDSP.Labels[testLblKey]).To(Equal(""))

		drbdspLabs := map[string]string{testLblKey: testLblValue}
		testDRBDSP.Labels = drbdspLabs

		err := controller.UpdateDRBDOperatorStoragePool(ctx, cl, testDRBDSP)
		Expect(err).NotTo(HaveOccurred())

		updateddrbdsp, _ := controller.GetDRBDOperatorStoragePool(ctx, cl, testNameSpace, testName)
		Expect(updateddrbdsp.Labels[testLblKey]).To(Equal(testLblValue))
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

	It("Validations", func() {
		const (
			LvmVGOneOnFirstNodeName    = "lvmVG-1-on-FirstNode"
			ActualVGOneOnFirstNodeName = "actualVG-1-on-FirstNode"

			LvmVGTwoOnFirstNodeName    = "lvmVG-2-on-FirstNode"
			ActualVGTwoOnFirstNodeName = "actualVG-2-on-FirstNode"

			LvmVGOneOnSecondNodeName          = "lvmVG-1-on-SecondNode"
			LvmVGOneOnSecondNodeNameDublicate = "lvmVG-1-on-SecondNode"
			ActualVGOneOnSecondNodeName       = "actualVG-1-on-SecondNode"

			NotExistedlvnVGName = "not_existed_lvmVG"

			FirstNodeName  = "first_node"
			SecondNodeName = "second_node"
			ThirdNodeName  = "third_node"

			GoodDRBDOperatorStoragePoolName = "goodDRBDOperatorStoragePool"
			BadDRBDOperatorStoragePoolName  = "badDRBDOperatorStoragePool"
			TypeLVMThin                     = "LVMThin"
			TypeLVM                         = "LVM"
		)

		err := CreateLVMVolumeGroup(ctx, cl, LvmVGOneOnFirstNodeName, TypeLVM, ActualVGOneOnFirstNodeName, []string{FirstNodeName}, nil)
		Expect(err).NotTo(HaveOccurred())

		err = CreateLVMVolumeGroup(ctx, cl, LvmVGTwoOnFirstNodeName, TypeLVM, ActualVGTwoOnFirstNodeName, []string{FirstNodeName}, nil)
		Expect(err).NotTo(HaveOccurred())

		err = CreateLVMVolumeGroup(ctx, cl, LvmVGOneOnSecondNodeName, TypeLVM, ActualVGOneOnSecondNodeName, []string{SecondNodeName}, nil)
		Expect(err).NotTo(HaveOccurred())

		err = CreateDRBDOperatorStoragePool(ctx, cl, GoodDRBDOperatorStoragePoolName, TypeLVM, map[string]string{LvmVGOneOnFirstNodeName: "", LvmVGOneOnSecondNodeName: ""})
		Expect(err).NotTo(HaveOccurred())

		err = CreateDRBDOperatorStoragePool(ctx, cl, BadDRBDOperatorStoragePoolName, TypeLVM, map[string]string{LvmVGOneOnFirstNodeName: "", NotExistedlvnVGName: "", LvmVGOneOnSecondNodeName: "", LvmVGTwoOnFirstNodeName: "", LvmVGOneOnSecondNodeNameDublicate: ""})

		// err := CreateLVMVolumeGroup(ctx, cl, LvmVGOneOnFirstNodeName, testNameSpace, []string{FirstNodeName})
		// Expect(err).NotTo(HaveOccurred())

		// err = CreateLVMVolumeGroup(ctx, cl, LvmVGTwoOnFirstNodeName, testNameSpace, []string{FirstNodeName})
		// Expect(err).NotTo(HaveOccurred())

		// err = CreateLVMVolumeGroup(ctx, cl, LvmVGOneOnSecondNodeName, testNameSpace, []string{SecondNodeName})
		// Expect(err).NotTo(HaveOccurred())

		// err = CreateDRBDOperatorStoragePool(ctx, cl, GoodDRBDOperatorStoragePoolName, testNameSpace, []string{LvmVGOneOnFirstNodeName, LvmVGOneOnSecondNodeName})
		// Expect(err).NotTo(HaveOccurred())

		// 		goodDRBDOperatorStoragePool := GetDRBDOperatorStoragePool(testDRBDSP, []string{LvmVGOneOnFirstNodeName, LvmVGOneOnSecondNodeName})
		// 		badDRBDOperatorStoragePool := GetDRBDOperatorStoragePool(testDRBDSP, []string{LvmVGOneOnFirstNodeName, NotExistedlvnVGName, LvmVGOneOnSecondNodeName, LvmVGTwoOnFirstNodeName, LvmVGOneOnSecondNodeName})

		// 		// Check Kubernetes objects

		// 		// Check functions
		// 		ok, msg, _ := controller.GetAndValidateVolumeGroups(ctx, cl, goodDRBDOperatorStoragePool.Namespace, goodDRBDOperatorStoragePool.Spec.Type, goodDRBDOperatorStoragePool.Spec.LvmVolumeGroups)
		// 		//Expect(ok).To(BeTrue())
		// 		Expect(msg).To(HaveLen(0))

		// 		expectedMsg := `lvmVG-1-on-SecondNode: LvmVolumeGroup name is not unique
		// lvmVG-2-on-FirstNode: This LvmVolumeGroup have same node first_node as LvmVolumeGroup with name: lvmVG-1-on-FirstNode. LINSTOR Storage Pool is allowed to have only one LvmVolumeGroup per node
		// not_existed_lvmVG: Error getting LVMVolumeGroup: lvmvolumegroups.storage.deckhouse.io "not_existed_lvmVG" not found`

		// 		ok, msg, _ = controller.GetAndValidateVolumeGroups(ctx, cl, badDRBDOperatorStoragePool.Namespace, badDRBDOperatorStoragePool.Spec.Type, badDRBDOperatorStoragePool.Spec.LvmVolumeGroups)
		// 		Expect(ok).To(BeFalse())
		// 		Expect(strings.TrimSpace(msg)).To(Equal(strings.TrimSpace(expectedMsg)))

	})
})

// func CreateLVMVolumeGroup(ctx context.Context, cl client.WithWatch, name, namespace string, nodes []string) error {
// 	vgNodes := make([]v1alpha1.LvmVGNode, len(nodes))
// 	for i, node := range nodes {
// 		vgNodes[i] = v1alpha1.LvmVGNode{Name: node}
// 	}
// 	lvmVolumeGroup := &v1alpha1.LvmVolumeGroup{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name: name,
// 		},
// 		Status: v1alpha1.LvmVGStatus{Nodes: vgNodes},
// 	}

// 	err := cl.Create(ctx, lvmVolumeGroup)
// 	return err
// }

func CreateLVMVolumeGroup(ctx context.Context, cl client.WithWatch, lvmVolumeGroupName, lvmType, actualVGnameOnTheNode string, nodes []string, thinPools map[string]string) error {
	vgNodes := make([]v1alpha1.LvmVGNode, len(nodes))
	for i, node := range nodes {
		vgNodes[i] = v1alpha1.LvmVGNode{Name: node}
	}

	vgThinPools := make([]v1alpha1.ThinPool, len(thinPools))
	for thinPoolname, thinPoolsize := range thinPools {
		vgThinPools = append(vgThinPools, v1alpha1.ThinPool{Name: thinPoolname, Size: thinPoolsize})
	}

	lvmVolumeGroup := &v1alpha1.LvmVolumeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: lvmVolumeGroupName,
		},
		Spec: v1alpha1.LvmVGSpec{
			Type:                  lvmType,
			ActualVGnameOnTheNode: actualVGnameOnTheNode,
			ThinPools:             vgThinPools,
		},
		Status: v1alpha1.LvmVGStatus{
			Nodes: vgNodes,
		},
	}
	err := cl.Create(ctx, lvmVolumeGroup)
	return err
}

func CreateDRBDOperatorStoragePool(ctx context.Context, cl client.WithWatch, name, lvmType string, lvmVolumeGroups map[string]string) error {

	volumeGroups := make([]v1alpha1.DRBDStoragePoolLVMVolumeGroups, len(lvmVolumeGroups))
	for vgName, vgThinPoolName := range lvmVolumeGroups {
		volumeGroups = append(volumeGroups, v1alpha1.DRBDStoragePoolLVMVolumeGroups{
			Name:         vgName,
			ThinPoolName: vgThinPoolName,
		})
	}

	drbdsp := &v1alpha1.DRBDOperatorStoragePool{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.DRBDOperatorStoragePoolSpec{
			Type:            "LVM",
			LvmVolumeGroups: volumeGroups,
		},
	}

	err := cl.Create(ctx, drbdsp)
	return err
}

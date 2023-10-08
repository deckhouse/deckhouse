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
	v1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe(controller.LinstorStorageClassControllerName, func() {
	const (
		testLinstorName      = "test_linstor_name"
		testLinstorNamespace = "test_linstor_ns"
	)

	var (
		ctx = context.Background()
		cl  = NewFakeClient()

		testLsc = v1alpha1.LinstorStorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testLinstorName,
				Namespace: testLinstorNamespace,
			},
		}
	)

	It("GetLinstorStorageClass", func() {
		err := cl.Create(ctx, &testLsc)
		Expect(err).NotTo(HaveOccurred())

		lsc, err := controller.GetLinstorStorageClass(ctx, cl, testLinstorNamespace, testLinstorName)
		Expect(err).NotTo(HaveOccurred())
		Expect(lsc.Name).To(Equal(testLinstorName))
		Expect(lsc.Namespace).To(Equal(testLinstorNamespace))
	})

	It("UpdateLinstorStorageClass", func() {
		const (
			testLabelKey   = "test_label_key"
			testLabelValue = "test_label_value"
		)

		Expect(testLsc.Labels[testLabelKey]).To(Equal(""))

		updatedLabels := map[string]string{testLabelKey: testLabelValue}
		testLsc.Labels = updatedLabels

		err := controller.UpdateLinstorStorageClass(ctx, cl, &testLsc)
		Expect(err).NotTo(HaveOccurred())

		updatedLcs, err := controller.GetLinstorStorageClass(ctx, cl, testLinstorNamespace, testLinstorName)
		Expect(err).NotTo(HaveOccurred())
		Expect(updatedLcs.Labels[testLabelKey]).To(Equal(testLabelValue))
	})

	var sc *v1.StorageClass
	It("Create/GetStorageClass", func() {
		err := controller.CreateStorageClass(ctx, cl, &testLsc)
		Expect(err).NotTo(HaveOccurred())

		sc, err = controller.GetStorageClass(ctx, cl, testLinstorNamespace, testLinstorName)
		Expect(err).NotTo(HaveOccurred())
		Expect(sc.Name).To(Equal(testLinstorName))
		Expect(sc.Namespace).To(Equal(testLinstorNamespace))
	})

	It("UpdateStorageClass", func() {
		const (
			testLblKey   = "test_label_key"
			testLblValue = "test_label_value"
		)

		Expect(sc.Labels[testLblKey]).To(Equal(""))

		sc.Labels = map[string]string{testLblKey: testLblValue}

		err := controller.UpdateStorageClass(ctx, cl, sc)
		Expect(err).NotTo(HaveOccurred())

		sc, err = controller.GetStorageClass(ctx, cl, testLinstorNamespace, testLinstorName)
		Expect(err).NotTo(HaveOccurred())
		Expect(sc.Labels[testLblKey]).To(Equal(testLblValue))
	})

	It("DeleteStorageClass", func() {
		sc, err := controller.GetStorageClass(ctx, cl, testLinstorNamespace, testLinstorName)
		Expect(err).NotTo(HaveOccurred())
		Expect(sc.Name).To(Equal(testLinstorName))

		err = controller.DeleteStorageClass(ctx, cl, testLinstorNamespace, testLinstorName)
		Expect(err).NotTo(HaveOccurred())

		shouldBeNil, err := controller.GetStorageClass(ctx, cl, testLinstorNamespace, testLinstorName)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("storageclasses.storage.k8s.io \"test_linstor_name\" not found"))
		Expect(shouldBeNil).To(BeNil())
	})
})

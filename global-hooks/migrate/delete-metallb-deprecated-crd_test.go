/*
Copyright 2025 Flant JSC

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

package hooks

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("MetalLB hooks :: delete deprecated AddressPool CRD", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)
	Context("Deprecated CRD exists", func() {
		BeforeEach(func() {
			f.KubeStateSet("")

			gvr := schema.GroupVersionResource{
				Group:    "apiextensions.k8s.io",
				Version:  "v1",
				Resource: "customresourcedefinitions",
			}

			msObject := &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apiextensions.k8s.io/v1",
					"kind":       "CustomResourceDefinition",
					"metadata": map[string]any{
						"name": "addresspools.metallb.io",
					},
				},
			}

			k8sClient := dependency.TestDC.MustGetK8sClient().Dynamic().Resource(gvr)
			_, err := k8sClient.Create(context.TODO(), msObject, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.RunHook()

		})
		It("Must be run successfully and CRD deleted", func() {
			gvr := schema.GroupVersionResource{
				Group:    "apiextensions.k8s.io",
				Version:  "v1",
				Resource: "customresourcedefinitions",
			}

			k8sClient := dependency.TestDC.MustGetK8sClient().Dynamic().Resource(gvr)
			_, err := k8sClient.Get(context.TODO(), "addresspools.metallb.io", metav1.GetOptions{})
			Expect(err).To(Not(BeNil()))
		})
	})
})

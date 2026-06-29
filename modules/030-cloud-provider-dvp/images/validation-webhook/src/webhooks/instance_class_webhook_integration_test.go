// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webhooks

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("DVPInstanceClass webhook", func() {
	BeforeEach(func() {
		createValidDVPWebhookCluster()
	})

	AfterEach(func() {
		deleteDVPWebhookCluster()
	})

	It("rejects etcdDisk on worker instance class", func() {
		worker := &unstructured.Unstructured{}
		worker.SetGroupVersionKind(instanceClassGVK())
		Expect(testK8sClient.Get(testCtx, clientObjectKey("", "worker"), worker)).To(Succeed())

		Expect(unstructured.SetNestedMap(worker.Object, map[string]any{"size": "5Gi", "storageClass": "replicated"}, "spec", "etcdDisk")).To(Succeed())

		err := testK8sClient.Update(testCtx, worker)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring(`worker.spec.etcdDisk: Invalid value: map[string]interface {}{"size":"5Gi", "storageClass":"replicated"}: InstanceClass.spec.etcdDisk can be used only when class is attached to NodeGroup master`))
	})

	It("rejects deleting an instance class used by a NodeGroup", func() {
		worker := &unstructured.Unstructured{}
		worker.SetGroupVersionKind(instanceClassGVK())
		worker.SetName("worker")

		err := testK8sClient.Delete(testCtx, worker)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring(`worker: Invalid value: "worker": InstanceClass is used by NodeGroup "worker"`))
	})

	It("rejects removing etcdDisk from master instance class", func() {
		master := &unstructured.Unstructured{}
		master.SetGroupVersionKind(instanceClassGVK())
		Expect(testK8sClient.Get(testCtx, clientObjectKey("", "master-dvp"), master)).To(Succeed())

		unstructured.RemoveNestedField(master.Object, "spec", "etcdDisk")

		err := testK8sClient.Update(testCtx, master)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring(`master-dvp.spec.etcdDisk: Invalid value: "null": DVPInstanceClass for NodeGroup master must define spec.etcdDisk`))
	})
})

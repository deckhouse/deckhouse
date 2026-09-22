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

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

var _ = Describe("NodeGroup webhook", func() {
	BeforeEach(func() {
		createValidYandexWebhookCluster()
	})

	AfterEach(func() {
		deleteYandexWebhookCluster()
	})

	// The reviewed object is a NodeGroup, so the settings it is checked against come from the
	// ModuleConfig in the cluster: this spec fails if the webhook stops loading it.
	It("rejects more nodes than external IP addresses in the ModuleConfig", func() {
		moduleConfig := yandexModuleConfigIntegrationObject(map[string]any{
			"scaled": []any{"1.2.3.4"},
		})
		Expect(testK8sClient.Create(testCtx, moduleConfig)).To(Succeed())
		defer func() {
			Expect(testK8sClient.Delete(testCtx, moduleConfig)).To(Succeed())
		}()

		nodeGroup := yandexNodeGroupIntegrationObject("scaled", "worker")
		Expect(unstructured.SetNestedField(nodeGroup.Object, int64(2), "spec", "cloudInstances", "maxPerZone")).To(Succeed())

		err := testK8sClient.Create(testCtx, nodeGroup)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring(`number of nodes in NodeGroup "scaled" (2)`))
	})

	It("allows a NodeGroup with enough external IP addresses", func() {
		moduleConfig := yandexModuleConfigIntegrationObject(map[string]any{
			"scaled": []any{"1.2.3.4", "5.6.7.8"},
		})
		Expect(testK8sClient.Create(testCtx, moduleConfig)).To(Succeed())
		defer func() {
			Expect(testK8sClient.Delete(testCtx, moduleConfig)).To(Succeed())
		}()

		nodeGroup := yandexNodeGroupIntegrationObject("scaled", "worker")
		Expect(unstructured.SetNestedField(nodeGroup.Object, int64(2), "spec", "cloudInstances", "maxPerZone")).To(Succeed())

		Expect(testK8sClient.Create(testCtx, nodeGroup)).To(Succeed())
		Expect(testK8sClient.Delete(testCtx, nodeGroup)).To(Succeed())
	})

	// The class referenced by the reviewed NodeGroup is loaded from the cluster: master requires
	// an etcd disk, and the worker class in the fixture has none.
	It("rejects a master NodeGroup referencing a class without an etcd disk", func() {
		Expect(testK8sClient.Delete(testCtx, yandexNodeGroupIntegrationObject("master", "master-yandex"))).To(Succeed())

		master := yandexNodeGroupIntegrationObject("master", "worker")

		err := testK8sClient.Create(testCtx, master)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("must define spec.etcdDisk"))
	})

	// Admission does not verify that the referenced class exists: cpval.ValidateNodeGroupsClassReference
	// is called with verifyExistence=false here, because the class may legitimately be created
	// right after the NodeGroup. dhctl preflight is the surface that requires it.
	It("allows a NodeGroup referencing a class that does not exist yet", func() {
		nodeGroup := yandexNodeGroupIntegrationObject("orphan", "absent-class")

		Expect(testK8sClient.Create(testCtx, nodeGroup)).To(Succeed())
		Expect(testK8sClient.Delete(testCtx, nodeGroup)).To(Succeed())
	})

	// Static NodeGroups are none of the provider's business: they carry no cloud instances.
	It("allows a Static NodeGroup", func() {
		nodeGroup := &unstructured.Unstructured{}
		nodeGroup.SetGroupVersionKind(nodeGroupGVK())
		nodeGroup.SetName("static")
		nodeGroup.Object["spec"] = map[string]any{"nodeType": "Static"}

		Expect(testK8sClient.Create(testCtx, nodeGroup)).To(Succeed())
		Expect(testK8sClient.Delete(testCtx, nodeGroup)).To(Succeed())
	})

	It("allows deleting a CloudPermanent NodeGroup", func() {
		Expect(testK8sClient.Delete(testCtx, yandexNodeGroupIntegrationObject("worker", "worker"))).To(Succeed())

		nodeGroup := &unstructured.Unstructured{}
		nodeGroup.SetGroupVersionKind(nodeGroupGVK())
		err := testK8sClient.Get(testCtx, clientObjectKey("", "worker"), nodeGroup)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	// A CloudPermanent NodeGroup without cloudInstances.classReference passes: the rule skips
	// such a NodeGroup entirely (go_lib/cloud-provider/validation/node_group.go:75), even though
	// its godoc claims to check the field presence. Pinned here so the divergence is visible if
	// the rule ever starts reporting it.
	It("allows a CloudPermanent NodeGroup without a class reference", func() {
		nodeGroup := &unstructured.Unstructured{}
		nodeGroup.SetGroupVersionKind(nodeGroupGVK())
		nodeGroup.SetName("no-class")
		nodeGroup.Object["spec"] = map[string]any{"nodeType": string(cpapi.NodeTypeCloudPermanent)}

		Expect(testK8sClient.Create(testCtx, nodeGroup)).To(Succeed())
		Expect(testK8sClient.Delete(testCtx, nodeGroup)).To(Succeed())
	})
})

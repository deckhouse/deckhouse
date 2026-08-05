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

	ycmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/meta"
)

var _ = Describe("ModuleConfig webhook", func() {
	BeforeEach(func() {
		createValidYandexWebhookCluster()
	})

	AfterEach(func() {
		deleteYandexWebhookCluster()
	})

	// The reviewed object is the ModuleConfig, so the NodeGroups it is checked against come from
	// the cluster: this spec fails if the webhook stops loading them.
	It("rejects fewer external IP addresses than nodes in a NodeGroup", func() {
		moduleConfig := yandexModuleConfigIntegrationObject(map[string]any{
			"worker": []any{"1.2.3.4"},
		})

		err := testK8sClient.Create(testCtx, moduleConfig)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring(`number of nodes in NodeGroup "worker" (2)`))
	})

	It("allows as many external IP addresses as nodes", func() {
		moduleConfig := yandexModuleConfigIntegrationObject(map[string]any{
			"worker": []any{"1.2.3.4", "5.6.7.8"},
		})

		Expect(testK8sClient.Create(testCtx, moduleConfig)).To(Succeed())
		Expect(testK8sClient.Delete(testCtx, moduleConfig)).To(Succeed())
	})

	It("allows a ModuleConfig without external IP addresses", func() {
		moduleConfig := yandexModuleConfigIntegrationObject(nil)

		Expect(testK8sClient.Create(testCtx, moduleConfig)).To(Succeed())
		Expect(testK8sClient.Delete(testCtx, moduleConfig)).To(Succeed())
	})

	// ModuleConfig is cluster-scoped and shared by every module, so the webhook must let another
	// module's config through untouched even when it would violate the Yandex rules.
	It("ignores a ModuleConfig of another module", func() {
		moduleConfig := yandexModuleConfigIntegrationObject(map[string]any{
			"worker": []any{"1.2.3.4"},
		})
		moduleConfig.SetName("cloud-provider-dvp")

		Expect(testK8sClient.Create(testCtx, moduleConfig)).To(Succeed())
		Expect(testK8sClient.Delete(testCtx, moduleConfig)).To(Succeed())
	})
})

// yandexModuleConfigIntegrationObject builds the provider ModuleConfig, optionally assigning
// external IP addresses per NodeGroup.
func yandexModuleConfigIntegrationObject(externalIPAddresses map[string]any) *unstructured.Unstructured {
	nodesParameters := map[string]any{
		"layout":          "Standard",
		"nodeNetworkCIDR": "10.0.0.0/16",
		"sshPublicKey":    "ssh-rsa KEY",
	}
	if externalIPAddresses != nil {
		nodesParameters["externalIPAddresses"] = externalIPAddresses
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(moduleConfigGVK())
	obj.SetName(ycmeta.ModuleName)
	obj.Object["spec"] = map[string]any{
		"enabled": true,
		"version": int64(2),
		"settings": map[string]any{
			"provider": map[string]any{
				"parameters": map[string]any{"cloudID": "cloud-1", "folderID": "folder-1"},
			},
			"nodes": map[string]any{"parameters": nodesParameters},
		},
	}

	return obj
}

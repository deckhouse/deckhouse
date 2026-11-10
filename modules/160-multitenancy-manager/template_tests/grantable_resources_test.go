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

package template_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: multitenancy-manager :: helm template :: grantable resources ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", `{}`)
		f.ValuesSetFromYaml("global.enabledModules", `["cert-manager", "multitenancy-manager"]`)
		f.ValuesSetFromYaml("multitenancyManager.internal.admissionWebhookCert", `{"ca": "test-ca", "crt": "test", "key": "test"}`)
		f.ValuesSetFromYaml("multitenancyManager.internal.quotaLedgerCert", `{"ca": "test-ca", "crt": "test", "key": "test"}`)
		f.ApplyOpenAPIDefaults()
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	It("registers the cluster resources of Kubernetes itself", func() {
		for _, name := range []string{"storageclasses", "loadbalancerclasses", "clusterroles"} {
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceDefinition", name).Exists()).To(BeTrue(), name)
		}
		for _, name := range []string{"storageclasses-pvc", "loadbalancerclasses-service", "clusterroles-rolebinding"} {
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", name).Exists()).To(BeTrue(), name)
		}
	})

	It("leaves ClusterIssuer to the cert-manager module, even while it is enabled", func() {
		Expect(f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "clusterissuers").Exists()).To(BeFalse())
		Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", "clusterissuers-certificate").Exists()).To(BeFalse())
		Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", "clusterissuers-ingress").Exists()).To(BeFalse())
	})
})

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

var _ = Describe("Module :: registry-packages-proxy :: helm template :: published config", func() {
	f := SetupHelmConfig(``)

	Context("Cluster with a published public domain", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesWithCAs)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registryPackagesProxy", servingCertificatePresent)
			f.HelmRender()
		})

		It("tells a client where the proxy is and what to trust", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			config := f.KubernetesResource("ConfigMap", "d8-cloud-instance-manager", "registry-packages-proxy-config")
			Expect(config.Exists()).To(BeTrue())
			Expect(config.Field("data.endpoints").String()).To(Equal(`["192.168.0.1:4219"]`))
			Expect(config.Field(`data.ca\.crt`).String()).To(ContainSubstring("CACACA"))
			Expect(config.Field("data.publicEndpoint").String()).
				To(Equal("https://registry-packages-proxy.example.com"))
		})

		It("lets the download role read that config", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			role := f.KubernetesGlobalResource("ClusterRole", "d8:registry-packages-proxy:cli-download")
			Expect(role.Exists()).To(BeTrue())

			rules := role.Field("rules").String()
			Expect(rules).To(ContainSubstring(`"deployments/cli-binary"`))
			Expect(rules).To(ContainSubstring(`"configmaps"`))
			Expect(rules).To(ContainSubstring(`"registry-packages-proxy-config"`))
		})
	})

	Context("Cluster without a published public domain", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesNotBootstrapped)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registryPackagesProxy", servingCertificatePresent)
			f.HelmRender()
		})

		It("still publishes the master addresses and the CA", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			config := f.KubernetesResource("ConfigMap", "d8-cloud-instance-manager", "registry-packages-proxy-config")
			Expect(config.Exists()).To(BeTrue())
			Expect(config.Field("data.endpoints").String()).To(Equal(`["192.168.0.1:4219"]`))
			Expect(config.Field(`data.ca\.crt`).String()).To(ContainSubstring("CACACA"))
		})

		It("omits the public endpoint", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			config := f.KubernetesResource("ConfigMap", "d8-cloud-instance-manager", "registry-packages-proxy-config")
			Expect(config.Field("data.publicEndpoint").Exists()).To(BeFalse())
		})
	})
})

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

var _ = Describe("Module :: documentation :: helm template :: basic-auth secret", func() {
	f := SetupHelmConfig(``)

	Context("Ingress disabled, Gateway API enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ValuesSet("global.modules.ingress.enabled", false)
			f.ValuesSet("global.modules.https.mode", "CertManager")
			f.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
			f.ValuesSet("global.enabledModules", []string{"vertical-pod-autoscaler", "documentation", "cert-manager"})
			f.ValuesSet("global.discovery.gatewayAPIDefaultGateway.name", "shared-gateway")
			f.ValuesSet("global.discovery.gatewayAPIDefaultGateway.namespace", "d8-alb")
			f.ValuesSet("documentation.internal.auth.password", "plainstring")
			f.HelmRender()
		})

		// Basic auth has no per-mechanism dependency on Ingress vs Gateway API, so the secret must be
		// created whenever either mechanism is enabled, not just when Ingress is.
		It("Should create the basic-auth secret for Gateway API alone", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Secret", "d8-system", "documentation-basic-auth").Exists()).To(BeTrue())
		})
	})
})

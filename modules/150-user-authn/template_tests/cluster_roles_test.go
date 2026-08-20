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

var _ = Describe("Module :: user-authn :: helm template :: cluster roles", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.15.6")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.ca", "plainstring")

		hec.HelmRender()
	})

	Context("By default", func() {
		// A collection read of the Dex storage CRDs returns whole objects, `hash` and
		// `previousHashes` included, so no verb on them is safe to hand to a subject that is
		// not expected to read credentials. Only the dex and user-api service accounts may.
		It("Should not grant user-facing roles access to the Dex credential storage CRDs", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			for _, name := range []string{
				"d8:manage:permission:module:user-authn:view",
				"d8:manage:permission:module:user-authn:edit",
				"d8:user-authz:user-authn:cluster-admin",
			} {
				clusterRole := hec.KubernetesGlobalResource("ClusterRole", name)
				Expect(clusterRole.Exists()).To(BeTrue(), name)
				Expect(clusterRole.Field("rules.#.apiGroups").String()).ToNot(ContainSubstring("dex.coreos.com"), name)
			}
		})
	})
})

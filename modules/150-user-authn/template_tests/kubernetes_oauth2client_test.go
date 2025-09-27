/*
Copyright 2021 Flant JSC

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

var _ = Describe("Module :: user-authn :: helm template :: kubernetes oauth2client", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.15.6")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
	})

	Context("Without dex authenticator", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs", `[]`)
			hec.HelmRender()
		})
		It("Should not deploy kubernetes OAuth2Client", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			Expect(hec.KubernetesResource("OAuth2Client", "d8-user-authn", "nn2wezlsnzsxizltzpzjzzeeeirsk").Exists()).To(BeFalse())
		})
	})

	Context("With dex authenticator", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs", `
- name: test
  encodedName: justForTest
  allowAccessToKubernetes: true
  namespace: d8-test
  credentials:
    appDexSecret: dexSecret
    cookieSecret: cookieSecret
  spec:
    applicationDomain: authenticator.example.com
    applicationIngressCertificateSecretName: test
`)
			hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorNames", `
"test@d8-test":
  name: "test-dex-authenticator"
  truncated: false
  hash: ""
  secretName: "dex-authenticator-test"
  secretTruncated: false
  secretHash: ""
  ingressNames: {}
`)
			hec.HelmRender()
		})
		It("Should deploy kubernetes OAuth2Client", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			Expect(hec.KubernetesResource("OAuth2Client", "d8-user-authn", "nn2wezlsnzsxizltzpzjzzeeeirsk").Exists()).To(BeTrue())
		})
	})

	Context("With dex authenticator without access to Kubernetes API", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs", `
- name: test
  encodedName: justForTest
  namespace: d8-test
  credentials:
    appDexSecret: dexSecret
    cookieSecret: cookieSecret
  spec:
    applicationDomain: authenticator.example.com
    applicationIngressCertificateSecretName: test
`)
			hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorNames", `
"test@d8-test":
  name: "test-dex-authenticator"
  truncated: false
  hash: ""
  secretName: "dex-authenticator-test"
  secretTruncated: false
  secretHash: ""
  ingressNames: {}
`)
			hec.HelmRender()
		})
		It("Should not deploy kubernetes OAuth2Client", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			Expect(hec.KubernetesResource("OAuth2Client", "d8-user-authn", "nn2wezlsnzsxizltzpzjzzeeeirsk").Exists()).To(BeFalse())
		})
	})
})

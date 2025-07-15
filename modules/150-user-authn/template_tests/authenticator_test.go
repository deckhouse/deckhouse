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

var _ = Describe("Module :: user-authn :: helm template :: dex authenticator", func() {
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
	Context("With DexAuthenticator object", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs", `
- name: test
  encodedName: justForTest
  namespace: d8-test
  credentials:
    appDexSecret: dexSecret
    cookieSecret: cookieSecret
  spec:
    applications:
    - domain: authenticator.example.com
      ingressClassName: test
      ingressSecretName: test
      whitelistSourceRanges:
      - 1.1.1.1
      - 192.168.0.0/24
    - domain: authenticator-two.example.com
      ingressClassName: test-two
      ingressSecretName: test
    sendAuthorizationHeader: true
    keepUsersLoggedInFor: "1020h"
    allowedGroups:
    - everyone
    - admins
    allowedEmails:
    - test@mail.io
    nodeSelector:
      testnode: ""
    tolerations:
    - key: foo
      operator: Equal
      value: bar
- name: test-2
  encodedName: justForTest2
  namespace: d8-test
  credentials:
    appDexSecret: dexSecret
    cookieSecret: cookieSecret
  allowAccessToKubernetes: true
  spec:
    applications:
    - domain: authenticator.com
      ingressClassName: test
      ingressSecretName: test
    - domain: authenticator-two.com
      ingressClassName: test-two
      ingressSecretName: test
    sendAuthorizationHeader: false
- name: test-3
  encodedName: justForTest3
  namespace: d8-test
  credentials:
    appDexSecret: dexSecret
    cookieSecret: cookieSecret
  allowAccessToKubernetes: true
  spec:
    keepUsersLoggedInFor: "19m"
- name: test-4
  encodedName: justForTest4
  namespace: d8-test
  credentials:
    appDexSecret: dexSecret
    cookieSecret: cookieSecret
  allowAccessToKubernetes: true
  spec:
    keepUsersLoggedInFor: "2h20m4s"
`)
			hec.ValuesSet("userAuthn.idTokenTTL", "2h20m4s")
			hec.HelmRender()
		})
		It("Should create desired objects", func() {
			// Check that all main resources for DexAuthenticator are rendered
			Expect(hec.KubernetesResource("service", "d8-test", "test-dex-authenticator").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("poddisruptionbudget", "d8-test", "test-dex-authenticator").Exists()).To(BeTrue())

			vpa := hec.KubernetesResource("verticalpodautoscaler", "d8-test", "test-dex-authenticator")
			Expect(vpa.Exists()).To(BeTrue())
			// Check secret data
			secret := hec.KubernetesResource("secret", "d8-test", "dex-authenticator-test-dex-authenticator")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field("data.client-secret").String()).To(Equal("ZGV4U2VjcmV0"))
			Expect(secret.Field("data.cookie-secret").String()).To(Equal("Y29va2llU2VjcmV0"))
			// Check OAuth2Client
			oauth2clientTest := hec.KubernetesResource("oauth2client", "d8-user-authn", "test-dex-authenticator-d8-test")
			Expect(oauth2clientTest.Exists()).To(BeTrue())
			Expect(oauth2clientTest.Field("redirectURIs").String()).To(MatchJSON(`["https://authenticator.example.com/dex-authenticator/callback","https://authenticator-two.example.com/dex-authenticator/callback"]`))
			Expect(oauth2clientTest.Field("secret").String()).To(Equal("dexSecret"))
			Expect(oauth2clientTest.Field("allowedEmails").String()).To(MatchJSON(`["test@mail.io"]`))
			Expect(oauth2clientTest.Field("allowedGroups").String()).To(MatchJSON(`["everyone","admins"]`))
			// Check Ingress
			ingressTest := hec.KubernetesResource("ingress", "d8-test", "test-dex-authenticator")
			Expect(ingressTest.Exists()).To(BeTrue())
			Expect(ingressTest.Field("spec.ingressClassName").String()).To(Equal("test"))
			Expect(ingressTest.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/proxy-buffer-size").String()).To(Equal("32k"))
			Expect(ingressTest.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/whitelist-source-range").String()).To(Equal("1.1.1.1,192.168.0.0/24"))
			Expect(ingressTest.Field("spec.tls.0.hosts").String()).To(MatchJSON(`["authenticator.example.com"]`))
			Expect(ingressTest.Field("spec.tls.0.secretName").String()).To(Equal("test"))
		})
	})
})

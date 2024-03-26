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
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler-crd"})
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
    applicationDomain: authenticator.example.com
    applicationIngressCertificateSecretName: test
    applicationIngressClassName: test
    sendAuthorizationHeader: true
    keepUsersLoggedInFor: "1020h"
    whitelistSourceRanges:
    - 1.1.1.1
    - 192.168.0.0/24
    allowedGroups:
    - everyone
    - admins
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
    applicationDomain: authenticator.com
    applicationIngressCertificateSecretName: test
    applicationIngressClassName: test
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
`)
			hec.ValuesSet("userAuthn.idTokenTTL", "2h20m4s")
			hec.HelmRender()
		})
		It("Should create desired objects", func() {
			Expect(hec.KubernetesResource("Service", "d8-test", "test-dex-authenticator").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("PodDisruptionBudget", "d8-test", "test-dex-authenticator").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("VerticalPodAutoscaler", "d8-test", "test-dex-authenticator").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Secret", "d8-test", "registry-dex-authenticator").Exists()).To(BeTrue())

			secret := hec.KubernetesResource("Secret", "d8-test", "dex-authenticator-test")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field("data.client-secret").String()).To(Equal("ZGV4U2VjcmV0"))
			Expect(secret.Field("data.cookie-secret").String()).To(Equal("Y29va2llU2VjcmV0"))

			oauth2clientTest := hec.KubernetesResource("OAuth2Client", "d8-user-authn", "justForTest")
			Expect(oauth2clientTest.Exists()).To(BeTrue())
			Expect(oauth2clientTest.Field("redirectURIs").String()).To(MatchJSON(`["https://authenticator.example.com/dex-authenticator/callback"]`))
			Expect(oauth2clientTest.Field("secret").String()).To(Equal("dexSecret"))
			Expect(oauth2clientTest.Field("allowedGroups").String()).To(MatchJSON(`["everyone","admins"]`))

			ingressTest := hec.KubernetesResource("Ingress", "d8-test", "test-dex-authenticator")
			Expect(ingressTest.Exists()).To(BeTrue())
			Expect(ingressTest.Field("spec.ingressClassName").String()).To(Equal("test"))
			Expect(ingressTest.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/proxy-buffer-size").String()).To(Equal("32k"))
			Expect(ingressTest.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/whitelist-source-range").String()).To(Equal("1.1.1.1,192.168.0.0/24"))

			Expect(ingressTest.Field("spec.tls.0.hosts").String()).To(MatchJSON(`["authenticator.example.com"]`))
			Expect(ingressTest.Field("spec.tls.0.secretName").String()).To(Equal("test"))

			deploymentTest := hec.KubernetesResource("Deployment", "d8-test", "test-dex-authenticator")
			Expect(deploymentTest.Exists()).To(BeTrue())
			Expect(deploymentTest.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON(`{"testnode": ""}`))
			Expect(deploymentTest.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: foo
  operator: Equal
  value: "bar"
`))

			var oauth2proxyArgTest []string
			for _, result := range deploymentTest.Field("spec.template.spec.containers.0.args").Array() {
				oauth2proxyArgTest = append(oauth2proxyArgTest, result.String())
			}

			Expect(oauth2proxyArgTest).Should(ContainElement("--client-id=test-d8-test-dex-authenticator"))
			Expect(oauth2proxyArgTest).Should(ContainElement("--oidc-issuer-url=https://dex.example.com/"))
			Expect(oauth2proxyArgTest).Should(ContainElement("--redirect-url=https://authenticator.example.com"))
			Expect(oauth2proxyArgTest).Should(ContainElement("--set-authorization-header=true"))
			Expect(oauth2proxyArgTest).Should(ContainElement("--cookie-expire=1020h"))
			Expect(oauth2proxyArgTest).Should(ContainElement("--cookie-refresh=2h20m4s"))
			Expect(oauth2proxyArgTest).Should(ContainElement("--whitelist-domain=authenticator.example.com"))
			Expect(oauth2proxyArgTest).Should(ContainElement("--scope=groups email openid offline_access"))

			oauth2client2 := hec.KubernetesResource("OAuth2Client", "d8-user-authn", "justForTest2")
			Expect(oauth2client2.Exists()).To(BeTrue())
			Expect(oauth2client2.Field("redirectURIs").String()).To(MatchJSON(`["https://authenticator.com/dex-authenticator/callback"]`))
			Expect(oauth2client2.Field("secret").String()).To(Equal("dexSecret"))

			ingressTest2 := hec.KubernetesResource("Ingress", "d8-test", "test-2-dex-authenticator")
			Expect(ingressTest2.Exists()).To(BeTrue())
			Expect(ingressTest2.Field("spec.ingressClassName").String()).To(Equal("test"))

			Expect(ingressTest2.Field("spec.tls.0.hosts").String()).To(MatchJSON(`["authenticator.com"]`))
			Expect(ingressTest2.Field("spec.tls.0.secretName").String()).To(Equal("test"))
			Expect(ingressTest2.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/proxy-buffer-size").Exists()).To(BeFalse())
			Expect(ingressTest2.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/whitelist-source-range").Exists()).To(BeFalse())

			deploymentTest2 := hec.KubernetesResource("Deployment", "d8-test", "test-2-dex-authenticator")
			Expect(deploymentTest2.Exists()).To(BeTrue())
			Expect(deploymentTest2.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON(`{"node-role.deckhouse.io/system": ""}`))
			Expect(deploymentTest2.Field("spec.template.spec.tolerations").Exists()).To(BeTrue()) // default taints

			var oauth2proxyArgTest2 []string
			for _, result := range deploymentTest2.Field("spec.template.spec.containers.0.args").Array() {
				oauth2proxyArgTest2 = append(oauth2proxyArgTest2, result.String())
			}

			Expect(oauth2proxyArgTest2).Should(ContainElement("--client-id=test-2-d8-test-dex-authenticator"))
			Expect(oauth2proxyArgTest2).Should(ContainElement("--oidc-issuer-url=https://dex.example.com/"))
			Expect(oauth2proxyArgTest2).Should(ContainElement("--redirect-url=https://authenticator.com"))
			Expect(oauth2proxyArgTest2).ShouldNot(ContainElement("--set-authorization-header=true"))
			Expect(oauth2proxyArgTest2).Should(ContainElement("--cookie-expire=168h"))
			Expect(oauth2proxyArgTest2).Should(ContainElement("--cookie-refresh=2h20m4s"))
			Expect(oauth2proxyArgTest2).Should(ContainElement("--whitelist-domain=authenticator.com"))
			Expect(oauth2proxyArgTest2).Should(ContainElement("--scope=groups email openid offline_access audience:server:client_id:kubernetes"))

			deploymentTest3 := hec.KubernetesResource("Deployment", "d8-test", "test-3-dex-authenticator")
			Expect(deploymentTest3.Exists()).To(BeTrue())
			Expect(deploymentTest3.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON(`{"node-role.deckhouse.io/system": ""}`))
			Expect(deploymentTest3.Field("spec.template.spec.tolerations").Exists()).To(BeTrue()) // default taints

			var oauth2proxyArgTest3 []string
			for _, result := range deploymentTest3.Field("spec.template.spec.containers.0.args").Array() {
				oauth2proxyArgTest3 = append(oauth2proxyArgTest3, result.String())
			}

			Expect(oauth2proxyArgTest3).Should(ContainElement("--cookie-expire=2h20m5s"))
			Expect(oauth2proxyArgTest3).Should(ContainElement("--cookie-refresh=2h20m4s"))
		})
	})
})

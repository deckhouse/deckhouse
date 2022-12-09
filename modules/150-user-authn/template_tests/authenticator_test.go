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
		hec.ValuesSet("global.modulesImages.registry", "registry.example.com")
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
      value: bar`)
			hec.ValuesSet("userAuthn.idTokenTTL", "20m")
			hec.HelmRender()
		})
		It("Should create desired objects", func() {
			Expect(hec.KubernetesResource("Service", "d8-test", "test-dex-authenticator").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("PodDisruptionBudget", "d8-test", "test-dex-authenticator").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("VerticalPodAutoscaler", "d8-test", "test-dex-authenticator").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Secret", "d8-test", "registry-dex-authenticator").Exists()).To(BeTrue())

			oauth2client := hec.KubernetesResource("OAuth2Client", "d8-user-authn", "justForTest")
			Expect(oauth2client.Exists()).To(BeTrue())
			Expect(oauth2client.Field("redirectURIs").String()).To(MatchJSON(`["https://authenticator.example.com/dex-authenticator/callback"]`))
			Expect(oauth2client.Field("secret").String()).To(Equal("dexSecret"))
			Expect(oauth2client.Field("allowedGroups").String()).To(MatchJSON(`["everyone","admins"]`))

			ingress := hec.KubernetesResource("Ingress", "d8-test", "test-dex-authenticator")
			Expect(ingress.Exists()).To(BeTrue())
			Expect(ingress.Field("spec.ingressClassName").String()).To(Equal("test"))
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/proxy-buffer-size").String()).To(Equal("32k"))
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/whitelist-source-range").String()).To(Equal("1.1.1.1,192.168.0.0/24"))

			Expect(ingress.Field("spec.tls.0.hosts").String()).To(MatchJSON(`["authenticator.example.com"]`))
			Expect(ingress.Field("spec.tls.0.secretName").String()).To(Equal("test"))

			secret := hec.KubernetesResource("Secret", "d8-test", "dex-authenticator-test")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field("data.client-secret").String()).To(Equal("ZGV4U2VjcmV0"))
			Expect(secret.Field("data.cookie-secret").String()).To(Equal("Y29va2llU2VjcmV0"))

			deployment := hec.KubernetesResource("Deployment", "d8-test", "test-dex-authenticator")
			Expect(deployment.Exists()).To(BeTrue())
			Expect(deployment.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON(`{"testnode": ""}`))
			Expect(deployment.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: foo
  operator: Equal
  value: "bar"
`))

			var oauth2proxyArgs []string
			for _, result := range deployment.Field("spec.template.spec.containers.0.args").Array() {
				oauth2proxyArgs = append(oauth2proxyArgs, result.String())
			}

			Expect(oauth2proxyArgs).Should(ContainElement("--client-id=test-d8-test-dex-authenticator"))
			Expect(oauth2proxyArgs).Should(ContainElement("--oidc-issuer-url=https://dex.example.com/"))
			Expect(oauth2proxyArgs).Should(ContainElement("--redirect-url=https://authenticator.example.com"))
			Expect(oauth2proxyArgs).Should(ContainElement("--set-authorization-header=true"))
			Expect(oauth2proxyArgs).Should(ContainElement("--cookie-expire=1020h"))
			Expect(oauth2proxyArgs).Should(ContainElement("--cookie-refresh=20m"))
			Expect(oauth2proxyArgs).Should(ContainElement("--whitelist-domain=authenticator.example.com"))
		})
	})
})

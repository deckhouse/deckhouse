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

var _ = Describe("Module :: user-authn :: helm template :: publish api", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.15.6")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.ingressClass", "nginx")
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
		hec.ValuesSet("userAuthn.internal.selfSignedCA.cert", "test")
		hec.ValuesSet("userAuthn.internal.selfSignedCA.key", "test")

		hec.ValuesSet("userAuthn.internal.publishAPI.enabled", true)
		hec.ValuesSet("userAuthn.internal.publishAPI.addKubeconfigGeneratorEntry", true)
	})

	Context("By default", func() {
		BeforeEach(func() {
			hec.HelmRender()
		})
		It("Should deploy publish api and kubeconfig generator", func() {
			Expect(hec.KubernetesResource("Deployment", "d8-user-authn", "kubeconfig-generator").Exists()).To(BeTrue())
		})
	})

	Context("With LDAP provider with enableBasicAuth option", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.internal.basicAuthProxyCert", "dGVzdA==")
			hec.ValuesSet("userAuthn.internal.basicAuthProxyKey", "dGVzdA==")
			hec.ValuesSetFromYaml("userAuthn.internal.providers", `
- id: ldapID
  displayName: ldapDisplay
  type: LDAP
  ldap:
    enableBasicAuth: true
    host: ldap.example.com:636
    userSearch:
      baseDN: cn=users,dc=example,dc=com
      username: uid
      idAttr: uid
      emailAttr: mail
    groupSearch:
      baseDN: cn=groups,dc=example,dc=com
      userMatchers:
      - userAttr: uid
        groupAttr: member
      nameAttr: name
`)
			hec.HelmRender()
		})

		It("Should deploy basic auth proxy deployment and ingress for LDAP", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			Expect(hec.KubernetesResource("Deployment", "d8-user-authn", "basic-auth-proxy").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Ingress", "d8-user-authn", "basic-auth-proxy").Exists()).To(BeTrue())
		})
	})

	Context("With provider with enableBasicAuth option", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.internal.basicAuthProxyCert", "dGVzdA==")
			hec.ValuesSet("userAuthn.internal.basicAuthProxyKey", "dGVzdA==")
			hec.ValuesSet("userAuthn.internal.publishAPI.ingressClass", "internal")
			hec.ValuesSetFromYaml("userAuthn.internal.providers", `
- id: crowdNexID
  displayName: crowdNextName
  type: Crowd
  crowd:
    enableBasicAuth: true
    clientID: clientID
    clientSecret: secret
    baseURL: https://example.com`)
			hec.ValuesSetFromYaml("userAuthn.internal.publishAPI.whitelistSourceRanges", `
- 1.1.1.1
- 192.168.0.0/24
`)
			hec.HelmRender()
		})
		It("Should deploy basic auth proxy deployment and ingress", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			Expect(hec.KubernetesResource("Deployment", "d8-user-authn", "basic-auth-proxy").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Ingress", "d8-user-authn", "basic-auth-proxy").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Ingress", "d8-user-authn", "basic-auth-proxy").Field(
				"metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/whitelist-source-range").String()).To(
				Equal("1.1.1.1,192.168.0.0/24"))
			Expect(hec.KubernetesResource("Ingress", "d8-user-authn", "basic-auth-proxy").Field(
				"spec.ingressClassName").String()).To(Equal("internal"))

			Expect(hec.KubernetesResource("Deployment", "d8-user-authn", "kubeconfig-generator").Exists()).To(BeTrue())

		})
	})
})

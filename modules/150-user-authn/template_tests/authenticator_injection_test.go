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

// hostileSignOutURL is a sign-out path followed by the injected document, see
// injectedClusterRoleBinding in client_injection_test.go.
const hostileSignOutURL = "/logout" + injectedClusterRoleBinding

const dexAuthenticatorCRDs = `
- name: tenant
  encodedName: "m5zgcztbnzq4x4u44scceizf"
  namespace: tenant-ns
  credentials:
    appDexSecret: appDexSecretValue
    cookieSecret: cookieSecretValue
  spec:
    applications:
    - domain: tenant.example.com
      ingressClassName: nginx
      ingressSecretName: tenant-tls
`

const dexAuthenticatorNames = `
"tenant@tenant-ns":
  name: "tenant-dex-authenticator"
  truncated: false
  hash: ""
  secretName: "dex-authenticator-tenant"
  secretTruncated: false
  secretHash: ""
  ingressNames:
    "0":
      name: "tenant-dex-authenticator"
      truncated: false
      hash: ""
  signOutIngressNames:
    "0":
      name: "tenant-dex-authenticator-sign-out"
      truncated: false
      hash: ""
`

var _ = Describe("Module :: user-authn :: helm template :: DexAuthenticator value injection", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.29.1")
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
		hec.ValuesSet("userAuthn.internal.dexTLS.ca", "plainstring")

		hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs", dexAuthenticatorCRDs)
		hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorNames", dexAuthenticatorNames)
	})

	for _, field := range []string{"allowedEmails", "allowedGroups"} {
		Context("With a DexAuthenticator carrying a YAML document in "+field, func() {
			var rendered map[string]string

			BeforeEach(func() {
				hec.ValuesSet("userAuthn.internal.dexAuthenticatorCRDs.0.spec."+field, []string{hostileValue})

				rendered = map[string]string{}
				hec.HelmRender(WithFilteredRenderOutput(rendered, []string{"dex-authenticator/oauth2client.yaml"}))
			})

			It("Should not add an object to the release", func() {
				Expect(hec.RenderError).ToNot(HaveOccurred())

				Expect(hec.KubernetesGlobalResource("ClusterRoleBinding", injectedClusterRoleBindingName).Exists()).
					To(BeFalse(), "the value must not become a manifest of its own")

				for path, manifests := range rendered {
					Expect(renderedKinds(manifests)).To(Equal([]string{"OAuth2Client"}),
						"%s must render exactly one OAuth2Client object", path)
				}
			})

			It("Should keep the value a single scalar of the OAuth2Client", func() {
				Expect(hec.RenderError).ToNot(HaveOccurred())

				client := hec.KubernetesResource("OAuth2Client", "d8-user-authn", "m5zgcztbnzq4x4u44scceizf")
				Expect(client.Exists()).To(BeTrue())
				Expect(client.Field(field).AsStringSlice()).To(Equal([]string{hostileValue}))
				Expect(client.Field("secret").String()).To(Equal("appDexSecretValue"))
				Expect(client.Field("redirectURIs").AsStringSlice()).
					To(Equal([]string{"https://tenant.example.com/dex-authenticator/callback"}))
			})
		})
	}

	Context("With a DexAuthenticator carrying a YAML document in an application signOutURL", func() {
		var rendered map[string]string

		BeforeEach(func() {
			hec.ValuesSet("userAuthn.internal.dexAuthenticatorCRDs.0.spec.applications.0.signOutURL", hostileSignOutURL)

			rendered = map[string]string{}
			hec.HelmRender(WithFilteredRenderOutput(rendered, []string{"dex-authenticator/ingress.yaml"}))
		})

		It("Should not add an object to the release", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			Expect(hec.KubernetesGlobalResource("ClusterRoleBinding", injectedClusterRoleBindingName).Exists()).
				To(BeFalse(), "the value must not become a manifest of its own")

			for path, manifests := range rendered {
				Expect(renderedKinds(manifests)).To(Equal([]string{"Ingress", "Ingress"}),
					"%s must render exactly the application and the sign-out Ingress", path)
			}
		})

		It("Should keep the value a single path of the sign-out Ingress", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			ingress := hec.KubernetesResource("Ingress", "tenant-ns", "tenant-dex-authenticator-sign-out")
			Expect(ingress.Exists()).To(BeTrue())
			Expect(ingress.Field("spec.rules.0.http.paths.0.path").String()).To(Equal(hostileSignOutURL))
			Expect(ingress.Field("spec.rules.0.host").String()).To(Equal("tenant.example.com"))

			application := hec.KubernetesResource("Ingress", "tenant-ns", "tenant-dex-authenticator")
			Expect(application.Exists()).To(BeTrue())
			Expect(application.Field("spec.rules.0.http.paths.0.path").String()).To(Equal("/dex-authenticator"))
		})
	})
})

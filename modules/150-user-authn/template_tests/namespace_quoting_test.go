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
	"github.com/tidwall/gjson"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// A namespace may be named so that YAML reads the name as a number or a boolean; the objects the
// module renders into the namespace of a DexAuthenticator or a DexClient must keep it a string.
var _ = Describe("Module :: user-authn :: helm template :: namespace quoting", func() {
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
	})

	for _, ns := range []string{"123", "true"} {
		Context("With a DexAuthenticator and a DexClient in the namespace "+ns, func() {
			BeforeEach(func() {
				hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs", `
- name: web
  encodedName: justForTest
  namespace: "`+ns+`"
  credentials:
    appDexSecret: dexSecret
    cookieSecret: cookieSecret
  spec:
    applications:
    - domain: web.example.com
      ingressClassName: nginx
      ingressSecretName: web-tls
      signOutURL: /logout
`)
				hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorNames", `
"web@`+ns+`":
  name: web-dex-authenticator
  secretName: dex-authenticator-web
  ingressNames:
    "0":
      name: web-dex-authenticator
  signOutIngressNames:
    "0":
      name: web-dex-authenticator-sign-out
`)
				hec.ValuesSetFromYaml("userAuthn.internal.dexClientCRDs", `
- id: dex-client-app@`+ns+`
  encodedID: "m5zgcztbnzq4x4u44scceizf"
  name: app
  namespace: "`+ns+`"
  spec:
    redirectURIs:
    - https://app.example.com/callback
  legacyID: dex-client-app:`+ns+`
  legacyEncodedID: "m5zgcztbnzq4x4u44scceizfxxx"
  clientSecret: clientSecretValue
`)
				hec.HelmRender()
			})

			It("Should render the HTTPRoute with the namespace as a string", func() {
				hec.ValuesSet("global.discovery.gatewayAPIDefaultGateway.name", "shared-gateway")
				hec.ValuesSet("global.discovery.gatewayAPIDefaultGateway.namespace", "d8-alb")
				hec.ValuesSet("userAuthn.internal.dexAuthenticatorCRDs.0.spec.applications.0.gatewayAPI.httpRouteListenerSetName", "default-listener-set")
				hec.HelmRender(WithAPIVersions(httpRouteAPIVersion))
				Expect(hec.RenderError).ToNot(HaveOccurred())

				route := hec.KubernetesResource("HTTPRoute", ns, "web-dex-authenticator")
				Expect(route.Exists()).To(BeTrue())
				Expect(route.Field("metadata.namespace").Type).To(Equal(gjson.String))
				Expect(route.Field("metadata.namespace").String()).To(Equal(ns))
			})

			It("Should render every namespaced object with the namespace as a string", func() {
				Expect(hec.RenderError).ToNot(HaveOccurred())

				for _, object := range [][2]string{
					{"Service", "web-dex-authenticator"},
					{"Deployment", "web-dex-authenticator"},
					{"PodDisruptionBudget", "web-dex-authenticator"},
					{"VerticalPodAutoscaler", "web-dex-authenticator"},
					{"Ingress", "web-dex-authenticator"},
					{"Ingress", "web-dex-authenticator-sign-out"},
					{"Secret", "dex-authenticator-web"},
					{"Secret", "registry-dex-authenticator"},
					{"Secret", "dex-client-app"},
				} {
					resource := hec.KubernetesResource(object[0], ns, object[1])
					Expect(resource.Exists()).To(BeTrue(), "%s %s/%s", object[0], ns, object[1])
					Expect(resource.Field("metadata.namespace").Type).To(Equal(gjson.String), "%s %s", object[0], object[1])
					Expect(resource.Field("metadata.namespace").String()).To(Equal(ns), "%s %s", object[0], object[1])
				}
			})
		})
	}
})

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

// httpRouteAPIVersion makes helm_lib_kind_exists report the Gateway API as installed. Without it
// the template renders nothing at all, so every assertion below would pass vacuously.
const httpRouteAPIVersion = "gateway.networking.k8s.io/v1/HTTPRoute"

var _ = Describe("Module :: user-authn :: helm template :: DexAuthenticator HTTPRoute value injection", func() {
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
		hec.ValuesSet("global.discovery.gatewayAPIDefaultGateway.name", "shared-gateway")
		hec.ValuesSet("global.discovery.gatewayAPIDefaultGateway.namespace", "d8-alb")

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.ca", "plainstring")

		hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs", dexAuthenticatorCRDs)
		hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorNames", dexAuthenticatorNames)
		hec.ValuesSet("userAuthn.internal.dexAuthenticatorCRDs.0.spec.applications.0.gatewayAPI.httpRouteListenerSetName", "default-listener-set")
	})

	Context("With the Gateway API absent from the cluster", func() {
		BeforeEach(func() {
			hec.HelmRender()
		})

		It("Should not render the HTTPRoute at all", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			Expect(hec.KubernetesResource("HTTPRoute", "tenant-ns", "tenant-dex-authenticator").Exists()).To(BeFalse())
		})
	})

	Context("With a DexAuthenticator carrying a YAML document in an application signOutURL", func() {
		var rendered map[string]string

		BeforeEach(func() {
			hec.ValuesSet("userAuthn.internal.dexAuthenticatorCRDs.0.spec.applications.0.signOutURL", hostileSignOutURL)

			rendered = map[string]string{}
			hec.HelmRender(
				WithFilteredRenderOutput(rendered, []string{"dex-authenticator/httproute.yaml"}),
				WithAPIVersions(httpRouteAPIVersion),
			)
		})

		It("Should not add an object to the release", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			Expect(hec.KubernetesGlobalResource("ClusterRoleBinding", injectedClusterRoleBindingName).Exists()).
				To(BeFalse(), "the value must not become a manifest of its own")

			Expect(rendered).ToNot(BeEmpty(), "the template must render, otherwise nothing is under test")
			for path, manifests := range rendered {
				Expect(renderedKinds(manifests)).To(Equal([]string{"HTTPRoute"}),
					"%s must render exactly one HTTPRoute object", path)
			}
		})

		It("Should keep the value a single path match of the sign-out rule", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			route := hec.KubernetesResource("HTTPRoute", "tenant-ns", "tenant-dex-authenticator")
			Expect(route.Exists()).To(BeTrue())

			signOut := route.Field("spec.rules.1")
			Expect(signOut.Get("matches.0.path.type").String()).To(Equal("PathPrefix"))
			Expect(signOut.Get("matches.0.path.value").String()).To(Equal(hostileSignOutURL))
			Expect(signOut.Get("filters.0.urlRewrite.path.replaceFullPath").String()).
				To(Equal("/dex-authenticator/sign_out"))
		})

		It("Should keep the rest of the HTTPRoute intact", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			route := hec.KubernetesResource("HTTPRoute", "tenant-ns", "tenant-dex-authenticator")
			Expect(route.Field("apiVersion").String()).To(Equal("gateway.networking.k8s.io/v1"))
			Expect(route.Field("spec.hostnames").AsStringSlice()).To(Equal([]string{"tenant.example.com"}))
			Expect(route.Field("spec.parentRefs.0.kind").String()).To(Equal("ListenerSet"))
			Expect(route.Field("spec.parentRefs.0.name").String()).To(Equal("default-listener-set"))
			Expect(route.Field("spec.parentRefs.0.namespace").String()).To(Equal("tenant-ns"))

			application := route.Field("spec.rules.0")
			Expect(application.Get("matches.0.path.value").String()).To(Equal("/dex-authenticator"))
			Expect(application.Get("backendRefs.0.name").String()).To(Equal("tenant-dex-authenticator"))
			Expect(application.Get("backendRefs.0.port").Int()).To(Equal(int64(443)))
		})
	})
})

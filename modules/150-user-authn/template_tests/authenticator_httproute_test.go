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

var _ = Describe("Module :: user-authn :: helm template :: DexAuthenticator HTTPRoute", func() {
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

	// The v1 → v2alpha1 conversion emits a gatewayAPI object full of nulls for every Ingress-only
	// application, and the hook reads v2alpha1. Rendering it would produce an HTTPRoute with an empty
	// parentRefs name, which the API server rejects.
	Context("With an application whose gatewayAPI carries only null fields", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs.0.spec.applications.0.gatewayAPI", `
httpRouteListenerSetName: null
httpRouteListenerSetNamespace: null
httpRouteListenerSetSectionName: null
`)
			hec.HelmRender(WithAPIVersions(httpRouteAPIVersion))
		})

		It("Should not render the HTTPRoute", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			Expect(hec.KubernetesResource("HTTPRoute", "tenant-ns", "tenant-dex-authenticator").Exists()).To(BeFalse())
		})

		It("Should keep the Ingress", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			Expect(hec.KubernetesResource("Ingress", "tenant-ns", "tenant-dex-authenticator").Exists()).To(BeTrue())
		})
	})

	Context("With an application whose gatewayAPI names a ListenerSet", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs.0.spec.applications.0.gatewayAPI", `
httpRouteListenerSetName: app-listeners
httpRouteListenerSetNamespace: null
httpRouteListenerSetSectionName: null
`)
			hec.HelmRender(WithAPIVersions(httpRouteAPIVersion))
		})

		It("Should render the HTTPRoute attached to that ListenerSet", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			route := hec.KubernetesResource("HTTPRoute", "tenant-ns", "tenant-dex-authenticator")
			Expect(route.Exists()).To(BeTrue())
			Expect(route.Field("spec.parentRefs.0.name").String()).To(Equal("app-listeners"))
			Expect(route.Field("spec.parentRefs.0.namespace").String()).To(Equal("tenant-ns"))
			Expect(route.Field("spec.parentRefs.0.sectionName").Exists()).To(BeFalse())
		})
	})
})

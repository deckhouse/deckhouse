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
	"crypto/sha256"
	"fmt"

	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: user-authn :: helm template :: DexAuthenticator routing", func() {
	hec := SetupHelmConfig("")
	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.29.1")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler", "operator-prometheus-crd"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")
		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.ca", "plainstring")
		hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs", `
- name: routing
  encodedName: routing
  namespace: tenant-ns
  credentials:
    appDexSecret: appSecret
    cookieSecret: cookieSecret
  spec:
    applications:
    - domain: app.example.com
      signOutURL: /logout
`)
		hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorNames", `
"routing@tenant-ns":
  name: routing-dex-authenticator
  secretName: dex-authenticator-routing
  ingressNames:
    "0":
      name: routing-dex-authenticator
    "1": {}
  signOutIngressNames:
    "0":
      name: routing-dex-authenticator-sign-out
    "1": {}
`)
	})

	for _, tc := range []struct {
		name                         string
		ingress, gateway, apiPresent bool
	}{
		{"Ingress only", true, false, true},
		{"Gateway API only", false, true, true},
		{"Ingress and Gateway API", true, true, true},
		{"Gateway API unavailable", false, true, false},
	} {
		tc := tc
		It(tc.name, func() {
			hec.ValuesSet("global.discovery.gatewayAPIDefaultGateway.name", "shared-gateway")
			hec.ValuesSet("global.discovery.gatewayAPIDefaultGateway.namespace", "d8-alb")

			if tc.ingress {
				hec.ValuesSet("userAuthn.internal.dexAuthenticatorCRDs.0.spec.applications.0.ingressClassName", "nginx")
			}
			if tc.gateway {
				hec.ValuesSet("userAuthn.internal.dexAuthenticatorCRDs.0.spec.applications.0.gatewayAPI.httpRouteListenerSetName", "app-listeners")
			}
			if tc.apiPresent {
				hec.HelmRender(WithAPIVersions("gateway.networking.k8s.io/v1/HTTPRoute"))
			} else {
				hec.HelmRender()
			}
			Expect(hec.RenderError).NotTo(HaveOccurred())
			ingress := hec.KubernetesResource("Ingress", "tenant-ns", "routing-dex-authenticator")
			Expect(ingress.Exists()).To(Equal(tc.ingress))
			Expect(hec.KubernetesResource("Ingress", "tenant-ns", "routing-dex-authenticator-sign-out").Exists()).To(Equal(tc.ingress))
			if tc.ingress {
				Expect(ingress.Field("spec.ingressClassName").String()).To(Equal("nginx"))
			}
			route := hec.KubernetesResource("HTTPRoute", "tenant-ns", "routing-dex-authenticator")
			Expect(route.Exists()).To(Equal(tc.gateway && tc.apiPresent))
			if tc.gateway && tc.apiPresent {
				Expect(route.Field("spec.parentRefs.0.name").String()).To(Equal("app-listeners"))
				Expect(route.Field("spec.rules.0.matches.0.path.value").String()).To(Equal("/dex-authenticator"))
				Expect(route.Field("spec.rules.1.matches.0.path.value").String()).To(Equal("/logout"))
				Expect(route.Field("spec.rules.1.filters.0.urlRewrite.path.replaceFullPath").String()).To(Equal("/dex-authenticator/sign_out"))
				Expect(route.Field("spec.rules.0.backendRefs.0.name").String()).To(Equal("routing-dex-authenticator"))
			}
		})
	}

	for _, tc := range []struct {
		name                        string
		global                      *bool
		apiPresent, ingress, routed bool
	}{
		{name: "explicit ListenerSet without default Gateway", apiPresent: true, routed: true},
		{name: "global Gateway API enabled", global: ptr.To(true), apiPresent: true, routed: true},
		{name: "HTTPRoute API unavailable"},
		{name: "global Gateway API disabled", global: ptr.To(false), apiPresent: true},
		{name: "Ingress remains available without HTTPRoute API", ingress: true},
		{name: "Ingress remains available when Gateway API is disabled", ingress: true, global: ptr.To(false), apiPresent: true},
	} {
		tc := tc
		for _, additional := range []bool{false, true} {
			additional := additional
			It(fmt.Sprintf("%s, additional=%v", tc.name, additional), func() {
				if tc.global != nil {
					hec.ValuesSet("global.modules.gatewayAPI.enabled", *tc.global)
				}
				idx := 0
				routeName := "routing-dex-authenticator"
				if additional {
					idx = 1
					hec.ValuesSet("userAuthn.internal.dexAuthenticatorCRDs.0.spec.applications.0.domain", "first.example.com")
					hec.ValuesSet("userAuthn.internal.dexAuthenticatorCRDs.0.spec.applications.0.ingressClassName", "nginx")
					hash := fmt.Sprintf("%x", sha256.Sum256([]byte("app.example.com")))[:8]
					routeName = "routing-" + hash + "-dex-authenticator"
				}
				path := fmt.Sprintf("userAuthn.internal.dexAuthenticatorCRDs.0.spec.applications.%d", idx)
				hec.ValuesSet(path+".domain", "app.example.com")
				hec.ValuesSet(path+".gatewayAPI.httpRouteListenerSetName", "app-listeners")
				if tc.ingress {
					hec.ValuesSet(path+".ingressClassName", "nginx")
				}
				if tc.apiPresent {
					hec.HelmRender(WithAPIVersions("gateway.networking.k8s.io/v1/HTTPRoute"))
				} else {
					hec.HelmRender()
				}
				Expect(hec.RenderError).NotTo(HaveOccurred())
				route := hec.KubernetesResource("HTTPRoute", "tenant-ns", routeName)
				Expect(route.Exists()).To(Equal(tc.routed))
				if route.Exists() {
					Expect(route.Field("spec.parentRefs.0.name").String()).To(Equal("app-listeners"))
					Expect(route.Field("spec.parentRefs.0.namespace").String()).To(Equal("tenant-ns"))
				}
				Expect(hec.KubernetesResource("Ingress", "tenant-ns", routeName).Exists()).To(Equal(tc.ingress))
				if !tc.routed {
					// Restoring Gateway API creates the route without touching the resource.
					hec.ValuesSet("global.modules.gatewayAPI.enabled", true)
					hec.HelmRender(WithAPIVersions("gateway.networking.k8s.io/v1/HTTPRoute"))
					Expect(hec.RenderError).NotTo(HaveOccurred())
					Expect(hec.KubernetesResource("HTTPRoute", "tenant-ns", routeName).Exists()).To(BeTrue())
				}
			})
		}
	}

	// Authenticator routes carry an explicit ListenerSet in parentRefs, so they render regardless of
	// whether Dex itself has a publication path — that is exactly what makes a cluster without any
	// IngressClass workable.
	for _, tc := range []struct {
		name                                               string
		ingress, gateway, disabled, httpRoute, listenerSet bool
	}{
		{"no Gateway", false, false, false, true, true},
		{"Gateway API disabled", false, true, true, true, true},
		{"no HTTPRoute API", false, true, false, false, true},
		{"no ListenerSet API", false, true, false, true, false},
		{"Dex published through Gateway", false, true, false, true, true},
		{"Dex published through Ingress", true, false, true, false, false},
	} {
		tc := tc
		It("Checks public Dex publication: "+tc.name, func() {
			hec.ValuesSet("global.modules.ingress.enabled", tc.ingress)
			hec.ValuesSet("global.modules.gatewayAPI.enabled", !tc.disabled)
			if tc.gateway {
				hec.ValuesSet("global.discovery.gatewayAPIDefaultGateway.name", "shared-gateway")
				hec.ValuesSet("global.discovery.gatewayAPIDefaultGateway.namespace", "d8-alb")
			}
			hec.ValuesSet("userAuthn.internal.dexAuthenticatorCRDs.0.spec.applications.0.gatewayAPI.httpRouteListenerSetName", "app-listeners")
			hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs.0.spec.applications.1", `
domain: second.example.com
gatewayAPI:
  httpRouteListenerSetName: app-listeners
`)
			apiVersions := []string{}
			if tc.httpRoute {
				apiVersions = append(apiVersions, "gateway.networking.k8s.io/v1/HTTPRoute")
			}
			if tc.listenerSet {
				apiVersions = append(apiVersions, "gateway.networking.k8s.io/v1/ListenerSet")
			}
			hec.HelmRender(WithAPIVersions(apiVersions...))
			Expect(hec.RenderError).NotTo(HaveOccurred())
			Expect(hec.KubernetesResource("HTTPRoute", "tenant-ns", "routing-dex-authenticator").Exists()).To(Equal(!tc.disabled && tc.httpRoute))
			Expect(hec.KubernetesResource("Ingress", "d8-user-authn", "dex").Exists()).To(Equal(tc.ingress))
			Expect(hec.KubernetesResource("ListenerSet", "d8-user-authn", "user-authn").Exists()).To(Equal(!tc.disabled && tc.gateway && tc.listenerSet))
			Expect(hec.KubernetesResource("HTTPRoute", "d8-user-authn", "dex").Exists()).To(Equal(!tc.disabled && tc.gateway && tc.httpRoute))
		})
	}

	// Converters before this change emitted a gatewayAPI object full of nulls for every Ingress-only
	// application. Rendering it would produce an HTTPRoute with an empty parentRefs name.
	It("Skips an application whose gatewayAPI carries no ListenerSet name", func() {
		hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs.0.spec.applications.0", `
domain: app.example.com
ingressClassName: nginx
gatewayAPI:
  httpRouteListenerSetName: null
  httpRouteListenerSetNamespace: null
  httpRouteListenerSetSectionName: null
`)
		hec.HelmRender(WithAPIVersions("gateway.networking.k8s.io/v1/HTTPRoute"))
		Expect(hec.RenderError).NotTo(HaveOccurred())
		Expect(hec.KubernetesResource("HTTPRoute", "tenant-ns", "routing-dex-authenticator").Exists()).To(BeFalse())
		Expect(hec.KubernetesResource("Ingress", "tenant-ns", "routing-dex-authenticator").Exists()).To(BeTrue())
	})

	It("Keeps the Ingress of another application when the first uses only Gateway API", func() {
		hec.ValuesSet("userAuthn.internal.dexAuthenticatorCRDs.0.spec.applications.0.gatewayAPI.httpRouteListenerSetName", "app-listeners")
		hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs.0.spec.applications.1", `
domain: second.example.com
ingressClassName: nginx
signOutURL: /logout
`)
		hec.HelmRender(WithAPIVersions("gateway.networking.k8s.io/v1/HTTPRoute"))
		Expect(hec.RenderError).NotTo(HaveOccurred())
		Expect(hec.KubernetesResource("Ingress", "tenant-ns", "routing-dex-authenticator").Exists()).To(BeFalse())
		Expect(hec.KubernetesResource("HTTPRoute", "tenant-ns", "routing-dex-authenticator").Exists()).To(BeTrue())
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte("second.example.com")))[:8]
		name := "routing-" + hash + "-dex-authenticator"
		Expect(hec.KubernetesResource("Ingress", "tenant-ns", name).Exists()).To(BeTrue())
		Expect(hec.KubernetesResource("Ingress", "tenant-ns", name+"-sign-out").Exists()).To(BeTrue())
		Expect(hec.KubernetesResource("HTTPRoute", "tenant-ns", name).Exists()).To(BeFalse())
	})
})

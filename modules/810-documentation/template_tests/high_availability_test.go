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

var _ = Describe("Module :: documentation :: helm template :: replicas", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSet("global.clusterIsBootstrapped", true)
		f.ValuesSet("global.discovery.ingressNginxAnnotationsVersion", "1.3.0")
		f.ValuesSet("global.discovery.gatewayAPIDefaultGateway.name", "shared-gateway")
		f.ValuesSet("global.discovery.gatewayAPIDefaultGateway.namespace", "d8-alb")
		f.ValuesSetFromYaml("documentation", customCertificatePresent)
		f.ValuesSet("documentation.internal.auth.password", "plainstring")
	})

	render := func() {
		f.ApplyOpenAPIDefaults()
		f.HelmRender(WithAPIVersions("gateway.networking.k8s.io/v1/HTTPRoute", "gateway.networking.k8s.io/v1/ListenerSet"))
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}
	pod := func(path string) string {
		return f.KubernetesResource("Deployment", "d8-system", "documentation").Field(path).String()
	}
	replicas := func() int64 {
		return f.KubernetesResource("Deployment", "d8-system", "documentation").Field("spec.replicas").Int()
	}
	minAvailable := func() int64 {
		return f.KubernetesResource("PodDisruptionBudget", "d8-system", "documentation").Field("spec.minAvailable").Int()
	}
	pagesIngress := func(annotation string) string {
		return f.KubernetesResource("Ingress", "d8-system", "documentation").Field(`metadata.annotations.nginx\.ingress\.kubernetes\.io/` + annotation).String()
	}

	Context("Replicas", func() {
		It("Runs one replica without the HA mode", func() {
			render()

			Expect(replicas()).To(Equal(int64(1)))
			Expect(pod("spec.template.spec.affinity")).To(BeEmpty())
			Expect(minAvailable()).To(Equal(int64(0)))
		})

		It("Runs two replicas on different nodes, one at a time down, in the HA mode of the cluster", func() {
			f.ValuesSet("global.highAvailability", true)
			render()

			Expect(replicas()).To(Equal(int64(2)))
			Expect(pod("spec.strategy.rollingUpdate.maxSurge")).To(Equal("0"))
			Expect(pod("spec.strategy.rollingUpdate.maxUnavailable")).To(Equal("1"))
			term := "spec.template.spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution.0."
			Expect(pod(term + "labelSelector.matchLabels.app")).To(Equal("documentation"))
			Expect(pod(term + "topologyKey")).To(Equal("kubernetes.io/hostname"))
			// Scoped to the revision: a rollout on one suitable node does not wait for a second one.
			Expect(pod(term + "matchLabelKeys")).To(MatchJSON(`["pod-template-hash"]`))
			Expect(minAvailable()).To(Equal(int64(1)))
		})

		It("Follows the setting of the module over the cluster, the authenticator as well", func() {
			f.ValuesSet("documentation.internal.deployDexAuthenticator", true)
			authenticatorHA := func() bool {
				return f.KubernetesResource("DexAuthenticator", "d8-system", "documentation").Field("spec.highAvailability").Bool()
			}

			f.ValuesSet("global.highAvailability", true)
			f.ValuesSet("documentation.highAvailability", false)
			render()
			Expect(replicas()).To(Equal(int64(1)))
			Expect(authenticatorHA()).To(BeFalse())

			f.ValuesSet("global.highAvailability", false)
			f.ValuesSet("documentation.highAvailability", true)
			render()
			Expect(replicas()).To(Equal(int64(2)))
			Expect(authenticatorHA()).To(BeTrue())
		})
	})

	Context("A user stays on one replica", func() {
		It("Pins a user by a cookie on the Ingress of the pages", func() {
			render()

			Expect(pagesIngress("affinity")).To(Equal("cookie"))
			Expect(pagesIngress("affinity-mode")).To(Equal("persistent"))
			Expect(pagesIngress("session-cookie-name")).To(Equal("d8-documentation-affinity"))
			Expect(pagesIngress("session-cookie-path")).To(Equal("/"))
			Expect(pagesIngress("session-cookie-max-age")).To(Equal("86400"))
			Expect(pagesIngress("session-cookie-secure")).To(Equal("true"))

			// Icons and images are the same on every replica.
			assets := f.KubernetesResource("Ingress", "d8-system", "documentation-free-assets")
			Expect(assets.Field(`metadata.annotations.nginx\.ingress\.kubernetes\.io/affinity`).Exists()).To(BeFalse())
		})

		It("Keeps the cookie with an external authentication", func() {
			f.ValuesSet("documentation.auth.externalAuthentication", map[string]string{"authURL": "https://auth.example.com/auth", "authSignInURL": "https://auth.example.com/sign_in"})
			render()

			Expect(pagesIngress("auth-url")).To(Equal("https://auth.example.com/auth"))
			Expect(pagesIngress("affinity")).To(Equal("cookie"))
		})

		It("Keeps the cookie with an ingress-nginx older than 1.3", func() {
			f.ValuesSet("global.discovery.ingressNginxAnnotationsVersion", "1.2.0")
			render()

			Expect(pagesIngress("configuration-snippet")).NotTo(BeEmpty())
			Expect(pagesIngress("affinity")).To(Equal("cookie"))
		})

		It("Leaves the cookie insecure without https", func() {
			f.ValuesSet("global.modules.https.mode", "Disabled")
			f.ValuesSet("documentation.https.mode", "Disabled")
			render()

			Expect(pagesIngress("affinity")).To(Equal("cookie"))
			Expect(pagesIngress("session-cookie-secure")).To(BeEmpty())
		})

		It("Pins a user by the same cookie on the HTTPRoute of the pages", func() {
			render()

			route := f.KubernetesResource("HTTPRoute", "d8-system", "documentation")
			Expect(route.Field(`metadata.annotations.alb\.network\.deckhouse\.io/session-affinity`).String()).
				To(MatchJSON(`{"mode":"cookie","path":"/","cookieName":"d8-documentation-affinity","ttl":0}`))

			assets := f.KubernetesResource("HTTPRoute", "d8-system", "documentation-free-assets")
			Expect(assets.Exists()).To(BeTrue())
			Expect(assets.Field(`metadata.annotations.alb\.network\.deckhouse\.io/session-affinity`).Exists()).To(BeFalse())
		})
	})
})

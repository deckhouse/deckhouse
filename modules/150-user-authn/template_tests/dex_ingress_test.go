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
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// A browser sign-in spans several requests; while an old and a new Dex pod run side by side in a
// rollout they serve different paths, so both Dex Ingresses keep a sign-in on one pod.
var _ = Describe("Module :: user-authn :: helm template :: Dex Ingress", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.33.0")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
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
		hec.HelmRender()
	})

	// Dex trusts the client address in X-Real-IP only from a trusted proxy: ingress-nginx presents its
	// client certificate, the Gateway sends the trusted-proxy secret, and a pod calling the Dex Service
	// directly has neither.
	Context("With the trusted-proxy secret and the CA of the ingress-nginx client certificate", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.internal.dexTrustedProxySecret", "s3cr3tvalue")
			hec.ValuesSet("global.discovery.gatewayAPIDefaultGateway.name", "shared-gateway")
			hec.ValuesSet("global.discovery.gatewayAPIDefaultGateway.namespace", "d8-alb")
			hec.HelmRender(WithAPIVersions("gateway.networking.k8s.io/v1/HTTPRoute"))
		})

		It("Should have ingress-nginx present its client certificate and keep the secret out of the Ingress", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			for _, name := range []string{"dex", "dex-auth"} {
				annotations := hec.KubernetesResource("Ingress", "d8-user-authn", name).Field("metadata.annotations")
				snippet := annotations.Get(`nginx\.ingress\.kubernetes\.io/configuration-snippet`).String()
				Expect(snippet).To(ContainSubstring("proxy_ssl_certificate /etc/nginx/ssl/client.crt;"), name)
				Expect(snippet).To(ContainSubstring("proxy_ssl_certificate_key /etc/nginx/ssl/client.key;"), name)
				Expect(annotations.Raw).ToNot(ContainSubstring("s3cr3tvalue"), name)
				Expect(annotations.Get(`nginx\.ingress\.kubernetes\.io/proxy-ssl-use-controller-certificate`).Exists()).To(BeFalse(), name)
			}
		})

		It("Should use the annotation of ingress-nginx v1.3.0 and newer instead of the snippet", func() {
			hec.ValuesSet("global.discovery.ingressNginxAnnotationsVersion", "1.3.0")
			hec.HelmRender(WithAPIVersions("gateway.networking.k8s.io/v1/HTTPRoute"))
			Expect(hec.RenderError).ToNot(HaveOccurred())

			for _, name := range []string{"dex", "dex-auth"} {
				annotations := hec.KubernetesResource("Ingress", "d8-user-authn", name).Field("metadata.annotations")
				Expect(annotations.Get(`nginx\.ingress\.kubernetes\.io/proxy-ssl-use-controller-certificate`).String()).To(Equal("true"), name)
				Expect(annotations.Get(`nginx\.ingress\.kubernetes\.io/configuration-snippet`).Exists()).To(BeFalse(), name)
				Expect(annotations.Raw).ToNot(ContainSubstring("s3cr3tvalue"), name)
			}
		})

		It("Should pass the secret from the Gateway and configure Dex with the secret and the CA", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			route := hec.KubernetesResource("HTTPRoute", "d8-user-authn", "dex")
			Expect(route.Exists()).To(BeTrue())
			filter := route.Field("spec.rules.0.filters.0")
			Expect(filter.Get("type").String()).To(Equal("RequestHeaderModifier"))
			Expect(filter.Get("requestHeaderModifier.set.0.name").String()).To(Equal("X-Deckhouse-Trusted-Proxy"))
			Expect(filter.Get("requestHeaderModifier.set.0.value").String()).To(Equal("s3cr3tvalue"))
			Expect(route.Field("spec.rules.0.filters.0.requestHeaderModifier.remove").AsStringSlice()).To(Equal([]string{"X-Real-IP"}))

			secret := hec.KubernetesResource("Secret", "d8-user-authn", "dex")
			config, err := base64.StdEncoding.DecodeString(secret.Field(`data.config\.yaml`).String())
			Expect(err).ToNot(HaveOccurred())
			Expect(string(config)).To(ContainSubstring(`trustedProxySecret: "s3cr3tvalue"`))
			Expect(string(config)).To(ContainSubstring("trustedProxyCAFile: /etc/dex/config/trusted-proxy-ca.crt"))
			Expect(string(config)).To(ContainSubstring("trustedProxyCertOrganization: ingress-nginx:auth"))
			// The test harness sets the CA of kube-rbac-proxy to "test".
			Expect(secret.Field(`data.trusted-proxy-ca\.crt`).String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("test"))))

			items := hec.KubernetesResource("Deployment", "d8-user-authn", "dex").Field(`spec.template.spec.volumes.#(name=="original-config").secret.items.#.key`).String()
			Expect(items).To(ContainSubstring("trusted-proxy-ca.crt"))

			stored := hec.KubernetesResource("Secret", "d8-user-authn", "dex-trusted-proxy-secret")
			Expect(stored.Field("data.secret").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("s3cr3tvalue"))))
		})
	})

	It("Should keep a sign-in on one Dex pod", func() {
		Expect(hec.RenderError).ToNot(HaveOccurred())

		for _, name := range []string{"dex", "dex-auth"} {
			ingress := hec.KubernetesResource("Ingress", "d8-user-authn", name)
			Expect(ingress.Exists()).To(BeTrue(), name)
			annotations := ingress.Field("metadata.annotations").Map()
			Expect(annotations["nginx.ingress.kubernetes.io/affinity"].String()).To(Equal("cookie"), name)
			Expect(annotations["nginx.ingress.kubernetes.io/affinity-mode"].String()).To(Equal("persistent"), name)
			Expect(annotations["nginx.ingress.kubernetes.io/session-cookie-name"].String()).To(Equal("d8-dex-affinity"), name)
			Expect(annotations["nginx.ingress.kubernetes.io/session-cookie-secure"].String()).To(Equal("true"), name)
			Expect(annotations["nginx.ingress.kubernetes.io/session-cookie-path"].String()).To(Equal("/"), name)
		}
	})
})

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

// networkPolicyAuthenticators are two authenticators sharing a namespace, so a test can tell that each
// policy opens its own pods only, and a third one whose names the hook had to truncate. None has an
// ingressClassName: the policy does not depend on how requests are routed to the authenticator.
const networkPolicyAuthenticators = `
- name: first
  encodedName: first
  namespace: project-ns
  credentials:
    appDexSecret: dexSecret
    cookieSecret: cookieSecret
  spec:
    applications:
    - domain: first.example.com
- name: second
  encodedName: second
  namespace: project-ns
  credentials:
    appDexSecret: dexSecret
    cookieSecret: cookieSecret
  spec:
    applications:
    - domain: second.example.com
- name: very-long-authenticator-name-that-the-hook-had-to-truncate-for-kubernetes
  encodedName: long
  namespace: other-ns
  credentials:
    appDexSecret: dexSecret
    cookieSecret: cookieSecret
  spec:
    applications:
    - domain: long.example.com
`

const networkPolicyAuthenticatorNames = `
"very-long-authenticator-name-that-the-hook-had-to-truncate-for-kubernetes@other-ns":
  name: "very-long-authenticator-name-that-the-hook-ha-1a2b3c4d"
  truncated: true
  hash: "1a2b3c4d"
  secretName: "dex-authenticator-very-long-authenticator-name-that-t-1a2b3c4d"
  secretTruncated: true
  secretHash: "1a2b3c4d"
`

var _ = Describe("Module :: user-authn :: helm template :: dex authenticator network policy", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.33.1")
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

		hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs", networkPolicyAuthenticators)
		hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorNames", networkPolicyAuthenticatorNames)
		hec.HelmRender()
	})

	It("selects exactly the pods of its own authenticator", func() {
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		for _, da := range []struct{ namespace, name string }{
			{"project-ns", "first-dex-authenticator"},
			{"project-ns", "second-dex-authenticator"},
			{"other-ns", "very-long-authenticator-name-that-the-hook-ha-1a2b3c4d"},
		} {
			deployment := hec.KubernetesResource("Deployment", da.namespace, da.name)
			policy := hec.KubernetesResource("NetworkPolicy", da.namespace, da.name)
			Expect(deployment.Exists()).To(BeTrue(), da.name)
			Expect(policy.Exists()).To(BeTrue(), da.name)

			Expect(policy.Field("spec.podSelector.matchLabels").String()).
				To(MatchJSON(deployment.Field("spec.selector.matchLabels").String()), da.name)
		}

		first := hec.KubernetesResource("NetworkPolicy", "project-ns", "first-dex-authenticator")
		second := hec.KubernetesResource("NetworkPolicy", "project-ns", "second-dex-authenticator")
		Expect(first.Field("spec.podSelector.matchLabels.name").String()).To(Equal("first"))
		Expect(second.Field("spec.podSelector.matchLabels.name").String()).To(Equal("second"))

		long := hec.KubernetesResource("NetworkPolicy", "other-ns", "very-long-authenticator-name-that-the-hook-ha-1a2b3c4d")
		Expect(long.Field(`metadata.labels.deckhouse\.io/name-truncated`).String()).To(Equal("true"))
		Expect(long.Field(`metadata.labels.deckhouse\.io/name-hash`).String()).To(Equal("1a2b3c4d"))
	})

	// The ingress rule names no source: an ingress controller in hostNetwork arrives with the identity of
	// a node, and a gateway runs in a namespace of its own; a pod selector would match neither.
	It("lets anyone reach the authenticator port and nothing else", func() {
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		deployment := hec.KubernetesResource("Deployment", "project-ns", "first-dex-authenticator")
		authenticatorPort := deployment.Field(`spec.template.spec.containers.#(name=="dex-authenticator").ports.#(name=="dex-https").containerPort`)
		Expect(authenticatorPort.Int()).To(BeEquivalentTo(8443))

		policy := hec.KubernetesResource("NetworkPolicy", "project-ns", "first-dex-authenticator")
		Expect(policy.Field("spec.policyTypes").String()).To(MatchJSON(`["Ingress","Egress"]`))
		Expect(policy.Field("spec.ingress").String()).To(MatchJSON(`[{"ports":[{"protocol":"TCP","port":8443}]}]`))
	})

	// Redis is a sidecar on localhost, so dex, the revocation feed of user-authn-controller and DNS are all
	// the authenticator has to reach.
	It("lets the authenticator reach dex, the revocation feed and DNS, and nothing else", func() {
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		dex := hec.KubernetesResource("Deployment", "d8-user-authn", "dex")
		dexPort := dex.Field(`spec.template.spec.containers.#(name=="dex").ports.#(name=="https").containerPort`)
		Expect(dexPort.Int()).To(BeEquivalentTo(5556))

		// The feed rule names the port instead of numbering it: until the controller declares a port named
		// "feed" the rule matches nothing, and once it does the rule follows that port, which must be TCP.
		controller := hec.KubernetesResource("Deployment", "d8-user-authn", "user-authn-controller")
		Expect(controller.Field("spec.template.metadata.labels.app").String()).To(Equal("user-authn-controller"))
		for _, port := range controller.Field(`spec.template.spec.containers.#.ports.#(name=="feed")`).Array() {
			Expect(port.Get("protocol").String()).To(BeElementOf("", "TCP"))
		}

		// CoreDNS of the kube-dns module listens on 5353 behind its Service; 53 stays for other resolvers.
		policy := hec.KubernetesResource("NetworkPolicy", "project-ns", "first-dex-authenticator")
		Expect(policy.Field("spec.egress").String()).To(MatchJSON(`[
  {
    "to": [{
      "namespaceSelector": {"matchLabels": {"kubernetes.io/metadata.name": "d8-user-authn"}},
      "podSelector": {"matchLabels": {"app": "dex"}}
    }],
    "ports": [{"protocol": "TCP", "port": 5556}]
  },
  {
    "to": [{
      "namespaceSelector": {"matchLabels": {"kubernetes.io/metadata.name": "d8-user-authn"}},
      "podSelector": {"matchLabels": {"app": "user-authn-controller"}}
    }],
    "ports": [{"protocol": "TCP", "port": "feed"}]
  },
  {
    "to": [{"namespaceSelector": {"matchLabels": {"kubernetes.io/metadata.name": "kube-system"}}}],
    "ports": [
      {"protocol": "UDP", "port": 53},
      {"protocol": "TCP", "port": 53},
      {"protocol": "UDP", "port": 5353},
      {"protocol": "TCP", "port": 5353}
    ]
  }
]`))
	})

	// The policy makes the authenticator pods egress-isolated in every namespace, so an Istio sidecar injected
	// into them has to keep reaching istiod, or the pod never becomes ready.
	It("lets an Istio sidecar of the authenticator reach istiod when istio is enabled", func() {
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "istio"})
		hec.HelmRender()
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		policy := hec.KubernetesResource("NetworkPolicy", "project-ns", "first-dex-authenticator")
		Expect(policy.Field("spec.egress").Array()).To(HaveLen(4))
		Expect(policy.Field(`spec.egress.#(to.0.podSelector.matchLabels.app=="istiod")`).String()).To(MatchJSON(`{
  "to": [{
    "namespaceSelector": {"matchLabels": {"kubernetes.io/metadata.name": "d8-istio"}},
    "podSelector": {"matchLabels": {"app": "istiod"}}
  }],
  "ports": [{"protocol": "TCP", "port": 15012}]
}`))
	})

	// Ingress is isolated as well, so the ports Istio itself receives on stay open: HBONE of ambient mode for
	// mesh clients, and the merged metrics of a sidecar for Prometheus only.
	It("lets Istio reach the authenticator pods when istio is enabled", func() {
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "istio"})
		hec.HelmRender()
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		policy := hec.KubernetesResource("NetworkPolicy", "project-ns", "first-dex-authenticator")
		Expect(policy.Field("spec.ingress").String()).To(MatchJSON(`[
  {"ports": [{"protocol": "TCP", "port": 8443}, {"protocol": "TCP", "port": 15008}]},
  {
    "from": [{
      "namespaceSelector": {"matchLabels": {"kubernetes.io/metadata.name": "d8-monitoring"}},
      "podSelector": {"matchLabels": {"app.kubernetes.io/name": "prometheus"}}
    }],
    "ports": [{"protocol": "TCP", "port": 15020}]
  }
]`))
	})

	It("renders no policy without authenticators", func() {
		hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs", `[]`)
		hec.HelmRender()
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		Expect(hec.KubernetesResource("NetworkPolicy", "project-ns", "first-dex-authenticator").Exists()).To(BeFalse())
	})
})

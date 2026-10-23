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
	"github.com/tidwall/gjson"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// rulesFor returns the rules of a Role or ClusterRole that grant the resource.
func rulesFor(rules gjson.Result, resource string) []gjson.Result {
	var out []gjson.Result
	for _, rule := range rules.Array() {
		for _, r := range rule.Get("resources").Array() {
			if r.String() == resource {
				out = append(out, rule)
			}
		}
	}
	return out
}

func verbsOf(rule gjson.Result) []string {
	var verbs []string
	for _, v := range rule.Get("verbs").Array() {
		verbs = append(verbs, v.String())
	}
	return verbs
}

var _ = Describe("Module :: user-authn :: helm template :: dexclient-controller", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.29.1")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.ingressClass", "nginx")
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
		hec.ValuesSet("userAuthn.internal.selfSignedCA.cert", "test")
		hec.ValuesSet("userAuthn.internal.selfSignedCA.key", "test")
		hec.ValuesSet("userAuthn.internal.revocationFeedTLS.crt", "crt")
		hec.ValuesSet("userAuthn.internal.revocationFeedTLS.key", "key")
		hec.ValuesSet("userAuthn.internal.revocationFeedTLS.ca", "ca")
		hec.HelmRender()
	})

	It("Should run the DexClient reconciler alone, under its own ServiceAccount and without a network API", func() {
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		deployment := hec.KubernetesResource("Deployment", "d8-user-authn", "dexclient-controller")
		Expect(deployment.Exists()).To(BeTrue())
		spec := deployment.Field("spec.template.spec")
		Expect(spec.Get("serviceAccountName").String()).To(Equal("dexclient-controller"))

		container := spec.Get(`containers.#(name=="dexclient-controller")`)
		Expect(container.Exists()).To(BeTrue())
		Expect(container.Get("image").String()).To(Equal(
			hec.KubernetesResource("Deployment", "d8-user-authn", "user-authn-controller").Field("spec.template.spec.containers.0.image").String()))
		Expect(container.Get("args").String()).To(MatchJSON(`["dexclient"]`))
		Expect(container.Get("ports").String()).To(MatchJSON(`[
			{"containerPort": 9090, "name": "healthz", "protocol": "TCP"},
			{"containerPort": 9091, "name": "metrics", "protocol": "TCP"}
		]`))
		Expect(spec.Get("volumes").Exists()).To(BeFalse())

		Expect(hec.KubernetesResource("PodDisruptionBudget", "d8-user-authn", "dexclient-controller").Exists()).To(BeTrue())
		Expect(hec.KubernetesResource("VerticalPodAutoscaler", "d8-user-authn", "dexclient-controller").Exists()).To(BeTrue())
	})

	It("Should give the Secret rights to dexclient-controller only", func() {
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		dexClusterRole := hec.KubernetesGlobalResource("ClusterRole", "d8:user-authn:dexclient-controller")
		Expect(dexClusterRole.Exists()).To(BeTrue())
		secrets := rulesFor(dexClusterRole.Field("rules").Result, "secrets")
		Expect(secrets).To(HaveLen(1))
		Expect(verbsOf(secrets[0])).To(ConsistOf("get", "list", "watch", "create", "update", "delete"))
		Expect(rulesFor(dexClusterRole.Field("rules").Result, "dexclients")).To(HaveLen(1))
		Expect(hec.KubernetesGlobalResource("ClusterRoleBinding", "d8:user-authn:dexclient-controller").Field("subjects.0.name").String()).To(Equal("dexclient-controller"))

		dexRole := hec.KubernetesResource("Role", "d8-user-authn", "dexclient-controller")
		Expect(dexRole.Exists()).To(BeTrue())
		Expect(rulesFor(dexRole.Field("rules").Result, "oauth2clients")).To(HaveLen(1))
		Expect(rulesFor(dexRole.Field("rules").Result, "leases")).To(HaveLen(1))
		Expect(rulesFor(dexRole.Field("rules").Result, "secrets")).To(BeEmpty())
		Expect(hec.KubernetesResource("RoleBinding", "d8-user-authn", "dexclient-controller").Field("subjects.0.name").String()).To(Equal("dexclient-controller"))

		// user-authn-controller serves the revocation feed. It reads Secrets of d8-user-authn only (the
		// client secret of the kubernetes client, the Kerberos keytab of a DexProviderCheck) and writes
		// none.
		Expect(rulesFor(hec.KubernetesGlobalResource("ClusterRole", "d8:user-authn:controller").Field("rules").Result, "secrets")).To(BeEmpty())
		controllerSecrets := rulesFor(hec.KubernetesResource("Role", "d8-user-authn", "controller").Field("rules").Result, "secrets")
		Expect(controllerSecrets).To(HaveLen(1))
		Expect(verbsOf(controllerSecrets[0])).To(ConsistOf("get"))
		Expect(controllerSecrets[0].Get("resourceNames").Exists()).To(BeFalse())
	})
})

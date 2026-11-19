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

var _ = Describe("Module :: user-authn :: helm template :: cluster roles", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.15.6")
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

	Context("By default", func() {
		It("Should let the IAM manager create and read UserOperations", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			manager := hec.KubernetesGlobalResource("ClusterRole", "d8:system-capability:user-authn:edit")
			Expect(manager.Field("metadata.labels.rbac\\.deckhouse\\.io/aggregate-to-iam-as").String()).To(Equal("manager"))
			verbs := []string{}
			for _, rule := range manager.Field(`rules.#(resources.#(=="useroperations"))#`).Array() {
				Expect(rule.Get("apiGroups").String()).To(MatchJSON(`["deckhouse.io"]`))
				Expect(rule.Get("resourceNames").Exists()).To(BeFalse())
				for _, verb := range rule.Get("verbs").Array() {
					verbs = append(verbs, verb.String())
				}
			}
			Expect(verbs).To(ConsistOf("create", "get", "list", "watch"))

			viewer := hec.KubernetesGlobalResource("ClusterRole", "d8:system-capability:user-authn:view")
			Expect(viewer.Field("rules.#.resources").String()).ToNot(ContainSubstring("useroperations"))
		})

		// A collection read of the Dex storage CRDs returns whole objects, `hash` and
		// `previousHashes` included, so no verb on them is safe to hand to a subject that is
		// not expected to read credentials. Only the dex and user-api service accounts may.
		It("Should not grant user-facing roles access to the Dex credential storage CRDs", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			for _, name := range []string{
				"d8:system-capability:user-authn:view",
				"d8:system-capability:user-authn:edit",
				"d8:user-authz:user-authn:cluster-admin",
			} {
				clusterRole := hec.KubernetesGlobalResource("ClusterRole", name)
				Expect(clusterRole.Exists()).To(BeTrue(), name)
				Expect(clusterRole.Field("rules.#.apiGroups").String()).ToNot(ContainSubstring("dex.coreos.com"), name)
			}
		})

		// The password hash of a User, the new password hash of a UserOperation and the secrets of a
		// DexProvider are x-kubernetes-sensitive-data: without the sensitive subresource the API returns
		// <omitted> instead of them. The IAM manager and the cluster admin read them, the other roles do not.
		It("Should grant the sensitive fields of user-authn resources only to the manager and the cluster admin", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			for _, name := range []string{
				"d8:system-capability:user-authn:edit",
				"d8:user-authz:user-authn:cluster-admin",
			} {
				clusterRole := hec.KubernetesGlobalResource("ClusterRole", name)
				Expect(clusterRole.Exists()).To(BeTrue(), name)
				rules := clusterRole.Field(`rules.#(resources.#(%"*/sensitive"))#`).Array()
				Expect(rules).To(HaveLen(1), name)
				Expect(rules[0].Get("apiGroups").String()).To(MatchJSON(`["deckhouse.io"]`), name)
				Expect(rules[0].Get("resources").String()).To(MatchJSON(`["dexproviders/sensitive","users/sensitive","useroperations/sensitive"]`), name)
				Expect(rules[0].Get("verbs").String()).To(MatchJSON(`["get","list","watch"]`), name)
			}

			for _, name := range []string{
				"d8:system-capability:user-authn:view",
				"d8:user-authz:user-authn:editor",
				"d8:user-authz:user-authn:user",
			} {
				clusterRole := hec.KubernetesGlobalResource("ClusterRole", name)
				Expect(clusterRole.Exists()).To(BeTrue(), name)
				Expect(clusterRole.Field("rules.#.resources").String()).ToNot(ContainSubstring("/sensitive"), name)
			}
		})
	})
})

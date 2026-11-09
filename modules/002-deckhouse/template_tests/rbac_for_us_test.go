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

// rbac-for-us.yaml lets two node-manager components read the registry access that lives in d8-system:
// bashible-apiserver, which renders it into the node configuration, and registry-packages-proxy, which
// pulls packages with it. The ServiceAccount of Deckhouse in the same template is checked in
// module_test.go.

var _ = Describe("Module :: deckhouse :: registry access of node-manager components ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	// boundTo checks that the RoleBinding of the given name binds the Role of that name to the
	// ServiceAccount of that name in d8-cloud-instance-manager, the namespace of node-manager.
	boundTo := func(name, serviceAccount string) {
		binding := f.KubernetesResource("RoleBinding", "d8-system", name)
		Expect(binding.Exists()).To(BeTrue(), "the %s binding should be rendered", name)
		Expect(binding.Field("roleRef").String()).To(MatchJSON(`{"apiGroup":"rbac.authorization.k8s.io","kind":"Role","name":"` + name + `"}`))
		Expect(binding.Field("subjects").String()).To(MatchJSON(`[{"kind":"ServiceAccount","name":"` + serviceAccount + `","namespace":"d8-cloud-instance-manager"}]`))
	}

	It("lets bashible-apiserver read the secrets of d8-system", func() {
		Expect(roleGrants(f.KubernetesResource("Role", "d8-system", "bashible-apiserver-readregsecret"))).
			To(Equal(map[string][]string{"secrets": rbacReadVerbs}))
		boundTo("bashible-apiserver-readregsecret", "bashible-apiserver")
	})

	It("lets registry-packages-proxy read the registry access by name, and the registry configuration of nodes not at all", func() {
		grants := roleGrants(f.KubernetesResource("Role", "d8-system", "registry-packages-proxy"))
		Expect(grants).To(Equal(map[string][]string{
			"secrets:deckhouse-registry":        rbacReadVerbs,
			"secrets:registry-storage-client":   rbacReadVerbs,
			"configmaps:registry-image-address": rbacReadVerbs,
		}), "a resourceNames Role that misses an object fails silently: the watch returns nothing")
		Expect(grants).ToNot(HaveKey("secrets:registry-bashible-config"),
			"what nodes are told carries upstream credentials the proxy no longer needs")
		boundTo("registry-packages-proxy", "registry-packages-proxy")
	})
})

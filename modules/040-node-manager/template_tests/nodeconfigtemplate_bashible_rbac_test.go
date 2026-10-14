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
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: node-manager :: helm template :: NodeConfigTemplate rights of bashible", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerValues)
		setBashibleAPIServerTLSValues(f)
		f.HelmRender()
	})

	It("lets a node and a joining machine get a template, and nothing more", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		role := f.KubernetesGlobalResource("ClusterRole", "d8:node-manager:bashible:nodeconfigtemplates")
		Expect(role.Exists()).To(BeTrue())
		var rules []rbacv1.PolicyRule
		Expect(json.Unmarshal([]byte(role.Field("rules").String()), &rules)).To(Succeed())
		Expect(rules).To(Equal([]rbacv1.PolicyRule{{
			APIGroups: []string{"templates.internal.deckhouse.io"},
			Resources: []string{"nodeconfigtemplates"},
			Verbs:     []string{"get"},
		}}), "get on templates and nothing else")

		binding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:bashible:nodeconfigtemplates")
		Expect(binding.Exists()).To(BeTrue())
		Expect(binding.Field("roleRef.name").String()).To(Equal("d8:node-manager:bashible:nodeconfigtemplates"))
		var subjects []rbacv1.Subject
		Expect(json.Unmarshal([]byte(binding.Field("subjects").String()), &subjects)).To(Succeed())
		Expect(subjects).To(ConsistOf(
			rbacv1.Subject{Kind: "Group", APIGroup: "rbac.authorization.k8s.io", Name: "system:nodes"},
			rbacv1.Subject{Kind: "Group", APIGroup: "rbac.authorization.k8s.io", Name: "system:bootstrappers:d8-node-manager"},
		))
	})
})

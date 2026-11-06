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

// Every authenticated identity reads ApplicationPackageVersion through a binding to
// system:authenticated, so these tests keep the role to reading that one resource.

const applicationPackageVersionsReader = "d8:deckhouse:application-package-versions-reader"

var _ = Describe("Module :: deckhouse :: application package versions read ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	It("lets the role only read ApplicationPackageVersion", func() {
		role := f.KubernetesGlobalResource("ClusterRole", applicationPackageVersionsReader)
		Expect(role.Exists()).To(BeTrue())
		Expect(role.Field("rules").String()).To(MatchJSON(`[
			{"apiGroups":["deckhouse.io"],"resources":["applicationpackageversions"],"verbs":["get","list","watch"]}
		]`), "the role is bound to every authenticated identity")
	})

	It("binds the role to every authenticated identity", func() {
		binding := f.KubernetesGlobalResource("ClusterRoleBinding", applicationPackageVersionsReader+":system-authenticated")
		Expect(binding.Exists()).To(BeTrue())
		Expect(binding.Field("roleRef").String()).To(MatchJSON(`{"apiGroup":"rbac.authorization.k8s.io","kind":"ClusterRole","name":"` + applicationPackageVersionsReader + `"}`))
		Expect(binding.Field("subjects").String()).To(MatchJSON(`[{"kind":"Group","apiGroup":"rbac.authorization.k8s.io","name":"system:authenticated"}]`))
	})
})

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

var _ = Describe("Module :: deckhouse :: sudouser alias ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	It("makes system:sudouser a cluster-admin, and nobody else", func() {
		binding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8-sudouser")
		Expect(binding.Exists()).To(BeTrue())
		Expect(binding.Field("roleRef").String()).To(MatchJSON(`{"apiGroup":"rbac.authorization.k8s.io","kind":"ClusterRole","name":"cluster-admin"}`))
		Expect(binding.Field("subjects").String()).To(MatchJSON(`[{"apiGroup":"rbac.authorization.k8s.io","kind":"User","name":"system:sudouser"}]`),
			"the binding gives cluster-admin to whoever it names")
	})
})

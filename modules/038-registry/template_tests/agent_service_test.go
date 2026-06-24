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

const agentServiceModuleValues = `
cache:
  enabled: false
internal: {}
`

var _ = Describe("Module :: registry :: agent Service", func() {
	f := SetupHelmConfig(``)

	Context("phase New", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", agentServiceModuleValues)
			f.ValuesSet("registry.internal.takeover.phase", "New")
			f.HelmRender()
		})

		It("renders the agent Service with Local traffic policy", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			s := f.KubernetesResource("Service", "d8-system", "registry")
			Expect(s.Exists()).To(BeTrue())
			Expect(s.Field("spec.type").String()).To(Equal("ClusterIP"))
			Expect(s.Field("spec.internalTrafficPolicy").String()).To(Equal("Local"))
			Expect(s.Field("spec.selector.app").String()).To(Equal("registry-agent"))
			Expect(s.Field("spec.ports.0.port").Int()).To(BeEquivalentTo(5001))
		})
	})

	Context("phase Legacy", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", agentServiceModuleValues)
			f.ValuesSet("registry.internal.takeover.phase", "Legacy")
			f.HelmRender()
		})

		It("does not render the new agent Service", func() {
			// With no internal.orchestrator set, neither legacy nor new Service renders.
			Expect(f.KubernetesResource("Service", "d8-system", "registry").Exists()).To(BeFalse())
		})
	})
})

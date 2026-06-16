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

var _ = Describe("Module :: admissionPolicyEngine :: gatekeeper deployment scheduling", func() {
	f := SetupHelmConfig(`
admissionPolicyEngine:
  podSecurityStandards: {}
  internal:
    bootstrapped: true
    ratify:
      webhook:
        ca: test-ca-placeholder
        crt: test-crt-placeholder
        key: test-key-placeholder
    podSecurityStandards:
      enforcementActions:
        - deny
    trackedConstraintResources: []
    trackedMutateResources: []
    webhook:
      ca: test-ca-placeholder
      crt: test-crt-placeholder
      key: test-key-placeholder
`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
	})

	Context("When system nodes are available", func() {
		BeforeEach(func() {
			f.HelmRender()
		})

		It("must keep webhook Gatekeeper on system nodes", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			dp := f.KubernetesResource("Deployment", nsName, "gatekeeper-controller-manager")
			Expect(dp.Exists()).To(BeTrue())

			Expect(dp.Field("spec.template.spec.nodeSelector").Exists()).To(BeFalse())
			rendered := dp.ToYaml()
			Expect(rendered).To(ContainSubstring("requiredDuringSchedulingIgnoredDuringExecution"))
			Expect(rendered).To(ContainSubstring("preferredDuringSchedulingIgnoredDuringExecution"))
			Expect(rendered).To(ContainSubstring("key: node-role.deckhouse.io/system"))
			Expect(rendered).To(ContainSubstring("key: node-role.deckhouse.io/admission-policy-engine"))
			Expect(rendered).To(ContainSubstring("key: node-role.kubernetes.io/control-plane"))
			Expect(rendered).To(ContainSubstring("key: node-role.kubernetes.io/master"))
		})
	})

	Context("When system nodes are absent", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 0)
			f.HelmRender()
		})

		It("must fallback webhook Gatekeeper to control-plane nodes", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			dp := f.KubernetesResource("Deployment", nsName, "gatekeeper-controller-manager")
			Expect(dp.Exists()).To(BeTrue())

			Expect(dp.Field("spec.template.spec.nodeSelector").Exists()).To(BeFalse())
			rendered := dp.ToYaml()
			Expect(rendered).To(ContainSubstring("requiredDuringSchedulingIgnoredDuringExecution"))
			Expect(rendered).To(ContainSubstring("preferredDuringSchedulingIgnoredDuringExecution"))
			Expect(rendered).To(ContainSubstring("key: node-role.deckhouse.io/system"))
			Expect(rendered).To(ContainSubstring("key: node-role.deckhouse.io/admission-policy-engine"))
			Expect(rendered).To(ContainSubstring("key: node-role.kubernetes.io/control-plane"))
			Expect(rendered).To(ContainSubstring("key: node-role.kubernetes.io/master"))
			Expect(rendered).To(ContainSubstring("key: node-role.kubernetes.io/master"))
		})
	})

	Context("When system node counter is not set", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", `{}`)
			f.HelmRender()
		})

		It("must fallback webhook Gatekeeper to control-plane nodes", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			dp := f.KubernetesResource("Deployment", nsName, "gatekeeper-controller-manager")
			Expect(dp.Exists()).To(BeTrue())

			Expect(dp.Field("spec.template.spec.nodeSelector").Exists()).To(BeFalse())
			rendered := dp.ToYaml()
			Expect(rendered).To(ContainSubstring("requiredDuringSchedulingIgnoredDuringExecution"))
			Expect(rendered).To(ContainSubstring("preferredDuringSchedulingIgnoredDuringExecution"))
			Expect(rendered).To(ContainSubstring("key: node-role.deckhouse.io/system"))
			Expect(rendered).To(ContainSubstring("key: node-role.deckhouse.io/admission-policy-engine"))
			Expect(rendered).To(ContainSubstring("key: node-role.kubernetes.io/control-plane"))
		})
	})
})

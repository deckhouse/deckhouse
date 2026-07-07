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

var _ = Describe("Module :: admissionPolicyEngine :: admission VAP", func() {
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
		f.HelmRender()
	})

	It("renders VAP protecting Deckhouse finalizers", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		vap := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", "deny-deckhouse-finalizers.deckhouse.io")
		Expect(vap.Exists()).To(BeTrue())
		Expect(vap.Field("spec.matchConstraints.resourceRules.0.operations").String()).To(MatchJSON(`["UPDATE"]`))
		Expect(vap.Field("spec.validations.0.message").String()).To(
			Equal("Removing Deckhouse finalizers (containing '.deckhouse.io/') is forbidden"),
		)

		vapBinding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", "deny-deckhouse-finalizers.deckhouse.io")
		Expect(vapBinding.Exists()).To(BeTrue())
		Expect(vapBinding.Field("spec.policyName").String()).To(Equal("deny-deckhouse-finalizers.deckhouse.io"))
		Expect(vapBinding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny","Audit"]`))
	})
})

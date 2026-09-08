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
		Expect(vap.Field("spec.failurePolicy").String()).To(Equal("Ignore"))
		Expect(vap.Field("spec.matchConstraints.resourceRules.0.operations").String()).To(MatchJSON(`["UPDATE"]`))
		Expect(vap.Field("spec.matchConditions").Array()).To(HaveLen(4))
		Expect(vap.Field(`spec.matchConditions.#(name=="finalizers-changed").name`).String()).To(Equal("finalizers-changed"))
		Expect(vap.Field("spec.validations.0.message").String()).To(
			Equal("Removing Deckhouse finalizers (containing 'deckhouse.io') is forbidden"),
		)

		vapBinding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", "deny-deckhouse-finalizers.deckhouse.io")
		Expect(vapBinding.Exists()).To(BeTrue())
		Expect(vapBinding.Field("spec.policyName").String()).To(Equal("deny-deckhouse-finalizers.deckhouse.io"))
		Expect(vapBinding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny","Audit"]`))
	})

	// Assign/AssignMetadata/ModifySet/AssignImage CRDs are imported from upstream Gatekeeper as-is
	// (see crds/gatekeeper/update.sh), so content restrictions that Gatekeeper's own mutation webhook
	// enforces (but that can't live in the vendored CRD schema) are enforced here instead. Each
	// policy is named with a .deckhouse.io suffix because these are cluster-scoped: an unqualified
	// name would collide with a same-named VAP a user already created and fail the Helm release.
	// Whether the CEL expressions themselves accept and reject the right objects is covered in
	// admission_mutators_validation_cel_test.go, not here.
	It("renders VAPs validating Gatekeeper mutator resources", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		for _, policyName := range []string{
			"deny-invalid-assign-location.deckhouse.io",
			"deny-invalid-assignmetadata-location.deckhouse.io",
			"deny-invalid-assignmetadata-value.deckhouse.io",
			"deny-invalid-mutator-frommetadata-field.deckhouse.io",
			"deny-invalid-assignmetadata-externaldata-datasource.deckhouse.io",
			"deny-mutator-without-match-kinds.deckhouse.io",
		} {
			vap := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", policyName)
			Expect(vap.Exists()).To(BeTrue(), "policy %s must be rendered", policyName)
			Expect(vap.Field("spec.failurePolicy").String()).To(Equal("Fail"))
			Expect(vap.Field("spec.matchConstraints.resourceRules.0.apiGroups").String()).To(
				MatchJSON(`["mutations.gatekeeper.sh"]`),
			)

			vapBinding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", policyName)
			Expect(vapBinding.Exists()).To(BeTrue(), "binding %s must be rendered", policyName)
			Expect(vapBinding.Field("spec.policyName").String()).To(Equal(policyName))
			Expect(vapBinding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny"]`))
		}

		assignLocationVAP := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", "deny-invalid-assign-location.deckhouse.io")
		Expect(assignLocationVAP.Field("spec.matchConstraints.resourceRules.0.resources").String()).To(
			MatchJSON(`["assign"]`),
		)

		matchKindsVAP := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", "deny-mutator-without-match-kinds.deckhouse.io")
		Expect(matchKindsVAP.Field("spec.matchConstraints.resourceRules.0.resources").String()).To(
			MatchJSON(`["assign", "assignmetadata", "modifyset", "assignimage"]`),
		)

		fromMetadataVAP := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", "deny-invalid-mutator-frommetadata-field.deckhouse.io")
		Expect(fromMetadataVAP.Field("spec.matchConstraints.resourceRules.0.resources").String()).To(
			MatchJSON(`["assign", "assignmetadata"]`),
		)
	})
})

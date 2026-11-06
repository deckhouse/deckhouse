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
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

// validation-modules.yaml holds three policies over the module updates: the update policy a
// ModuleRelease is labeled with is chosen by Deckhouse, and an update window of the deckhouse
// ModuleConfig or of a ModuleUpdatePolicy has to end after it starts. The expressions are evaluated
// with admissionPolicyVerdict (registry_validation_test.go).

const (
	updatePolicyLabelPolicy      = "update-policy-label-objects.deckhouse.io"
	moduleConfigUpdateWindows    = "mc-update-windows.deckhouse.io"
	updatePolicyUpdateWindows    = "mup-update-windows.deckhouse.io"
	moduleReleaseUpdatePolicyKey = "modules.deckhouse.io/update-policy"
)

// updateWindow is an update window of the update settings, from and to in the HH:MM form of the schema.
func updateWindow(from, to string) map[string]any {
	return map[string]any{"from": from, "to": to}
}

var _ = Describe("Module :: deckhouse :: module update policies ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
	})

	render := func() {
		f.HelmRender(WithAPIVersions(validatingAdmissionPolicyAPI, validatingAdmissionPolicyBindingAPI))
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	policy := func(name string) object_store.KubeObject {
		return f.KubernetesGlobalResource("ValidatingAdmissionPolicy", name)
	}

	bindingActions := func(name string) string {
		binding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", name)
		Expect(binding.Exists()).To(BeTrue(), "the %s binding should be rendered", name)
		Expect(binding.Field("spec.policyName").String()).To(Equal(name))
		Expect(binding.Field("spec.matchResources").Exists()).To(BeFalse(), "the %s binding should select every object", name)
		return binding.Field("spec.validationActions").String()
	}

	It("renders no policies where the API server serves no ValidatingAdmissionPolicy", func() {
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		for _, name := range []string{updatePolicyLabelPolicy, moduleConfigUpdateWindows, updatePolicyUpdateWindows} {
			Expect(policy(name).Exists()).To(BeFalse(), name)
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", name).Exists()).To(BeFalse(), name)
		}
	})

	Context("the update policy of a ModuleRelease", func() {
		BeforeEach(func() {
			render()
		})

		release := func(updatePolicy string) map[string]any {
			labels := map[string]any{"module": "console"}
			if updatePolicy != "" {
				labels[moduleReleaseUpdatePolicyKey] = updatePolicy
			}
			return map[string]any{"metadata": map[string]any{"name": "console-v1.2.3", "labels": labels}}
		}

		update := func(username, oldPolicy, newPolicy string) verdict {
			request := map[string]any{"operation": "UPDATE", "userInfo": map[string]any{"username": username}}
			decision, _ := admissionPolicyVerdict(policy(updatePolicyLabelPolicy), request, release(oldPolicy), release(newPolicy))
			return decision
		}

		It("reviews every update of a ModuleRelease and denies what it refuses", func() {
			Expect(policy(updatePolicyLabelPolicy).Field("spec.failurePolicy").String()).To(Equal("Fail"))
			Expect(policy(updatePolicyLabelPolicy).Field("spec.matchConstraints.resourceRules").String()).To(MatchJSON(`[
				{"apiGroups":["deckhouse.io"],"apiVersions":["*"],"operations":["UPDATE"],"resources":["modulereleases"]}
			]`))
			Expect(bindingActions(updatePolicyLabelPolicy)).To(MatchJSON(`["Deny"]`))
		})

		It("lets nobody but Deckhouse move a release to another update policy", func() {
			Expect(update("admin@example.com", "stable", "manual")).To(Equal(verdictDenied))
			Expect(update("system:serviceaccount:d8-system:deckhouse", "stable", "manual")).To(Equal(verdictAllowed))
		})

		It("lets anyone remove the label so that Deckhouse chooses again, label a release without one, or keep it", func() {
			Expect(update("admin@example.com", "stable", "")).To(Equal(verdictAllowed), "removing the label")
			Expect(update("admin@example.com", "", "stable")).To(Equal(verdictAllowed), "labeling a release without one")
			Expect(update("admin@example.com", "stable", "stable")).To(Equal(verdictAllowed), "keeping the label")
		})
	})

	Context("the update windows of the deckhouse ModuleConfig", func() {
		BeforeEach(func() {
			render()
		})

		write := func(name string, windows ...map[string]any) verdict {
			moduleConfig := map[string]any{
				"metadata": map[string]any{"name": name},
				"spec":     map[string]any{"version": int64(1), "settings": map[string]any{"update": map[string]any{"windows": windows}}},
			}
			request := map[string]any{"operation": "UPDATE", "userInfo": map[string]any{"username": "admin@example.com"}}
			decision, _ := admissionPolicyVerdict(policy(moduleConfigUpdateWindows), request, moduleConfig, moduleConfig)
			return decision
		}

		It("reviews the ModuleConfigs written, and denies and audits what it refuses", func() {
			Expect(policy(moduleConfigUpdateWindows).Field("spec.failurePolicy").String()).To(Equal("Fail"))
			Expect(policy(moduleConfigUpdateWindows).Field("spec.matchConstraints.resourceRules").String()).To(MatchJSON(`[
				{"apiGroups":["deckhouse.io"],"apiVersions":["*"],"operations":["CREATE","UPDATE"],"resources":["moduleconfigs"]}
			]`))
			Expect(bindingActions(moduleConfigUpdateWindows)).To(MatchJSON(`["Deny","Audit"]`))
		})

		It("admits a window that ends after it starts", func() {
			Expect(write("deckhouse", updateWindow("08:00", "10:30"), updateWindow("1:00", "2:00"))).To(Equal(verdictAllowed))
		})

		It("refuses a window that ends before it starts, or when it starts", func() {
			Expect(write("deckhouse", updateWindow("08:00", "10:30"), updateWindow("22:00", "02:00"))).To(Equal(verdictDenied),
				"a window does not cross midnight")
			Expect(write("deckhouse", updateWindow("10:00", "10:00"))).To(Equal(verdictDenied))
		})

		It("leaves the other ModuleConfigs and the deckhouse one without windows alone", func() {
			Expect(write("console", updateWindow("22:00", "02:00"))).To(Equal(verdictSkipped))
			Expect(write("deckhouse")).To(Equal(verdictSkipped))
		})
	})

	Context("the update windows of a ModuleUpdatePolicy", func() {
		BeforeEach(func() {
			render()
		})

		write := func(windows ...map[string]any) verdict {
			updatePolicy := map[string]any{
				"metadata": map[string]any{"name": "console"},
				"spec":     map[string]any{"update": map[string]any{"mode": "Auto", "windows": windows}},
			}
			request := map[string]any{"operation": "CREATE", "userInfo": map[string]any{"username": "admin@example.com"}}
			decision, _ := admissionPolicyVerdict(policy(updatePolicyUpdateWindows), request, nil, updatePolicy)
			return decision
		}

		It("reviews every ModuleUpdatePolicy written, and denies and audits what it refuses", func() {
			Expect(policy(updatePolicyUpdateWindows).Field("spec.failurePolicy").String()).To(Equal("Fail"))
			Expect(policy(updatePolicyUpdateWindows).Field("spec.matchConstraints.resourceRules").String()).To(MatchJSON(`[
				{"apiGroups":["deckhouse.io"],"apiVersions":["*"],"operations":["CREATE","UPDATE"],"resources":["moduleupdatepolicies"]}
			]`))
			Expect(bindingActions(updatePolicyUpdateWindows)).To(MatchJSON(`["Deny","Audit"]`))
		})

		It("admits a window that ends after it starts, and refuses one that does not", func() {
			Expect(write(updateWindow("08:00", "10:30"))).To(Equal(verdictAllowed))
			Expect(write(updateWindow("08:00", "10:30"), updateWindow("23:00", "01:00"))).To(Equal(verdictDenied))
		})

		It("leaves a policy without windows alone", func() {
			Expect(write()).To(Equal(verdictSkipped))
		})
	})
})

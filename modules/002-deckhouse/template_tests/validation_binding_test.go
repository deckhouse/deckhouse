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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// aliased: this package already has a labels() helper for test objects.
	k8slabels "k8s.io/apimachinery/pkg/labels"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

// This test covers what the bindings of the d8a-prefix.deckhouse.io policy select,
// which is where the exemption for an application under maintenance lives: the
// policy itself guards every object named after the prefix, and the bindings are
// what keeps it off the objects the platform no longer reconciles.
//
// The rendered selectors are evaluated the way the API server does: a binding
// matches when the old or the new object carries labels its objectSelector accepts
// (MatchObjectSelector in k8s.io/apiserver), and a policy is enforced when any of
// its bindings matches. The union of the two bindings is therefore a disjunction,
// which is what lets a pair of label selectors carve out an exception neither of
// them could express alone.

const (
	applicationPrefixPolicy          = "d8a-prefix.deckhouse.io"
	applicationPrefixHeritageBinding = "heritage-d8a-prefix.deckhouse.io"
	heritageLabelObjectsBinding      = "heritage-label-objects.deckhouse.io"

	maintenanceLabel = "maintenance.deckhouse.io/no-resource-reconciliation"

	// absent stands for a label the object does not carry at all, which is not the
	// same as the empty value nelm writes.
	absent = "<absent>"
)

// objectSelector reads a binding's objectSelector the way the API server parses it.
func objectSelector(binding object_store.KubeObject) k8slabels.Selector {
	var spec metav1.LabelSelector
	Expect(json.Unmarshal([]byte(binding.Field("spec.matchResources.objectSelector").Raw), &spec)).To(Succeed())

	selector, err := metav1.LabelSelectorAsSelector(&spec)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(selector.Empty()).To(BeFalse(), "an empty selector would bind the policy to every object")

	return selector
}

var _ = Describe("Module :: deckhouse :: application prefix bindings ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	var selectors []k8slabels.Selector

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.HelmRender(WithAPIVersions(validatingAdmissionPolicyAPI, validatingAdmissionPolicyBindingAPI))
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		selectors = nil
		for _, name := range []string{applicationPrefixPolicy, applicationPrefixHeritageBinding} {
			binding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", name)
			Expect(binding.Exists()).To(BeTrue(), "the %s binding should be rendered", name)
			Expect(binding.Field("spec.policyName").String()).To(Equal(applicationPrefixPolicy),
				"the %s binding should bind the application prefix policy", name)
			Expect(binding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny","Audit"]`),
				"a binding that denies less than the others would be a hole of its own")

			selectors = append(selectors, objectSelector(binding))
		}
	})

	// guarded reports whether the policy is enforced for a request carrying these
	// objects: nil stands for the object a CREATE has no old version of and the one
	// a DELETE has no new version of.
	guarded := func(oldLabels, newLabels map[string]string) bool {
		for _, selector := range selectors {
			for _, set := range []map[string]string{oldLabels, newLabels} {
				if set != nil && selector.Matches(k8slabels.Set(set)) {
					return true
				}
			}
		}
		return false
	}

	objectLabels := func(heritage, maintenance string) map[string]string {
		set := map[string]string{"app": "console"}
		if heritage != absent {
			set["heritage"] = heritage
		}
		if maintenance != absent {
			set[maintenanceLabel] = maintenance
		}
		return set
	}

	It("guards every object but the ones of an application under maintenance", func() {
		for _, heritage := range []string{absent, "deckhouse", "tenant"} {
			for _, maintenance := range []string{absent, "", "true", "false", "later"} {
				set := objectLabels(heritage, maintenance)
				// Only the pair nelm stamps together is let through, with the values
				// the heritage binding lists.
				exempt := heritage == "deckhouse" && (maintenance == "" || maintenance == "true")

				Expect(guarded(set, set)).To(Equal(!exempt),
					"an object labeled heritage=%q, %s=%q", heritage, maintenanceLabel, maintenance)
			}
		}
	})

	It("guards an object that carries no labels at all", func() {
		Expect(guarded(map[string]string{}, map[string]string{})).To(BeTrue())
	})

	It("keeps the maintenance label itself out of a user's reach", func() {
		ours := objectLabels("deckhouse", absent)
		underMaintenance := objectLabels("deckhouse", "")

		// A binding matches on either object, so the protected side of the update is
		// what decides: only the platform, which the policy exempts by service
		// account, can stamp the label or take it off.
		Expect(guarded(ours, underMaintenance)).To(BeTrue(), "putting the label on an object by hand")
		Expect(guarded(underMaintenance, ours)).To(BeTrue(), "taking the label off an object by hand")

		Expect(guarded(underMaintenance, underMaintenance)).To(BeFalse(), "editing an object under maintenance")
		Expect(guarded(underMaintenance, nil)).To(BeFalse(), "deleting an object under maintenance")
		Expect(guarded(nil, underMaintenance)).To(BeFalse(), "creating an object that claims both labels")
	})

	It("exempts what the heritage binding exempts, by the very same selector", func() {
		heritage := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", heritageLabelObjectsBinding)
		Expect(heritage.Exists()).To(BeTrue(), "the heritage binding is rendered outside dev clusters")

		exemption := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", applicationPrefixHeritageBinding)
		Expect(exemption.Field("spec.matchResources.objectSelector").String()).
			To(MatchJSON(heritage.Field("spec.matchResources.objectSelector").String()),
				"the two exemptions should stay the same one")
	})
})

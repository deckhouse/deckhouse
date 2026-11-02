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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const projectGrantsPolicyName = "d8-multitenancy-manager-project-grants"

var _ = Describe("Module :: multitenancy-manager :: helm template :: project grants ::", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		renderQuota(hec, true)
	})

	// A write of the status through v1alpha2 erases spec.grants as a write of the project does, so the
	// webhook of the main resource has a sibling for the subresource, with the same exclusions.
	It("checks the status writes of a project for erasing grants", func() {
		Expect(hec.RenderError).ShouldNot(HaveOccurred())
		webhooks := hec.KubernetesGlobalResource("ValidatingWebhookConfiguration", "d8-multitenancy-manager-webhook")
		found := false
		for _, webhook := range webhooks.Field("webhooks").Array() {
			if webhook.Get("name").String() != "project-status.multitenancy-webhook.deckhouse.io" {
				continue
			}
			found = true
			Expect(webhook.Get("rules.0.apiVersions").String()).To(MatchJSON(`["v1alpha3"]`))
			Expect(webhook.Get("rules.0.resources").String()).To(MatchJSON(`["projects/status"]`))
			Expect(webhook.Get("rules.0.operations").String()).To(MatchJSON(`["UPDATE"]`))
			Expect(webhook.Get("matchPolicy").String()).To(Equal("Equivalent"), "a write through v1alpha2 is converted to the rule version")
			Expect(webhook.Get("clientConfig.service.path").String()).To(Equal("/validate/v1alpha3/projects/status"))
			Expect(webhook.Get("matchConditions.0.name").String()).To(Equal("exclude-system-writers"))
		}
		Expect(found).To(BeTrue())
	})

	It("keeps the project-* grant policies to the controller", func() {
		Expect(hec.RenderError).ShouldNot(HaveOccurred())
		Expect(hec.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", projectGrantsPolicyName).Exists()).To(BeTrue())
		policy := compileRenderedPolicy(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", projectGrantsPolicyName).ToYaml())

		resource := schema.GroupVersionResource{Group: "multitenancy.deckhouse.io", Version: "v1alpha2", Resource: "clusterresourcegrantpolicies"}
		managed := map[string]any{"metadata": map[string]any{"labels": map[string]any{"multitenancy.deckhouse.io/managed-by-project": "team"}}}
		grantPolicy := func(operation admission.Operation, name, username string, content map[string]any) admission.Attributes {
			return write(resource, "", operation, "", name, username, content)
		}
		cases := []struct {
			name    string
			attrs   admission.Attributes
			allowed bool
		}{
			{name: "a user policy", attrs: grantPolicy(admission.Create, "web", projectAdministrator, nil), allowed: true},
			{name: "a user policy under a reserved name", attrs: grantPolicy(admission.Create, "project-team-inline", projectAdministrator, nil)},
			{name: "the controller makes a managed policy", attrs: grantPolicy(admission.Create, "project-team-inline", controllerServiceAccount, managed), allowed: true},
			{name: "a user changes a managed policy", attrs: grantPolicy(admission.Update, "project-team-inline", projectAdministrator, managed)},
			{name: "a user deletes a managed policy", attrs: grantPolicy(admission.Delete, "project-team-inline", projectAdministrator, managed)},
			{name: "the garbage collector deletes it with the project", attrs: grantPolicy(admission.Delete, "project-team-inline",
				"system:serviceaccount:kube-system:generic-garbage-collector", managed), allowed: true},
			{name: "a policy named project-* before the names were reserved", attrs: grantPolicy(admission.Update, "project-legacy", projectAdministrator, nil), allowed: true},
		}
		for _, tc := range cases {
			allowed, _, err := policy.admit(tc.attrs, nil)
			Expect(err).ToNot(HaveOccurred(), tc.name)
			Expect(allowed).To(Equal(tc.allowed), tc.name)
		}
	})
})

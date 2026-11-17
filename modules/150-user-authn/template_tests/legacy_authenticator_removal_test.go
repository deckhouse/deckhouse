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

var _ = Describe("Module :: user-authn :: helm template :: removal of the per-object dex-authenticator by dexclient-controller", func() {
	hec := SetupHelmConfig("")

	const policyName = "dexclient-controller-legacy.user-authn.deckhouse.io"

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.33.1")
		hec.ValuesSet("global.discovery.clusterDomain", "cluster.local")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler", "operator-prometheus"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.ca", "plainstring")
		hec.ValuesSetFromYaml("userAuthn.internal.sharedDexAuthenticator.redisPasswords", `
dex-authenticator-system: dh-test-secret-0123456789abcdef0
dex-authenticator: dh-test-secret-0123456789abcdef1
`)
		hec.HelmRender(WithAPIVersions(validatingAdmissionPolicyAPIVersion, validatingAdmissionPolicyBindingAPIVersion))
	})

	kinds := []struct {
		gvk      schema.GroupVersionKind
		resource string
	}{
		{schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, "deployments"},
		{schema.GroupVersionKind{Group: "policy", Version: "v1", Kind: "PodDisruptionBudget"}, "poddisruptionbudgets"},
		{schema.GroupVersionKind{Group: "autoscaling.k8s.io", Version: "v1", Kind: "VerticalPodAutoscaler"}, "verticalpodautoscalers"},
		{schema.GroupVersionKind{Group: "networking.k8s.io", Version: "v1", Kind: "NetworkPolicy"}, "networkpolicies"},
	}

	It("grants the deletion and nothing else on these kinds", func() {
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		clusterRole := hec.KubernetesGlobalResource("ClusterRole", "d8:user-authn:dexclient-controller")
		Expect(clusterRole.Exists()).To(BeTrue())
		for _, kind := range kinds {
			rules := rulesFor(clusterRole.Field("rules").Result, kind.resource)
			Expect(rules).To(HaveLen(1), kind.resource)
			Expect(verbsOf(rules[0])).To(ConsistOf("get", "list", "delete"), kind.resource)
		}
	})

	It("lets dexclient-controller delete only what the module rendered for a DexAuthenticator", func() {
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		rendered := hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", policyName)
		Expect(rendered.Exists()).To(BeTrue())
		Expect(rendered.Field("spec.failurePolicy").String()).To(Equal("Fail"))
		resources := []string{}
		for _, rule := range rendered.Field("spec.matchConstraints.resourceRules").Array() {
			Expect(rule.Get("operations").String()).To(MatchJSON(`["CREATE","UPDATE","DELETE"]`))
			resources = append(resources, rule.Get("resources.0").String())
		}
		Expect(resources).To(ConsistOf("deployments", "poddisruptionbudgets", "verticalpodautoscalers", "networkpolicies"))

		binding := hec.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", policyName)
		Expect(binding.Exists()).To(BeTrue())
		Expect(binding.Field("spec.policyName").String()).To(Equal(policyName))
		Expect(binding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny"]`))
		Expect(binding.Field("spec.matchResources").Exists()).To(BeFalse())

		policy := compileRenderedPolicy(rendered.ToYaml())
		chart := map[string]string{"app": "dex-authenticator", "heritage": "deckhouse", "module": "user-authn", "deckhouse.io/dex-authenticator-for": "app"}
		for _, tc := range []struct {
			name      string
			username  string
			operation admission.Operation
			old, new  map[string]string
			allowed   bool
		}{
			{name: "deletes an object of the chart", operation: admission.Delete, old: chart, allowed: true},
			{name: "cannot delete the workload of a user", operation: admission.Delete, old: map[string]string{"app": "shop"}},
			{name: "cannot delete an object without labels", operation: admission.Delete, old: map[string]string{}},
			{
				name: "cannot delete an object a user called dex-authenticator", operation: admission.Delete,
				old: map[string]string{"app": "dex-authenticator", "module": "user-authn"},
			},
			{
				name: "cannot delete an object of another module", operation: admission.Delete,
				old: map[string]string{"app": "dex-authenticator", "heritage": "deckhouse", "module": "console"},
			},
			{name: "cannot create even an object with the labels of the chart", operation: admission.Create, new: chart},
			{name: "cannot change an object of the chart", operation: admission.Update, old: chart, new: chart},
			{name: "leaves a user alone", username: "editor@example.com", operation: admission.Delete, old: map[string]string{"app": "shop"}, allowed: true},
			{
				name: "leaves another controller of the module alone", username: "system:serviceaccount:d8-user-authn:controller",
				operation: admission.Delete, old: map[string]string{"app": "shop"}, allowed: true,
			},
		} {
			username := tc.username
			if username == "" {
				username = dexClientControllerServiceAccount
			}
			for _, kind := range kinds {
				By(tc.name + ": " + kind.resource)
				allowed, err := objectWriteAdmitted(policy, username, kind.gvk, kind.resource, tc.operation, tc.old, tc.new, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(allowed).To(Equal(tc.allowed))
			}
		}
	})
})

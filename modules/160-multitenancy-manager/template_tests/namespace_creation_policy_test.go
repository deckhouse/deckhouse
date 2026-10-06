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
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

const (
	namespaceCreationPolicy             = "d8-multitenancy-manager-namespace-creation"
	validatingAdmissionPolicyAPI        = "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy"
	validatingAdmissionPolicyBindingAPI = "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicyBinding"
)

// namespaceRequest is a namespace CREATE as the API server hands it to the policy.
type namespaceRequest struct {
	name     string
	username string
	groups   []string
}

// admitsNamespace runs the CEL of the rendered policy the way the API server does: a request the match
// conditions leave out is admitted without the validations, any other one only when every validation
// holds. A bare CEL environment is enough to catch what is worth catching here, a syntax error or an
// operation that does not exist, without standing up an API server.
func admitsNamespace(policy object_store.KubeObject, req namespaceRequest) bool {
	env, err := cel.NewEnv(
		ext.Strings(),
		cel.Variable("object", cel.DynType),
		cel.Variable("oldObject", cel.DynType),
		cel.Variable("request", cel.DynType),
	)
	Expect(err).ShouldNot(HaveOccurred())

	groups := make([]interface{}, 0, len(req.groups))
	for _, group := range req.groups {
		groups = append(groups, group)
	}
	activation := map[string]interface{}{
		"object":    map[string]interface{}{"metadata": map[string]interface{}{"name": req.name}},
		"oldObject": nil,
		"request": map[string]interface{}{
			"operation": "CREATE",
			"name":      req.name,
			"userInfo":  map[string]interface{}{"username": req.username, "groups": groups},
		},
	}
	holds := func(expression string) bool {
		ast, issues := env.Compile(expression)
		Expect(issues.Err()).ShouldNot(HaveOccurred(), "expression should compile: %s", expression)
		program, err := env.Program(ast)
		Expect(err).ShouldNot(HaveOccurred())
		out, _, err := program.Eval(activation)
		Expect(err).ShouldNot(HaveOccurred(), "expression should evaluate: %s", expression)
		return out.Value() == true
	}

	for _, condition := range policy.Field("spec.matchConditions").Array() {
		if !holds(condition.Get("expression").String()) {
			return true
		}
	}
	for _, validation := range policy.Field("spec.validations").Array() {
		if !holds(validation.Get("expression").String()) {
			return false
		}
	}
	return true
}

var _ = Describe("Module :: multitenancy-manager :: helm template :: namespace creation policy ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", `{}`)
		f.ValuesSetFromYaml("multitenancyManager.internal.admissionWebhookCert", `{"ca": "test-ca", "crt": "test", "key": "test"}`)
		f.ApplyOpenAPIDefaults()
	})

	render := func(allow bool) {
		f.ValuesSet("multitenancyManager.allowNamespacesWithoutProjects", allow)
		f.HelmRender(WithAPIVersions(validatingAdmissionPolicyAPI, validatingAdmissionPolicyBindingAPI))
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	Context("namespaces outside projects are allowed", func() {
		BeforeEach(func() {
			render(true)
		})

		It("renders no policy", func() {
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", namespaceCreationPolicy).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", namespaceCreationPolicy).Exists()).To(BeFalse())
		})
	})

	Context("namespaces outside projects are forbidden", func() {
		var policy object_store.KubeObject

		BeforeEach(func() {
			render(false)
			policy = f.KubernetesGlobalResource("ValidatingAdmissionPolicy", namespaceCreationPolicy)
		})

		It("checks only the creation of a namespace and denies through its binding", func() {
			Expect(policy.Exists()).To(BeTrue())
			Expect(policy.Field("spec.failurePolicy").String()).To(Equal("Fail"))
			Expect(policy.Field("spec.matchConstraints.resourceRules").String()).To(MatchJSON(
				`[{"apiGroups": [""], "apiVersions": ["v1"], "operations": ["CREATE"], "resources": ["namespaces"]}]`))

			binding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", namespaceCreationPolicy)
			Expect(binding.Exists()).To(BeTrue())
			Expect(binding.Field("spec.policyName").String()).To(Equal(namespaceCreationPolicy))
			Expect(binding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny", "Audit"]`))
		})

		// The cases mirror the namespaces webhook of DKP 1.77: its match conditions and the service
		// accounts its handler let through. system:sudouser is let through as well, as in the other
		// checks of the module.
		for _, c := range []struct {
			title    string
			req      namespaceRequest
			admitted bool
		}{
			{"refuses a user", namespaceRequest{"team-a", "alice@example.com", []string{"system:authenticated"}}, false},
			{"refuses a service account outside the platform", namespaceRequest{"team-a", "system:serviceaccount:argocd:argocd-application-controller", []string{"system:serviceaccounts", "system:serviceaccounts:argocd", "system:authenticated"}}, false},
			{"lets system:sudouser through, as the other checks of the module do", namespaceRequest{"team-a", "system:sudouser", []string{"system:authenticated"}}, true},
			{"lets a user create the default namespace", namespaceRequest{"default", "alice@example.com", []string{"system:authenticated"}}, true},
			{"lets the module controller create a project namespace", namespaceRequest{"team-a", "system:serviceaccount:d8-multitenancy-manager:multitenancy-manager", []string{"system:serviceaccounts", "system:serviceaccounts:d8-multitenancy-manager", "system:authenticated"}}, true},
			{"lets Deckhouse create a module namespace", namespaceRequest{"d8-example", "system:serviceaccount:d8-system:deckhouse", []string{"system:serviceaccounts", "system:serviceaccounts:d8-system", "system:authenticated"}}, true},
			{"lets the upmeter agent create a probe namespace", namespaceRequest{"upmeter-probe-namespace-abc", "system:serviceaccount:d8-upmeter:upmeter-agent", []string{"system:serviceaccounts", "system:serviceaccounts:d8-upmeter", "system:authenticated"}}, true},
			{"lets another service account of d8-system through", namespaceRequest{"team-a", "system:serviceaccount:d8-system:other", []string{"system:serviceaccounts", "system:serviceaccounts:d8-system", "system:authenticated"}}, true},
			{"lets a service account of kube-system through", namespaceRequest{"team-a", "system:serviceaccount:kube-system:namespace-controller", []string{"system:serviceaccounts", "system:serviceaccounts:kube-system", "system:authenticated"}}, true},
			{"lets the API server recreate its namespaces", namespaceRequest{"kube-public", "system:apiserver", []string{"system:authenticated"}}, true},
			{"lets the system:masters group through", namespaceRequest{"team-a", "kubernetes-admin", []string{"system:masters", "system:authenticated"}}, true},
			{"lets a node through", namespaceRequest{"team-a", "system:node:worker-0", []string{"system:nodes", "system:authenticated"}}, true},
		} {
			c := c
			It(c.title, func() {
				Expect(admitsNamespace(policy, c.req)).To(Equal(c.admitted))
			})
		}
	})
})

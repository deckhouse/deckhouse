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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

// This test covers the two policies that keep the licensing state in one writer's
// hands: licensing.deckhouse.io over the custom resources, and
// licensing-storage.deckhouse.io over the Secret and the ConfigMap they are
// computed from. A policy here is its resourceRules plus a handful of
// expressions, so both halves are asserted against the rendered chart, and the
// expressions are compiled and evaluated the way the API server does.

const (
	licensingPolicy        = "licensing.deckhouse.io"
	licensingStoragePolicy = "licensing-storage.deckhouse.io"
	deckhouseController    = "system:serviceaccount:d8-system:deckhouse"
	licensingDenial        = "Licensing state is managed by Deckhouse and cannot be modified manually"
)

// licensingRules is what the resource policy must match, subresources included:
// an EffectiveLicense in full, and the status of a ClusterLicense. The main
// ClusterLicense resource is deliberately absent - that is how a key is installed.
const licensingRules = `[
  {
    "apiGroups": ["deckhouse.io"],
    "apiVersions": ["*"],
    "operations": ["CREATE", "UPDATE", "DELETE"],
    "resources": ["effectivelicenses", "effectivelicenses/status"]
  },
  {
    "apiGroups": ["deckhouse.io"],
    "apiVersions": ["*"],
    "operations": ["UPDATE"],
    "resources": ["clusterlicenses/status"]
  }
]`

const licensingStorageRules = `[
  {
    "apiGroups": [""],
    "apiVersions": ["*"],
    "operations": ["CREATE", "UPDATE", "DELETE"],
    "resources": ["secrets", "configmaps"]
  }
]`

var _ = Describe("Module :: deckhouse :: licensing admission policy ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.HelmRender(WithAPIVersions(validatingAdmissionPolicyAPI, validatingAdmissionPolicyBindingAPI))
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	It("guards the licensing resources and nothing else", func() {
		policy := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", licensingPolicy)
		Expect(policy.Exists()).To(BeTrue(), "the %s policy should be rendered", licensingPolicy)
		Expect(policy.Field("spec.failurePolicy").String()).To(Equal("Fail"),
			"a licensing guard that fails open is not a guard")
		Expect(policy.Field("spec.matchConstraints.resourceRules").String()).To(MatchJSON(licensingRules))
		Expect(policy.Field("spec.matchConditions").Exists()).To(BeFalse(),
			"matchConditions are ANDed over the whole policy, so one here would weaken every rule")
	})

	It("is bound with no selector, so nothing can be labeled out of its reach", func() {
		binding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", licensingPolicy)
		Expect(binding.Exists()).To(BeTrue(), "the %s binding should be rendered", licensingPolicy)
		Expect(binding.Field("spec.policyName").String()).To(Equal(licensingPolicy))
		Expect(binding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny","Audit"]`))
		Expect(binding.Field("spec.matchResources").Exists()).To(BeFalse())
	})

	It("guards the two objects the licensing state is kept in", func() {
		policy := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", licensingStoragePolicy)
		Expect(policy.Exists()).To(BeTrue(), "the %s policy should be rendered", licensingStoragePolicy)
		Expect(policy.Field("spec.failurePolicy").String()).To(Equal("Fail"))
		Expect(policy.Field("spec.matchConstraints.resourceRules").String()).To(MatchJSON(licensingStorageRules))

		binding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", licensingStoragePolicy)
		Expect(binding.Exists()).To(BeTrue(), "the %s binding should be rendered", licensingStoragePolicy)
		Expect(binding.Field("spec.policyName").String()).To(Equal(licensingStoragePolicy))
		Expect(binding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny","Audit"]`))
		Expect(binding.Field("spec.matchResources.namespaceSelector.matchLabels").String()).
			To(MatchJSON(`{"kubernetes.io/metadata.name":"d8-system"}`))
	})

	It("narrows the storage policy to the two objects, whichever side of the request carries them", func() {
		conditions := matchConditions(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", licensingStoragePolicy))
		Expect(conditions).To(HaveLen(2))

		namespace := celProgram(conditions["only-system-namespace"], cel.BoolType)
		named := celProgram(conditions["only-licensing-storage"], cel.BoolType)

		secret := func(name string) object {
			return object{"metadata": object{"name": name, "namespace": "d8-system"}}
		}

		// CREATE and UPDATE carry the new object, DELETE only the old one.
		for _, name := range []string{"cluster-key", "consumption-journal"} {
			Expect(celEval(named, admission("UPDATE", "d8-system", "admin", secret(name), secret(name)))).
				To(BeTrue(), "updating %s", name)
			Expect(celEval(named, admission("CREATE", "d8-system", "admin", nil, secret(name)))).
				To(BeTrue(), "creating %s", name)
			Expect(celEval(named, admission("DELETE", "d8-system", "admin", secret(name), nil))).
				To(BeTrue(), "deleting %s, where only oldObject is left", name)
		}

		other := secret("deckhouse-registry")
		Expect(celEval(named, admission("UPDATE", "d8-system", "admin", other, other))).To(BeFalse())
		Expect(celEval(named, admission("DELETE", "d8-system", "admin", other, nil))).To(BeFalse())

		// The namespace is checked on the request, so a same-named object of a
		// user namespace is none of this policy's business.
		elsewhere := object{"metadata": object{"name": "cluster-key", "namespace": "default"}}
		Expect(celEval(namespace, admission("UPDATE", "d8-system", "admin", nil, secret("cluster-key")))).To(BeTrue())
		Expect(celEval(namespace, admission("UPDATE", "default", "admin", elsewhere, elsewhere))).To(BeFalse())
	})

	It("lets the deckhouse controller write, and no one else", func() {
		for _, name := range []string{licensingPolicy, licensingStoragePolicy} {
			policy := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", name)

			validations := policy.Field("spec.validations").Array()
			Expect(validations).To(HaveLen(1), "one rule, one expression, in %s", name)
			Expect(validations[0].Get("reason").String()).To(Equal("Forbidden"))

			allows := celProgram(validations[0].Get("expression").String(), cel.BoolType)
			message := celProgram(validations[0].Get("messageExpression").String(), cel.StringType)

			request := func(username string) map[string]any {
				return admission("UPDATE", "d8-system", username, nil, nil)
			}

			Expect(celEval(allows, request(deckhouseController))).To(BeTrue(), "in %s", name)
			for _, username := range []string{
				"admin",
				"dhctl",
				"system:admin",
				"system:apiserver",
				"system:serviceaccount:kube-system:generic-garbage-collector",
				// A neighbouring service account of the very same namespace: the
				// other policies of this chart wave those through by group, these
				// two do not.
				"system:serviceaccount:d8-system:d8-service-accounts",
				// Close enough to fool a startsWith check.
				"system:serviceaccount:d8-system:deckhouse-2",
			} {
				Expect(celEval(allows, request(username))).To(BeFalse(),
					"%s should not be able to write the licensing state guarded by %s", username, name)
			}

			out, _, err := message.Eval(request("admin"))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(out.Value()).To(Equal(licensingDenial))
		}
	})
})

// matchConditions reads a policy's match conditions into name -> expression.
func matchConditions(policy object_store.KubeObject) map[string]string {
	out := map[string]string{}
	for _, condition := range policy.Field("spec.matchConditions").Array() {
		out[condition.Get("name").String()] = condition.Get("expression").String()
	}
	return out
}

// admission builds the activation a ValidatingAdmissionPolicy expression sees.
// A nil object stands for the one a CREATE has no old version of and the one a
// DELETE has no new version of.
func admission(operation, namespace, username string, oldObject, newObject any) map[string]any {
	return map[string]any{
		"request": object{
			"operation": operation,
			"namespace": namespace,
			"userInfo":  object{"username": username, "groups": []any{"system:authenticated"}},
		},
		"object":    celValue(newObject),
		"oldObject": celValue(oldObject),
	}
}

// celProgram compiles one policy expression the way the API server does.
func celProgram(expression string, want *cel.Type) cel.Program {
	Expect(expression).ShouldNot(BeEmpty())

	env, err := cel.NewEnv(
		cel.Variable("object", cel.DynType),
		cel.Variable("oldObject", cel.DynType),
		cel.Variable("request", cel.DynType),
	)
	Expect(err).ShouldNot(HaveOccurred())

	ast, issues := env.Compile(expression)
	Expect(issues.Err()).ShouldNot(HaveOccurred(), "compiling %q", expression)
	Expect(ast.OutputType().IsExactType(want)).To(BeTrue(), "%q evaluates to %s, want %s", expression, ast.OutputType(), want)

	program, err := env.Program(ast)
	Expect(err).ShouldNot(HaveOccurred())

	return program
}

func celEval(program cel.Program, activation map[string]any) bool {
	out, _, err := program.Eval(activation)
	Expect(err).ShouldNot(HaveOccurred())

	value, ok := out.Value().(bool)
	Expect(ok).To(BeTrue(), "the expression returned %T, want bool", out.Value())

	return value
}

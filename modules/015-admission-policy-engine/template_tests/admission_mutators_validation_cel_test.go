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
)

// evalMutatorValidation compiles and evaluates a single ValidatingAdmissionPolicy validation
// expression against a request object, the way the apiserver would for a request that reaches
// validation (the six policies in templates/admission-mutators-validation.yaml declare no
// matchConditions, so every request their resourceRules select does). Compiling against a bare CEL
// environment with an `object` dyn variable is enough to catch what these table tests are for: the
// expression accepting or rejecting the wrong shape of object, not apiserver wiring.
func evalMutatorValidation(expression string, object map[string]interface{}) bool {
	env, err := cel.NewEnv(cel.Variable("object", cel.DynType))
	Expect(err).ShouldNot(HaveOccurred())

	ast, issues := env.Compile(expression)
	Expect(issues.Err()).ShouldNot(HaveOccurred(), "expression should compile: %s", expression)

	program, err := env.Program(ast)
	Expect(err).ShouldNot(HaveOccurred())

	out, _, err := program.Eval(map[string]interface{}{"object": object})
	Expect(err).ShouldNot(HaveOccurred(), "expression should evaluate without error: %s\nobject: %#v", expression, object)

	return out.Value() == true
}

// assign builds the spec.parameters.assign object the fromMetadata/value/externalData policies
// look at, so each table below only has to name the one field it varies.
func assign(fields map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"parameters": map[string]interface{}{"assign": fields}}
}

// matchKind builds one entry of spec.match.kinds. Passing nil for apiGroups or kinds omits the
// field entirely, rather than setting it to an empty list, which is the shape a manifest that never
// mentions the field takes.
func matchKind(apiGroups, kinds []string) map[string]interface{} {
	entry := map[string]interface{}{}
	if apiGroups != nil {
		entry["apiGroups"] = toInterfaceSlice(apiGroups)
	}
	if kinds != nil {
		entry["kinds"] = toInterfaceSlice(kinds)
	}
	return entry
}

func toInterfaceSlice(s []string) []interface{} {
	out := make([]interface{}, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}

var _ = Describe("Module :: admissionPolicyEngine :: mutator validation CEL", func() {
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
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	// expressionOf reads spec.validations.0.expression from the rendered policy, so every table
	// below exercises the template's actual text rather than a copy that could drift from it.
	expressionOf := func(policyName string) string {
		policy := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", policyName)
		Expect(policy.Exists()).To(BeTrue(), "policy %s must be rendered", policyName)
		expression := policy.Field("spec.validations.0.expression").String()
		Expect(expression).ToNot(BeEmpty(), "policy %s", policyName)
		return expression
	}

	// objectCase is one request object and the verdict the expression must reach for it. spec nil
	// means the object carries no spec field at all, which the vendored CRDs allow.
	type objectCase struct {
		spec interface{}
		want bool
		why  string
	}

	runTable := func(policyName string, cases []objectCase) {
		Expect(cases).ToNot(BeEmpty(), "a table that iterates nothing would report green")
		expression := expressionOf(policyName)
		for _, c := range cases {
			object := map[string]interface{}{}
			if c.spec != nil {
				object["spec"] = c.spec
			}
			Expect(evalMutatorValidation(expression, object)).To(Equal(c.want), c.why)
		}
	}

	It("deny-invalid-assign-location.deckhouse.io denies the metadata root however it is spelled", func() {
		runTable("deny-invalid-assign-location.deckhouse.io", []objectCase{
			{map[string]interface{}{"location": "spec.containers[name: main].image"}, true, "an ordinary location"},
			{map[string]interface{}{"location": "metadata"}, false, "the bare root"},
			{map[string]interface{}{"location": "metadata.labels.foo"}, false, "a metadata subpath"},
			{map[string]interface{}{"location": `"metadata".labels.x`}, false,
				"a double-quoted root: Gatekeeper's parser strips the quotes before comparing, so this is metadata too"},
			{map[string]interface{}{"location": "'metadata'.name"}, false, "a single-quoted root"},
			{map[string]interface{}{"location": "metadata[name: x].foo"}, false,
				"a bare root followed by a list spec: the parser appends the Object node before looking for [, so hasMetadataRoot still matches"},
			{map[string]interface{}{"location": `"metadata"[name: x].foo`}, false, "the same list spec after a quoted root"},
			{map[string]interface{}{"location": "metadataFoo.bar"}, true, "a field that only starts with the word metadata"},
			{nil, true, "no spec at all must not crash the expression"},
			{map[string]interface{}{}, true, "spec present, location absent"},
		})
	})

	It("deny-invalid-assignmetadata-location.deckhouse.io matches Gatekeeper's own location grammar", func() {
		runTable("deny-invalid-assignmetadata-location.deckhouse.io", []objectCase{
			{map[string]interface{}{"location": "metadata.labels.foo"}, true, "a bare key"},
			{map[string]interface{}{"location": `metadata.annotations."example.com/key"`}, true, "a quoted key with a slash"},
			{map[string]interface{}{"location": "metadata.labels.example.com/key"}, false, "the same key left unquoted"},
			{map[string]interface{}{"location": "metadata.name"}, false, "not labels or annotations"},
			{map[string]interface{}{"location": "metadata.labels"}, false, "no key at all"},
			{map[string]interface{}{"location": "metadata.labels.foo.bar"}, false, "an extra path segment"},
			{map[string]interface{}{"location": "metadata . labels . foo"}, true,
				"whitespace around separators: Gatekeeper's scanner skips it before every token"},
			{map[string]interface{}{"location": `metadata."labels".foo`}, true, "a quoted second segment"},
			{map[string]interface{}{"location": `"metadata".labels.foo`}, true, "a quoted first segment"},
			{map[string]interface{}{"location": "metadata.labels.123"}, false,
				"a bare key starting with a digit, which Gatekeeper's scanner tokenizes as a number, not an identifier"},
			{map[string]interface{}{"location": "metadata.labels._ok"}, true, "a bare key starting with an underscore"},
			{map[string]interface{}{"location": "metadata.labels.-ok"}, true, "a bare key starting with a hyphen"},
			{nil, true, "no spec at all must not crash the expression"},
			{map[string]interface{}{}, true, "spec present, location absent"},
		})
	})

	It("deny-invalid-assignmetadata-value.deckhouse.io accepts only a string value", func() {
		runTable("deny-invalid-assignmetadata-value.deckhouse.io", []objectCase{
			{assign(map[string]interface{}{"value": "admin"}), true, "a string value"},
			{assign(map[string]interface{}{"value": 123}), false, "a number"},
			{assign(map[string]interface{}{"value": true}), false, "a boolean"},
			{assign(map[string]interface{}{"value": map[string]interface{}{"a": 1}}), false, "an object"},
			{assign(map[string]interface{}{}), true, "value absent"},
			{nil, true, "no spec at all must not crash the expression"},
			{map[string]interface{}{}, true, "spec present, parameters absent"},
		})
	})

	It("deny-invalid-mutator-frommetadata-field.deckhouse.io requires field once fromMetadata is set", func() {
		runTable("deny-invalid-mutator-frommetadata-field.deckhouse.io", []objectCase{
			{assign(map[string]interface{}{"fromMetadata": map[string]interface{}{"field": "namespace"}}), true, "namespace"},
			{assign(map[string]interface{}{"fromMetadata": map[string]interface{}{"field": "name"}}), true, "name"},
			{assign(map[string]interface{}{"fromMetadata": map[string]interface{}{"field": "uid"}}), false, "not in the enum"},
			{assign(map[string]interface{}{"fromMetadata": map[string]interface{}{}}), false, "fromMetadata set but field missing"},
			{assign(map[string]interface{}{"fromMetadata": map[string]interface{}{"field": ""}}), false, "field set to an empty string"},
			{assign(map[string]interface{}{}), true, "fromMetadata absent entirely"},
			{nil, true, "no spec at all must not crash the expression"},
			{map[string]interface{}{}, true, "spec present, parameters absent"},
		})
	})

	It("deny-invalid-assignmetadata-externaldata-datasource.deckhouse.io accepts only Username", func() {
		runTable("deny-invalid-assignmetadata-externaldata-datasource.deckhouse.io", []objectCase{
			{assign(map[string]interface{}{"externalData": map[string]interface{}{"dataSource": "Username"}}), true, "Username"},
			{assign(map[string]interface{}{"externalData": map[string]interface{}{"dataSource": "ValueAtLocation"}}), false,
				"the CRD's own default, still rejected for AssignMetadata"},
			{assign(map[string]interface{}{"externalData": map[string]interface{}{}}), true, "dataSource absent"},
			{assign(map[string]interface{}{}), true, "externalData absent entirely"},
			{nil, true, "no spec at all must not crash the expression"},
			{map[string]interface{}{}, true, "spec present, parameters absent"},
		})
	})

	It("deny-mutator-without-match-kinds.deckhouse.io requires every entry to resolve to a concrete resource", func() {
		runTable("deny-mutator-without-match-kinds.deckhouse.io", []objectCase{
			{map[string]interface{}{"match": map[string]interface{}{
				"kinds": []interface{}{matchKind([]string{"apps"}, []string{"Deployment"})},
			}}, true, "a fully specified entry"},
			{map[string]interface{}{"match": map[string]interface{}{
				"kinds": []interface{}{matchKind(nil, nil)},
			}}, false, "an entry with neither apiGroups nor kinds"},
			{map[string]interface{}{"match": map[string]interface{}{
				"kinds": []interface{}{matchKind([]string{"*"}, nil)},
			}}, false, "apiGroups without kinds"},
			{map[string]interface{}{"match": map[string]interface{}{
				"kinds": []interface{}{matchKind(nil, []string{"Pod"})},
			}}, false, "kinds without apiGroups"},
			{map[string]interface{}{"match": map[string]interface{}{
				"kinds": []interface{}{matchKind([]string{"*"}, []string{"*"})},
			}}, false, "a wildcard kind name, which findGVKsForWildcard cannot resolve to one Kind"},
			{map[string]interface{}{"match": map[string]interface{}{
				"kinds": []interface{}{matchKind([]string{""}, []string{"*"})},
			}}, false, "core group with a wildcard kind name"},
			{map[string]interface{}{"match": map[string]interface{}{
				"kinds": []interface{}{
					matchKind([]string{""}, []string{"Pod"}),
					matchKind(nil, []string{"Pod"}),
				},
			}}, false, "one well-formed entry does not excuse a second, malformed one"},
			{map[string]interface{}{"match": map[string]interface{}{"namespaces": []interface{}{"test"}}}, false, "kinds missing entirely"},
			{map[string]interface{}{"match": map[string]interface{}{"kinds": []interface{}{}}}, false, "kinds present but empty"},
			{map[string]interface{}{}, false, "match missing entirely"},
			{nil, false, "no spec at all: still denied, and must not crash the expression"},
		})
	})
})

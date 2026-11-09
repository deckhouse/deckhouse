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
	"os"
	"path/filepath"

	"github.com/google/cel-go/cel"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apiserver/pkg/cel/environment"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

// The expressions of a rendered ValidatingAdmissionPolicy are evaluated in the base CEL environment of
// kube-apiserver at the oldest Kubernetes version the release supports: with failurePolicy Fail, an
// expression that does not compile there would deny every request the policy matches.

// oldestKubernetesVersion is the lowest Kubernetes version of candi/version_map.yml, the oldest
// cluster a release from this branch runs on.
func oldestKubernetesVersion() *version.Version {
	raw, err := os.ReadFile(filepath.Join(repositoryRoot(), "candi", "version_map.yml"))
	Expect(err).ShouldNot(HaveOccurred())

	var versionMap struct {
		K8s map[string]json.RawMessage `json:"k8s"`
	}
	Expect(yaml.Unmarshal(raw, &versionMap)).To(Succeed())
	Expect(versionMap.K8s).ToNot(BeEmpty())

	var oldest *version.Version
	for supported := range versionMap.K8s {
		parsed, err := version.ParseGeneric(supported)
		Expect(err).ShouldNot(HaveOccurred(), "Kubernetes version %q", supported)
		if oldest == nil || parsed.LessThan(oldest) {
			oldest = parsed
		}
	}
	return oldest
}

// admissionPolicyVerdict is what kube-apiserver does with a request to a rendered
// ValidatingAdmissionPolicy: a request a match condition rejects is not validated, and a validated one
// is denied with the message of the first validation that does not hold. An expression that fails to
// evaluate fails the test, where kube-apiserver would apply the failure policy of the policy to it.
func admissionPolicyVerdict(policy object_store.KubeObject, request, oldObject, newObject map[string]any) (verdict, string) {
	Expect(policy.Exists()).To(BeTrue(), "the policy should be rendered")

	env, err := environment.MustBaseEnvSet(oldestKubernetesVersion()).Env(environment.NewExpressions)
	Expect(err).ShouldNot(HaveOccurred())
	env, err = env.Extend(
		cel.Variable("object", cel.DynType),
		cel.Variable("oldObject", cel.DynType),
		cel.Variable("request", cel.DynType),
	)
	Expect(err).ShouldNot(HaveOccurred())

	activation := map[string]any{"request": request, "object": celValue(newObject), "oldObject": celValue(oldObject)}
	holds := func(expression string) bool {
		ast, issues := env.Compile(expression)
		Expect(issues.Err()).ShouldNot(HaveOccurred(), "the expression should compile: %s", expression)
		Expect(ast.OutputType()).To(Equal(cel.BoolType), "the expression should be a condition: %s", expression)
		program, err := env.Program(ast)
		Expect(err).ShouldNot(HaveOccurred())
		out, _, err := program.Eval(activation)
		Expect(err).ShouldNot(HaveOccurred(), "the expression should evaluate: %s", expression)
		return out.Value() == true
	}

	for _, condition := range policy.Field("spec.matchConditions").Array() {
		if !holds(condition.Get("expression").String()) {
			return verdictSkipped, ""
		}
	}
	for _, validation := range policy.Field("spec.validations").Array() {
		if !holds(validation.Get("expression").String()) {
			return verdictDenied, validation.Get("message").String()
		}
	}
	return verdictAllowed, ""
}

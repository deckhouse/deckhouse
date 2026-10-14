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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// The gator suites check a ConstraintTemplate against the constraint stored in
// constraints/pss_<standard>.yaml of a test directory. If that fixture carries
// other parameters than the module renders for the Pod Security Standards, the
// suite tests a policy that never runs in a cluster, and a hole in the real one
// goes unnoticed. This test keeps the fixtures equal to the rendered constraints.
var _ = Describe("Module :: admissionPolicyEngine :: pod security standards test fixtures ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("admissionPolicyEngine", `
podSecurityStandards:
  defaultPolicy: Privileged
  enforcementAction: Deny
internal:
  podSecurityStandards:
    enforcementActions:
      - deny
  bootstrapped: true
  ratify:
    webhook:
      ca: test-ratify-webhook-ca
      crt: test-ratify-webhook-crt
      key: test-ratify-webhook-key
  webhook:
    ca: test-webhook-ca
    crt: test-webhook-crt
    key: test-webhook-key
  trackedConstraintResources: []
  trackedMutateResources: []
`)
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.HelmRender()
	})

	It("PSS constraint fixtures must carry the parameters the module renders", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		testsRoot := filepath.Join(moduleRootDir(), "charts", "constraint-templates", "tests", "test_cases", "constraints")
		var fixtures []string
		for _, standard := range []string{"baseline", "restricted"} {
			matches, err := filepath.Glob(filepath.Join(testsRoot, "*", "*", "constraints", "pss_"+standard+".yaml"))
			Expect(err).ShouldNot(HaveOccurred())
			fixtures = append(fixtures, matches...)
		}
		Expect(fixtures).NotTo(BeEmpty())

		var mismatches []string
		for _, fixture := range fixtures {
			standard := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(fixture), "pss_"), ".yaml")
			rel, err := filepath.Rel(testsRoot, fixture)
			Expect(err).ShouldNot(HaveOccurred())

			data, err := os.ReadFile(fixture)
			Expect(err).ShouldNot(HaveOccurred())
			var doc struct {
				Kind string `json:"kind"`
				Spec struct {
					Parameters map[string]interface{} `json:"parameters"`
				} `json:"spec"`
			}
			Expect(yaml.Unmarshal(data, &doc)).To(Succeed(), "parse %s", rel)

			rendered := f.KubernetesGlobalResource(doc.Kind, "d8-pod-security-"+standard+"-deny-default")
			if !rendered.Exists() {
				mismatches = append(mismatches, fmt.Sprintf("%s: the module renders no %s constraint for the %s standard", rel, doc.Kind, standard))
				continue
			}

			expected := "{}"
			if p := rendered.Field("spec.parameters"); p.Exists() {
				expected = p.Raw
			}
			actual := []byte("{}")
			if doc.Spec.Parameters != nil {
				actual, err = json.Marshal(doc.Spec.Parameters)
				Expect(err).ShouldNot(HaveOccurred())
			}
			if ok, _ := MatchJSON(expected).Match(string(actual)); !ok {
				mismatches = append(mismatches, fmt.Sprintf(
					"%s: spec.parameters differ from the %s constraint rendered by templates/policies/pod-security-standards/%s/constraint.yaml\n    fixture:  %s\n    rendered: %s",
					rel, doc.Kind, standard, actual, expected))
			}
		}
		if len(mismatches) > 0 {
			Fail("PSS constraint fixtures drifted from the module:\n  " + strings.Join(mismatches, "\n  "))
		}
	})
})

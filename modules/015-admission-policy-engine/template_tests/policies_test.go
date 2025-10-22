/*
Copyright 2022 Flant JSC

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
	"fmt"
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: admissionPolicyEngine :: pod security policies ::", func() {
	var gatorPath string
	var gatorFound bool
	f := SetupHelmConfig(`
admissionPolicyEngine:
  internal:
    ratify:
      webhook:
        ca: YjY0ZW5jX3N0cmluZwo=
        crt: YjY0ZW5jX3N0cmluZwo=
        key: YjY0ZW5jX3N0cmluZwo=
    podSecurityStandards:
      enforcementActions:
      - deny
    bootstrapped: true
    webhook:
      ca: YjY0ZW5jX3N0cmluZwo=
      crt: YjY0ZW5jX3N0cmluZwo=
      key: YjY0ZW5jX3N0cmluZwo=
    trackedConstraintResources: []
    trackedMutateResources: []
  denyVulnerableImages: {}
  podSecurityStandards:
    policies:
      hostPorts:
        knownRanges:
          - max: 35000
            min: 30000
          - max: 44000
            min: 42000
`)

	Context("Test rego policies", func() {
		BeforeEach(func() {
			if gatorPath, gatorFound = gatorAvailable(); !gatorFound {
				Skip("gator binary is not available")
			}

			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.HelmRender()
		})

		It("Rego policy test must have passed", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			gatorCLI := exec.Command(gatorPath, "verify", "-v", "../charts/constraint-templates/tests/...")
			res, err := gatorCLI.Output()
			if err != nil {
				output := strings.ReplaceAll(string(res), "modules/015-admission-policy-engine/charts/constraint-templates", "...")
				fmt.Println(output)
				Fail("Gatekeeper policy tests failed:" + err.Error())
			}
		})
	})
})

func gatorAvailable() (string, bool) {
	gatorPath, err := exec.LookPath("gator")
	if err != nil {
		return "", false
	}

	info, err := os.Lstat(gatorPath)
	return gatorPath, err == nil && (info.Mode().Perm()&0111 != 0)
}

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
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: admissionPolicyEngine :: pod security policies ::", func() {
	f := SetupHelmConfig(`
admissionPolicyEngine:
  internal:
    bootstrapped: true
    webhook:
      ca: YjY0ZW5jX3N0cmluZwo=
      crt: YjY0ZW5jX3N0cmluZwo=
      key: YjY0ZW5jX3N0cmluZwo=
    trackedConstraintResources: []
    trackedMutateResources: []
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
		renderedOutput := make(map[string]string)
		BeforeEach(func() {
			if !gatorAvailable() {
				Skip("gator binary is not available")
			}

			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.HelmRender(WithFilteredRenderOutput(renderedOutput, "admission-policy-engine/templates/policies/"))
		})

		It("Rego policy test must have passed", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			tmpDir, err := os.MkdirTemp("", "policy-*")
			defer os.RemoveAll(tmpDir)
			Expect(err).To(BeNil())

			for filePath, content := range renderedOutput {
				newPath := path.Join(tmpDir, filePath)
				_ = os.MkdirAll(path.Dir(newPath), 0755)
				_ = os.WriteFile(newPath, []byte(content), 0444)
			}
			_ = filepath.Walk("../templates/policies", func(fpath string, info fs.FileInfo, err error) error {
				if strings.HasSuffix(fpath, "test_suite.yaml") || strings.Contains(fpath, "/test_samples/") {
					newPath := path.Join(tmpDir, strings.Replace(fpath, "../", "admission-policy-engine/", 1))
					_ = os.MkdirAll(path.Dir(newPath), 0755)
					input, _ := os.ReadFile(fpath)
					_ = os.WriteFile(newPath, input, 0644)
				}
				return nil
			})
			gatorCLI := exec.Command("/deckhouse/bin/gator", "verify", "-v", path.Join(tmpDir, "..."))
			res, err := gatorCLI.Output()
			if err != nil {
				output := strings.ReplaceAll(string(res), strings.TrimPrefix(path.Join(tmpDir, "admission-policy-engine"), "/"), "")
				fmt.Println(output)
				Fail("Gatekeeper policy tests failed:" + err.Error())
			}
		})
	})
})

const gatorPath = "/deckhouse/bin/gator"

func gatorAvailable() bool {
	info, err := os.Lstat(gatorPath)
	return err == nil && (info.Mode().Perm()&0111 != 0)
}

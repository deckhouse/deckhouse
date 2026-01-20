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
	"gopkg.in/yaml.v3"

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
			res, err := gatorCLI.CombinedOutput()
			if err != nil {
				output := strings.ReplaceAll(string(res), "modules/015-admission-policy-engine/charts/constraint-templates", "...")
				fmt.Println(output)
				Fail("Gatekeeper policy tests failed:" + err.Error())
			}
		})
	})

	Context("Pod security standards constraints YAML validation", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.HelmRender()
		})

		It("All pod security standards baseline constraints must have valid YAML", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			baselineConstraints := []string{
				"D8HostNetwork",
				"D8HostProcesses",
				"D8AppArmor",
				"D8AllowedCapabilities",
				"D8AllowedHostPaths",
				"D8PrivilegedContainer",
				"D8AllowedProcMount",
				"D8SeLinux",
				"D8AllowedSysctls",
				"D8AllowedSeccompProfiles",
			}

			for _, constraintKind := range baselineConstraints {
				constraint := f.KubernetesGlobalResource(constraintKind, "d8-pod-security-baseline-deny-default")
				if constraint.Exists() {
					// Get the resource as a map to validate YAML structure
					var resourceMap map[string]interface{}
					err := yaml.Unmarshal([]byte(constraint.ToYaml()), &resourceMap)
					if err != nil {
						Fail(fmt.Sprintf("Invalid YAML for resource %s: %v\nYAML content:\n%s", constraintKind, err, constraint.ToYaml()))
					}
					validateYAML(resourceMap, constraintKind)
				}
			}
		})

		It("All pod security standards restricted constraints must have valid YAML", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			restrictedConstraints := []string{
				"D8AllowedCapabilities",
				"D8AllowPrivilegeEscalation",
				"D8AllowedVolumeTypes",
				"D8AllowedUsers",
				"D8AllowedSeccompProfiles",
			}

			for _, constraintKind := range restrictedConstraints {
				constraint := f.KubernetesGlobalResource(constraintKind, "d8-pod-security-restricted-deny-default")
				if constraint.Exists() {
					// Get the resource as a map to validate YAML structure
					var resourceMap map[string]interface{}
					err := yaml.Unmarshal([]byte(constraint.ToYaml()), &resourceMap)
					if err != nil {
						Fail(fmt.Sprintf("Invalid YAML for resource %s: %v\nYAML content:\n%s", constraintKind, err, constraint.ToYaml()))
					}
					validateYAML(resourceMap, constraintKind)
				}
			}
		})
	})

	Context("Pod security standards with explicit defaultPolicy: Privileged and enforcementAction: Deny", func() {
		BeforeEach(func() {
			f.ValuesSet("admissionPolicyEngine.podSecurityStandards.defaultPolicy", "Privileged")
			f.ValuesSet("admissionPolicyEngine.podSecurityStandards.enforcementAction", "Deny")
			f.ValuesSetFromYaml("admissionPolicyEngine.internal.podSecurityStandards.enforcementActions", `["deny"]`)
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.HelmRender()
		})

		It("All pod security standards baseline constraints must have valid YAML with defaultPolicy: Privileged", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			baselineConstraints := []string{
				"D8HostNetwork",
				"D8HostProcesses",
				"D8AppArmor",
				"D8AllowedCapabilities",
				"D8AllowedHostPaths",
				"D8PrivilegedContainer",
				"D8AllowedProcMount",
				"D8SeLinux",
				"D8AllowedSysctls",
				"D8AllowedSeccompProfiles",
			}

			for _, constraintKind := range baselineConstraints {
				constraint := f.KubernetesGlobalResource(constraintKind, "d8-pod-security-baseline-deny-default")
				if constraint.Exists() {
					// Get the resource as a map to validate YAML structure
					var resourceMap map[string]interface{}
					err := yaml.Unmarshal([]byte(constraint.ToYaml()), &resourceMap)
					if err != nil {
						Fail(fmt.Sprintf("Invalid YAML for resource %s: %v\nYAML content:\n%s", constraintKind, err, constraint.ToYaml()))
					}
					validateYAML(resourceMap, constraintKind)
				}
			}
		})

		It("All pod security standards restricted constraints must have valid YAML with defaultPolicy: Privileged", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			restrictedConstraints := []string{
				"D8AllowedCapabilities",
				"D8AllowPrivilegeEscalation",
				"D8AllowedVolumeTypes",
				"D8AllowedUsers",
				"D8AllowedSeccompProfiles",
			}

			for _, constraintKind := range restrictedConstraints {
				constraint := f.KubernetesGlobalResource(constraintKind, "d8-pod-security-restricted-deny-default")
				if constraint.Exists() {
					// Get the resource as a map to validate YAML structure
					var resourceMap map[string]interface{}
					err := yaml.Unmarshal([]byte(constraint.ToYaml()), &resourceMap)
					if err != nil {
						Fail(fmt.Sprintf("Invalid YAML for resource %s: %v\nYAML content:\n%s", constraintKind, err, constraint.ToYaml()))
					}
					validateYAML(resourceMap, constraintKind)
				}
			}
		})
	})

	// ============================================================================
	// TODO: REMOVE THIS SECTION AFTER REMOVING d8-prefixed constraints from templates
	// ============================================================================
	// This section tests constraints with "-d8" suffix that are generated
	// when defaultPolicy != "privileged" (for baseline) or != "restricted" (for restricted).
	// These constraints are marked for removal in templates/_helpers.tpl with TODO comment.
	// After removing the template code that generates these constraints, this test section
	// should be deleted as well.
	// ============================================================================
	Context("Pod security standards constraints with -d8 suffix (temporary, for removal)", func() {
		BeforeEach(func() {
			// Set defaultPolicy to Baseline to trigger generation of -d8 constraints for baseline
			// Set defaultPolicy to Privileged to trigger generation of -d8 constraints for restricted
			f.ValuesSet("admissionPolicyEngine.podSecurityStandards.defaultPolicy", "Baseline")
			f.ValuesSet("admissionPolicyEngine.podSecurityStandards.enforcementAction", "Deny")
			f.ValuesSetFromYaml("admissionPolicyEngine.internal.podSecurityStandards.enforcementActions", `["deny"]`)
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.HelmRender()
		})

		It("All pod security standards baseline constraints with -d8-default suffix must have valid YAML", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			baselineConstraints := []string{
				"D8HostNetwork",
				"D8HostProcesses",
				"D8AppArmor",
				"D8AllowedCapabilities",
				"D8AllowedHostPaths",
				"D8PrivilegedContainer",
				"D8AllowedProcMount",
				"D8SeLinux",
				"D8AllowedSysctls",
				"D8AllowedSeccompProfiles",
			}

			for _, constraintKind := range baselineConstraints {
				// Check constraint with -d8-default suffix (when policyAction matches default enforcement action)
				constraint := f.KubernetesGlobalResource(constraintKind, "d8-pod-security-baseline-deny-d8-default")
				if constraint.Exists() {
					var resourceMap map[string]interface{}
					err := yaml.Unmarshal([]byte(constraint.ToYaml()), &resourceMap)
					if err != nil {
						Fail(fmt.Sprintf("Invalid YAML for resource %s (d8-default): %v\nYAML content:\n%s", constraintKind, err, constraint.ToYaml()))
					}
					validateYAML(resourceMap, fmt.Sprintf("%s (d8-default)", constraintKind))
				}
			}
		})

		It("All pod security standards restricted constraints with -d8-default suffix must have valid YAML", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			restrictedConstraints := []string{
				"D8AllowedCapabilities",
				"D8AllowPrivilegeEscalation",
				"D8AllowedVolumeTypes",
				"D8AllowedUsers",
				"D8AllowedSeccompProfiles",
			}

			for _, constraintKind := range restrictedConstraints {
				// Check constraint with -d8-default suffix (when policyAction matches default enforcement action)
				constraint := f.KubernetesGlobalResource(constraintKind, "d8-pod-security-restricted-deny-d8-default")
				if constraint.Exists() {
					var resourceMap map[string]interface{}
					err := yaml.Unmarshal([]byte(constraint.ToYaml()), &resourceMap)
					if err != nil {
						Fail(fmt.Sprintf("Invalid YAML for resource %s (d8-default): %v\nYAML content:\n%s", constraintKind, err, constraint.ToYaml()))
					}
					validateYAML(resourceMap, fmt.Sprintf("%s (d8-default)", constraintKind))
				}
			}
		})
	})

	Context("Pod security standards constraints with -d8 suffix (non-default action, temporary, for removal)", func() {
		BeforeEach(func() {
			// Set defaultPolicy to Baseline to trigger generation of -d8 constraints for baseline
			// Set enforcementAction to "warn" (non-default) to test -d8 suffix (without -default)
			f.ValuesSet("admissionPolicyEngine.podSecurityStandards.defaultPolicy", "Baseline")
			f.ValuesSet("admissionPolicyEngine.podSecurityStandards.enforcementAction", "Deny")
			f.ValuesSetFromYaml("admissionPolicyEngine.internal.podSecurityStandards.enforcementActions", `["deny", "warn"]`)
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.HelmRender()
		})

		It("All pod security standards baseline constraints with -d8 suffix (non-default) must have valid YAML", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			baselineConstraints := []string{
				"D8HostNetwork",
				"D8HostProcesses",
				"D8AppArmor",
				"D8AllowedCapabilities",
				"D8AllowedHostPaths",
				"D8PrivilegedContainer",
				"D8AllowedProcMount",
				"D8SeLinux",
				"D8AllowedSysctls",
				"D8AllowedSeccompProfiles",
			}

			for _, constraintKind := range baselineConstraints {
				// Check constraint with -d8 suffix (when policyAction doesn't match default enforcement action)
				constraint := f.KubernetesGlobalResource(constraintKind, "d8-pod-security-baseline-warn-d8")
				if constraint.Exists() {
					var resourceMap map[string]interface{}
					err := yaml.Unmarshal([]byte(constraint.ToYaml()), &resourceMap)
					if err != nil {
						Fail(fmt.Sprintf("Invalid YAML for resource %s (d8): %v\nYAML content:\n%s", constraintKind, err, constraint.ToYaml()))
					}
					validateYAML(resourceMap, fmt.Sprintf("%s (d8)", constraintKind))
				}
			}
		})

		It("All pod security standards restricted constraints with -d8 suffix (non-default) must have valid YAML", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			restrictedConstraints := []string{
				"D8AllowedCapabilities",
				"D8AllowPrivilegeEscalation",
				"D8AllowedVolumeTypes",
				"D8AllowedUsers",
				"D8AllowedSeccompProfiles",
			}

			for _, constraintKind := range restrictedConstraints {
				// Check constraint with -d8 suffix (when policyAction doesn't match default enforcement action)
				constraint := f.KubernetesGlobalResource(constraintKind, "d8-pod-security-restricted-warn-d8")
				if constraint.Exists() {
					var resourceMap map[string]interface{}
					err := yaml.Unmarshal([]byte(constraint.ToYaml()), &resourceMap)
					if err != nil {
						Fail(fmt.Sprintf("Invalid YAML for resource %s (d8): %v\nYAML content:\n%s", constraintKind, err, constraint.ToYaml()))
					}
					validateYAML(resourceMap, fmt.Sprintf("%s (d8)", constraintKind))
				}
			}
		})
	})
	// ============================================================================
	// END OF TEMPORARY SECTION - REMOVE AFTER REMOVING d8-prefixed constraints
	// ============================================================================
})

func gatorAvailable() (string, bool) {
	gatorPath, err := exec.LookPath("gator")
	if err != nil {
		return "", false
	}

	info, err := os.Lstat(gatorPath)
	return gatorPath, err == nil && (info.Mode().Perm()&0o111 != 0)
}

// validateYAML checks if the constraint resource has valid YAML structure
// by attempting to marshal it back to YAML
func validateYAML(constraint interface{}, resourceName string) {
	// Try to marshal the resource to YAML to validate its structure
	yamlBytes, err := yaml.Marshal(constraint)
	if err != nil {
		Fail(fmt.Sprintf("Failed to marshal resource %s to YAML: %v", resourceName, err))
	}

	// Try to unmarshal it back to validate it's valid YAML
	var result interface{}
	err = yaml.Unmarshal(yamlBytes, &result)
	if err != nil {
		Fail(fmt.Sprintf("Invalid YAML for resource %s: %v\nYAML content:\n%s", resourceName, err, string(yamlBytes)))
	}
}

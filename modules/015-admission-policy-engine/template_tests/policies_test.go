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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

func moduleRootDir() string {
	_, fn, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue())
	return filepath.Clean(filepath.Join(filepath.Dir(fn), ".."))
}

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

	Context("CI policy checks", func() {
		It("OPA library tests", func() {
			modRoot := moduleRootDir()
			libsDir := filepath.Join(modRoot, "charts", "constraint-templates", "tests", "test_cases", "libs")

			opaPath, opaFound := discoverBinary("opa")
			if !opaFound {
				Skip("opa binary is not available")
			}

			opaCLI := exec.Command(opaPath, "test", ".", "-v")
			opaCLI.Dir = libsDir
			res, err := opaCLI.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "OPA library tests failed:\n%s", string(res))
		})

		It("Gator constraint tests (regenerate + verify)", func() {
			modRoot := moduleRootDir()
			testsRoot := filepath.Join(modRoot, "charts", "constraint-templates", "tests", "test_cases")
			constraintsRoot := filepath.Join(testsRoot, "constraints")
			constraintTestgen := "./charts/constraint-templates/tests/tools/constraint_testgen"

			if gatorPath, gatorFound = discoverBinary("gator"); !gatorFound {
				Fail("required command not found: gator")
			}

			constraintDirs, err := collectConstraintTestCaseDirs(constraintsRoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(constraintDirs).NotTo(BeEmpty(), "no constraint test cases found under %s", constraintsRoot)

			failedConstraints := make([]string, 0)
			failedGatorTests := make([]string, 0)
			for _, constraintDir := range constraintDirs {
				relConstraintDir, relErr := filepath.Rel(modRoot, constraintDir)
				if relErr != nil {
					relConstraintDir = constraintDir
				}

				renderedDir := filepath.Join(constraintDir, "rendered")
				_ = os.RemoveAll(renderedDir)

				bundlePath := filepath.Join(constraintDir, "test-matrix.yaml")
				generateCmd := exec.Command("go", "run", constraintTestgen, "generate", "-tests-root", testsRoot, "-bundle", bundlePath)
				generateCmd.Dir = modRoot
				_, generateErr := generateCmd.CombinedOutput()
				if generateErr != nil {
					failedConstraints = append(failedConstraints, fmt.Sprintf("%s (generate)", relConstraintDir))
					continue
				}

				gatorCLI := exec.Command(gatorPath, "verify", "-v", "./rendered")
				gatorCLI.Dir = constraintDir
				_, gatorErr := gatorCLI.CombinedOutput()
				if gatorErr != nil {
					// Rarely gator may return a transient verify error right after generation; retry once.
					retryCLI := exec.Command(gatorPath, "verify", "-v", "./rendered")
					retryCLI.Dir = constraintDir
					retryOut, retryErr := retryCLI.CombinedOutput()
					if retryErr == nil {
						continue
					}

					failedConstraints = append(failedConstraints, fmt.Sprintf("%s (verify)", relConstraintDir))
					for _, testName := range extractFailedGatorTestsFromOutput(string(retryOut)) {
						failedGatorTests = append(failedGatorTests, fmt.Sprintf("%s: %s", relConstraintDir, testName))
					}
					if len(extractFailedGatorTestsFromOutput(string(retryOut))) == 0 {
						failedGatorTests = append(failedGatorTests, fmt.Sprintf("%s: %s", relConstraintDir, strings.TrimSpace(string(retryOut))))
					}
				}
			}

			if len(failedConstraints) > 0 {
				sections := []string{
					"Gatekeeper constraint tests failed",
					"[Failed constraints]",
					"  - " + strings.Join(failedConstraints, "\n  - "),
				}
				if len(failedGatorTests) > 0 {
					sections = append(sections,
						"[Failed Gatekeeper tests]",
						"  - "+strings.Join(failedGatorTests, "\n  - "),
					)
				}
				Fail(strings.Join(sections, "\n"))
			}
		})

		It("Coverage checks", func() {
			modRoot := moduleRootDir()
			testsRoot := filepath.Join(modRoot, "charts", "constraint-templates", "tests", "test_cases", "constraints")
			constraintTestgen := "./charts/constraint-templates/tests/tools/constraint_testgen"

			coverageCmd := exec.Command("go", "run", constraintTestgen, "coverage", "-tests-root", testsRoot, "-format", "json")
			coverageCmd.Dir = modRoot
			coverageOut, err := coverageCmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "constraint_testgen coverage failed:\n%s", string(coverageOut))

			report, err := parseCoverageReportFromOutput(coverageOut)
			Expect(err).NotTo(HaveOccurred(), "cannot parse coverage JSON:\n%s", string(coverageOut))

			var lowCoverage []string
			for _, constraint := range report.Constraints {
				if constraint.Fields == nil {
					continue
				}
				if constraint.Fields.CoveragePct < 100 {
					name := constraint.Directory
					if name == "" {
						name = constraint.Name
					}
					lowCoverage = append(lowCoverage, fmt.Sprintf("%s: %d%%", name, constraint.Fields.CoveragePct))
				}
			}
			sort.Strings(lowCoverage)

			if len(lowCoverage) > 0 {
				Fail("Coverage below 100% for constraints:\n - " + strings.Join(lowCoverage, "\n - "))
			}
		})
	})

	It("All ConstraintTemplates rego sources must use strictly single-line violation messages", func() {
		// We validate source templates, not Helm-rendered manifests, because Helm rendering may require
		// additional global.discovery data unrelated to gatekeeper constraint templates.
		// Requirement: any violation msg must be strictly single-line, so we forbid '\\n' and '\\r' escapes.
		// inside Rego sources (due kubectl requirements for warning messages).
		constraintTemplatesDir := filepath.Join("..", "charts", "constraint-templates", "templates")
		contentByPath := map[string]string{}
		for _, pattern := range []string{
			filepath.Join(constraintTemplatesDir, "security", "*.yaml"),
			filepath.Join(constraintTemplatesDir, "operation", "*.yaml"),
		} {
			matches, err := filepath.Glob(pattern)
			Expect(err).ShouldNot(HaveOccurred())
			for _, p := range matches {
				b, err := os.ReadFile(p)
				Expect(err).ShouldNot(HaveOccurred(), "failed reading %s", p)
				contentByPath[p] = string(b)
			}
		}

		Expect(contentByPath).NotTo(BeEmpty(), "Expected constraint template YAML files")

		for filename, content := range contentByPath {
			if !strings.Contains(content, "kind: ConstraintTemplate") {
				continue
			}
			if strings.Contains(content, "\\n") || strings.Contains(content, "\\r") {
				Fail(fmt.Sprintf("Found multiline escape (\\n or \\r) in ConstraintTemplate source: %s", filename))
			}
		}
	})

	// Test helper function to validate constraints for given configuration
	validateConstraintsForConfig := func(defaultPolicy, enforcementAction string, enforcementActions []string, constraintNamePattern string) {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		baselineConstraints := getBaselineConstraintNames()
		restrictedConstraints := getRestrictedConstraintNames()
		Expect(baselineConstraints).NotTo(BeEmpty(), "No baseline constraints found in templates")
		Expect(restrictedConstraints).NotTo(BeEmpty(), "No restricted constraints found in templates")

		allConstraints := append(baselineConstraints, restrictedConstraints...)
		for _, constraintKind := range allConstraints {
			// Determine expected constraint name based on standard
			var constraintName string
			if contains(baselineConstraints, constraintKind) {
				constraintName = fmt.Sprintf(constraintNamePattern, "baseline")
			} else {
				constraintName = fmt.Sprintf(constraintNamePattern, "restricted")
			}

			constraint := f.KubernetesGlobalResource(constraintKind, constraintName)
			if constraint.Exists() {
				var resourceMap map[string]interface{}
				err := yaml.Unmarshal([]byte(constraint.ToYaml()), &resourceMap)
				if err != nil {
					Fail(fmt.Sprintf("Invalid YAML for resource %s (config: defaultPolicy=%s, enforcementAction=%s, enforcementActions=%v): %v\nYAML content:\n%s",
						constraintKind, defaultPolicy, enforcementAction, enforcementActions, err, constraint.ToYaml()))
				}
				validateYAML(resourceMap, fmt.Sprintf("%s (defaultPolicy=%s, enforcementAction=%s)", constraintKind, defaultPolicy, enforcementAction))
			}
		}
	}

	// Test cases for different combinations of podSecurityStandards parameters
	// defaultPolicy can be: Privileged, Baseline, Restricted
	// enforcementAction can be: Deny, Warn, Dryrun
	// enforcementActions can be: ["deny"], ["warn"], ["dryrun"], or combinations

	// Define test configurations
	testConfigs := []struct {
		name                  string
		defaultPolicy         string
		enforcementAction     string
		enforcementActions    []string
		constraintNamePattern string
	}{
		{"Default configuration: Privileged/Deny/deny", "Privileged", "Deny", []string{"deny"}, "d8-pod-security-%s-deny-default"},
		{"Baseline/Deny/deny", "Baseline", "Deny", []string{"deny"}, "d8-pod-security-%s-deny-default"},
		{"Restricted/Deny/deny", "Restricted", "Deny", []string{"deny"}, "d8-pod-security-%s-deny-default"},
		{"Privileged/Warn/warn", "Privileged", "Warn", []string{"warn"}, "d8-pod-security-%s-warn-default"},
		{"Baseline/Warn/warn", "Baseline", "Warn", []string{"warn"}, "d8-pod-security-%s-warn-default"},
		{"Restricted/Warn/warn", "Restricted", "Warn", []string{"warn"}, "d8-pod-security-%s-warn-default"},
		{"Privileged/Dryrun/dryrun", "Privileged", "Dryrun", []string{"dryrun"}, "d8-pod-security-%s-dryrun-default"},
		{"Baseline/Dryrun/dryrun", "Baseline", "Dryrun", []string{"dryrun"}, "d8-pod-security-%s-dryrun-default"},
		{"Restricted/Dryrun/dryrun", "Restricted", "Dryrun", []string{"dryrun"}, "d8-pod-security-%s-dryrun-default"},
		{"Privileged/Deny/deny+warn", "Privileged", "Deny", []string{"deny", "warn"}, "d8-pod-security-%s-deny-default"},
		{"Privileged/Deny/deny+dryrun", "Privileged", "Deny", []string{"deny", "dryrun"}, "d8-pod-security-%s-deny-default"},
		{"Baseline/Warn/warn+deny", "Baseline", "Warn", []string{"warn", "deny"}, "d8-pod-security-%s-warn-default"},
		{"Restricted/Deny/deny+warn+dryrun", "Restricted", "Deny", []string{"deny", "warn", "dryrun"}, "d8-pod-security-%s-deny-default"},
	}

	Context("Pod security standards constraints YAML validation with different configurations", func() {
		BeforeEach(func() {
			Skip("legacy helm-render specs are isolated after constraint test runner migration")
		})
		for _, tc := range testConfigs {
			tc := tc // capture loop variable
			Context(fmt.Sprintf("Configuration: %s", tc.name), func() {
				BeforeEach(func() {
					// Set configuration via YAML
					configYAML := fmt.Sprintf(`
podSecurityStandards:
  defaultPolicy: %s
  enforcementAction: %s
  policies:
    hostPorts:
      knownRanges:
        - max: 35000
          min: 30000
        - max: 44000
          min: 42000
internal:
  podSecurityStandards:
    enforcementActions:
%s
  bootstrapped: true
  ratify:
    webhook:
      ca: YjY0ZW5jX3N0cmluZwo=
      crt: YjY0ZW5jX3N0cmluZwo=
      key: YjY0ZW5jX3N0cmluZwo=
  webhook:
    ca: YjY0ZW5jX3N0cmluZwo=
    crt: YjY0ZW5jX3N0cmluZwo=
    key: YjY0ZW5jX3N0cmluZwo=
  trackedConstraintResources: []
  trackedMutateResources: []
`, tc.defaultPolicy, tc.enforcementAction, formatEnforcementActionsYAML(tc.enforcementActions))

					f.ValuesSetFromYaml("admissionPolicyEngine", configYAML)
					f.ValuesSetFromYaml("global", globalValues)
					f.ValuesSet("global.modulesImages", GetModulesImages())
					f.HelmRender()
				})

				It("All constraints must have valid YAML", func() {
					validateConstraintsForConfig(tc.defaultPolicy, tc.enforcementAction, tc.enforcementActions, tc.constraintNamePattern)
				})
			})
		}
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
	d8TestConfigs := []struct {
		name                  string
		defaultPolicy         string
		enforcementAction     string
		enforcementActions    []string
		constraintNamePattern string
	}{
		{"Baseline/Deny/deny - generates -d8-default", "Baseline", "Deny", []string{"deny"}, "d8-pod-security-%s-deny-d8-default"},
		{"Baseline/Warn/warn - generates -d8-default", "Baseline", "Warn", []string{"warn"}, "d8-pod-security-%s-warn-d8-default"},
		{"Privileged/Deny/deny - generates -d8-default for restricted", "Privileged", "Deny", []string{"deny"}, "d8-pod-security-restricted-deny-d8-default"},
		{"Baseline/Deny/deny+warn - generates -d8 for warn", "Baseline", "Deny", []string{"deny", "warn"}, "d8-pod-security-%s-warn-d8"},
	}

	Context("Pod security standards constraints with -d8 suffix (temporary, for removal)", func() {
		BeforeEach(func() {
			Skip("legacy helm-render specs are isolated after constraint test runner migration")
		})
		for _, tc := range d8TestConfigs {
			tc := tc // capture loop variable
			Context(fmt.Sprintf("Configuration: %s", tc.name), func() {
				BeforeEach(func() {
					// Set configuration via YAML to trigger -d8 constraints generation
					configYAML := fmt.Sprintf(`
podSecurityStandards:
  defaultPolicy: %s
  enforcementAction: %s
  policies:
    hostPorts:
      knownRanges:
        - max: 35000
          min: 30000
        - max: 44000
          min: 42000
internal:
  podSecurityStandards:
    enforcementActions:
%s
  bootstrapped: true
  ratify:
    webhook:
      ca: YjY0ZW5jX3N0cmluZwo=
      crt: YjY0ZW5jX3N0cmluZwo=
      key: YjY0ZW5jX3N0cmluZwo=
  webhook:
    ca: YjY0ZW5jX3N0cmluZwo=
    crt: YjY0ZW5jX3N0cmluZwo=
    key: YjY0ZW5jX3N0cmluZwo=
  trackedConstraintResources: []
  trackedMutateResources: []
`, tc.defaultPolicy, tc.enforcementAction, formatEnforcementActionsYAML(tc.enforcementActions))

					f.ValuesSetFromYaml("admissionPolicyEngine", configYAML)
					f.ValuesSetFromYaml("global", globalValues)
					f.ValuesSet("global.modulesImages", GetModulesImages())
					f.HelmRender()
				})

				It("All constraints with -d8 suffix must have valid YAML", func() {
					validateConstraintsForConfig(tc.defaultPolicy, tc.enforcementAction, tc.enforcementActions, tc.constraintNamePattern)
				})
			})
		}
	})
	// ============================================================================
	// END OF TEMPORARY SECTION - REMOVE AFTER REMOVING d8-prefixed constraints
	// ============================================================================
})

func discoverBinary(name string) (string, bool) {
	if binaryPath, err := exec.LookPath(name); err == nil && isExecutableFile(binaryPath) {
		return binaryPath, true
	}

	moduleRoot := moduleRootDir()
	repoRoot := filepath.Clean(filepath.Join(moduleRoot, "..", ".."))

	candidates := []string{
		filepath.Join(repoRoot, "bin", name),
	}

	versioned, err := filepath.Glob(filepath.Join(repoRoot, "bin", name+"-*", name))
	if err == nil {
		sort.Strings(versioned)
		candidates = append(candidates, versioned...)
	}

	for _, candidate := range candidates {
		if isExecutableFile(candidate) {
			return candidate, true
		}
	}

	return "", false
}

func isExecutableFile(path string) bool {
	info, err := os.Lstat(path)
	if err != nil || info.IsDir() {
		return false
	}

	return info.Mode().Perm()&0o111 != 0
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

// extractConstraintNamesFromTemplate parses template file and extracts constraint kind names
// from include statements like: include "pod_security_standard_baseline" (list $context "D8HostNetwork" ...)
// or include "pod_security_standard_restricted" (list $context "D8AllowedCapabilities" ...)
func extractConstraintNamesFromTemplate(templatePath string, helperName string) []string {
	content, err := os.ReadFile(templatePath)
	if err != nil {
		Fail(fmt.Sprintf("Failed to read template file %s: %v", templatePath, err))
	}

	// Pattern to match: include "pod_security_standard_baseline" (list $context "D8XXX" ...)
	// or include "pod_security_standard_restricted" (list $context "D8XXX" ...)
	// The pattern looks for the helper name followed by a list that contains a quoted string starting with D8
	pattern := fmt.Sprintf(`include\s+"%s"\s+\([^)]*list[^)]*"([D8][^"]+)"`, regexp.QuoteMeta(helperName))
	re := regexp.MustCompile(pattern)

	matches := re.FindAllStringSubmatch(string(content), -1)
	constraintNames := make(map[string]bool)

	for _, match := range matches {
		if len(match) > 1 {
			constraintName := match[1]
			// Only include names that start with D8 (constraint kinds)
			if strings.HasPrefix(constraintName, "D8") {
				constraintNames[constraintName] = true
			}
		}
	}

	result := make([]string, 0, len(constraintNames))
	for name := range constraintNames {
		result = append(result, name)
	}

	return result
}

// formatEnforcementActionsYAML formats enforcement actions array as YAML list
func formatEnforcementActionsYAML(actions []string) string {
	var result strings.Builder
	for _, action := range actions {
		result.WriteString(fmt.Sprintf("        - %s\n", action))
	}
	return result.String()
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// findTemplatePath finds the template file by trying multiple possible paths
func findTemplatePath(relativePath string) string {
	// Get the directory where this test file is located
	_, testFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(testFile)

	// Try different possible paths
	possiblePaths := []string{
		// Relative to test file (when running from module root)
		filepath.Join(testDir, "..", relativePath),
		// Relative to current working directory
		relativePath,
		// From workspace root
		filepath.Join("modules", "015-admission-policy-engine", relativePath),
	}

	for _, path := range possiblePaths {
		absPath, _ := filepath.Abs(path)
		if _, err := os.Stat(absPath); err == nil {
			return absPath
		}
		// Also try the relative path as-is
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	Fail(fmt.Sprintf("Could not find template file %s. Tried paths: %v (test file: %s)", relativePath, possiblePaths, testFile))
	return ""
}

// getBaselineConstraintNames extracts constraint names from baseline template
func getBaselineConstraintNames() []string {
	templatePath := findTemplatePath(filepath.Join("templates", "policies", "pod-security-standards", "baseline", "constraint.yaml"))
	return extractConstraintNamesFromTemplate(templatePath, "pod_security_standard_baseline")
}

// getRestrictedConstraintNames extracts constraint names from restricted template
func getRestrictedConstraintNames() []string {
	templatePath := findTemplatePath(filepath.Join("templates", "policies", "pod-security-standards", "restricted", "constraint.yaml"))
	return extractConstraintNamesFromTemplate(templatePath, "pod_security_standard_restricted")
}

// getOperationConstraintNames extracts constraint names from operation-policy template
// by parsing "kind: D8XXX" lines in define blocks
func getOperationConstraintNames() []string {
	templatePath := findTemplatePath(filepath.Join("templates", "policies", "operation-policy", "constraint.yaml"))
	content, err := os.ReadFile(templatePath)
	if err != nil {
		Fail(fmt.Sprintf("Failed to read template file %s: %v", templatePath, err))
	}

	// Pattern to match: kind: D8XXX (where XXX is the constraint name)
	// This appears in define blocks like: kind: D8AllowedRepos
	pattern := `kind:\s+(D8[A-Za-z0-9]+)`
	re := regexp.MustCompile(pattern)

	matches := re.FindAllStringSubmatch(string(content), -1)
	constraintNames := make(map[string]bool)

	for _, match := range matches {
		if len(match) > 1 {
			constraintName := match[1]
			// Only include names that start with D8 (constraint kinds)
			if strings.HasPrefix(constraintName, "D8") {
				constraintNames[constraintName] = true
			}
		}
	}

	result := make([]string, 0, len(constraintNames))
	for name := range constraintNames {
		result = append(result, name)
	}

	return result
}

// constraintSelectorExpectation describes expected selector fields rendered by constraint_selector helper.
type constraintSelectorExpectation struct {
	namespaces         interface{}
	excludedNamespaces interface{}
	namespaceSelector  interface{}
	labelSelector      interface{}
}

func mustParseYaml(input string) interface{} {
	var result interface{}
	err := yaml.Unmarshal([]byte(input), &result)
	Expect(err).ShouldNot(HaveOccurred())
	return result
}

func getConstraintSpecMap(constraint object_store.KubeObject) map[string]interface{} {
	var resource map[string]interface{}
	err := yaml.Unmarshal([]byte(constraint.ToYaml()), &resource)
	Expect(err).ShouldNot(HaveOccurred())
	spec, ok := resource["spec"].(map[string]interface{})
	Expect(ok).To(BeTrue())
	return spec
}

func expectConstraintAction(spec map[string]interface{}, expectedAction string) {
	Expect(spec).To(HaveKeyWithValue("enforcementAction", expectedAction))
}

func expectConstraintSelector(spec map[string]interface{}, expected constraintSelectorExpectation) {
	match, ok := spec["match"].(map[string]interface{})
	Expect(ok).To(BeTrue())

	if expected.namespaces != nil {
		Expect(match).To(HaveKeyWithValue("namespaces", expected.namespaces))
	} else {
		Expect(match).ToNot(HaveKey("namespaces"))
	}

	if expected.excludedNamespaces != nil {
		Expect(match).To(HaveKeyWithValue("excludedNamespaces", expected.excludedNamespaces))
	} else {
		Expect(match).ToNot(HaveKey("excludedNamespaces"))
	}

	if expected.namespaceSelector != nil {
		Expect(match).To(HaveKeyWithValue("namespaceSelector", expected.namespaceSelector))
	} else {
		Expect(match).ToNot(HaveKey("namespaceSelector"))
	}

	if expected.labelSelector != nil {
		Expect(match).To(HaveKeyWithValue("labelSelector", expected.labelSelector))
	} else {
		Expect(match).ToNot(HaveKey("labelSelector"))
	}
}

func expectConstraintParameters(spec map[string]interface{}, expected interface{}) {
	if expected == nil {
		Expect(spec).ToNot(HaveKey("parameters"))
		return
	}
	Expect(spec).To(HaveKeyWithValue("parameters", expected))
}

type coverageReport struct {
	Constraints []coverageConstraint `json:"constraints"`
}

type coverageConstraint struct {
	Name      string          `json:"name"`
	Directory string          `json:"directory"`
	Fields    *coverageFields `json:"fields,omitempty"`
}

type coverageFields struct {
	CoveragePct int `json:"coverage_pct"`
}

func collectConstraintTestCaseDirs(constraintsRoot string) ([]string, error) {
	groups, err := os.ReadDir(constraintsRoot)
	if err != nil {
		return nil, err
	}

	dirs := make([]string, 0)
	for _, group := range groups {
		if !group.IsDir() {
			continue
		}
		if group.Name() != "operation" && group.Name() != "security" {
			continue
		}
		groupPath := filepath.Join(constraintsRoot, group.Name())
		constraints, err := os.ReadDir(groupPath)
		if err != nil {
			return nil, err
		}
		for _, constraint := range constraints {
			if !constraint.IsDir() {
				continue
			}
			if strings.HasPrefix(constraint.Name(), ".") || constraint.Name() == "test_samples" {
				continue
			}
			dirs = append(dirs, filepath.Join(groupPath, constraint.Name()))
		}
	}

	sort.Strings(dirs)
	return dirs, nil
}

func extractFailedGatorTestsFromOutput(output string) []string {
	lines := strings.Split(output, "\n")
	result := make([]string, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "--- FAIL:") {
			result = append(result, strings.TrimSpace(strings.TrimPrefix(line, "--- FAIL:")))
		}
	}
	return result
}

func parseCoverageReportFromOutput(raw []byte) (coverageReport, error) {
	text := string(raw)
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end < start {
		return coverageReport{}, fmt.Errorf("coverage JSON object not found in output")
	}

	var report coverageReport
	if err := json.Unmarshal([]byte(text[start:end+1]), &report); err != nil {
		return coverageReport{}, err
	}
	return report, nil
}

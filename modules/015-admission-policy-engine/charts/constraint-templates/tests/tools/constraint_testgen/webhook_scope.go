// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// webhookFinding represents a webhook scope issue found during static analysis.
type webhookFinding struct {
	File    string
	Line    int
	Rule    string
	Message string
}

// runWebhookScope scans the admission-policy-engine Helm templates for known
// webhook scope anti-patterns. It also scans test constraint fixture files so
// that drift between the workload_kinds helper and fixture match.kinds is
// caught.
//
// The point is the webhook blast radius, not just the constraint scope:
// constraint-exporter derives the ValidatingWebhookConfiguration rules from the
// deployed constraints' match.kinds, so a kind added to a Constraint widens what
// the webhook intercepts cluster-wide.
func runWebhookScope(templatesRoot string) error {
	findings, err := lintWebhookTemplates(templatesRoot)
	if err != nil {
		return err
	}

	// Also scan test constraint fixtures for ReplicaSet drift — the fixture
	// files under tests/test_cases/constraints/**/constraints/*.yaml declare
	// match.kinds and may still reference kinds removed from the helper.
	fixtureFindings, err := lintTestConstraintFixtures(templatesRoot)
	if err != nil {
		return err
	}
	findings = append(findings, fixtureFindings...)

	if len(findings) == 0 {
		fmt.Println("constraint_testgen webhook-scope: OK (no scope issues found)")
		return nil
	}
	for _, f := range findings {
		fmt.Fprintf(os.Stderr, "webhook-scope: %s:%d [%s] %s\n", f.File, f.Line, f.Rule, f.Message)
	}
	return fmt.Errorf("webhook-scope: %d issue(s) found", len(findings))
}

// lintTestConstraintFixtures walks the test_cases/constraints directory for
// constraint fixture files (constraints/*.yaml) and checks for ReplicaSet in
// match.kinds, which was removed from the workload_kinds helper.
func lintTestConstraintFixtures(templatesRoot string) ([]webhookFinding, error) {
	var findings []webhookFinding
	abs, err := filepath.Abs(templatesRoot)
	if err != nil {
		return nil, err
	}
	// templatesRoot is either the module's own templates directory
	// (<module>/templates) or the chart's one
	// (<module>/charts/constraint-templates/templates). The fixtures always live
	// under <module>/charts/constraint-templates/tests/test_cases/constraints, so
	// try both layouts instead of assuming one — the earlier single guess pointed
	// at modules/tests/test_cases/constraints, which never exists, and silently
	// disabled this whole check.
	moduleRoot := filepath.Dir(abs)
	candidates := []string{
		filepath.Join(moduleRoot, "charts", "constraint-templates", "tests", "test_cases", "constraints"),
		filepath.Join(moduleRoot, "tests", "test_cases", "constraints"),
	}
	testsRoot := ""
	for _, c := range candidates {
		if st, statErr := os.Stat(c); statErr == nil && st.IsDir() {
			testsRoot = c
			break
		}
	}
	if testsRoot == "" {
		return nil, nil
	}
	err = filepath.Walk(testsRoot, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		dirName := filepath.Base(filepath.Dir(p))
		if dirName != "constraints" {
			return nil
		}
		if !strings.HasSuffix(p, ".yaml") {
			return nil
		}
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		content := string(data)
		fileFindings := lintFixtureContent(p, content)
		findings = append(findings, fileFindings...)
		return nil
	})
	return findings, err
}

// lintFixtureContent honours the same webhook-scope:allow= directive as the
// template rules, so a fixture that intentionally covers a kind outside
// workload_kinds can document that instead of being rewritten.

// lintFixtureContent checks a test constraint fixture for ReplicaSet in
// match.kinds.
func lintFixtureContent(file, content string) []webhookFinding {
	var findings []webhookFinding
	if _, ok := allowedRules(content)["replicaset-in-test-fixture"]; ok {
		return nil
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, "ReplicaSet") {
			findings = append(findings, webhookFinding{
				File:    file,
				Line:    i + 1,
				Rule:    "replicaset-in-test-fixture",
				Message: "ReplicaSet in test fixture match.kinds — ReplicaSet was removed from the workload_kinds helper, so this fixture validates a scope that is never deployed — update it to match",
			})
		}
	}
	return findings
}

// lintWebhookTemplates walks the admission-policy-engine templates directory
// and checks for webhook scope anti-patterns.
func lintWebhookTemplates(templatesRoot string) ([]webhookFinding, error) {
	var findings []webhookFinding
	abs, err := filepath.Abs(templatesRoot)
	if err != nil {
		return nil, err
	}
	err = filepath.Walk(abs, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".yaml") && !strings.HasSuffix(p, ".tpl") {
			return nil
		}
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		content := string(data)
		fileFindings := lintWebhookContent(p, content)
		findings = append(findings, fileFindings...)
		return nil
	})
	return findings, err
}

// allowDirectiveRe matches a deliberate, documented suppression of one rule for
// the whole file:
//
//	# webhook-scope:allow=<rule> — <reason>
//	{{/* webhook-scope:allow=<rule> — <reason> */}}
//
// The reason is mandatory: a bare directive with nothing after the rule name is
// ignored, so silencing a rule always leaves the justification next to it.
var webhookAllowDirectiveRe = regexp.MustCompile(`webhook-scope:allow=([a-z-]+)\s+(\S.*)`)

// allowedRules collects the rules a file deliberately opts out of.
func allowedRules(content string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, m := range webhookAllowDirectiveRe.FindAllStringSubmatch(content, -1) {
		reason := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(m[2]), "*/}}"))
		if reason == "" {
			continue
		}
		out[m[1]] = struct{}{}
	}
	return out
}

// lintWebhookContent scans a single file's content for webhook scope issues.
func lintWebhookContent(file, content string) []webhookFinding {
	var findings []webhookFinding
	lines := strings.Split(content, "\n")
	fileName := filepath.Base(file)
	allowed := allowedRules(content)

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "{{/*") {
			// Skip YAML comments and Helm template comments ({{/* ... */}}) —
			// otherwise prose that documents *why* an anti-pattern was avoided
			// (e.g. a comment mentioning "DELETE") trips the same finding it's
			// explaining the absence of.
			continue
		}

		// Rule: replicaset-in-kinds
		// Only flag in constraint.yaml files (where match.kinds is defined)
		// or _helpers.tpl (where the workload kinds block is defined).
		if _, ok := allowed["replicaset-in-kinds"]; !ok &&
			(strings.Contains(fileName, "constraint.yaml") || strings.Contains(fileName, "_helpers.tpl")) &&
			strings.Contains(line, "ReplicaSet") {
			findings = append(findings, webhookFinding{
				File:    file,
				Line:    lineNum,
				Rule:    "replicaset-in-kinds",
				Message: "ReplicaSet in match.kinds — an RS is generated by the Deployment controller, so a denial there surfaces only in the Deployment status and gives none of the early feedback controller-level checks aim for. Consider removing ReplicaSet from the kind list",
			})
		}

		// Rule: webhook-delete-on-controllers
		// Only flag in validatingwebhookconfiguration.yaml
		if _, ok := allowed["webhook-delete-on-controllers"]; !ok &&
			strings.Contains(fileName, "validatingwebhookconfiguration") {
			if strings.Contains(strings.ToLower(trimmed), "delete") {
				findings = append(findings, webhookFinding{
					File:    file,
					Line:    lineNum,
					Rule:    "webhook-delete-on-controllers",
					Message: "DELETE operation intercepted by webhook — during Gatekeeper degradation you cannot tear down or roll back a workload, which is exactly what you need during an incident. Restrict controller-level webhook rules to CREATE and UPDATE, or record the decision with a webhook-scope:allow directive",
				})
			}
		}
	}

	// Rule: duplicated-kinds-block
	// The same kinds block used to be pasted ~30 times across these files.
	// Count occurrences of the workload kinds pattern across the file
	_, dupAllowed := allowed["duplicated-kinds-block"]
	if !dupAllowed && (strings.Contains(fileName, "constraint.yaml") || strings.Contains(fileName, "_helpers.tpl")) {
		workloadKindCount := strings.Count(content, "Deployment, StatefulSet, DaemonSet")
		if workloadKindCount > 1 {
			findings = append(findings, webhookFinding{
				File:    file,
				Line:    0,
				Rule:    "duplicated-kinds-block",
				Message: fmt.Sprintf("the same workload kinds block (Deployment, StatefulSet, DaemonSet) appears %d times in this file — extract into a single {{- include \"workload_kinds\" . }} helper", workloadKindCount),
			})
		}
	}

	return findings
}

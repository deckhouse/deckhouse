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
// T4: Validate the webhook blast radius and constraint match.kinds scope.
// Catches: H2 (ReplicaSet in scope, DELETE on controllers), L1 (duplicated
// kinds: block).
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
	// templatesRoot is .../templates; test fixtures live under
	// .../tests/test_cases/constraints/
	testsRoot := filepath.Join(filepath.Dir(filepath.Dir(abs)), "tests", "test_cases", "constraints")
	if _, err := os.Stat(testsRoot); os.IsNotExist(err) {
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

// lintFixtureContent checks a test constraint fixture for ReplicaSet in
// match.kinds.
func lintFixtureContent(file, content string) []webhookFinding {
	var findings []webhookFinding
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, "ReplicaSet") {
			findings = append(findings, webhookFinding{
				File:    file,
				Line:    i + 1,
				Rule:    "replicaset-in-test-fixture",
				Message: "ReplicaSet in test fixture match.kinds — ReplicaSet was removed from the workload_kinds helper; update this fixture to match (see PR #21556 review N4)",
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

// lintWebhookContent scans a single file's content for webhook scope issues.
func lintWebhookContent(file, content string) []webhookFinding {
	var findings []webhookFinding
	lines := strings.Split(content, "\n")
	fileName := filepath.Base(file)

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Rule: replicaset-in-kinds
		// Catches: H2 — ReplicaSet in constraint match.kinds
		// Only flag in constraint.yaml files (where match.kinds is defined)
		// or _helpers.tpl (where the workload kinds block is defined).
		if (strings.Contains(fileName, "constraint.yaml") || strings.Contains(fileName, "_helpers.tpl")) &&
			strings.Contains(line, "ReplicaSet") {
			findings = append(findings, webhookFinding{
				File:    file,
				Line:    lineNum,
				Rule:    "replicaset-in-kinds",
				Message: "ReplicaSet in match.kinds — an RS is generated by the Deployment controller, so a denial there surfaces only in the Deployment status and gives none of the early feedback this feature aims for. Consider removing ReplicaSet from the kind list (see PR #21556 review H2)",
			})
		}

		// Rule: webhook-delete-on-controllers
		// Catches: H2 — DELETE intercepted on controllers
		// Only flag in validatingwebhookconfiguration.yaml
		if strings.Contains(fileName, "validatingwebhookconfiguration") {
			if strings.Contains(strings.ToLower(trimmed), "delete") {
				findings = append(findings, webhookFinding{
					File:    file,
					Line:    lineNum,
					Rule:    "webhook-delete-on-controllers",
					Message: "DELETE operation intercepted by webhook — during Gatekeeper degradation you cannot tear down or roll back a workload, which is exactly what you need during an incident. Restrict controller-level webhook rules to CREATE only (see PR #21556 review H2)",
				})
			}
		}
	}

	// Rule: duplicated-kinds-block
	// Catches: L1 — the same 6-line kinds: block pasted ~30 times
	// Count occurrences of the workload kinds pattern across the file
	if strings.Contains(fileName, "constraint.yaml") || strings.Contains(fileName, "_helpers.tpl") {
		workloadKindCount := strings.Count(content, "Deployment, StatefulSet, DaemonSet, ReplicaSet")
		if workloadKindCount > 1 {
			findings = append(findings, webhookFinding{
				File:    file,
				Line:    0,
				Rule:    "duplicated-kinds-block",
				Message: fmt.Sprintf("the same workload kinds block (Deployment, StatefulSet, DaemonSet, ReplicaSet) appears %d times in this file — extract into a single {{- include \"workload_kinds\" . }} helper (see PR #21556 review L1)", workloadKindCount),
			})
		}
	}

	return findings
}

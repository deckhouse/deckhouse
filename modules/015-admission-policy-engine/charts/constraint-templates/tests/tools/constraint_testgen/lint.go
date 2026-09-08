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

// lintFinding represents a single lint issue.
type lintFinding struct {
	File    string
	Line    int
	Rule    string
	Message string
}

// objectGetEmptyDefaultRe matches object.get(_, "<field>", "") calls where the
// default value is an empty string literal.  This is the anti-pattern that
// turns a previously-undefined field into a concrete "" value, which can cause
// false-positive violations when used in list_contains / equality / not checks.
//
// Matches:
//
//	object.get(container, "imagePullPolicy", "")
//	object.get(obj, "priorityClassName", "")
//
// Does NOT match:
//
//	object.get(container, "imagePullPolicy", "IfNotPresent")
//	object.get(obj, "priorityClassName", "system-cluster-critical")
var objectGetEmptyDefaultRe = regexp.MustCompile(
	`object\.get\(\s*[^,]+,\s*"[^"]+"\s*,\s*""\s*\)`,
)

// listContainsEmptyRe matches list_contains(..., "") — checking for an empty
// string in an allow-list, which is the direct cause of the C2 regression
// (absent priorityClassName → "" → list_contains(["foo"], "") is undefined →
// not undefined → violation fires).
var listContainsEmptyRe = regexp.MustCompile(
	`list_contains\([^,]+,\s*""\s*\)`,
)

// notHasFieldThenGetRe matches patterns where a field is accessed with
// object.get and then checked with `not` without a has_field guard — the
// C2/C3 class of bug.  This is intentionally conservative: it only flags
// `not <something with object.get(..., "")>` on the same line.
var notObjectGetEmptyRe = regexp.MustCompile(
	`not\s+.*object\.get\([^,]+,\s*"[^"]+"\s*,\s*""\s*\)`,
)

// allowDirectiveRe matches a deliberate opt-out of one lint rule, written as a
// Rego comment on the line directly above the code it covers:
//
//	# lint:allow=object-get-empty-default — the "" default is rejected below
//	name := object.get(input.review, "namespace", "")
//
// The reason after the rule name is mandatory, so an opt-out always states why
// the pattern is safe here. Scoping the directive to a single line keeps it from
// silently covering code added later in the same file.
var lintAllowDirectiveRe = regexp.MustCompile(`lint:allow=([a-z-]+)\s+(\S.*)`)

// isRuleAllowedAt reports whether the line above lines[i] opts out of rule.
// Consecutive comment lines are walked upwards, so a directive may sit above a
// longer explanatory comment.
func isRuleAllowedAt(lines []string, i int, rule string) bool {
	for j := i - 1; j >= 0; j-- {
		trimmed := strings.TrimSpace(lines[j])
		if !strings.HasPrefix(trimmed, "#") {
			return false
		}
		m := lintAllowDirectiveRe.FindStringSubmatch(trimmed)
		if m != nil && m[1] == rule && strings.TrimSpace(m[2]) != "" {
			return true
		}
	}
	return false
}

// runLint scans ConstraintTemplate Rego source files under templatesRoot for
// known anti-patterns that have caused review regressions.
func runLint(templatesRoot string) error {
	findings, err := lintTemplatesDir(templatesRoot)
	if err != nil {
		return err
	}
	// T9: Also scan for fail-open on unknown kind in common.rego.
	failOpenFindings, ferr := lintFailOpen(templatesRoot)
	if ferr != nil {
		return ferr
	}
	findings = append(findings, failOpenFindings...)
	if len(findings) == 0 {
		fmt.Println("constraint_testgen lint: OK (no anti-patterns found)")
		return nil
	}
	for _, f := range findings {
		fmt.Fprintf(os.Stderr, "lint: %s:%d [%s] %s\n", f.File, f.Line, f.Rule, f.Message)
	}
	return fmt.Errorf("lint: %d anti-pattern(s) found", len(findings))
}

// lintTemplatesDir walks templatesRoot recursively and lints every .yaml
// file that contains ConstraintTemplate Rego code.
func lintTemplatesDir(templatesRoot string) ([]lintFinding, error) {
	var findings []lintFinding
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
		if !strings.HasSuffix(p, ".yaml") && !strings.HasSuffix(p, ".rego") {
			return nil
		}
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		content := string(data)
		// Only lint files that contain Rego code (ConstraintTemplate sources).
		if !strings.Contains(content, "package ") && !strings.Contains(content, "violation[") {
			return nil
		}
		fileFindings := lintContent(p, content)
		findings = append(findings, fileFindings...)
		return nil
	})
	return findings, err
}

// lintContent scans a single file's content for anti-patterns and returns
// findings with line numbers.
func lintContent(file, content string) []lintFinding {
	var findings []lintFinding
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)
		// Skip comments.
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Rule: object-get-empty-default
		// Catches: C2 (priority-class), C3 (image-pull-policy)
		// Pattern: object.get(x, "field", "") used in a violation-trigger context.
		if loc := objectGetEmptyDefaultRe.FindStringIndex(line); loc != nil {
			// The empty default is fine when the very next line rejects "" — the
			// absent case is then handled explicitly rather than turned into a
			// concrete value that feeds a comparison.
			if i+1 < len(lines) && strings.Contains(lines[i+1], "!=") && strings.Contains(lines[i+1], `""`) {
				continue
			}
			// Anything else must carry an explicit, justified opt-out on the
			// preceding line. Matching on field names ("image", "dnsPolicy", …)
			// instead would tie the linter to identifiers it has no business
			// knowing: renaming a variable silently changes what is checked, and
			// a genuinely new bug next to a blessed field name is swallowed.
			if isRuleAllowedAt(lines, i, "object-get-empty-default") {
				continue
			}
			findings = append(findings, lintFinding{
				File:    file,
				Line:    lineNum,
				Rule:    "object-get-empty-default",
				Message: "object.get(..., \"\") turns an absent field into a concrete empty string — use has_field() guard or a non-empty default to avoid false-positive violations (see PR #21556 review C2/C3)",
			})
		}

		// Rule: list-contains-empty
		// Catches: C2 (priority-class) — list_contains(["foo"], "") is the direct trigger
		if loc := listContainsEmptyRe.FindStringIndex(line); loc != nil && !isRuleAllowedAt(lines, i, "list-contains-empty") {
			findings = append(findings, lintFinding{
				File:    file,
				Line:    lineNum,
				Rule:    "list-contains-empty",
				Message: "list_contains(..., \"\") is undefined in OPA, so `not list_contains(..., \"\")` is always true — guard with has_field() before the membership check (see PR #21556 review C2)",
			})
		}

		// Rule: not-object-get-empty
		// Catches: C2, C3 — the `not` + object.get(..., "") combination
		if loc := notObjectGetEmptyRe.FindStringIndex(line); loc != nil && !isRuleAllowedAt(lines, i, "not-object-get-empty") {
			findings = append(findings, lintFinding{
				File:    file,
				Line:    lineNum,
				Rule:    "not-object-get-empty",
				Message: "`not` with object.get(..., \"\") silently inverts to true when the field is absent — add a has_field() guard (see PR #21556 review C2/C3)",
			})
		}
	}

	return findings
}

// workloadKindAllowListRe matches the hardcoded workload_kind allow-list in
// common.rego. When an unknown/absent kind yields an empty pod spec, all
// container-level checks silently pass (fail-open). For a security module this
// is dangerous. (PR #21556 review M1)
var workloadKindAllowListRe = regexp.MustCompile(`workload_kind`)

// emptyPodSpecRe matches patterns where pod_spec is set to {} for unknown
// kinds, causing input_containers to be empty and checks to silently pass.
var emptyPodSpecRe = regexp.MustCompile(`pod_spec\s*:=\s*\{\}`)

// lintFailOpen scans common.rego for the fail-open-on-unknown-kind pattern.
func lintFailOpen(templatesRoot string) ([]lintFinding, error) {
	var findings []lintFinding
	// Walk from the chart root (parent of templates) so we also find files/libs/common.rego
	chartRoot := filepath.Dir(templatesRoot)
	abs, err := filepath.Abs(chartRoot)
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
		if filepath.Base(p) != "common.rego" {
			return nil
		}
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		content := string(data)
		lines := strings.Split(content, "\n")
		hasWorkloadKind := false
		for i, line := range lines {
			if workloadKindAllowListRe.MatchString(line) {
				hasWorkloadKind = true
			}
			if hasWorkloadKind && emptyPodSpecRe.MatchString(line) {
				findings = append(findings, lintFinding{
					File:    p,
					Line:    i + 1,
					Rule:    "fail-open-unknown-kind",
					Message: "pod_spec defaults to {} for unknown kind — container-level checks silently pass (fail-open). For a security module an unrecognised input should not mean 'allowed'. Consider falling back to object.get(obj, \"spec\", {}) or emitting an explicit violation on unknown kind (see PR #21556 review M1)",
				})
			}
		}
		return nil
	})
	return findings, err
}

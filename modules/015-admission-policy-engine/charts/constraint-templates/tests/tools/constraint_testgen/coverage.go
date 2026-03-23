// Copyright 2025 Flant JSC
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
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type coverageReport struct {
	Constraints []constraintCoverage `json:"constraints"`
	Summary     coverageSummary      `json:"summary"`
}

type constraintCoverage struct {
	Name      string                    `json:"name"`
	Directory string                    `json:"directory"`
	Tracks    map[string]*trackCoverage `json:"tracks"`
	Fields    *fieldCoverage            `json:"fields,omitempty"`
	Status    string                    `json:"status"`
	Warnings  []string                  `json:"warnings"`
}

type trackCoverage struct {
	Blocks     []string       `json:"blocks"`
	Cases      int            `json:"cases"`
	Patterns   map[string]int `json:"patterns"`
	BlockCases map[string]int `json:"-"`
	CaseNames  []string       `json:"-"`
}

type fieldCoverage struct {
	ObjectTotal      int      `json:"object_total"`
	ObjectCovered    int      `json:"object_covered"`
	SpeTotal         int      `json:"spe_total"`
	SpeCovered       int      `json:"spe_covered"`
	ScenarioTotal    int      `json:"scenario_total"`
	ScenarioCovered  int      `json:"scenario_covered"`
	CoveragePct      int      `json:"coverage_pct"`
	MissingScenarios []string `json:"missing_scenarios,omitempty"`
}

type coverageSummary struct {
	TotalConstraints int `json:"total_constraints"`
	TotalCases       int `json:"total_cases"`
	OK               int `json:"ok"`
	Warnings         int `json:"warnings"`
}

type suiteCoverageDoc struct {
	Tests []suiteTestBlock `yaml:"tests"`
}

type matrixCoverageDoc struct {
	Kind string `yaml:"kind"`
	Spec struct {
		Blocks []struct {
			Name  string `yaml:"name"`
			Cases []struct {
				Name   string            `yaml:"name"`
				Fields []matrixCaseField `yaml:"fields"`
			} `yaml:"cases"`
		} `yaml:"blocks"`
	} `yaml:"spec"`
}

type suiteTestBlock struct {
	Name  string      `yaml:"name"`
	Cases []suiteCase `yaml:"cases"`
}

type suiteCase struct {
	Name string `yaml:"name"`
}

func runCoverage(testsRoot, format, constraint string) error {
	baseRoot, _, err := resolveTestsRoot(testsRoot)
	if err != nil {
		return err
	}
	profiles := loadProfiles(baseRoot)
	constraintDirs, err := resolveCoverageConstraintDirs(testsRoot, baseRoot, constraint)
	if err != nil {
		return err
	}
	var report coverageReport
	for _, dir := range constraintDirs {
		cc, err := analyzeConstraintCoverage(dir, profiles)
		if err != nil {
			return err
		}
		report.Constraints = append(report.Constraints, cc)
	}
	report.Summary = computeSummary(report.Constraints)
	if len(report.Constraints) == 0 {
		return fmt.Errorf("no constraint suites found under %s (expected rendered/test_suite.yaml)", baseRoot)
	}
	switch strings.ToLower(format) {
	case "json":
		return outputCoverageJSON(report)
	case "markdown", "md":
		return outputCoverageMarkdown(report)
	default:
		return outputCoverageTable(report)
	}
}

func resolveCoverageConstraintDirs(testsRoot, baseRoot, constraint string) ([]string, error) {
	if constraint != "" {
		if isConstraintDir(constraint) {
			return []string{constraint}, nil
		}
		if isConstraintDir(filepath.Join(baseRoot, constraint)) {
			return []string{filepath.Join(baseRoot, constraint)}, nil
		}
		entries, err := os.ReadDir(baseRoot)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			candidate := filepath.Join(baseRoot, e.Name(), constraint)
			if isConstraintDir(candidate) {
				return []string{candidate}, nil
			}
		}
		return nil, fmt.Errorf("constraint %q not found under %s", constraint, baseRoot)
	}
	if isConstraintDir(testsRoot) {
		return []string{testsRoot}, nil
	}
	return findConstraintDirs(baseRoot)
}

func isConstraintDir(dir string) bool {
	suitePath := filepath.Join(dir, "rendered", "test_suite.yaml")
	st, err := os.Stat(suitePath)
	return err == nil && !st.IsDir()
}

func findConstraintDirs(baseRoot string) ([]string, error) {
	entries, err := os.ReadDir(baseRoot)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "profiles" || name == "common" || name == "test_samples" {
			continue
		}
		suitePath := filepath.Join(baseRoot, name, "rendered", "test_suite.yaml")
		if st, err := os.Stat(suitePath); err == nil && !st.IsDir() {
			dirs = append(dirs, filepath.Join(baseRoot, name))
			continue
		}
		groupDir := filepath.Join(baseRoot, name)
		groupEntries, err := os.ReadDir(groupDir)
		if err != nil {
			return nil, err
		}
		for _, ge := range groupEntries {
			if !ge.IsDir() {
				continue
			}
			childName := ge.Name()
			if childName == "profiles" || childName == "common" || childName == "test_samples" {
				continue
			}
			childSuitePath := filepath.Join(groupDir, childName, "rendered", "test_suite.yaml")
			if st, err := os.Stat(childSuitePath); err == nil && !st.IsDir() {
				dirs = append(dirs, filepath.Join(groupDir, childName))
				continue
			}
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}

func analyzeConstraintCoverage(dir string, profiles map[string]profileDoc) (constraintCoverage, error) {
	name := filepath.Base(dir)
	suitePath := filepath.Join(dir, "rendered", "test_suite.yaml")
	b, err := os.ReadFile(suitePath)
	if err != nil {
		return constraintCoverage{}, fmt.Errorf("read suite %s: %w", suitePath, err)
	}
	var suite suiteCoverageDoc
	if err := yaml.Unmarshal(b, &suite); err != nil {
		return constraintCoverage{}, fmt.Errorf("parse suite %s: %w", suitePath, err)
	}
	cc := constraintCoverage{
		Name:      name,
		Directory: name,
		Tracks:    make(map[string]*trackCoverage),
	}
	for _, t := range suite.Tests {
		track := classifyTrack(t.Name)
		if cc.Tracks[track] == nil {
			cc.Tracks[track] = &trackCoverage{
				Patterns:   make(map[string]int),
				BlockCases: make(map[string]int),
			}
		}
		tc := cc.Tracks[track]
		tc.Blocks = append(tc.Blocks, t.Name)
		for _, c := range t.Cases {
			tc.Cases++
			tc.BlockCases[t.Name]++
			tc.CaseNames = append(tc.CaseNames, c.Name)
			pattern := classifyPattern(c.Name)
			tc.Patterns[pattern]++
		}
	}
	fields, err := analyzeFieldCoverage(dir)
	if err != nil {
		return constraintCoverage{}, err
	}
	if fields != nil {
		cc.Fields = fields
	}
	if p, ok := profiles[name]; ok {
		cc.Warnings = append(cc.Warnings, checkCoverageProfile(cc, p)...)
	}
	cc.Status = "ok"
	if len(cc.Warnings) > 0 {
		cc.Status = "warn"
	}
	return cc, nil
}

func classifyTrack(blockName string) string {
	name := strings.ToLower(blockName)
	switch {
	case strings.Contains(name, "spe-container"):
		return "securityPolicyExceptionContainer"
	case strings.Contains(name, "spe-pod") || strings.Contains(name, "with-exception") || strings.Contains(name, "spe"):
		return "securityPolicyExceptionPod"
	default:
		return "functional"
	}
}

func classifyPattern(caseName string) string {
	name := strings.ToLower(caseName)
	switch {
	case strings.Contains(name, "allowed-by-exception"):
		return "allowed-by-exception"
	case strings.Contains(name, "disallowed-by-exception"):
		return "disallowed-by-exception"
	case strings.Contains(name, "disallowed-no-exception"):
		return "disallowed-no-exception"
	case strings.Contains(name, "empty-spec"):
		return "disallowed-empty-spec"
	case strings.Contains(name, "allowed") && strings.Contains(name, "multi"):
		return "allowed-multi"
	case strings.Contains(name, "disallowed") && strings.Contains(name, "multi"):
		return "disallowed-multi"
	case strings.Contains(name, "allowed") && strings.Contains(name, "init"):
		return "allowed-init"
	case strings.Contains(name, "disallowed") && strings.Contains(name, "init"):
		return "disallowed-init"
	case strings.Contains(name, "allowed"):
		return "allowed"
	case strings.Contains(name, "disallowed"):
		return "disallowed"
	default:
		return "other"
	}
}

func checkCoverageProfile(cc constraintCoverage, p profileDoc) []string {
	var warns []string
	cov := p.Spec.Coverage
	if cov == nil {
		return warns
	}
	if cov.MinimumCasesPerBlock > 0 {
		for _, t := range cc.Tracks {
			for _, block := range t.Blocks {
				if lenBlockCases(cc, block) < cov.MinimumCasesPerBlock {
					warns = append(warns, fmt.Sprintf("block %q has < %d cases", block, cov.MinimumCasesPerBlock))
				}
			}
		}
	}
	for track, patterns := range cov.RequiredPatterns {
		for _, pattern := range patterns {
			if !hasPattern(cc, track, pattern) {
				warns = append(warns, fmt.Sprintf("track %q missing case matching %q", track, pattern))
			}
		}
	}
	return warns
}

func lenBlockCases(cc constraintCoverage, blockName string) int {
	for _, t := range cc.Tracks {
		if count, ok := t.BlockCases[blockName]; ok {
			return count
		}
	}
	return 0
}

func hasPattern(cc constraintCoverage, track, pattern string) bool {
	for trackName, t := range cc.Tracks {
		if trackName != track {
			continue
		}
		for _, name := range t.CaseNames {
			if matchGlob(pattern, name) {
				return true
			}
		}
	}
	return false
}

func computeSummary(constraints []constraintCoverage) coverageSummary {
	var s coverageSummary
	for _, c := range constraints {
		s.TotalConstraints++
		for _, t := range c.Tracks {
			s.TotalCases += t.Cases
		}
		switch c.Status {
		case "ok":
			s.OK++
		case "warn":
			s.Warnings++
		}
	}
	return s
}

func outputCoverageJSON(report coverageReport) error {
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func outputCoverageMarkdown(report coverageReport) error {
	fmt.Println("| Constraint | ObjFields | SPEFields | Scenarios | Covered | FieldCov% | Functional | SPE Pod | SPE Container | Total | Status |")
	fmt.Println("|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|")
	for _, c := range report.Constraints {
		objFields, speFields, scenarios, covered, pct := fieldCoverageCells(c)
		funcCases := trackCases(c, "functional")
		spePod := trackCases(c, "securityPolicyExceptionPod")
		speCtr := trackCases(c, "securityPolicyExceptionContainer")
		total := funcCases + spePod + speCtr
		fmt.Printf("| %s | %s | %s | %s | %s | %s | %d | %d | %d | %d | %s |\n", c.Name, objFields, speFields, scenarios, covered, pct, funcCases, spePod, speCtr, total, strings.ToUpper(c.Status))
		for _, missing := range fieldCoverageMissingLines(c) {
			fmt.Printf("| %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s |\n", "", "", "", "", "", "", "", "", "", "", missing)
		}
	}
	return nil
}

func outputCoverageTable(report coverageReport) error {
	head := []string{"Constraint", "ObjFields", "SPEFields", "Scenarios", "Covered", "FieldCov%", "Functional", "SPE-Pod", "SPE-Container", "Total", "Status"}
	rows := [][]string{head}
	for _, c := range report.Constraints {
		objFields, speFields, scenarios, covered, pct := fieldCoverageCells(c)
		funcCases := trackCases(c, "functional")
		spePod := trackCases(c, "securityPolicyExceptionPod")
		speCtr := trackCases(c, "securityPolicyExceptionContainer")
		total := funcCases + spePod + speCtr
		rows = append(rows, []string{
			c.Name,
			objFields,
			speFields,
			scenarios,
			covered,
			pct,
			fmt.Sprintf("%d", funcCases),
			fmt.Sprintf("%d", spePod),
			fmt.Sprintf("%d", speCtr),
			fmt.Sprintf("%d", total),
			strings.ToUpper(c.Status),
		})
		for _, missing := range fieldCoverageMissingLines(c) {
			rows = append(rows, []string{"", "", "", "", "", "", "", "", "", "", missing})
		}
	}
	widths := make([]int, len(head))
	for _, row := range rows {
		for i, col := range row {
			if len(col) > widths[i] {
				widths[i] = len(col)
			}
		}
	}
	for i, row := range rows {
		for j, col := range row {
			pad := widths[j] - len(col)
			if j == 0 {
				fmt.Print(col)
				fmt.Print(strings.Repeat(" ", pad+2))
			} else {
				fmt.Print(strings.Repeat(" ", pad))
				fmt.Print(col)
				fmt.Print("  ")
			}
		}
		fmt.Println()
		if i == 0 {
			for j := range row {
				fmt.Print(strings.Repeat("-", widths[j]))
				fmt.Print("  ")
			}
			fmt.Println()
		}
	}
	return nil
}

func trackCases(c constraintCoverage, track string) int {
	if t, ok := c.Tracks[track]; ok {
		return t.Cases
	}
	return 0
}

func fieldCoverageCells(c constraintCoverage) (string, string, string, string, string) {
	if c.Fields == nil {
		return "-", "-", "-", "-", "-"
	}
	obj := fmt.Sprintf("%d/%d", c.Fields.ObjectCovered, c.Fields.ObjectTotal)
	spe := fmt.Sprintf("%d/%d", c.Fields.SpeCovered, c.Fields.SpeTotal)
	scenarios := fmt.Sprintf("%d", c.Fields.ScenarioTotal)
	covered := fmt.Sprintf("%d", c.Fields.ScenarioCovered)
	pct := fmt.Sprintf("%d%%", c.Fields.CoveragePct)
	return obj, spe, scenarios, covered, pct
}

func fieldCoverageMissingLines(c constraintCoverage) []string {
	if c.Fields == nil || len(c.Fields.MissingScenarios) == 0 {
		return nil
	}
	lines := make([]string, 0, len(c.Fields.MissingScenarios))
	for _, missing := range c.Fields.MissingScenarios {
		lines = append(lines, fmt.Sprintf("missing: %s", missing))
	}
	return lines
}

func analyzeFieldCoverage(dir string) (*fieldCoverage, error) {
	fieldsPath := filepath.Join(dir, "test_fields.yaml")
	if _, err := os.Stat(fieldsPath); err != nil {
		return nil, nil
	}
	doc, err := loadTestFields(fieldsPath)
	if err != nil {
		return nil, fmt.Errorf("parse test_fields.yaml: %w", err)
	}
	if doc.Kind != "ConstraintTestFields" {
		return nil, nil
	}
	if err := validateTestFields(doc); err != nil {
		return nil, fmt.Errorf("invalid test_fields.yaml: %w", err)
	}
	cases, err := loadMatrixCases(dir)
	if err != nil {
		return nil, err
	}
	cov := &fieldCoverage{}
	cov.ObjectTotal = len(doc.Spec.ObjectFields)
	cov.SpeTotal = len(doc.Spec.SpeFields)
	fieldTrack := requiredScenarioTracks(doc)
	caseCoverage := buildCaseScenarioCoverage(cases, doc, fieldTrack)
	cov.ScenarioTotal = caseCoverage.RequiredTotal
	cov.ScenarioCovered = caseCoverage.CoveredTotal
	cov.MissingScenarios = caseCoverage.Missing
	for _, f := range doc.Spec.ObjectFields {
		if caseCoverage.HasField(f.Path, false) {
			cov.ObjectCovered++
		}
	}
	for _, f := range doc.Spec.SpeFields {
		if caseCoverage.HasField(f.Path, true) {
			cov.SpeCovered++
		}
	}
	if cov.ScenarioTotal > 0 {
		cov.CoveragePct = int(math.Round(float64(cov.ScenarioCovered) / float64(cov.ScenarioTotal) * 100))
	}
	return cov, nil
}

func loadMatrixCases(dir string) ([]matrixCaseCoverage, error) {
	matrixPath := filepath.Join(dir, "test-matrix.yaml")
	b, err := os.ReadFile(matrixPath)
	if err != nil {
		return nil, nil
	}
	var doc matrixCoverageDoc
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("parse test-matrix.yaml: %w", err)
	}
	if doc.Kind != "ConstraintTestMatrix" {
		return nil, nil
	}
	var out []matrixCaseCoverage
	for _, block := range doc.Spec.Blocks {
		for _, c := range block.Cases {
			out = append(out, matrixCaseCoverage{Name: c.Name, Fields: c.Fields})
		}
	}
	return out, nil
}

func loadProfiles(testsRoot string) map[string]profileDoc {
	profiles := make(map[string]profileDoc)
	_ = filepath.WalkDir(testsRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Base(p) != "test_profile.yaml" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		var doc profileDoc
		if err := yaml.Unmarshal(b, &doc); err != nil {
			return nil
		}
		if doc.Kind != "ConstraintTestProfile" {
			return nil
		}
		if doc.Spec.TestDirectory == "" {
			return nil
		}
		profiles[doc.Spec.TestDirectory] = doc
		return nil
	})
	return profiles
}

func matchGlob(pattern, value string) bool {
	ok, err := path.Match(pattern, value)
	if err != nil {
		return false
	}
	return ok
}

func requiredScenarioTracks(doc *testFieldsDoc) map[string]string {
	tracks := map[string]string{}
	if doc.Spec.ApplicableTracks.Functional != nil && *doc.Spec.ApplicableTracks.Functional {
		tracks["functional"] = "functional"
	}
	if doc.Spec.ApplicableTracks.SpePod != nil && *doc.Spec.ApplicableTracks.SpePod {
		tracks["securityPolicyExceptionPod"] = "securityPolicyExceptionPod"
	}
	if doc.Spec.ApplicableTracks.SpeContainer != nil && *doc.Spec.ApplicableTracks.SpeContainer {
		tracks["securityPolicyExceptionContainer"] = "securityPolicyExceptionContainer"
	}
	return tracks
}

func buildCaseScenarioCoverage(cases []matrixCaseCoverage, doc *testFieldsDoc, track map[string]string) caseScenarioCoverage {
	required := make(map[string]struct{})
	missing := make([]string, 0)
	covered := make([]string, 0)
	coveredSet := make(map[string]struct{})
	fieldTypes := make(map[string]bool)
	for _, f := range doc.Spec.ObjectFields {
		fieldTypes[f.Path] = false
		for _, scenario := range f.RequiredScenarios {
			required[scenarioKey(f.Path, scenario)] = struct{}{}
		}
	}
	for _, f := range doc.Spec.SpeFields {
		fieldTypes[f.Path] = true
		for _, scenario := range f.RequiredScenarios {
			required[scenarioKey(f.Path, scenario)] = struct{}{}
		}
	}
	for _, c := range cases {
		explicitCount := 0
		for _, f := range c.Fields {
			path := strings.TrimSpace(f.Path)
			scenario := strings.TrimSpace(f.Scenario)
			if path == "" || scenario == "" {
				continue
			}
			key := scenarioKey(path, scenario)
			if _, ok := coveredSet[key]; ok {
				explicitCount++
				continue
			}
			coveredSet[key] = struct{}{}
			explicitCount++
			covered = append(covered, key)
		}
		if explicitCount > 0 {
			continue
		}
		for key := range inferScenariosFromCaseName(c.Name, required) {
			if _, ok := coveredSet[key]; ok {
				continue
			}
			coveredSet[key] = struct{}{}
			covered = append(covered, key)
		}
	}
	coveredRequired := 0
	for key := range required {
		if _, ok := coveredSet[key]; ok {
			coveredRequired++
			continue
		}
		missing = append(missing, key)
	}
	sort.Strings(missing)
	return caseScenarioCoverage{
		RequiredTotal: len(required),
		CoveredTotal:  coveredRequired,
		Missing:       missing,
		FieldTypes:    fieldTypes,
		CoveredKeys:   coveredSet,
		CoveredOrder:  covered,
	}
}

func inferScenariosFromCaseName(name string, required map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{})
	lower := strings.ToLower(name)
	var scenario string
	switch {
	case strings.Contains(lower, "multi"):
		scenario = "multiContainer"
	case strings.Contains(lower, "ephemeral"):
		scenario = "ephemeralContainer"
	case strings.Contains(lower, "init"):
		scenario = "initContainer"
	case strings.Contains(lower, "no-hostnetwork") || strings.Contains(lower, "no-hostpid") || strings.Contains(lower, "no-hostipc") || strings.Contains(lower, "no-seccomp") || strings.Contains(lower, "no-toleration") || strings.Contains(lower, "no-sysctl") || strings.Contains(lower, "no-sysctls") || strings.Contains(lower, "no-exception") || strings.Contains(lower, "empty"):
		scenario = "absent"
	case strings.Contains(lower, "disallowed") || strings.Contains(lower, "deny"):
		scenario = "negative"
	case strings.Contains(lower, "allowed"):
		scenario = "positive"
	}
	if scenario == "" {
		return result
	}
	for key := range required {
		_, reqScenario := parseScenarioKey(key)
		if reqScenario == scenario {
			result[key] = struct{}{}
		}
	}
	return result
}

func scenarioKey(path, scenario string) string {
	return fmt.Sprintf("%s/%s", path, scenario)
}

func parseScenarioKey(key string) (string, string) {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return key, ""
}

func (c caseScenarioCoverage) HasField(path string, isSpe bool) bool {
	for key := range c.CoveredKeys {
		p, _ := parseScenarioKey(key)
		if p == path {
			return true
		}
	}
	return false
}

type matrixCaseCoverage struct {
	Name   string
	Fields []matrixCaseField
}

type caseScenarioCoverage struct {
	RequiredTotal int
	CoveredTotal  int
	Missing       []string
	FieldTypes    map[string]bool
	CoveredKeys   map[string]struct{}
	CoveredOrder  []string
}

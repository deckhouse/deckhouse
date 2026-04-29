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
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/releaseutil"

	testhelm "github.com/deckhouse/deckhouse/testing/library/helm"
)

// ConstraintTestMatrix: bases + per-case merge fragments → gator fixtures.
type matrixDoc struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec matrixSpec `yaml:"spec"`
}

type matrixExternalData struct {
	Providers []interface{} `yaml:"providers"`
}

type matrixSpec struct {
	SuiteName           string `yaml:"suiteName"`
	OutputTestDirectory string `yaml:"outputTestDirectory"`
	// DefaultObjectBase is used when a case object omits base (overridable per block).
	DefaultObjectBase string `yaml:"defaultObjectBase"`
	// DefaultInventory is prepended to every case's inventory (e.g. shared Namespace ref).
	DefaultInventory []interface{}                `yaml:"defaultInventory"`
	ExternalData     *matrixExternalData          `yaml:"externalData,omitempty"`
	Bases            map[string]matrixBase        `yaml:"bases"`
	NamedExceptions  map[string]namedExceptionDef `yaml:"namedExceptions"`
	Blocks           []*matrixBlock               `yaml:"blocks"`
}

// namedExceptionDef is a reusable SecurityPolicyException (or other base) merge fragment keyed by name.
type namedExceptionDef struct {
	Base  string                 `yaml:"base"`
	Merge map[string]interface{} `yaml:"merge"`
}

type matrixBase struct {
	Document map[string]interface{} `yaml:"document"`
}

type matrixCase struct {
	Name             string `yaml:"name"`
	Violations       string `yaml:"violations"`
	AssertionMessage string `yaml:"assertionMessage"`
	// Fields declares which field+scenario this case covers.
	Fields []matrixCaseField `yaml:"fields"`
	// Exception / ExceptionRef name a spec.namedExceptions entry: generates SPE inventory + pod label.
	Exception    string              `yaml:"exception"`
	ExceptionRef string              `yaml:"exceptionRef"`
	Inventory    []interface{}       `yaml:"inventory"`
	ExternalData *matrixExternalData `yaml:"externalData,omitempty"`
	Object       interface{}         `yaml:"object"`
}

type matrixCaseField struct {
	Path     string `yaml:"path"`
	Scenario string `yaml:"scenario"`
}

func generateFromMatrix(matrixPath, testsRoot string) error {
	raw, err := os.ReadFile(matrixPath)
	if err != nil {
		return fmt.Errorf("read matrix: %w", err)
	}
	var doc matrixDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse matrix: %w", err)
	}
	if doc.Kind != "ConstraintTestMatrix" {
		return fmt.Errorf("expected kind ConstraintTestMatrix, got %q", doc.Kind)
	}
	normalizeMatrixDoc(&doc)
	if doc.Spec.OutputTestDirectory == "" || doc.Spec.SuiteName == "" {
		return fmt.Errorf("spec.suiteName and spec.outputTestDirectory are required")
	}
	if len(doc.Spec.Bases) == 0 {
		return fmt.Errorf("spec.bases is required")
	}
	matrixDir := filepath.Dir(matrixPath)
	outputDir := strings.TrimSpace(doc.Spec.OutputTestDirectory)
	if outputDir == "" {
		return fmt.Errorf("spec.outputTestDirectory is required")
	}
	baseDir := resolveMatrixOutputDir(matrixDir, outputDir, testsRoot)
	samplesDir := filepath.Join(baseDir, "test_samples")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return err
	}
	refPaths := collectMatrixRefPaths(&doc.Spec, samplesDir, baseDir)
	tracker := newGeneratedSampleTracker(refPaths, baseDir, samplesDir)
	if err := cleanupRenderedSamples(samplesDir, tracker.refPaths); err != nil {
		return fmt.Errorf("clean test_samples: %w", err)
	}
	if err := os.MkdirAll(samplesDir, 0o755); err != nil {
		return err
	}

	suite := suiteOut{
		Kind:       "Suite",
		APIVersion: "test.gatekeeper.sh/v1alpha1",
	}
	if err := validateRFC1123SubdomainName(doc.Spec.SuiteName); err != nil {
		return fmt.Errorf("spec.suiteName: %w", err)
	}
	suite.Metadata.Name = doc.Spec.SuiteName

	specDefObj := strings.TrimSpace(doc.Spec.DefaultObjectBase)
	var seq genSeqCounters
	for _, b := range doc.Spec.Blocks {
		if b == nil {
			continue
		}
		gbn := b.gatorTestBlockName()
		if gbn == "" || b.Template == "" || b.Constraint == "" {
			return fmt.Errorf("block gator name (gatorBlock or name), template, constraint are required")
		}
		blockDefObj := strings.TrimSpace(b.DefaultObjectBase)
		constraintPath, err := resolveRenderedConstraintPath(baseDir, matrixDir, testsRoot, b.Constraint)
		if err != nil {
			return fmt.Errorf("block %q constraint: %w", gbn, err)
		}
		templatePath, err := resolveRenderedTemplatePath(baseDir, matrixDir, testsRoot, b.Template, b.Constraint)
		if err != nil {
			return fmt.Errorf("block %q template: %w", gbn, err)
		}
		tb := testBlockOut{Name: gbn, Template: templatePath, Constraint: constraintPath}
		for i := range b.Cases {
			c := &b.Cases[i]
			if c.Name == "" || (c.Violations != "yes" && c.Violations != "no") {
				return fmt.Errorf("block %q case %q: name and violations (yes|no) required", gbn, c.Name)
			}
			if err := applyNamedException(&doc.Spec, c); err != nil {
				return fmt.Errorf("block %q case %q: %w", gbn, c.Name, err)
			}
			assertion := map[string]interface{}{"violations": c.Violations}
			if c.AssertionMessage != "" {
				assertion["message"] = c.AssertionMessage
			}
			co := caseOut{
				Name:       c.Name,
				Assertions: []map[string]interface{}{assertion},
			}
			invList := append(append([]interface{}(nil), doc.Spec.DefaultInventory...), c.Inventory...)
			externalInvRef, err := resolveCaseExternalDataInventory(c, doc.Spec.ExternalData, samplesDir, baseDir, &seq, tracker)
			if err != nil {
				return fmt.Errorf("block %q case %s externalData inventory: %w", gbn, c.Name, err)
			}
			if externalInvRef != "" {
				invList = append(invList, map[string]interface{}{"ref": externalInvRef})
			}
			for j, inv := range invList {
				p, err := resolveMatrixInventoryItem(inv, doc.Spec.Bases, samplesDir, baseDir, fmt.Sprintf("%s-inv-%d", c.Name, j), &seq, tracker)
				if err != nil {
					return fmt.Errorf("block %q case %s inventory: %w", gbn, c.Name, err)
				}
				if p != "" {
					co.Inventory = append(co.Inventory, p)
				}
			}
			objDef := blockDefObj
			if objDef == "" {
				objDef = specDefObj
			}
			objPath, err := resolveMatrixObject(c.Object, doc.Spec.Bases, samplesDir, baseDir, c.Name, objDef, &seq, tracker)
			if err != nil {
				return fmt.Errorf("block %q case %s object: %w", gbn, c.Name, err)
			}
			co.Object = filepath.ToSlash(objPath)
			tb.Cases = append(tb.Cases, co)
		}
		suite.Tests = append(suite.Tests, tb)
	}

	buf := bytes.NewBufferString(generatedHeader)
	enc := yaml.NewEncoder(buf)
	enc.SetIndent(2)
	if err := enc.Encode(suite); err != nil {
		return err
	}
	_ = enc.Close()
	suitePath := filepath.Join(baseDir, "test_suite.yaml")
	if err := os.WriteFile(suitePath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write test_suite.yaml: %w", err)
	}
	return nil
}

func resolveCaseExternalDataInventory(c *matrixCase, defaults *matrixExternalData, samplesDir, outDir string, seq *genSeqCounters, tracker *generatedSampleTracker) (string, error) {
	cfg := defaults
	if c != nil && c.ExternalData != nil {
		cfg = c.ExternalData
	}
	if cfg == nil || len(cfg.Providers) == 0 {
		return "", nil
	}

	doc, err := buildExternalDataInventoryDoc(cfg)
	if err != nil {
		return "", err
	}

	caseName := "external-data"
	if c != nil && strings.TrimSpace(c.Name) != "" {
		caseName = c.Name
	}
	relPath := fmt.Sprintf("test_samples/external-data/%03d-%s.yaml", seq.next("other"), pathSlug(caseName+"-external-data"))
	return writeMatrixGeneratedYAML(relPath, doc, samplesDir, outDir, tracker, true)
}

func buildExternalDataInventoryDoc(cfg *matrixExternalData) (map[string]interface{}, error) {
	providers, err := matrixExternalDataProvidersToMap(cfg)
	if err != nil {
		return nil, err
	}
	doc := map[string]interface{}{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ExternalDataInventory",
		"metadata": map[string]interface{}{
			"name":      "default",
			"namespace": "testns",
		},
		"spec": map[string]interface{}{
			"providers": providers,
		},
	}
	if err := ensureObjectMetadataNameRFC1123StrictOrFallback(doc, "external-data", "externalData inventory"); err != nil {
		return nil, err
	}
	return doc, nil
}

func matrixExternalDataProvidersToMap(cfg *matrixExternalData) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(cfg.Providers))
	for i, pAny := range cfg.Providers {
		provider := map[string]interface{}{}
		b, err := yaml.Marshal(pAny)
		if err != nil {
			return nil, fmt.Errorf("provider[%d]: marshal: %w", i, err)
		}
		if err := yaml.Unmarshal(b, &provider); err != nil {
			return nil, fmt.Errorf("provider[%d]: parse: %w", i, err)
		}
		name, _ := provider["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("provider[%d]: field name is required", i)
		}
		delete(provider, "name")
		if _, exists := result[name]; exists {
			return nil, fmt.Errorf("duplicate provider name %q", name)
		}
		result[name] = provider
	}
	return result, nil
}

func resolveRenderedTemplatePath(renderedDir, sourceDir, testsRoot, relPath, constraintRelPath string) (string, error) {
	target := filepath.Join(renderedDir, "constraint-template.yaml")
	if err := renderConstraintTemplateFromHelm(target, renderedDir, sourceDir, testsRoot, constraintRelPath); err != nil {
		if copyErr := copyIntoRendered(target, renderedDir, sourceDir, testsRoot, relPath); copyErr != nil {
			return "", fmt.Errorf("render from helm failed: %v; fallback copy failed: %w", err, copyErr)
		}
	}
	if err := applyGatorTemplateOverride(target, sourceDir); err != nil {
		return "", err
	}
	return "constraint-template.yaml", nil
}

func applyGatorTemplateOverride(target, sourceDir string) error {
	overridePath := filepath.Join(sourceDir, "constraint-template.gator.yaml")
	if st, err := os.Stat(overridePath); err != nil || st.IsDir() {
		return nil
	}
	if err := copyFile(overridePath, target); err != nil {
		return fmt.Errorf("copy gator template override %q -> %q: %w", overridePath, target, err)
	}
	return nil
}

func resolveRenderedConstraintPath(renderedDir, sourceDir, testsRoot, relPath string) (string, error) {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(relPath)))
	if clean == "." || clean == "" {
		return "", fmt.Errorf("empty path")
	}

	// For constraints placed in the suite root constraints/ directory,
	// keep direct reference from rendered/test_suite.yaml and avoid copying.
	if strings.HasPrefix(clean, "constraints/") {
		baseRoot := filepath.Clean(filepath.Join(renderedDir, ".."))
		rootConstraint := filepath.Join(baseRoot, filepath.FromSlash(clean))
		if st, err := os.Stat(rootConstraint); err == nil && !st.IsDir() {
			return filepath.ToSlash(path.Join("..", clean)), nil
		}
	}

	var targetRel string
	switch {
	case strings.HasPrefix(clean, "rendered/constraints/"):
		targetRel = strings.TrimPrefix(clean, "rendered/")
	case strings.HasPrefix(clean, "constraints/"):
		targetRel = clean
	default:
		targetRel = path.Join("constraints", path.Base(clean))
	}

	target := filepath.Join(renderedDir, filepath.FromSlash(targetRel))
	if err := copyIntoRendered(target, renderedDir, sourceDir, testsRoot, relPath); err != nil {
		return "", err
	}
	return filepath.ToSlash(targetRel), nil
}

func copyIntoRendered(target, renderedDir, sourceDir, testsRoot, relPath string) error {
	if !strings.HasPrefix(target, renderedDir+string(os.PathSeparator)) && target != renderedDir {
		return fmt.Errorf("target path escapes rendered dir")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	src, err := resolveSourcePath(renderedDir, sourceDir, testsRoot, relPath)
	if err != nil {
		if _, statErr := os.Stat(target); statErr == nil {
			return nil
		}
		return err
	}
	return copyFile(src, target)
}

func resolveSourcePath(renderedDir, sourceDir, testsRoot, relPath string) (string, error) {
	cleanInput := strings.TrimSpace(relPath)
	if cleanInput == "" {
		return "", fmt.Errorf("empty path")
	}
	if abs, ok, err := resolveTokenToAbsPath(cleanInput, sourceDir, renderedDir); err != nil {
		return "", err
	} else if ok {
		if st, statErr := os.Stat(abs); statErr == nil && !st.IsDir() {
			return abs, nil
		}
		return "", fmt.Errorf("source file not found for %q", relPath)
	}

	clean := filepath.Clean(cleanInput)
	if clean == "." || clean == "" {
		return "", fmt.Errorf("empty path")
	}
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("absolute paths are not supported")
	}

	baseRoot := filepath.Clean(filepath.Join(renderedDir, ".."))
	cleanSlash := filepath.ToSlash(clean)
	candidates := []string{}
	if strings.TrimSpace(sourceDir) != "" {
		if strings.HasPrefix(cleanSlash, "../rendered/") {
			candidates = append(candidates,
				filepath.Clean(filepath.Join(sourceDir, "rendered", filepath.FromSlash(strings.TrimPrefix(cleanSlash, "../rendered/")))),
			)
		}
		candidates = append(candidates,
			filepath.Clean(filepath.Join(sourceDir, filepath.FromSlash(cleanSlash))),
			filepath.Clean(filepath.Join(sourceDir, "rendered", filepath.FromSlash(cleanSlash))),
		)
	}
	candidates = append(candidates,
		filepath.Clean(filepath.Join(baseRoot, filepath.FromSlash(cleanSlash))),
	)
	if strings.HasPrefix(cleanSlash, "rendered/") {
		candidates = append(candidates,
			filepath.Clean(filepath.Join(baseRoot, filepath.FromSlash(strings.TrimPrefix(cleanSlash, "rendered/")))),
		)
	}
	if strings.TrimSpace(testsRoot) != "" {
		if baseRoot, rootErr := resolveTestsRoot(testsRoot); rootErr == nil && baseRoot != "" {
			testsDir := filepath.Join(baseRoot, "tests")
			candidates = append(candidates,
				filepath.Clean(filepath.Join(testsDir, filepath.FromSlash(cleanSlash))),
				filepath.Clean(filepath.Join(baseRoot, filepath.FromSlash(cleanSlash))),
			)
		}
	}
	if strings.HasPrefix(cleanSlash, "constraints/") {
		candidates = append(candidates,
			filepath.Clean(filepath.Join(sourceDir, "..", "rendered", filepath.FromSlash(cleanSlash))),
		)
	}
	if idx := strings.Index(cleanSlash, "templates/"); idx >= 0 {
		if templatesRoot := findTemplatesRoot(sourceDir); templatesRoot != "" {
			templatesRel := cleanSlash[idx:]
			candidates = append(candidates,
				filepath.Clean(filepath.Join(templatesRoot, filepath.FromSlash(strings.TrimPrefix(templatesRel, "templates/")))),
			)
		}
	}

	for _, src := range candidates {
		if st, err := os.Stat(src); err == nil && !st.IsDir() {
			return src, nil
		}
	}
	return "", fmt.Errorf("source file not found for %q", relPath)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

var renderedConstraintTemplateCache = map[string]map[string][]byte{}

func renderConstraintTemplateFromHelm(target, renderedDir, sourceDir, testsRoot, constraintRelPath string) error {
	if !strings.HasPrefix(target, renderedDir+string(os.PathSeparator)) && target != renderedDir {
		return fmt.Errorf("target path escapes rendered dir")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	constraintSourcePath, err := resolveSourcePath(renderedDir, sourceDir, testsRoot, constraintRelPath)
	if err != nil {
		return fmt.Errorf("resolve constraint source %q: %w", constraintRelPath, err)
	}
	constraintKind, err := readConstraintKind(constraintSourcePath)
	if err != nil {
		return fmt.Errorf("read constraint kind from %q: %w", constraintSourcePath, err)
	}

	chartRoot := findConstraintTemplatesChartRoot(sourceDir, testsRoot, constraintSourcePath, renderedDir)
	if chartRoot == "" {
		return fmt.Errorf("cannot locate chart root for %q", constraintSourcePath)
	}
	templatesByKind, err := renderConstraintTemplatesByKind(chartRoot)
	if err != nil {
		return err
	}
	renderedTemplate, ok := templatesByKind[constraintKind]
	if !ok {
		available := make([]string, 0, len(templatesByKind))
		for k := range templatesByKind {
			available = append(available, k)
		}
		sort.Strings(available)
		return fmt.Errorf("constraint template for kind %q not found in chart %q (available: %s)", constraintKind, chartRoot, strings.Join(available, ", "))
	}
	return os.WriteFile(target, renderedTemplate, 0o644)
}

func readConstraintKind(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var head struct {
		Kind string `yaml:"kind"`
	}
	if err := yaml.Unmarshal(data, &head); err != nil {
		return "", err
	}
	kind := strings.TrimSpace(head.Kind)
	if kind == "" {
		return "", fmt.Errorf("kind is empty")
	}
	return kind, nil
}

func renderConstraintTemplatesByKind(chartRoot string) (map[string][]byte, error) {
	chartRoot = filepath.Clean(chartRoot)
	if cached, ok := renderedConstraintTemplateCache[chartRoot]; ok {
		return cached, nil
	}

	renderer := testhelm.Renderer{LintMode: false}
	files, err := renderer.RenderChartFromDir(chartRoot, "{}")
	if err != nil {
		return nil, fmt.Errorf("render chart %q: %w", chartRoot, err)
	}

	byKind := map[string][]byte{}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		rendered := files[p]
		if strings.TrimSpace(rendered) == "" {
			continue
		}
		docs := releaseutil.SplitManifests(rendered)
		docKeys := make([]string, 0, len(docs))
		for k := range docs {
			docKeys = append(docKeys, k)
		}
		sort.Strings(docKeys)

		for _, docKey := range docKeys {
			doc := strings.TrimSpace(docs[docKey])
			if doc == "" {
				continue
			}
			var meta struct {
				Kind string `yaml:"kind"`
				Spec struct {
					CRD struct {
						Spec struct {
							Names struct {
								Kind string `yaml:"kind"`
							} `yaml:"names"`
						} `yaml:"spec"`
					} `yaml:"crd"`
				} `yaml:"spec"`
			}
			if err := yaml.Unmarshal([]byte(doc), &meta); err != nil {
				continue
			}
			if meta.Kind != "ConstraintTemplate" {
				continue
			}
			constraintKind := strings.TrimSpace(meta.Spec.CRD.Spec.Names.Kind)
			if constraintKind == "" {
				continue
			}
			if _, exists := byKind[constraintKind]; exists {
				continue
			}

			renderedDoc := append([]byte(doc), '\n')
			sourcePath := sourceTemplatePathFromRenderedFilePath(chartRoot, p)
			renderedDoc, err = ensureConstraintTemplateLibs(renderedDoc, sourcePath)
			if err != nil {
				return nil, fmt.Errorf("ensure libs for %q (%q): %w", constraintKind, sourcePath, err)
			}
			byKind[constraintKind] = renderedDoc
		}
	}

	if len(byKind) == 0 {
		return nil, fmt.Errorf("no ConstraintTemplate manifests found in chart %q", chartRoot)
	}
	renderedConstraintTemplateCache[chartRoot] = byKind
	return byKind, nil
}

var (
	tplDefineRe   = regexp.MustCompile(`(?s)\{\{-\s*define\s+"([^"]+)"\s*-\}\}\s*\{\{\s*\.Files\.Get\s+"([^"]+)"\s*\}\}\s*\{\{-\s*end\s*-\}\}`)
	includeLineRe = regexp.MustCompile(`(?m)^([\t ]*)\{\{[- ]*include\s+"([^"]+)"\s+\.\s+\|\s+nindent\s+([0-9]+)\s*\}\}[\t ]*$`)
)

func ensureConstraintTemplateLibs(renderedDoc []byte, sourcePath string) ([]byte, error) {
	if strings.TrimSpace(sourcePath) == "" {
		return renderedDoc, nil
	}
	empty, err := hasEmptyConstraintTemplateLibs(renderedDoc)
	if err != nil || !empty {
		return renderedDoc, err
	}

	sourceLibs, err := extractConstraintTemplateLibsFromSource(sourcePath)
	if err != nil {
		return nil, err
	}
	if len(sourceLibs) == 0 {
		return renderedDoc, nil
	}
	return setConstraintTemplateLibs(renderedDoc, sourceLibs)
}

func hasEmptyConstraintTemplateLibs(renderedDoc []byte) (bool, error) {
	var ct map[string]interface{}
	if err := yaml.Unmarshal(renderedDoc, &ct); err != nil {
		return false, err
	}
	spec, _ := ct["spec"].(map[string]interface{})
	targets, _ := spec["targets"].([]interface{})
	for _, t := range targets {
		tm, _ := t.(map[string]interface{})
		code, _ := tm["code"].([]interface{})
		for _, c := range code {
			cm, _ := c.(map[string]interface{})
			source, _ := cm["source"].(map[string]interface{})
			libs, _ := source["libs"].([]interface{})
			if len(libs) == 0 {
				continue
			}
			hasEmpty := false
			for _, l := range libs {
				s, _ := l.(string)
				if strings.TrimSpace(s) == "" {
					hasEmpty = true
					break
				}
			}
			if hasEmpty {
				return true, nil
			}
		}
	}
	return false, nil
}

func setConstraintTemplateLibs(renderedDoc []byte, libs []string) ([]byte, error) {
	var ct map[string]interface{}
	if err := yaml.Unmarshal(renderedDoc, &ct); err != nil {
		return nil, err
	}
	spec, _ := ct["spec"].(map[string]interface{})
	targets, _ := spec["targets"].([]interface{})
	for i, t := range targets {
		tm, _ := t.(map[string]interface{})
		code, _ := tm["code"].([]interface{})
		for j, c := range code {
			cm, _ := c.(map[string]interface{})
			source, _ := cm["source"].(map[string]interface{})
			if source == nil {
				continue
			}
			arr := make([]interface{}, 0, len(libs))
			for _, l := range libs {
				arr = append(arr, l)
			}
			source["libs"] = arr
			cm["source"] = source
			code[j] = cm
		}
		tm["code"] = code
		targets[i] = tm
	}
	spec["targets"] = targets
	ct["spec"] = spec

	b, err := yaml.Marshal(ct)
	if err != nil {
		return nil, err
	}
	return append(bytes.TrimSpace(b), '\n'), nil
}

func extractConstraintTemplateLibsFromSource(sourcePath string) ([]string, error) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, err
	}
	rendered, _, err := renderConstraintTemplateIncludes(data, sourcePath)
	if err != nil {
		return nil, err
	}
	var ct map[string]interface{}
	if err := yaml.Unmarshal(rendered, &ct); err != nil {
		return nil, err
	}
	spec, _ := ct["spec"].(map[string]interface{})
	targets, _ := spec["targets"].([]interface{})
	for _, t := range targets {
		tm, _ := t.(map[string]interface{})
		code, _ := tm["code"].([]interface{})
		for _, c := range code {
			cm, _ := c.(map[string]interface{})
			source, _ := cm["source"].(map[string]interface{})
			libsAny, _ := source["libs"].([]interface{})
			libs := make([]string, 0, len(libsAny))
			for _, l := range libsAny {
				s, _ := l.(string)
				if strings.TrimSpace(s) != "" {
					libs = append(libs, s)
				}
			}
			if len(libs) > 0 {
				return libs, nil
			}
		}
	}
	return nil, nil
}

func sourceTemplatePathFromRenderedFilePath(chartRoot, renderedFilePath string) string {
	clean := filepath.ToSlash(strings.TrimSpace(renderedFilePath))
	if clean == "" {
		return ""
	}
	clean = strings.TrimPrefix(clean, "constraint-templates/")
	clean = strings.TrimPrefix(clean, "./")
	path := filepath.Join(chartRoot, filepath.FromSlash(clean))
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		return path
	}
	return ""
}

func renderConstraintTemplateIncludes(data []byte, srcPath string) ([]byte, bool, error) {
	if len(includeLineRe.FindAllSubmatchIndex(data, -1)) == 0 {
		return data, false, nil
	}

	chartRoot := findConstraintTemplatesChartRoot(srcPath)
	if chartRoot == "" {
		return nil, false, fmt.Errorf("cannot resolve chart root for template source %q", srcPath)
	}
	tplPath := filepath.Join(chartRoot, "templates", "libs", "_rego-libs.tpl")
	tplData, err := os.ReadFile(tplPath)
	if err != nil {
		return nil, false, fmt.Errorf("read rego libs template %q: %w", tplPath, err)
	}

	libByInclude := map[string]string{}
	for _, m := range tplDefineRe.FindAllSubmatch(tplData, -1) {
		if len(m) < 3 {
			continue
		}
		libByInclude[string(m[1])] = string(m[2])
	}

	var out bytes.Buffer
	last := 0
	for _, m := range includeLineRe.FindAllSubmatchIndex(data, -1) {
		if len(m) < 8 {
			continue
		}
		out.Write(data[last:m[0]])
		includeName := string(data[m[4]:m[5]])
		nindentRaw := string(data[m[6]:m[7]])
		nindentVal, convErr := strconv.Atoi(nindentRaw)
		if convErr != nil {
			return nil, false, fmt.Errorf("invalid nindent value %q for include %q: %w", nindentRaw, includeName, convErr)
		}
		libRel, ok := libByInclude[includeName]
		if !ok {
			return nil, false, fmt.Errorf("unknown include %q in %s", includeName, srcPath)
		}
		libPath := filepath.Join(chartRoot, filepath.FromSlash(libRel))
		libData, readErr := os.ReadFile(libPath)
		if readErr != nil {
			return nil, false, fmt.Errorf("read include file %q: %w", libPath, readErr)
		}
		trimmed := strings.TrimRight(string(libData), "\n")
		if trimmed != "" {
			lines := strings.Split(trimmed, "\n")
			indent := strings.Repeat(" ", nindentVal)
			for i, ln := range lines {
				if i > 0 {
					out.WriteByte('\n')
				}
				out.WriteString(indent)
				out.WriteString(ln)
			}
		}
		out.WriteByte('\n')
		last = m[1]
	}
	out.Write(data[last:])
	return out.Bytes(), true, nil
}

func findConstraintTemplatesChartRoot(paths ...string) string {
	for _, p := range paths {
		dir := strings.TrimSpace(p)
		if dir == "" {
			continue
		}
		dir = filepath.Clean(dir)
		if st, err := os.Stat(dir); err == nil && !st.IsDir() {
			dir = filepath.Dir(dir)
		}
		for {
			if isConstraintTemplatesChartRoot(dir) {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return ""
}

func isConstraintTemplatesChartRoot(dir string) bool {
	chartPath := filepath.Join(dir, "Chart.yaml")
	templatesPath := filepath.Join(dir, "templates")
	chartInfo, chartErr := os.Stat(chartPath)
	templatesInfo, templatesErr := os.Stat(templatesPath)
	return chartErr == nil && !chartInfo.IsDir() && templatesErr == nil && templatesInfo.IsDir()
}

func findTemplatesRoot(sourceDir string) string {
	dir := filepath.Clean(strings.TrimSpace(sourceDir))
	if dir == "" {
		return ""
	}
	for {
		templatesDir := filepath.Join(dir, "templates")
		if st, err := os.Stat(templatesDir); err == nil && st.IsDir() {
			return templatesDir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func resolveMatrixOutputDir(matrixDir, outputDir, testsRoot string) string {
	if filepath.IsAbs(outputDir) {
		return outputDir
	}
	local := filepath.Clean(filepath.Join(matrixDir, outputDir))
	if outputDir == "rendered" {
		return local
	}
	if strings.TrimSpace(testsRoot) == "" {
		return local
	}
	baseRoot, err := resolveTestsRoot(testsRoot)
	if err != nil || baseRoot == "" {
		return local
	}
	cand := filepath.Clean(filepath.Join(baseRoot, "tests", filepath.FromSlash(filepath.ToSlash(outputDir))))
	if _, statErr := os.Stat(filepath.Dir(cand)); statErr == nil {
		return cand
	}
	return local
}

func normalizeMatrixDoc(doc *matrixDoc) {
	for _, b := range doc.Spec.Blocks {
		if b == nil {
			continue
		}
		if b.Name == "" && strings.TrimSpace(b.GatorBlock) != "" {
			b.Name = b.GatorBlock + "-cases"
		}
	}
}

const spePodLabelKey = "security.deckhouse.io/security-policy-exception"

func exceptionNameFromCase(c *matrixCase) string {
	if s := strings.TrimSpace(c.ExceptionRef); s != "" {
		return s
	}
	return strings.TrimSpace(c.Exception)
}

func applyNamedException(spec *matrixSpec, c *matrixCase) error {
	rawExName := exceptionNameFromCase(c)
	if rawExName == "" {
		return nil
	}
	if spec.NamedExceptions == nil {
		return fmt.Errorf("namedExceptions undefined but case references %q", rawExName)
	}
	def, ok := spec.NamedExceptions[rawExName]
	if !ok {
		return fmt.Errorf("unknown namedExceptions key %q", rawExName)
	}
	if err := validateRFC1123SubdomainName(rawExName); err != nil {
		return fmt.Errorf("exception %q name: %w", rawExName, err)
	}
	exName := rawExName
	for _, inv := range c.Inventory {
		m, ok := inv.(map[string]interface{})
		if !ok {
			continue
		}
		bn, _ := m["base"].(string)
		if bn == "securityPolicyException" {
			return fmt.Errorf("use either exception/exceptionRef or explicit inventory base securityPolicyException, not both")
		}
	}
	baseName := strings.TrimSpace(def.Base)
	if baseName == "" {
		baseName = "securityPolicyException"
	}
	mergeTpl := def.Merge
	if mergeTpl == nil {
		mergeTpl = map[string]interface{}{}
	}
	metaNamePatch := map[string]interface{}{
		"metadata": map[string]interface{}{"name": exName},
	}
	fullMerge := deepMerge(mergeTpl, metaNamePatch)
	synth := map[string]interface{}{
		"base":  baseName,
		"merge": fullMerge,
	}
	c.Inventory = append([]interface{}{synth}, c.Inventory...)

	objMap, ok := c.Object.(map[string]interface{})
	if !ok {
		return fmt.Errorf("exception requires object as a mapping with base/merge (not ref)")
	}
	if ref, _ := objMap["ref"].(string); strings.TrimSpace(ref) != "" {
		return fmt.Errorf("exception cannot be used together with object ref")
	}
	labPatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				spePodLabelKey: exName,
			},
		},
	}
	var curMerge map[string]interface{}
	if m, ok := objMap["merge"].(map[string]interface{}); ok && m != nil {
		curMerge = m
	}
	objMap["merge"] = deepMerge(labPatch, curMerge)
	return nil
}

func applyDefaultPodMetadataName(doc any, podName string) {
	m, ok := doc.(map[string]interface{})
	if !ok {
		return
	}
	kind, _ := m["kind"].(string)
	if !strings.EqualFold(kind, "Pod") {
		return
	}
	meta, _ := m["metadata"].(map[string]interface{})
	if meta == nil {
		meta = make(map[string]interface{})
		m["metadata"] = meta
	}
	n, has := meta["name"]
	nameStr, _ := n.(string)
	if has && strings.TrimSpace(nameStr) != "" {
		return
	}
	if strings.TrimSpace(podName) == "" {
		return
	}
	meta["name"] = podName
}

func extractMergeAndContainerPatches(v map[string]interface{}) (map[string]interface{}, []interface{}, []interface{}) {
	merge, _ := v["merge"].(map[string]interface{})
	var cMerges []interface{}
	if x, ok := v["containerMerges"].([]interface{}); ok {
		cMerges = x
	}
	var icMerges []interface{}
	if x, ok := v["initContainerMerges"].([]interface{}); ok {
		icMerges = x
	}
	return merge, cMerges, icMerges
}

func resolveMatrixInventoryItem(raw interface{}, bases map[string]matrixBase, samplesDir, outDir, autoSlug string, seq *genSeqCounters, tracker *generatedSampleTracker) (string, error) {
	switch v := raw.(type) {
	case string:
		ref := strings.TrimSpace(v)
		if ref == "" {
			return "", fmt.Errorf("empty inventory ref")
		}
		ref, err := normalizeRefForSuite(ref, outDir)
		if err != nil {
			return "", err
		}
		if tracker != nil {
			return tracker.dedupRefPath(ref)
		}
		return filepath.ToSlash(ref), nil
	case map[string]interface{}:
		if ref, ok := v["ref"].(string); ok && ref != "" {
			if v["base"] != nil || v["merge"] != nil || v["path"] != nil || v["containerMerges"] != nil || v["initContainerMerges"] != nil {
				return "", fmt.Errorf("inventory item: ref cannot mix with base/merge/path/containerMerges/initContainerMerges")
			}
			ref, err := normalizeRefForSuite(ref, outDir)
			if err != nil {
				return "", err
			}
			if tracker != nil {
				return tracker.dedupRefPath(ref)
			}
			return filepath.ToSlash(ref), nil
		}
		baseName, _ := v["base"].(string)
		if baseName == "" {
			return "", fmt.Errorf("inventory item: need ref or (base + optional path + optional merge + optional containerMerges)")
		}
		pathOut, _ := v["path"].(string)
		explicitPath := strings.TrimSpace(pathOut) != ""
		if !explicitPath {
			pathOut = allocGenPath(seq, autoSlug, bases, baseName)
		}
		merge, cj, icj := extractMergeAndContainerPatches(v)
		merged, err := mergeDocFromMatrixParts(bases, baseName, merge, cj, icj)
		if err != nil {
			return "", err
		}
		if err := ensureObjectMetadataNameRFC1123StrictOrFallback(merged, autoSlug, fmt.Sprintf("inventory item base=%q", baseName)); err != nil {
			return "", err
		}
		rel, err := writeMatrixGeneratedYAML(pathOut, merged, samplesDir, outDir, tracker, !explicitPath)
		if err != nil {
			return "", err
		}
		return rel, nil
	default:
		return "", fmt.Errorf("inventory item: unsupported type %T", raw)
	}
}

func resolveMatrixObject(raw interface{}, bases map[string]matrixBase, samplesDir, outDir, autoSlug, objectDefaultBase string, seq *genSeqCounters, tracker *generatedSampleTracker) (string, error) {
	v, ok := raw.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("object must be a mapping, got %T", raw)
	}
	podName, _ := v["podName"].(string)
	podName = strings.TrimSpace(podName)

	if ref, ok := v["ref"].(string); ok && ref != "" {
		if v["base"] != nil || v["merge"] != nil || v["path"] != nil || v["containerMerges"] != nil || v["initContainerMerges"] != nil || podName != "" {
			return "", fmt.Errorf("object: ref cannot mix with base/merge/path/containerMerges/podName")
		}
		ref, err := normalizeRefForSuite(ref, outDir)
		if err != nil {
			return "", err
		}
		if tracker != nil {
			return tracker.dedupRefPath(ref)
		}
		return filepath.ToSlash(ref), nil
	}
	baseName, _ := v["base"].(string)
	baseName = strings.TrimSpace(baseName)
	if baseName == "" {
		baseName = strings.TrimSpace(objectDefaultBase)
	}
	if baseName == "" {
		return "", fmt.Errorf("object: need ref or base (or spec/block defaultObjectBase)")
	}
	pathOut, _ := v["path"].(string)
	explicitPath := strings.TrimSpace(pathOut) != ""
	if !explicitPath {
		pathOut = allocGenPath(seq, autoSlug, bases, baseName)
	}
	merge, cj, icj := extractMergeAndContainerPatches(v)
	merged, err := mergeDocFromMatrixParts(bases, baseName, merge, cj, icj)
	if err != nil {
		return "", err
	}
	applyDefaultPodMetadataName(merged, podName)
	if err := ensureObjectMetadataNameRFC1123StrictOrFallback(merged, autoSlug, fmt.Sprintf("object base=%q", baseName)); err != nil {
		return "", err
	}
	if err := validatePodExceptionLabelRFC1123(merged, fmt.Sprintf("object base=%q", baseName)); err != nil {
		return "", err
	}
	rel, err := writeMatrixGeneratedYAML(pathOut, merged, samplesDir, outDir, tracker, !explicitPath)
	if err != nil {
		return "", err
	}
	return rel, nil
}

func collectMatrixRefPaths(spec *matrixSpec, samplesDir, outDir string) map[string]struct{} {
	refs := map[string]struct{}{}
	addRef := func(ref string) {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return
		}
		ref = filepath.ToSlash(ref)
		if !strings.HasPrefix(ref, "test_samples/") {
			return
		}
		full := outputPathForWrite(ref, samplesDir, outDir)
		refs[full] = struct{}{}
	}
	if spec == nil {
		return refs
	}
	for _, inv := range spec.DefaultInventory {
		switch v := inv.(type) {
		case string:
			addRef(v)
		case map[string]interface{}:
			if ref, ok := v["ref"].(string); ok {
				addRef(ref)
			}
		}
	}
	for _, b := range spec.Blocks {
		if b == nil {
			continue
		}
		for i := range b.Cases {
			c := &b.Cases[i]
			for _, inv := range c.Inventory {
				switch v := inv.(type) {
				case string:
					addRef(v)
				case map[string]interface{}:
					if ref, ok := v["ref"].(string); ok {
						addRef(ref)
					}
				}
			}
			if obj, ok := c.Object.(map[string]interface{}); ok {
				if ref, ok := obj["ref"].(string); ok {
					addRef(ref)
				}
			}
		}
	}
	return refs
}

type generatedSampleTracker struct {
	refPaths   map[string]struct{}
	hashToRel  map[string]string
	samplesDir string
	outDir     string
}

func newGeneratedSampleTracker(refPaths map[string]struct{}, outDir, samplesDir string) *generatedSampleTracker {
	refs := map[string]struct{}{}
	for k := range refPaths {
		refs[k] = struct{}{}
	}
	return &generatedSampleTracker{
		refPaths:   refs,
		hashToRel:  map[string]string{},
		samplesDir: samplesDir,
		outDir:     outDir,
	}
}

func (t *generatedSampleTracker) dedupRefPath(ref string) (string, error) {
	if t == nil {
		return filepath.ToSlash(ref), nil
	}
	ref = filepath.ToSlash(strings.TrimSpace(ref))
	if ref == "" {
		return "", fmt.Errorf("empty inventory ref")
	}
	if !strings.HasPrefix(ref, "test_samples/") {
		return ref, nil
	}
	full := outputPathForWrite(ref, t.samplesDir, t.outDir)
	t.refPaths[full] = struct{}{}
	data, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			return ref, nil
		}
		return "", err
	}
	if !bytes.HasPrefix(data, []byte(generatedHeader)) {
		return ref, nil
	}
	hash := hashBytes(data)
	if existing, ok := t.hashToRel[hash]; ok && existing != "" {
		return existing, nil
	}
	t.hashToRel[hash] = ref
	return ref, nil
}

func (t *generatedSampleTracker) isRefPath(path string) bool {
	if t == nil {
		return false
	}
	_, ok := t.refPaths[path]
	return ok
}

func (t *generatedSampleTracker) lookupHash(hash string) (string, bool) {
	if t == nil {
		return "", false
	}
	rel, ok := t.hashToRel[hash]
	return rel, ok
}

func (t *generatedSampleTracker) recordHash(hash, rel string) {
	if t == nil || hash == "" || rel == "" {
		return
	}
	if _, ok := t.hashToRel[hash]; ok {
		return
	}
	t.hashToRel[hash] = rel
}

func writeMatrixGeneratedYAML(relPath string, doc interface{}, samplesDir, outDir string, tracker *generatedSampleTracker, allowDedup bool) (string, error) {
	b, err := yaml.Marshal(doc)
	if err != nil {
		return "", err
	}
	out := append([]byte(generatedHeader), b...)
	hash := hashBytes(out)
	full := outputPathForWrite(relPath, samplesDir, outDir)
	rel, err := filepath.Rel(outDir, full)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	if tracker != nil {
		if tracker.isRefPath(full) {
			data, err := os.ReadFile(full)
			if err != nil {
				if !os.IsNotExist(err) {
					return "", err
				}
			} else {
				if !bytes.Equal(data, out) {
					return "", fmt.Errorf("ref path %s does not match generated content", rel)
				}
				return rel, nil
			}
		} else if allowDedup {
			if existing, ok := tracker.lookupHash(hash); ok {
				return existing, nil
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(full, out, 0o644); err != nil {
		return "", err
	}
	if tracker != nil && allowDedup {
		tracker.recordHash(hash, rel)
	}
	return rel, nil
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func cleanupRenderedSamples(samplesDir string, refPaths map[string]struct{}) error {
	if _, err := os.Stat(samplesDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var dirs []string
	err := filepath.WalkDir(samplesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != samplesDir {
				dirs = append(dirs, path)
			}
			return nil
		}
		if _, ok := refPaths[path]; ok {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		generated, err := isGeneratedFile(path)
		if err != nil {
			return err
		}
		if generated {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		_ = os.Remove(dirs[i])
	}
	return nil
}

func isGeneratedFile(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return bytes.HasPrefix(data, []byte(generatedHeader)), nil
}

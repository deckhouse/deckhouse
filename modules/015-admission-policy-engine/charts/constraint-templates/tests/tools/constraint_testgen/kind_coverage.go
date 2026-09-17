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
	"sort"

	"gopkg.in/yaml.v3"
)

// kindCoverageDoc is a minimal parser for test-matrix.yaml that extracts
// the object kind from each case's base document, without depending on the
// full matrix.go types.
type kindCoverageDoc struct {
	Kind string `yaml:"kind"`
	Spec struct {
		DefaultObjectBase string `yaml:"defaultObjectBase"`
		Bases             map[string]struct {
			Document struct {
				Kind string `yaml:"kind"`
			} `yaml:"document"`
		} `yaml:"bases"`
		Blocks []struct {
			Name              string `yaml:"name"`
			DefaultObjectBase string `yaml:"defaultObjectBase"`
			Cases             []struct {
				Name   string `yaml:"name"`
				Object struct {
					Base string `yaml:"base"`
				} `yaml:"object"`
			} `yaml:"cases"`
		} `yaml:"blocks"`
	} `yaml:"spec"`
}

// kindCoverageResult reports which required object kinds are covered by the
// test matrix, and which are missing.
//
// Admission operations are deliberately not tracked here. A gator suite case
// carries only object, inventory and assertions — there is no way to declare
// the review operation — so any per-operation requirement would be reported as
// permanently unmet. A check that can never pass is worse than no check: it
// trains readers to skip the warnings that do mean something. Operation-level
// behaviour (for example the `is_update` guard in
// automount-service-account-token) is covered by the OPA unit tests under
// files/libs instead.
type kindCoverageResult struct {
	RequiredKinds     []string
	CoveredKinds      []string
	MissingKinds      []string
	NameInferredCount int
}

// computeKindCoverage reads test_fields.yaml (for required kinds) and
// test-matrix.yaml (for actual case kinds), then cross-checks them.
//
// Object kind is a first-class coverage dimension: a rule extended to match
// controllers but tested only against Pods looks fully covered otherwise.
//
// It also counts cases that rely on name-based scenario inference (no explicit
// fields[] block) and warns about them, since a renamed case then silently
// stops counting towards coverage.
func computeKindCoverage(dir string, fields *testFieldsDoc) (*kindCoverageResult, error) {
	matrixPath := filepath.Join(dir, "test-matrix.yaml")
	b, err := os.ReadFile(matrixPath)
	if err != nil {
		return nil, nil // no matrix = no kind coverage to compute
	}
	var doc kindCoverageDoc
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("parse test-matrix.yaml for kind coverage: %w", err)
	}
	if doc.Kind != "ConstraintTestMatrix" {
		return nil, nil
	}

	// Collect covered kinds from case bases.
	coveredKindSet := map[string]struct{}{}
	for _, block := range doc.Spec.Blocks {
		blockDefault := block.DefaultObjectBase
		if blockDefault == "" {
			blockDefault = doc.Spec.DefaultObjectBase
		}
		for _, c := range block.Cases {
			// Determine the kind from the object base.
			baseName := c.Object.Base
			if baseName == "" {
				baseName = blockDefault
			}
			if baseName != "" {
				if base, ok := doc.Spec.Bases[baseName]; ok && base.Document.Kind != "" {
					coveredKindSet[base.Document.Kind] = struct{}{}
				}
			}
		}
	}

	// fields[] are not visible in the minimal struct above, so count
	// name-inferred cases from a full parse of the same bytes.
	nameInferred := countNameInferredCases(b)

	result := &kindCoverageResult{
		NameInferredCount: nameInferred,
	}

	// Required kinds from test_fields.yaml.
	if fields != nil && len(fields.Spec.ObjectKinds) > 0 {
		result.RequiredKinds = fields.Spec.ObjectKinds
		for _, k := range fields.Spec.ObjectKinds {
			if _, ok := coveredKindSet[k]; ok {
				result.CoveredKinds = append(result.CoveredKinds, k)
			} else {
				result.MissingKinds = append(result.MissingKinds, k)
			}
		}
	} else {
		// No required kinds declared — just report what we found.
		for k := range coveredKindSet {
			result.CoveredKinds = append(result.CoveredKinds, k)
		}
	}

	sort.Strings(result.CoveredKinds)
	sort.Strings(result.MissingKinds)

	return result, nil
}

// countNameInferredCases parses the full matrix YAML and counts cases that
// have no explicit fields[] block — i.e. cases that rely on name-based
// scenario inference (the fragile T6 pattern).
func countNameInferredCases(data []byte) int {
	var doc matrixCoverageDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return 0
	}
	if doc.Kind != "ConstraintTestMatrix" {
		return 0
	}
	count := 0
	for _, block := range doc.Spec.Blocks {
		for _, c := range block.Cases {
			if len(c.Fields) == 0 {
				count++
			}
		}
	}
	return count
}

// kindCoverageWarnings converts a kindCoverageResult into profile warnings.
func kindCoverageWarnings(r *kindCoverageResult) []string {
	if r == nil {
		return nil
	}
	var warns []string
	for _, k := range r.MissingKinds {
		warns = append(warns, fmt.Sprintf("missing test case for required object kind %q (declared in test_fields.yaml spec.objectKinds)", k))
	}
	// Warn when cases rely on name-based inference.
	if r.NameInferredCount > 0 {
		warns = append(warns, fmt.Sprintf("%d case(s) have no explicit fields[] block — relying on case-name substring matching for scenario coverage (deprecated, add fields[] to each case)", r.NameInferredCount))
	}
	return warns
}

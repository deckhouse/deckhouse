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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const sampleConstraintName = "allow-host-network"

type profileDoc struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		TestDirectory string `yaml:"testDirectory"`
		CheckedFields []struct {
			Path    string `yaml:"path"`
			Type    string `yaml:"type"`
			SpePath string `yaml:"spePath"`
		} `yaml:"checkedFields"`
		Coverage *struct {
			MinimumCasesPerBlock int                 `yaml:"minimumCasesPerBlock"`
			RequiredPatterns     map[string][]string `yaml:"requiredPatterns"`
		} `yaml:"coverage"`
		Suite struct {
			ExpectedTestBlockNames []string `yaml:"expectedTestBlockNames"`
		} `yaml:"suite"`
	} `yaml:"spec"`
}

type suiteDoc struct {
	Tests []struct {
		Name  string `yaml:"name"`
		Cases []struct {
			Name string `yaml:"name"`
		} `yaml:"cases"`
	} `yaml:"tests"`
}

func verify(testsRoot string) error {
	var errs []string
	err := filepath.WalkDir(testsRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Base(path) != "test_profile.yaml" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var doc profileDoc
		if err := yaml.Unmarshal(b, &doc); err != nil {
			errs = append(errs, fmt.Sprintf("%s: parse profile: %v", path, err))
			return nil
		}
		if doc.Kind != "ConstraintTestProfile" {
			return nil
		}
		if doc.Spec.TestDirectory == "" {
			errs = append(errs, fmt.Sprintf("%s: spec.testDirectory is empty", path))
			return nil
		}
		want := doc.Spec.Suite.ExpectedTestBlockNames
		if len(want) == 0 {
			errs = append(errs, fmt.Sprintf("%s: spec.suite.expectedTestBlockNames is empty", path))
			return nil
		}
		suitePath := filepath.Join(filepath.Dir(path), "rendered", "test_suite.yaml")
		sb, err := os.ReadFile(suitePath)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: read suite %s: %v", filepath.Base(path), suitePath, err))
			return nil
		}
		var suite suiteDoc
		if err := yaml.Unmarshal(sb, &suite); err != nil {
			errs = append(errs, fmt.Sprintf("%s: parse %s: %v", filepath.Base(path), suitePath, err))
			return nil
		}
		got := make(map[string]struct{}, len(suite.Tests))
		for _, t := range suite.Tests {
			if t.Name != "" {
				got[t.Name] = struct{}{}
			}
		}
		for _, name := range want {
			if _, ok := got[name]; !ok {
				errs = append(errs, fmt.Sprintf("%s (%s): suite missing test block %q", filepath.Base(path), doc.Spec.TestDirectory, name))
			}
		}
		if doc.Spec.Coverage != nil {
			cov := doc.Spec.Coverage
			if cov.MinimumCasesPerBlock > 0 {
				for _, t := range suite.Tests {
					if len(t.Cases) < cov.MinimumCasesPerBlock {
						errs = append(errs, fmt.Sprintf("%s (%s): block %q has %d cases, minimum %d", filepath.Base(path), doc.Spec.TestDirectory, t.Name, len(t.Cases), cov.MinimumCasesPerBlock))
					}
				}
			}
			for track, patterns := range cov.RequiredPatterns {
				for _, pattern := range patterns {
					if !verifyHasPattern(suite.Tests, track, pattern) {
						errs = append(errs, fmt.Sprintf("%s (%s): missing case matching %q in track %q", filepath.Base(path), doc.Spec.TestDirectory, pattern, track))
					}
				}
			}
		}
		testFieldsPath := filepath.Join(filepath.Dir(path), "test_fields.yaml")
		if _, err := os.Stat(testFieldsPath); err == nil {
			fieldsDoc, err := loadTestFields(testFieldsPath)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s (%s): read test_fields.yaml: %v", filepath.Base(path), doc.Spec.TestDirectory, err))
				return nil
			}
			if err := validateTestFields(fieldsDoc); err != nil {
				errs = append(errs, fmt.Sprintf("%s (%s): invalid test_fields.yaml: %v", filepath.Base(path), doc.Spec.TestDirectory, err))
				return nil
			}
			if fieldsDoc.Metadata.Name != filepath.Base(doc.Spec.TestDirectory) {
				errs = append(errs, fmt.Sprintf("%s (%s): test_fields.yaml metadata.name must be %q", filepath.Base(path), doc.Spec.TestDirectory, filepath.Base(doc.Spec.TestDirectory)))
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}

func verifyHasPattern(blocks []struct {
	Name  string `yaml:"name"`
	Cases []struct {
		Name string `yaml:"name"`
	} `yaml:"cases"`
}, track, pattern string) bool {
	for _, t := range blocks {
		if classifyTrack(t.Name) != track {
			continue
		}
		for _, c := range t.Cases {
			if matchGlob(pattern, c.Name) {
				return true
			}
		}
	}
	return false
}

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

	"gopkg.in/yaml.v3"
)

type testFieldsDoc struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		ObjectKind       string          `yaml:"objectKind"`
		ObjectFields     []testFieldSpec `yaml:"objectFields"`
		SpeFields        []testFieldSpec `yaml:"speFields"`
		ApplicableTracks struct {
			Functional   *bool `yaml:"functional"`
			SpePod       *bool `yaml:"spePod"`
			SpeContainer *bool `yaml:"speContainer"`
		} `yaml:"applicableTracks"`
	} `yaml:"spec"`
}

type testFieldSpec struct {
	Path              string   `yaml:"path"`
	Level             string   `yaml:"level"`
	DefaultBehavior   string   `yaml:"defaultBehavior"`
	Description       string   `yaml:"description"`
	RequiredScenarios []string `yaml:"requiredScenarios"`
}

func loadTestFields(path string) (*testFieldsDoc, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc testFieldsDoc
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	normalizeTestFields(&doc)
	return &doc, nil
}

func validateTestFields(doc *testFieldsDoc) error {
	if doc.Kind != "ConstraintTestFields" {
		return fmt.Errorf("kind must be ConstraintTestFields")
	}
	if doc.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is empty")
	}
	if doc.Spec.ObjectKind == "" {
		return fmt.Errorf("spec.objectKind is empty")
	}
	if len(doc.Spec.ObjectFields) == 0 {
		return fmt.Errorf("spec.objectFields is empty")
	}
	for i, field := range doc.Spec.ObjectFields {
		if field.Path == "" {
			return fmt.Errorf("spec.objectFields[%d].path is empty", i)
		}
		if field.Level != "" && !isAllowedFieldLevel(field.Level, true) {
			return fmt.Errorf("spec.objectFields[%d].level is invalid", i)
		}
		for _, scenario := range field.RequiredScenarios {
			if !isAllowedScenario(scenario, false) {
				return fmt.Errorf("spec.objectFields[%d].requiredScenarios includes invalid value %q", i, scenario)
			}
		}
	}
	for i, field := range doc.Spec.SpeFields {
		if field.Path == "" {
			return fmt.Errorf("spec.speFields[%d].path is empty", i)
		}
		if field.Level != "" && !isAllowedFieldLevel(field.Level, false) {
			return fmt.Errorf("spec.speFields[%d].level is invalid", i)
		}
		for _, scenario := range field.RequiredScenarios {
			if !isAllowedScenario(scenario, true) {
				return fmt.Errorf("spec.speFields[%d].requiredScenarios includes invalid value %q", i, scenario)
			}
		}
	}
	if !hasApplicableTrack(doc.Spec.ApplicableTracks) {
		return fmt.Errorf("spec.applicableTracks has no enabled tracks")
	}
	return nil
}

func isAllowedFieldLevel(level string, allowInit bool) bool {
	switch level {
	case "pod", "container":
		return true
	case "initContainer":
		return allowInit
	default:
		return false
	}
}

func isAllowedScenario(scenario string, isSpe bool) bool {
	switch scenario {
	case "positive", "negative", "absent", "multiContainer", "initContainer", "ephemeralContainer":
		return !isSpe
	case "speMatch", "speMismatch", "speAbsent", "speContainerSpecific":
		return isSpe
	default:
		return false
	}
}

func normalizeTestFields(doc *testFieldsDoc) {
	for i, field := range doc.Spec.ObjectFields {
		if len(field.RequiredScenarios) == 0 {
			doc.Spec.ObjectFields[i].RequiredScenarios = defaultRequiredScenarios(field.Level, false)
		}
	}
	for i, field := range doc.Spec.SpeFields {
		if len(field.RequiredScenarios) == 0 {
			doc.Spec.SpeFields[i].RequiredScenarios = defaultRequiredScenarios(field.Level, true)
		}
	}
}

func defaultRequiredScenarios(level string, isSpe bool) []string {
	if isSpe {
		switch level {
		case "container":
			return []string{"speMatch", "speMismatch", "speAbsent", "speContainerSpecific"}
		case "pod":
			return []string{"speMatch", "speMismatch", "speAbsent"}
		default:
			return nil
		}
	}
	switch level {
	case "container":
		return []string{"positive", "negative", "absent", "multiContainer", "initContainer", "ephemeralContainer"}
	case "initContainer", "pod":
		return []string{"positive", "negative", "absent"}
	default:
		return nil
	}
}

func hasApplicableTrack(tracks struct {
	Functional   *bool `yaml:"functional"`
	SpePod       *bool `yaml:"spePod"`
	SpeContainer *bool `yaml:"speContainer"`
}) bool {
	if tracks.Functional != nil && *tracks.Functional {
		return true
	}
	if tracks.SpePod != nil && *tracks.SpePod {
		return true
	}
	if tracks.SpeContainer != nil && *tracks.SpeContainer {
		return true
	}
	return false
}

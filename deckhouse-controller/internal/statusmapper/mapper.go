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

package statusmapper

import (
	"fmt"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

// Mapper computes external conditions from internal state using declarative rules.
// Thread-safe for concurrent use (stateless evaluation).
type Mapper struct {
	specs []Spec
}

// NewMapper creates a mapper with the given condition specs.
// Specs are validated by unit tests, not at runtime.
func NewMapper(specs []Spec) *Mapper {
	return &Mapper{specs: specs}
}

// Map computes all external conditions for the given input.
func (m *Mapper) Map(input *Input) []status.Condition {
	results := make([]status.Condition, 0, len(m.specs))
	for _, spec := range m.specs {
		if cond := spec.Map(input); cond != nil {
			results = append(results, *cond)
		}
	}
	return results
}

// DetectDuplicateCases analyzes cases for logical duplicates.
// Returns a list of warnings for review. Use in unit tests.
func (m *Mapper) DetectDuplicateCases() []string {
	var warnings []string

	for _, spec := range m.specs {
		// Check for cases that can never fire (shadowed by earlier cases)
		for i := 0; i < len(spec.Rule); i++ {
			for j := i + 1; j < len(spec.Rule); j++ {
				// nil When (default case) shadows everything after it
				if spec.Rule[i].When == nil {
					warnings = append(warnings,
						fmt.Sprintf("%s: case %d is shadowed by case %d (default case)",
							spec.Type, j, i))
					continue
				}
				// Always{} shadows everything after it
				if _, ok := spec.Rule[i].When.(Always); ok {
					warnings = append(warnings,
						fmt.Sprintf("%s: case %d is shadowed by case %d (Always matcher)",
							spec.Type, j, i))
					continue
				}
				// Same matcher string = duplicate
				if spec.Rule[j].When != nil && spec.Rule[i].When.String() == spec.Rule[j].When.String() {
					warnings = append(warnings,
						fmt.Sprintf("%s: cases %d and %d have identical matchers",
							spec.Type, i, j))
				}
			}
		}
	}

	return warnings
}

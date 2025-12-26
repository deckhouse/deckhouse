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

package status

import (
	"fmt"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/status/types"
)

// ConditionMapper computes external conditions from internal state using declarative rules.
// Thread-safe for concurrent use (stateless evaluation).
type ConditionMapper struct {
	specs []types.MappingSpec
}

// NewConditionMapper creates a mapper with the given condition specs.
// Specs are validated by unit tests, not at runtime.
func NewConditionMapper(specs []types.MappingSpec) *ConditionMapper {
	return &ConditionMapper{specs: specs}
}

// Map computes all external conditions for the given input.
func (m *ConditionMapper) Map(input *types.MappingInput) []types.ExternalCondition {
	results := make([]types.ExternalCondition, 0, len(m.specs))
	for _, spec := range m.specs {
		if cond := spec.Map(input); cond != nil {
			results = append(results, *cond)
		}
	}
	return results
}

// DetectDuplicateMappingRules analyzes rules for logical duplicates.
// Returns a list of warnings for review. Use in unit tests.
func (m *ConditionMapper) DetectDuplicateMappingRules() []string {
	var warnings []string

	for _, spec := range m.specs {
		// Check for rules that can never fire (shadowed by earlier rules)
		for i := 0; i < len(spec.MappingRules); i++ {
			for j := i + 1; j < len(spec.MappingRules); j++ {
				// Always{} shadows everything after it
				if _, ok := spec.MappingRules[i].Matcher.(types.Always); ok {
					warnings = append(warnings,
						fmt.Sprintf("%s: rule '%s' is shadowed by '%s' (Always matcher)",
							spec.Type, spec.MappingRules[j].Name, spec.MappingRules[i].Name))
				}
				// Same matcher string = duplicate
				if spec.MappingRules[i].Matcher.String() == spec.MappingRules[j].Matcher.String() {
					warnings = append(warnings,
						fmt.Sprintf("%s: rules '%s' and '%s' have identical matchers",
							spec.Type, spec.MappingRules[i].Name, spec.MappingRules[j].Name))
				}
			}
		}
	}

	return warnings
}

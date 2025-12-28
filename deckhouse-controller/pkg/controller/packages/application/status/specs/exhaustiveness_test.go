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

package specs

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/statusmapper"
)

// TestEverySpecIsStructurallyValid ensures all specs have valid structure.
func TestEverySpecIsStructurallyValid(t *testing.T) {
	for _, spec := range DefaultSpecs() {
		t.Run(string(spec.Type), func(t *testing.T) {
			require.NotEmpty(t, spec.Type, "spec has empty Type")
			require.NotEmpty(t, spec.Rule, "spec has no cases")
		})
	}
}

// TestEverySpecHasFallbackRule ensures every spec has a default case (When == nil) as last case.
// This guarantees that Map() never returns nil for applicable conditions.
func TestEverySpecHasFallbackRule(t *testing.T) {
	for _, spec := range DefaultSpecs() {
		t.Run(string(spec.Type), func(t *testing.T) {
			require.NotEmpty(t, spec.Rule, "spec has no cases")

			lastCase := spec.Rule[len(spec.Rule)-1]
			isDefault := lastCase.When == nil
			_, isAlways := lastCase.When.(statusmapper.Always)

			assert.True(t, isDefault || isAlways,
				"last case must be default (When == nil) or Always{} for exhaustiveness")
		})
	}
}

// TestEveryExternalConditionHasSpec ensures every defined external condition type
// has a corresponding Spec in DefaultSpecs().
func TestEveryExternalConditionHasSpec(t *testing.T) {
	specs := DefaultSpecs()
	specTypes := make(map[status.ConditionName]bool)
	for _, s := range specs {
		specTypes[s.Type] = true
	}

	for _, condType := range status.AllExternalConditions {
		assert.True(t, specTypes[condType],
			"external condition %q has no Spec", condType)
	}

	// Verify no orphan specs (specs for unknown condition types)
	for _, s := range specs {
		assert.True(t, slices.Contains(status.AllExternalConditions, s.Type),
			"spec exists for unknown condition type %q (add it to AllExternalConditions)", s.Type)
	}
}

// TestEveryInternalConditionHasMapping ensures all internal conditions from operator
// are used in at least one Spec. Uses status.AllConditions as single source of truth.
func TestEveryInternalConditionHasMapping(t *testing.T) {
	usageMap := make(map[status.ConditionName]bool)
	for _, name := range status.AllConditions {
		usageMap[name] = false
	}

	// Scan all specs for condition references
	for _, spec := range DefaultSpecs() {
		for _, c := range spec.Rule {
			if c.When != nil {
				collectConditionUsage(c.When, usageMap)
			}
			if c.MessageFrom != "" {
				usageMap[c.MessageFrom] = true
			}
		}
	}

	for name, isUsed := range usageMap {
		if slices.Contains(status.UnusedConditions, name) {
			continue // Intentionally unused (reserved for future)
		}
		assert.True(t, isUsed,
			"operator condition %q has no mapping in specs (add to spec or status.UnusedConditions)", name)
	}
}

// collectConditionUsage recursively marks conditions as used.
func collectConditionUsage(m statusmapper.Matcher, usageMap map[status.ConditionName]bool) {
	switch v := m.(type) {
	case statusmapper.ConditionIs:
		usageMap[v.Name] = true
	case statusmapper.ConditionNotTrue:
		usageMap[v.Name] = true
	case statusmapper.AllOf:
		for _, sub := range v {
			collectConditionUsage(sub, usageMap)
		}
	case statusmapper.AnyOf:
		for _, sub := range v {
			collectConditionUsage(sub, usageMap)
		}
	}
}

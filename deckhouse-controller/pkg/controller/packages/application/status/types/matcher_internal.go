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

package types

import (
	"slices"

	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

// InternalIs matches when an internal condition has a specific status.
type InternalIs struct {
	Name   InternalConditionName
	Status corev1.ConditionStatus
}

func (i InternalIs) Match(input *MappingInput) bool {
	cond, exists := input.InternalConditions[string(i.Name)]
	if !exists {
		return i.Status == corev1.ConditionUnknown
	}
	return cond.Status == i.Status
}

func (i InternalIs) String() string {
	return string(i.Name) + "=" + string(i.Status)
}

// InternalTrue is shorthand for InternalIs{Name, True}.
func InternalTrue(name InternalConditionName) InternalIs {
	return InternalIs{Name: name, Status: corev1.ConditionTrue}
}

// InternalFalse is shorthand for InternalIs{Name, False}.
func InternalFalse(name InternalConditionName) InternalIs {
	return InternalIs{Name: name, Status: corev1.ConditionFalse}
}

// InternalNotTrue matches when internal condition is NOT True (False, Unknown, or missing).
type InternalNotTrue struct {
	Name InternalConditionName
}

func (i InternalNotTrue) Match(input *MappingInput) bool {
	cond, exists := input.InternalConditions[string(i.Name)]
	if !exists {
		return true // missing = not true
	}
	return cond.Status != corev1.ConditionTrue
}

func (i InternalNotTrue) String() string {
	return string(i.Name) + "!=True"
}

// NotTrue is shorthand constructor for InternalNotTrue.
func NotTrue(name InternalConditionName) InternalNotTrue {
	return InternalNotTrue{Name: name}
}

// =============================================================================
// Convenience Constructors
// =============================================================================

// AllInternalsTrue matches when all specified internal conditions are True.
func AllInternalsTrue(names ...InternalConditionName) AllOf {
	matchers := make([]Matcher, len(names))
	for i, name := range names {
		matchers[i] = InternalTrue(name)
	}
	return AllOf(matchers)
}

// AllInstallationConditionsMet matches when all installation-related conditions are True.
// Dynamically computed from status.AllConditions, excluding:
//   - status.UnusedConditions (reserved for future)
//   - SettingsIsValid (doesn't block installation)
func AllInstallationConditionsMet() Matcher {
	conditions := make([]InternalConditionName, 0, len(status.AllConditions))
	for _, c := range status.AllConditions {
		if slices.Contains(status.UnusedConditions, c) {
			continue
		}
		if c == status.ConditionSettingsIsValid {
			continue
		}
		conditions = append(conditions, c)
	}
	return AllInternalsTrue(conditions...)
}

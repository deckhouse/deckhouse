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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

// ConditionIs matches when an internal condition has a specific status.
type ConditionIs struct {
	Name   status.ConditionName
	Status metav1.ConditionStatus
}

func (c ConditionIs) Match(input *Input) bool {
	cond, exists := input.InternalConditions[c.Name]
	if !exists {
		return c.Status == metav1.ConditionUnknown
	}
	return cond.Status == c.Status
}

func (c ConditionIs) String() string {
	return string(c.Name) + "=" + string(c.Status)
}

// ConditionTrue is shorthand for ConditionIs{Name, True}.
func ConditionTrue(name status.ConditionName) ConditionIs {
	return ConditionIs{Name: name, Status: metav1.ConditionTrue}
}

// ConditionFalse is shorthand for ConditionIs{Name, False}.
func ConditionFalse(name status.ConditionName) ConditionIs {
	return ConditionIs{Name: name, Status: metav1.ConditionFalse}
}

// =============================================================================
// DSL Constructors for readable condition matching
// =============================================================================

// True checks if condition has True status.
func True(name status.ConditionName) ConditionIs { return ConditionTrue(name) }

// False checks if condition has False status.
func False(name status.ConditionName) ConditionIs { return ConditionFalse(name) }

// IsTrue is an alias for True - for readability in Case.When expressions.
func IsTrue(name status.ConditionName) ConditionIs { return True(name) }

// IsFalse is an alias for False - for readability in Case.When expressions.
func IsFalse(name status.ConditionName) ConditionIs { return False(name) }

// ConditionNotTrue matches when condition is NOT True (False, Unknown, or missing).
type ConditionNotTrue struct {
	Name status.ConditionName
}

func (c ConditionNotTrue) Match(input *Input) bool {
	cond, exists := input.InternalConditions[c.Name]
	if !exists {
		return true // missing = not true
	}
	return cond.Status != metav1.ConditionTrue
}

func (c ConditionNotTrue) String() string {
	return string(c.Name) + "!=True"
}

// NotTrue is shorthand constructor for ConditionNotTrue.
func NotTrue(name status.ConditionName) ConditionNotTrue {
	return ConditionNotTrue{Name: name}
}

// =============================================================================
// Convenience Constructors
// =============================================================================

// AllConditionsTrue matches when all specified conditions are True.
func AllConditionsTrue(names ...status.ConditionName) AllOf {
	matchers := make([]Matcher, len(names))
	for i, name := range names {
		matchers[i] = ConditionTrue(name)
	}
	return AllOf(matchers)
}

// AllTrue is an alias for AllConditionsTrue - matches when all conditions are True.
func AllTrue(names ...status.ConditionName) AllOf { return AllConditionsTrue(names...) }

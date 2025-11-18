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

package condition

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker"
)

// Condition is a function that evaluates a boolean condition.
// Examples:
//   - Bootstrap ready check
//   - Leader election status
//   - External dependency availability
//   - Feature flag evaluation
type Condition func() bool

// Checker wraps a condition function as a checker.Checker implementation.
type Checker struct {
	condition Condition // Function to evaluate
}

// NewChecker creates a condition checker.
func NewChecker(condition Condition) *Checker {
	ch := new(Checker)

	ch.condition = condition

	return ch
}

// Check evaluates the condition function and returns the result.
// Reason is always empty - condition functions don't provide reasons.
func (c *Checker) Check() checker.Result {
	return checker.Result{
		Enabled: c.condition(),
		Reason:  "", // Conditions don't provide reasons
	}
}

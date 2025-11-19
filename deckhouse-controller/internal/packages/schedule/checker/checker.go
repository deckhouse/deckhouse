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

package checker

// Checker evaluates a condition and returns whether a package should be enabled.
//
// Examples of checkers:
//   - Version checker: Validates Kubernetes/Deckhouse version against constraints
//   - Condition checker: Evaluates boolean conditions (e.g., bootstrap ready)
//   - Custom checkers: Any domain-specific requirements
type Checker interface {
	// Check evaluates the checker's condition and returns the result.
	// Called frequently during scheduler operations.
	Check() Result
}

// Result represents the outcome of a checker evaluation.
type Result struct {
	Enabled bool   // Whether the package should be enabled based on this check
	Reason  string // Human-readable reason (typically set when Enabled=false)
}

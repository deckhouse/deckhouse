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

package condmap

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Map computes at most one external condition from a mapping state.
// Return an empty condition to leave that external condition unchanged.
type Map func(state State) metav1.Condition

// Mapper applies condition maps to compute external conditions.
type Mapper struct {
	// Maps is the ordered list of condition maps. Order matters when callers
	// care about deterministic condition update ordering.
	Maps []Map
}

// Map evaluates all condition maps and returns non-empty external conditions.
func (m Mapper) Map(state State) []metav1.Condition {
	result := make([]metav1.Condition, 0, len(m.Maps))

	for _, mapper := range m.Maps {
		condition := mapper(state)
		if condition.Type == "" {
			continue
		}

		result = append(result, condition)
	}

	return result
}

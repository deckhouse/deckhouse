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
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReasonDeleting is the reason every external condition carries while its
// package is being removed. The runtime keeps its own copy for the internal
// conditions (packages/status.ConditionReasonDeleting); the two are the same
// word by intent, not by reference — internal reasons are never exported as-is.
const ReasonDeleting = "Deleting"

// Map computes at most one external condition from a mapping state. Type names
// the condition Fn produces, so a mapper knows its own vocabulary without
// evaluating anything; Fn returns an empty condition to leave it unchanged.
type Map struct {
	Type string
	Fn   func(state State) metav1.Condition
}

// Mapper applies condition maps to compute external conditions.
type Mapper struct {
	maps     []Map
	deleting []metav1.Condition
}

// NewMapper builds a mapper from an ordered list of condition maps. Order
// matters when callers care about deterministic condition update ordering.
func NewMapper(maps ...Map) Mapper {
	deleting := make([]metav1.Condition, 0, len(maps))
	for _, m := range maps {
		deleting = append(deleting, metav1.Condition{
			Type:   m.Type,
			Status: metav1.ConditionFalse,
			Reason: ReasonDeleting,
		})
	}

	return Mapper{maps: maps, deleting: deleting}
}

// Map evaluates all condition maps and returns non-empty external conditions.
//
// While the state reports a deletion it returns every condition the mapper can
// produce as False/ReasonDeleting instead. The maps read a state that still
// describes the last reconcile, and one that emits nothing would leave its
// condition — the sticky Installed above all — claiming the package is still
// there.
func (m Mapper) Map(state State) []metav1.Condition {
	if state.IsDeleting() {
		return slices.Clone(m.deleting)
	}

	result := make([]metav1.Condition, 0, len(m.maps))

	for _, mp := range m.maps {
		condition := mp.Fn(state)
		if condition.Type == "" {
			continue
		}

		result = append(result, condition)
	}

	return result
}

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
//
// Gate, when set, names the condition this one is reported behind: while the
// gate is not True once the run is applied, the condition is removed from the
// resource whatever Fn returns.
type Map struct {
	Type string
	Fn   func(state State) metav1.Condition
	Gate string
}

// Result is what one mapping run changes on the resource.
type Result struct {
	// Set holds the conditions to set, in evaluation order.
	Set []metav1.Condition
	// Remove holds the gated condition types whose gate is not True.
	Remove []string
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

// Map evaluates all condition maps and returns the non-empty external
// conditions to set, and the gated conditions to remove.
//
// While the state reports a deletion it returns every condition the mapper can
// produce as False/ReasonDeleting instead, gated or not. The maps read a state
// that still describes the last reconcile, and one that emits nothing would
// leave its condition — the sticky Installed above all — claiming the package
// is still there.
func (m Mapper) Map(state State) Result {
	if state.IsDeleting() {
		return Result{Set: slices.Clone(m.deleting)}
	}

	// Every map is evaluated before any gate is read, so a gate mapped in this
	// run counts whatever the declaration order.
	mapped := make([]metav1.Condition, 0, len(m.maps))
	for _, mp := range m.maps {
		mapped = append(mapped, mp.Fn(state))
	}

	result := Result{Set: make([]metav1.Condition, 0, len(m.maps))}

	for i, mp := range m.maps {
		if mp.Gate != "" && !isGateOpen(state, mapped, mp.Gate) {
			result.Remove = append(result.Remove, mp.Type)
			continue
		}

		if mapped[i].Type == "" {
			continue
		}

		result.Set = append(result.Set, mapped[i])
	}

	return result
}

// isGateOpen reports whether the gate condition is True once the run is
// applied: the value mapped in this run wins, the one on the resource stands
// otherwise.
func isGateOpen(state State, mapped []metav1.Condition, gate string) bool {
	for _, cond := range mapped {
		if cond.Type == gate {
			return cond.Status == metav1.ConditionTrue
		}
	}

	return state.ExtEqual(gate, metav1.ConditionTrue)
}

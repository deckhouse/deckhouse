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
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	gateType  = "Gate"
	gatedType = "Gated"
)

// fixed returns a map function that always produces cond.
func fixed(cond metav1.Condition) func(State) metav1.Condition {
	return func(State) metav1.Condition { return cond }
}

func TestMapperGate(t *testing.T) {
	gated := metav1.Condition{Type: gatedType, Status: metav1.ConditionTrue, Reason: gatedType}

	// The gated map is declared first: the gate mapped in the same run must count
	// whatever the declaration order.
	mapper := func(gate metav1.Condition, gatedFn func(State) metav1.Condition) Mapper {
		return NewMapper(
			Map{Type: gatedType, Fn: gatedFn, Gate: gateType},
			Map{Type: gateType, Fn: fixed(gate)},
		)
	}

	withGate := func(status metav1.ConditionStatus) State {
		return State{External: map[string]metav1.Condition{gateType: {Type: gateType, Status: status}}}
	}

	cases := []struct {
		name       string
		gate       metav1.Condition
		gatedFn    func(State) metav1.Condition
		state      State
		wantSet    []string
		wantRemove []string
	}{
		{
			name:    "gate mapped True in this run opens",
			gate:    metav1.Condition{Type: gateType, Status: metav1.ConditionTrue},
			gatedFn: fixed(gated),
			wantSet: []string{gatedType, gateType},
		},
		{
			name:       "gate mapped False in this run wins over the resource",
			gate:       metav1.Condition{Type: gateType, Status: metav1.ConditionFalse},
			gatedFn:    fixed(gated),
			state:      withGate(metav1.ConditionTrue),
			wantSet:    []string{gateType},
			wantRemove: []string{gatedType},
		},
		{
			name:    "gate left as is stays open when True on the resource",
			gatedFn: fixed(gated),
			state:   withGate(metav1.ConditionTrue),
			wantSet: []string{gatedType},
		},
		{
			name:       "gate absent keeps the gated condition off the resource",
			gatedFn:    fixed(gated),
			wantRemove: []string{gatedType},
		},
		{
			// An open gate does not turn "leave unchanged" into a removal.
			name:    "open gate with nothing mapped leaves the gated condition alone",
			gatedFn: fixed(metav1.Condition{}),
			state:   withGate(metav1.ConditionTrue),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := mapper(tc.gate, tc.gatedFn).Map(tc.state)

			set := make([]string, 0, len(result.Set))
			for _, cond := range result.Set {
				set = append(set, cond.Type)
			}

			assert.ElementsMatch(t, tc.wantSet, set, "set")
			assert.ElementsMatch(t, tc.wantRemove, result.Remove, "remove")
		})
	}
}

func TestMapperDeletingBypassesGate(t *testing.T) {
	mapper := NewMapper(
		Map{Type: gateType, Fn: fixed(metav1.Condition{Type: gateType, Status: metav1.ConditionFalse})},
		Map{Type: gatedType, Fn: fixed(metav1.Condition{Type: gatedType, Status: metav1.ConditionTrue}), Gate: gateType},
	)

	result := mapper.Map(State{Deleting: true})

	assert.Empty(t, result.Remove)
	assert.Equal(t, []metav1.Condition{
		{Type: gateType, Status: metav1.ConditionFalse, Reason: ReasonDeleting},
		{Type: gatedType, Status: metav1.ConditionFalse, Reason: ReasonDeleting},
	}, result.Set)
}

// Copyright 2024 Flant JSC
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

package fsm_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/dhctl/pkg/server/fsm"
)

func TestFiniteStateMachine(t *testing.T) {
	t.Parallel()

	type event struct {
		event       fsm.Event
		stateBefore fsm.State
		stateAfter  fsm.State
		errContains string
	}

	tests := map[string]struct {
		initialState fsm.State
		transitions  []fsm.Transition

		events []event
	}{
		"door": {
			initialState: "closed",
			transitions: []fsm.Transition{
				{
					Event:       "open",
					Sources:     []fsm.State{"closed"},
					Destination: "opened",
					Callback: func(source, destination fsm.State) error {
						assert.Equal(t, "closed", source)
						assert.Equal(t, "opened", destination)
						return nil
					},
				},
				{
					Event:       "close",
					Sources:     []fsm.State{"opened"},
					Destination: "closed",
					Callback:    nil,
				},
				{
					Event:       "open-slightly",
					Sources:     []fsm.State{"opened", "closed"},
					Destination: "slightly-opened",
					Callback: func(source, destination fsm.State) error {
						return fmt.Errorf("oh no")
					},
				},
			},
			events: []event{
				{
					event:       "open",
					stateBefore: "closed",
					stateAfter:  "opened",
					errContains: "",
				},
				{
					event:       "open",
					stateBefore: "opened",
					stateAfter:  "opened",
					errContains: "transition error: event \"open\" inappropriate in current state \"opened\"",
				},
				{
					event:       "close",
					stateBefore: "opened",
					stateAfter:  "closed",
					errContains: "",
				},
				{
					event:       "kick",
					stateBefore: "closed",
					stateAfter:  "closed",
					errContains: "transition error: unknown event \"kick\"",
				},
				{
					event:       "open",
					stateBefore: "closed",
					stateAfter:  "opened",
					errContains: "",
				},
				{
					event:       "open-slightly",
					stateBefore: "opened",
					stateAfter:  "opened",
					errContains: "callback error: oh no",
				},
			},
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := fsm.New(tt.initialState, tt.transitions)
			for _, e := range tt.events {
				assert.Equal(t, e.stateBefore, f.State())
				err := f.Event(e.event)
				assert.Equal(t, e.stateAfter, f.State())

				if e.errContains == "" {
					assert.NoError(t, err)
				} else {
					assert.ErrorContains(t, err, e.errContains)
				}
			}
		})
	}
}

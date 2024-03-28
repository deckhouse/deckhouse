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

package fsm

import (
	"errors"
	"fmt"
	"sync"
)

var (
	ErrTransitionError = errors.New("transition error")
	ErrCallbackError   = errors.New("callback error")
)

type Event = string

type State = string

type Callback func(source, destination State) error

type Transition struct {
	Event       Event
	Sources     []State
	Destination State
	Callback    Callback
}

type state struct {
	event  Event
	source State
}

type transition struct {
	destination State
	callback    Callback
}

type FiniteStateMachine struct {
	state       State
	events      map[Event]struct{}
	transitions map[state]transition

	m *sync.RWMutex
}

func New(initialState State, transitions []Transition) *FiniteStateMachine {
	fsm := &FiniteStateMachine{
		state:       initialState,
		events:      map[Event]struct{}{},
		transitions: map[state]transition{},
		m:           &sync.RWMutex{},
	}

	for _, tr := range transitions {
		for _, src := range tr.Sources {
			fsm.events[tr.Event] = struct{}{}
			fsm.transitions[state{
				event:  tr.Event,
				source: src,
			}] = transition{
				destination: tr.Destination,
				callback:    tr.Callback,
			}
		}
	}

	return fsm
}

func (f *FiniteStateMachine) State() State {
	f.m.RLock()
	defer f.m.RUnlock()

	return f.state
}

func (f *FiniteStateMachine) Event(event Event) error {
	f.m.Lock()
	defer f.m.Unlock()

	if _, ok := f.events[event]; !ok {
		return fmt.Errorf("%w: unknown event %q", ErrTransitionError, event)
	}

	tr, ok := f.transitions[state{
		event:  event,
		source: f.state,
	}]
	if !ok {
		return fmt.Errorf("%w: event %q inappropriate in current state %q", ErrTransitionError, event, f.state)
	}

	if tr.callback != nil {
		err := tr.callback(f.state, tr.destination)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrCallbackError, err)
		}
	}

	f.state = tr.destination

	return nil
}

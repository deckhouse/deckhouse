/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scheduler

import (
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/snapshot"
)

func Test_stsSelectorByPod_Select(t *testing.T) {
	const (
		// for indexing convenience
		a = iota
		b
		c
		d
		e
	)

	tests := []struct {
		name   string
		input  func() (State, []snapshot.Pod, bool)
		assert func(*testing.T, string, error)
	}{
		{
			name: "filled state but no pods, disruption forbidden; selects any to deploy",
			input: func() (State, []snapshot.Pod, bool) {
				return fakeState(), nil, false
			},
			assert: assertAny,
		},
		{
			name: "filled state but no pods, disruption allowed; selects any to deploy",
			input: func() (State, []snapshot.Pod, bool) {
				return fakeState(), nil, true
			},
			assert: assertAny,
		},
		{
			name: "pods and state are fine, disruption allowed; no selection, because no pods problem",
			input: func() (State, []snapshot.Pod, bool) {
				state := fakeState()
				pods := fakePods(5)
				return state, pods, true
			},
			assert: assertNone,
		},
		{
			name: "pods created more than 4 min ago is selected",
			input: func() (State, []snapshot.Pod, bool) {
				state := fakeState()
				pods := fakePods(5)
				pods[d].Created = time.Now().Add(-4*time.Minute - time.Millisecond)
				return state, pods, true
			},
			assert: assertOk("d"),
		},
		{
			name: "one lacking pod matches the selection",
			input: func() (State, []snapshot.Pod, bool) {
				state := fakeState()
				pods := []snapshot.Pod{
					fakePod(a), fakePod(b),
					fakePod(d), fakePod(e),
				}
				return state, pods, false // no c (2) node
			},
			assert: assertOk("c"),
		},
		{
			name: "one lacking pod matches the selection",
			input: func() (State, []snapshot.Pod, bool) {
				state := fakeState()
				pods := []snapshot.Pod{
					fakePod(a), fakePod(c),
					fakePod(d), fakePod(e),
				}
				return state, pods, false // no b (1) node
			},
			assert: assertOk("b"),
		},
		{
			name: "forbidden disruption aborts selection",
			input: func() (State, []snapshot.Pod, bool) {
				state := fakeState()
				pods := fakePods(5)
				return state, pods, false
			},
			assert: assertAbortion,
		},
		{
			name: "forbidden disruption precedes outdated pods decision",
			input: func() (State, []snapshot.Pod, bool) {
				state := fakeState()
				pods := fakePods(5)
				pods[c].Ready = false
				pods[d].Created = time.Now().Add(-4*time.Minute - time.Millisecond)
				return state, pods, false
			},
			assert: assertAbortion,
		},
		{
			name: "pending for more than 1 minutes precedes disruption control",
			input: func() (State, []snapshot.Pod, bool) {
				state := fakeState()
				pods := fakePods(5)
				pods[c].Ready = false
				pods[c].Created = time.Now().Add(-time.Minute - time.Millisecond)
				return state, pods, false
			},
			assert: assertOk("c"),
		},
		{
			name: "when all fine, oldest pod is prioritized",
			input: func() (State, []snapshot.Pod, bool) {
				state := fakeState()
				pods := fakePods(5)
				pods[b].Created = time.Now().Add(-5 * time.Minute)
				pods[c].Created = time.Now().Add(-7 * time.Minute)
				pods[d].Created = time.Now().Add(-6 * time.Minute)
				return state, pods, true
			},
			assert: assertOk("c"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, pods, disruptionAllowed := tt.input()
			s := &selectByPod{
				pods:              pods,
				disruptionAllowed: disruptionAllowed,
			}

			x, err := s.Select(state)

			tt.assert(t, x, err)
		})
	}
}

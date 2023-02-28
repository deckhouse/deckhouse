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

	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/snapshot"
)

func Test_stsSelectorByNode_Select(t *testing.T) {
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
		input  func() (State, []snapshot.Node)
		assert func(*testing.T, string, error)
	}{
		{
			name: "empty state and nodes, selects any to move from not existing node",
			input: func() (State, []snapshot.Node) {
				return newState(), nil
			},
			assert: assertAny,
		},
		{
			name: "filled state but no nodes, selects any to move from not existing node",
			input: func() (State, []snapshot.Node) {
				return fakeState(), nil
			},
			assert: assertAny,
		},
		{
			name: "nodes and state are fine, no selection, because no node problem",
			input: func() (State, []snapshot.Node) {
				state := fakeState()
				nodes := fakeNodes(5)
				return state, nodes
			},
			assert: assertNone,
		},
		{
			name: "one lacking node matches the selection",
			input: func() (State, []snapshot.Node) {
				state := fakeState()
				nodes := []snapshot.Node{
					fakeNode(a), fakeNode(b),
					fakeNode(d), fakeNode(e),
				}
				return state, nodes // no c (2) node
			},
			assert: assertOk("c"),
		},
		{
			name: "one lacking node matches the selection",
			input: func() (State, []snapshot.Node) {
				state := fakeState()
				nodes := []snapshot.Node{
					fakeNode(a), fakeNode(c),
					fakeNode(d), fakeNode(e),
				}
				return state, nodes // no b (1) node
			},
			assert: assertOk("b"),
		},
		{
			name: "one unavailable node matches the selection",
			input: func() (State, []snapshot.Node) {
				state := fakeState()
				nodes := fakeNodes(5)
				nodes[d].Schedulable = false
				return state, nodes
			},
			assert: assertOk("d"),
		},
		{
			name: "one unscheduled StatefulSet matches the selection",
			input: func() (State, []snapshot.Node) {
				state := fakeState()
				nodes := fakeNodes(5)
				state["b"].Node = ""
				return state, nodes
			},
			assert: assertOk("b"),
		},
		{
			name: "absent node is more important than unschedulable one",
			input: func() (State, []snapshot.Node) {
				state := fakeState()
				nodes := fakeNodes(3)
				state["b"].Node = ""
				nodes[a].Schedulable = false
				return state, nodes
			},
			assert: assertOk("b"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, nodes := tt.input()
			s := &selectByNode{
				nodes: nodes,
			}

			x, err := s.Select(state)

			tt.assert(t, x, err)
		})
	}
}

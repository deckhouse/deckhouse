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

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/snapshot"
)

func Test_FilterByMinSts_Filter(t *testing.T) {
	// The filter by sts has nothing to do with zones. It works with nodes it gets, and makes decisions
	// based on sts counts.
	type args struct {
		nodes []snapshot.Node
		x     string
	}

	tests := []struct {
		name  string
		state func() State
		args  args
		want  []snapshot.Node
	}{
		{
			name:  "no input nodes results no output nodes",
			state: fakeState,
			args:  args{nodes: []snapshot.Node{}, x: "c"},
			want:  []snapshot.Node{},
		},
		{
			name: "one node for all leaves the node as is",
			state: func() State {
				state := fakeState()
				node := fakeNode(1)
				for _, sts := range state {
					sts.Node = node.Name
					sts.Zone = node.Zone
				}
				return state
			},
			args: args{nodes: []snapshot.Node{fakeNode(1)}, x: "c"},
			want: []snapshot.Node{fakeNode(1)},
		},
		{
			name: "when all 2 nodes occupied, current sts node is omitted",
			state: func() State {
				state := fakeState()
				indexes := []string{"a", "b", "c", "d", "e"}
				for i, x := range indexes {
					node := fakeNode(i%2 + 1)
					state[x].Node = node.Name
					state[x].Zone = node.Zone
				}
				return state
			},
			args: args{
				x: "c",
				nodes: []snapshot.Node{
					fakeNode(1), // a, c, e
					fakeNode(2), // b, d
				},
			},
			// exclude node with c
			want: []snapshot.Node{fakeNode(2)},
		},
		{
			name: "when all 5 nodes occupied, current sts node is omitted",
			state: func() State {
				state := fakeState()
				indexes := []string{"a", "b", "c", "d", "e"}
				for i, x := range indexes {
					node := fakeNode(i + 1)
					state[x].Node = node.Name
					state[x].Zone = node.Zone
				}
				return state
			},
			args: args{
				x: "c",
				nodes: []snapshot.Node{
					fakeNode(1), // a
					fakeNode(2), // b
					fakeNode(3), // c
					fakeNode(4), // d
					fakeNode(5), // e
				},
			},
			want: []snapshot.Node{
				// exclude c
				fakeNode(1), // a
				fakeNode(2), // b
				fakeNode(4), // d
				fakeNode(5), // e
			},
		},
		{
			name: "when all 4/5 nodes occupied, only free node is returned",
			state: func() State {
				state := fakeState()
				indexes := []string{"a", "b", "c", "d", "e"}
				for i, x := range indexes {
					node := fakeNode(i + 1)
					state[x].Node = node.Name
					state[x].Zone = node.Zone
				}
				state["c"] = state["b"]
				return state
			},
			args: args{
				x: "c",
				nodes: []snapshot.Node{
					fakeNode(1), // a
					fakeNode(2), // b, c
					fakeNode(3), // —
					fakeNode(4), // d
					fakeNode(5), // e
				},
			},
			want: []snapshot.Node{fakeNode(3)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &filterByMinSts{state: tt.state()}

			got := f.Filter(tt.args.nodes, tt.args.x)

			assert.Equal(t, tt.want, got)
		})
	}
}

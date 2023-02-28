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
	"reflect"
	"testing"

	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/snapshot"
)

func Test_spread(t *testing.T) {
	type args struct {
		total  int
		counts []int
	}
	tests := []struct {
		name string
		args args
		want []int
	}{
		{
			name: "empty",
			args: args{total: 0, counts: []int{}},
			want: []int{},
		},
		{
			name: "one",
			args: args{total: 1, counts: []int{1}},
			want: []int{1},
		},
		{
			name: "ones",
			args: args{total: 2, counts: []int{1, 1}},
			want: []int{1, 1},
		},
		{
			name: "uniform twos",
			args: args{total: 2, counts: []int{2, 2}},
			want: []int{1, 1},
		},
		{
			name: "uniform 49s",
			args: args{total: 2, counts: []int{49, 49}},
			want: []int{1, 1},
		},
		{
			name: "total size of equal numbers",
			args: args{total: 5, counts: []int{49, 49, 49, 49, 49}},
			want: []int{1, 1, 1, 1, 1},
		},
		{
			name: "total size of different numbers",
			args: args{total: 5, counts: []int{49, 89, 5, 106, 5}},
			want: []int{1, 1, 1, 1, 1},
		},
		{
			name: "total size of equal numbers and total = size ×5",
			args: args{total: 10, counts: []int{49, 49}},
			want: []int{5, 5},
		},
		{
			name: "total size of equal numbers and total = size ×2",
			args: args{total: 10, counts: []int{49, 49, 49, 49, 49}},
			want: []int{2, 2, 2, 2, 2},
		},
		{
			name: "different numbers with second layer",
			args: args{total: 5, counts: []int{39, 20, 28}},
			want: []int{2, 1, 2},
		},
		{
			name: "different numbers with third layer",
			args: args{total: 5, counts: []int{8, 100, 7}},
			want: []int{2, 2, 1},
		},
		{
			name: "different numbers with third layer",
			args: args{total: 5, counts: []int{13, 100, 7}},
			want: []int{2, 2, 1},
		},
		{
			name: "different numbers with third layer",
			args: args{total: 5, counts: []int{14, 100, 7}},
			want: []int{2, 2, 1},
		},
		{
			name: "different numbers with third layer",
			args: args{total: 5, counts: []int{199, 100, 7}},
			want: []int{2, 2, 1},
		},
		{
			name: "total size of different numbers and total = size ×2",
			args: args{total: 10, counts: []int{49, 89, 5, 106, 5}},
			want: []int{2, 3, 1, 3, 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := spread(tt.args.total, tt.args.counts); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("spread(%d, %v) = %v, want %v", tt.args.total, tt.args.counts, got, tt.want)
			}
		})
	}

	{
		b1 := []int{49, 89, 5, 106, 5}
		b2 := []int{49, 89, 5, 106, 5}

		spread(10, b1)

		if !reflect.DeepEqual(b1, b2) {
			t.Errorf("spread should not modify input")
		}
	}
}

func stateByZones(xs ...string) State {
	state := newState()
	indexes := []string{"a", "b", "c", "d", "e"}
	for i, zone := range xs {
		x := indexes[i]
		state[x] = &XState{
			Zone: zone,
			// the state must be filled to considered "deployed"
			Image:        "some",
			Node:         "no matter",
			StorageClass: "some",
		}
	}
	return state
}

func Test_filterByZone_selectZone(t *testing.T) {
	type fields struct {
		state State
	}
	type args struct {
		nodes []snapshot.Node
		x     string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		// Note: sts count is always 5

		{
			name:   "1Z 1N 0Sts, zone preserved",
			fields: fields{newState()},
			args:   args{nodes: []snapshot.Node{{Zone: "A"}}, x: "c"},
			want:   "A",
		},
		{
			name:   "1Z 1N 1Sts, zone preserved",
			fields: fields{stateByZones("A")},
			args:   args{nodes: []snapshot.Node{{Zone: "A"}}, x: "c"},
			want:   "A",
		},
		// Two zones have equal number of sts
		{
			name:   "2Z 2N sts in different zones: zone does not change (A)",
			fields: fields{stateByZones("A", "A", "A", "B", "B")},
			args:   args{nodes: []snapshot.Node{{Zone: "A"}, {Zone: "B"}}, x: "a"},
			want:   "A",
		},
		{
			name:   "2Z 2N sts in different zones: zone does not change (B)",
			fields: fields{stateByZones("A", "A", "A", "B", "B")},
			args:   args{nodes: []snapshot.Node{{Zone: "A"}, {Zone: "B"}}, x: "d"},
			want:   "B",
		},
		// Two zones, A is filled, B is empty, sts should migrate to starving zone
		{
			name:   "2Z 2N sts in one zone: zone changes (sts a -> zone B)",
			fields: fields{stateByZones("A", "A", "A", "A", "A")},
			args:   args{nodes: []snapshot.Node{{Zone: "A"}, {Zone: "B"}}, x: "a"},
			want:   "B",
		},
		{
			name:   "2Z 2N sts in one zones: zone changes (sts c -> zone B)",
			fields: fields{stateByZones("A", "A", "A", "A", "A")},
			args:   args{nodes: []snapshot.Node{{Zone: "A"}, {Zone: "B"}}, x: "c"},
			want:   "B",
		},
		{
			name:   "2Z 2N sts in one zones: zone changes (sts c -> zone B)",
			fields: fields{stateByZones("A", "A", "A", "A", "A")},
			args:   args{nodes: []snapshot.Node{{Zone: "A"}, {Zone: "B"}}, x: "c"},
			want:   "B",
		},
		// Case when two zones are equal, but the number of sts is odd. Sts should not migrate
		{
			name: "2Z 2N sts in zones 2+3: zone is prioritized in alphabetical order (a)",
			// distributed as A=2,B=3; demands A=1,B=-1, sts 'a' stays at zone A
			fields: fields{stateByZones("A", "A", "B", "B", "B")},
			args:   args{nodes: []snapshot.Node{{Zone: "A"}, {Zone: "B"}}, x: "a"},
			want:   "A",
		},
		{
			name: "2Z 2N sts in zones 2+3: zone is prioritized in alphabetical order (a)",
			// distributed as A=A,B=3; demands A=1,B=-1, sts 'd' wants to zone A
			fields: fields{stateByZones("A", "A", "B", "B", "B")},
			args:   args{nodes: []snapshot.Node{{Zone: "A"}, {Zone: "B"}}, x: "d"},
			want:   "A",
		},
		{
			name: "2Z 2N sts in zones 3+2: zone is prioritized in alphabetical order (a)",
			// distributed as A=3,B=2; demands A=0, B=0, sts 'a' stays at zone A
			fields: fields{stateByZones("A", "A", "A", "B", "B")},
			args:   args{nodes: []snapshot.Node{{Zone: "A"}, {Zone: "B"}}, x: "a"},
			want:   "A",
		},
		{
			name: "2Z 2N sts in zones 3+2: zone is prioritized in alphabetical order (d)",
			// distributed as A=3,B=2; demands A=0, B=0, sts 'd' stays at zone B
			fields: fields{stateByZones("A", "A", "A", "B", "B")},
			args:   args{nodes: []snapshot.Node{{Zone: "A"}, {Zone: "B"}}, x: "d"},
			want:   "B",
		},
		{
			name: "3Z 2N sts in zones 2+2+1: zone is prioritized in alphabetical order (d)",
			// distributed as A=2,B=2,C=1; demands A=1, B=0, C=0,
			// sts 'd' stays at zone B because it does not need to leave
			fields: fields{stateByZones("A", "A", "B", "B", "C")},
			args:   args{nodes: []snapshot.Node{{Zone: "A"}, {Zone: "B"}}, x: "d"},
			want:   "B",
		},
		{
			name: "3Z 2N sts in zones 2+2+1: zone is prioritized in alphabetical order (e)",
			// distributed as A=2,B=2,C=1; demands A=1, B=0, C=0,
			// sts 'e' moves to A because C is not present within nodes
			fields: fields{stateByZones("A", "A", "B", "B", "C")},
			args:   args{nodes: []snapshot.Node{{Zone: "A"}, {Zone: "B"}}, x: "e"},
			want:   "A",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &filterByZone{
				state: tt.fields.state,
			}
			if got := f.selectZone(tt.args.nodes, tt.args.x); got != tt.want {
				t.Errorf("filterByZone.selectZone() = %v, want %v", got, tt.want)
			}
		})
	}
}

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

func Test_nodeStateFilterPipe_Filter(t *testing.T) {
	type args struct {
		nodes []snapshot.Node
		x     string
	}

	byZone := &filterBy{func(node snapshot.Node) bool {
		return node.Zone == "z"
	}}

	bySchedulable := &filterBy{func(node snapshot.Node) bool {
		return node.Schedulable
	}}

	tests := []struct {
		name string
		pipe NodeFilterPipe
		args args
		want []snapshot.Node
	}{
		{
			name: "no nodes return no nodes",
			pipe: NodeFilterPipe{byZone, bySchedulable},
			args: args{nodes: []snapshot.Node{}, x: ""},
			want: []snapshot.Node{},
		},
		{
			name: "all filters apply to the input",
			pipe: NodeFilterPipe{byZone, bySchedulable},
			args: args{nodes: []snapshot.Node{
				{Zone: "_", Schedulable: true},
				{Zone: "z", Schedulable: true},
				{Zone: "z", Schedulable: false},
			}},
			want: []snapshot.Node{{Zone: "z", Schedulable: true}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pipe.Filter(tt.args.nodes, tt.args.x)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NodeFilterPipe.Filter() = %v, want %v", got, tt.want)
			}
		})
	}
}

type filterBy struct {
	filter func(snapshot.Node) bool
}

func (f *filterBy) Filter(nodes []snapshot.Node, _ string) []snapshot.Node {
	return applyFilter(nodes, f.filter)
}

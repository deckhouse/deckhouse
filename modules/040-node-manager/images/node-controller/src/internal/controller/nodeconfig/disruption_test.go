/*
Copyright 2026 Flant JSC

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

package nodeconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// TestNeedDrain: a group of one is interrupted without a drain (nowhere for the
// workload to go), but status.nodes == 0 means "not counted yet", not "one".
func TestNeedDrain(t *testing.T) {
	tests := []struct {
		name string
		ng   *v1.NodeGroup
		want bool
	}{
		{
			name: "a group of one has nowhere to drain to",
			ng:   nodeGroupWithNodes(1),
			want: false,
		},
		{
			name: "a group whose nodes have not been counted yet is drained",
			ng:   nodeGroupWithNodes(0),
			want: true,
		},
		{
			name: "a group of many is drained",
			ng:   nodeGroupWithNodes(50),
			want: true,
		},
		{
			name: "the operator can turn the drain off",
			ng: func() *v1.NodeGroup {
				ng := nodeGroupWithNodes(50)
				ng.Spec.Disruptions = &v1.DisruptionsSpec{
					Automatic: &v1.AutomaticDisruptionSpec{DrainBeforeApproval: ptr.To(false)},
				}
				return ng
			}(),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, needDrain(tt.ng))
		})
	}
}

func nodeGroupWithNodes(count int32) *v1.NodeGroup {
	ng := &v1.NodeGroup{}
	ng.Name = "worker"
	ng.Status.Nodes = count
	return ng
}

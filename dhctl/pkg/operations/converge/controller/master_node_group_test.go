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

package controller

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

func TestSurvivingMasterNodeNames(t *testing.T) {
	tests := []struct {
		name      string
		nodeNames []string
		replaced  string
		expected  []string
	}{
		{
			name:      "single master has no surviving nodes",
			nodeNames: []string{"master-0"},
			replaced:  "master-0",
			expected:  []string{},
		},
		{
			name: "replaced master is excluded and result is sorted",
			nodeNames: []string{
				"master-2",
				"master-0",
				"master-1",
			},
			replaced: "master-0",
			expected: []string{
				"master-1",
				"master-2",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nodeState := make(map[string][]byte, len(test.nodeNames))
			for _, nodeName := range test.nodeNames {
				nodeState[nodeName] = nil
			}

			controller := &MasterNodeGroupController{
				NodeGroupController: &NodeGroupController{
					state: state.NodeGroupInfrastructureState{
						State: nodeState,
					},
				},
			}

			actual := controller.survivingMasterNodeNames(test.replaced)

			require.Equal(t, test.expected, actual)
		})
	}
}

func TestMasterNodeVariablesRefresherIsNilForSingleMaster(t *testing.T) {
	controller := &MasterNodeGroupController{
		NodeGroupController: &NodeGroupController{
			state: state.NodeGroupInfrastructureState{
				State: map[string][]byte{
					"master-0": nil,
				},
			},
		},
	}

	refresher := controller.makeMasterNodeVariablesRefresher(
		nil,
		nil,
		"master-0",
		0,
	)

	require.Nil(t, refresher)
}

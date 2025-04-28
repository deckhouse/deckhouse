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

package controller

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_sortNodeNames(t *testing.T) {
	tests := []struct {
		unsorted []string
		sorted   []string
		error    bool
	}{
		{
			unsorted: []string{"node-3", "node-2", "node-1"},
			sorted:   []string{"node-3", "node-2", "node-1"},
		},
		{
			unsorted: []string{"node-2", "node-1", "node-3"},
			sorted:   []string{"node-3", "node-2", "node-1"},
		},
		{
			unsorted: []string{"node-1", "node-2", "node-3"},
			sorted:   []string{"node-3", "node-2", "node-1"},
		},
		{
			unsorted: []string{"node-1", "node-3", "node-2"},
			sorted:   []string{"node-3", "node-2", "node-1"},
		},
		{
			unsorted: []string{"node-a-2", "node-b-3", "node-c-1"},
			sorted:   []string{"node-b-3", "node-a-2", "node-c-1"},
		},
		{
			unsorted: []string{"node-a", "node-b", "node-c"},
			error:    true,
		},
	}

	for _, test := range tests {
		t.Run(strings.Join(test.unsorted, " "), func(t *testing.T) {
			state := make(map[string][]byte)

			for _, node := range test.unsorted {
				state[node] = nil
			}

			sorted, err := sortNodeNames(state)

			if test.error {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			require.Equal(t, test.sorted, sorted)
		})
	}
}

func Test_getNodesToDeleteInfo(t *testing.T) {
	tests := []struct {
		name     string
		replicas int
		nodes    []string
		deleted  []string
		error    bool
	}{
		{
			name:     "empty",
			replicas: 0,
			nodes:    []string{},
			deleted:  []string{},
		},
		{
			name:     "zero replicas and no deleted nodes",
			replicas: 0,
			nodes:    []string{"node-3", "node-2", "node-1"},
			deleted:  []string{"node-3", "node-2", "node-1"},
		},
		{
			name:     "1 replica and 2 deleted nodes",
			replicas: 1,
			nodes:    []string{"node-2", "node-1", "node-3"},
			deleted:  []string{"node-3", "node-2"},
		},
		{
			name:     "2 replicas and 1 deleted node",
			replicas: 2,
			nodes:    []string{"node-1", "node-2", "node-3"},
			deleted:  []string{"node-3"},
		},
		{
			name:     "3 replicas and no deleted nodes",
			replicas: 3,
			nodes:    []string{"node-1", "node-3", "node-2"},
			deleted:  []string{},
		},
		{
			name:  "validation error",
			nodes: []string{"node-a", "node-b", "node-c"},
			error: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := make(map[string][]byte)

			for _, node := range test.nodes {
				state[node] = nil
			}

			nodeToDeleteInfo, err := getNodesToDeleteInfo(test.replicas, state)

			if test.error {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			deleted := make([]string, 0, len(nodeToDeleteInfo))

			for _, node := range nodeToDeleteInfo {
				deleted = append(deleted, node.name)
			}

			require.Equal(t, test.deleted, deleted)
		})
	}
}

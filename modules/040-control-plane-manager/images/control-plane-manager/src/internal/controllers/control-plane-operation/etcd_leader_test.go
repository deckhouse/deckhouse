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

package controlplaneoperation

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEtcdLeaderTransferees(t *testing.T) {
	const self = 1
	masters := map[string]bool{"master-0": true, "master-1": true, "master-2": true}

	tests := []struct {
		name    string
		members []etcdMemberInfo
		want    []string
	}{
		{
			name: "no arbiter: left to etcd",
			members: []etcdMemberInfo{
				{ID: self, Name: "master-0"},
				{ID: 2, Name: "master-1"},
				{ID: 3, Name: "master-2"},
			},
		},
		{
			name: "leader on a master with an arbiter: the other master",
			members: []etcdMemberInfo{
				{ID: self, Name: "master-0"},
				{ID: 2, Name: "arbiter-0"},
				{ID: 3, Name: "master-1"},
			},
			want: []string{"master-1"},
		},
		{
			name: "leader on the arbiter: masters in name order",
			members: []etcdMemberInfo{
				{ID: self, Name: "arbiter-0"},
				{ID: 2, Name: "master-1"},
				{ID: 3, Name: "master-0"},
			},
			want: []string{"master-0", "master-1"},
		},
		{
			name: "learner arbiter does not count",
			members: []etcdMemberInfo{
				{ID: self, Name: "master-0"},
				{ID: 2, Name: "arbiter-0", IsLearner: true},
				{ID: 3, Name: "master-1"},
			},
		},
		{
			name: "learner master is skipped",
			members: []etcdMemberInfo{
				{ID: self, Name: "master-0"},
				{ID: 2, Name: "arbiter-0"},
				{ID: 3, Name: "master-1", IsLearner: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var names []string
			for _, m := range etcdLeaderTransferees(self, tt.members, masters) {
				names = append(names, m.Name)
			}
			require.Equal(t, tt.want, names)
		})
	}
}

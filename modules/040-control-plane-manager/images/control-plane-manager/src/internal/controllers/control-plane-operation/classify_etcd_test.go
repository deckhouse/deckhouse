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

func TestClassifyEtcd(t *testing.T) {
	const (
		nodeName  = "master-0"
		ourPeer   = "https://10.0.0.1:2380"
		otherPeer = "https://10.0.0.2:2380"
		oldPeer   = "https://10.0.0.9:2380" // same node name, different IP -> conflict
	)

	tests := []struct {
		name           string
		members        []etcdMemberInfo
		dataDirPresent bool
		wantState      etcdJoinState
		wantIsLearner  bool
	}{
		{
			name:           "fresh: no members, no data dir",
			members:        nil,
			dataDirPresent: false,
			wantState:      etcdNeedsJoin,
		},
		{
			name: "fresh: unrelated members, no data dir",
			members: []etcdMemberInfo{
				{Name: "master-1", PeerURLs: []string{otherPeer}},
			},
			dataDirPresent: false,
			wantState:      etcdNeedsJoin,
		},
		{
			name: "orphan: our member absent but stale data dir present",
			members: []etcdMemberInfo{
				{Name: "master-1", PeerURLs: []string{otherPeer}},
			},
			dataDirPresent: true,
			wantState:      etcdOrphan,
		},
		{
			name: "interrupted join: our learner added but etcd never started (no data dir, empty name)",
			members: []etcdMemberInfo{
				{Name: "", PeerURLs: []string{ourPeer}, IsLearner: true},
			},
			dataDirPresent: false,
			wantState:      etcdNeedsJoin,
			wantIsLearner:  true,
		},
		{
			name: "joined voter: our member present, data dir present, not a learner",
			members: []etcdMemberInfo{
				{Name: nodeName, PeerURLs: []string{ourPeer}},
			},
			dataDirPresent: true,
			wantState:      etcdJoined,
			wantIsLearner:  false,
		},
		{
			name: "joined learner: our member present, data dir present, still a learner",
			members: []etcdMemberInfo{
				{Name: nodeName, PeerURLs: []string{ourPeer}, IsLearner: true},
			},
			dataDirPresent: true,
			wantState:      etcdJoined,
			wantIsLearner:  true,
		},
		{
			name: "peer URL matched in second position",
			members: []etcdMemberInfo{
				{Name: nodeName, PeerURLs: []string{otherPeer, ourPeer}},
			},
			dataDirPresent: true,
			wantState:      etcdJoined,
		},
		{
			name: "name conflict: same node name, different peer URL",
			members: []etcdMemberInfo{
				{Name: nodeName, PeerURLs: []string{oldPeer}},
			},
			dataDirPresent: false,
			wantState:      etcdNameConflict,
		},
		{
			name: "conflict wins over peer match: new exact-peer learner plus old same-name voter",
			members: []etcdMemberInfo{
				{Name: "", PeerURLs: []string{ourPeer}, IsLearner: true},
				{Name: nodeName, PeerURLs: []string{oldPeer}},
			},
			dataDirPresent: true,
			wantState:      etcdNameConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyEtcd(tt.members, tt.dataDirPresent, nodeName, ourPeer)
			require.Equal(t, tt.wantState, got.state, "state")
			require.Equal(t, tt.wantIsLearner, got.isLearner, "isLearner")
		})
	}
}

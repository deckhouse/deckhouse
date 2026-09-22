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

package controlplane

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// Output of `etcdctl member list -w json` while a replaced master is rejoining:
// it is already listed, but as a learner it neither votes nor counts to quorum.
const memberListWithLearner = `{
  "header": {"cluster_id": 14841639068965178418, "member_id": 10276657743932975437, "revision": 42, "raft_term": 5},
  "members": [
    {"ID": 10276657743932975437, "name": "cluster-master-0", "peerURLs": ["https://10.12.1.10:2380"], "clientURLs": ["https://10.12.1.10:2379"]},
    {"ID": 13753461711699528563, "name": "cluster-master-1", "peerURLs": ["https://10.12.1.11:2380"], "clientURLs": ["https://10.12.1.11:2379"]},
    {"ID": 15532776154202127212, "name": "cluster-master-2", "peerURLs": ["https://10.12.1.12:2380"], "isLearner": true}
  ]
}`

func TestHasVotingMember(t *testing.T) {
	var out memberListOutput
	require.NoError(t, json.Unmarshal([]byte(memberListWithLearner), &out))
	require.Len(t, out.Members, 3)

	require.True(t, hasVotingMember(out.Members, "cluster-master-0"))
	require.False(t, hasVotingMember(out.Members, "cluster-master-2"), "a learner must not pass as a returned member")
	require.False(t, hasVotingMember(out.Members, "cluster-master-9"))
}

// A replaced master is listed twice while the stale member is still being removed.
func TestHasVotingMemberWithStaleDuplicate(t *testing.T) {
	members := []etcdMember{
		{Name: "cluster-master-0"},
		{Name: "cluster-master-1"},
		{Name: "cluster-master-1", IsLearner: true},
	}

	require.False(t, hasVotingMember(members, "cluster-master-1"))
}

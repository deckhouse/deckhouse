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

// Quorum is counted over the etcd membership, not over the master nodes: two members no node
// answers for are two votes nobody casts, and removing a healthy master hands the cluster to
// them.
func TestEtcdQuorumBeforeRemoval(t *testing.T) {
	masters := map[string]struct{}{
		"cluster-master-0": {},
		"cluster-master-1": {},
	}

	t.Run("a healthy scale down passes", func(t *testing.T) {
		threeMasters := map[string]struct{}{
			"cluster-master-0": {}, "cluster-master-1": {},
		}
		members := []etcdMember{
			{Name: "cluster-master-0"}, {Name: "cluster-master-1"}, {Name: "cluster-master-2"},
		}

		voting, served := etcdQuorumBeforeRemoval(members, "cluster-master-2", threeMasters)
		require.Equal(t, 2, voting)
		require.GreaterOrEqual(t, served, voting/2+1, "3 -> 2 must pass")

		oneMaster := map[string]struct{}{"cluster-master-0": {}}
		members = []etcdMember{{Name: "cluster-master-0"}, {Name: "cluster-master-1"}}

		voting, served = etcdQuorumBeforeRemoval(members, "cluster-master-1", oneMaster)
		require.Equal(t, 1, voting)
		require.GreaterOrEqual(t, served, voting/2+1, "2 -> 1 must pass")
	})

	t.Run("orphan members count against the quorum", func(t *testing.T) {
		members := []etcdMember{
			{Name: "cluster-master-0"},
			{Name: "cluster-master-1"},
			{Name: "cluster-master-2"},
			{Name: "cluster-master-7"},
			{Name: "cluster-master-8"},
		}

		voting, served := etcdQuorumBeforeRemoval(members, "cluster-master-2", masters)

		require.Equal(t, 4, voting)
		require.Equal(t, 2, served)
		require.Less(t, served, voting/2+1)
	})

	t.Run("learners do not vote", func(t *testing.T) {
		members := []etcdMember{
			{Name: "cluster-master-0"},
			{Name: "cluster-master-1"},
			{Name: "cluster-master-2"},
			{Name: "cluster-master-9", IsLearner: true},
		}

		voting, served := etcdQuorumBeforeRemoval(members, "cluster-master-2", masters)

		require.Equal(t, 2, voting)
		require.Equal(t, 2, served)
	})

	t.Run("a member listed twice votes twice and answers once", func(t *testing.T) {
		members := []etcdMember{
			{Name: "cluster-master-0"},
			{Name: "cluster-master-1"},
			{Name: "cluster-master-1"},
			{Name: "cluster-master-2"},
		}

		voting, served := etcdQuorumBeforeRemoval(members, "cluster-master-2", masters)

		require.Equal(t, 3, voting)
		require.Equal(t, 2, served)
	})
}

// Output of `etcdctl endpoint health --cluster -w json` right after a master left: the
// endpoint that went with it is gone from the list, one of the rest does not answer.
const endpointHealthWithFailure = `[
  {"endpoint": "https://10.12.1.10:2379", "health": true, "took": "3.1ms"},
  {"endpoint": "https://10.12.1.11:2379", "health": false, "took": "5s", "error": "context deadline exceeded"}
]`

func TestUnhealthyEndpoints(t *testing.T) {
	var endpoints []endpointHealth
	require.NoError(t, json.Unmarshal([]byte(endpointHealthWithFailure), &endpoints))

	unhealthy := unhealthyEndpoints(endpoints, nil)

	require.Len(t, unhealthy, 1)
	require.Contains(t, unhealthy[0], "https://10.12.1.11:2379")
	require.Contains(t, unhealthy[0], "context deadline exceeded")
}

func TestUnhealthyEndpointsReportsSilentFailure(t *testing.T) {
	unhealthy := unhealthyEndpoints([]endpointHealth{
		{Endpoint: "https://10.12.1.10:2379", Health: true},
		{Endpoint: "https://10.12.1.12:2379"},
	}, nil)

	require.Len(t, unhealthy, 1)
	require.Contains(t, unhealthy[0], "unhealthy")
}

// The master on its way out is the one whose etcd may already be dead: requiring an answer
// from it would refuse every removal it is the reason for.
func TestUnhealthyEndpointsSkipsTheLeavingMember(t *testing.T) {
	members := []etcdMember{
		{Name: "cluster-master-0", ClientURLs: []string{"https://10.12.1.10:2379"}},
		{Name: "cluster-master-2", ClientURLs: []string{"https://10.12.1.12:2379/"}},
	}
	endpoints := []endpointHealth{
		{Endpoint: "https://10.12.1.10:2379", Health: true},
		{Endpoint: "https://10.12.1.12:2379", Error: "context deadline exceeded"},
	}

	require.Empty(t, unhealthyEndpoints(endpoints, memberEndpoints(members, "cluster-master-2")))
	require.Len(t, unhealthyEndpoints(endpoints, memberEndpoints(members, "cluster-master-0")), 1)
}

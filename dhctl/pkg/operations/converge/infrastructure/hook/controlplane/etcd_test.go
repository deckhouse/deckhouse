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

func TestEtcdQuorumBeforeRemoval(t *testing.T) {
	member := func(name, url string) etcdMember {
		return etcdMember{Name: name, ClientURLs: []string{url}}
	}
	tests := []struct {
		name            string
		members         []etcdMember
		endpoints       []endpointHealth
		removed         string
		voting, healthy int
	}{
		{
			name:      "three to two",
			members:   []etcdMember{member("a", "a"), member("b", "b"), member("c", "c")},
			endpoints: []endpointHealth{{Endpoint: "a", Health: true}, {Endpoint: "b", Health: true}},
			removed:   "c", voting: 2, healthy: 2,
		},
		{
			name:      "two to one",
			members:   []etcdMember{member("a", "a"), member("b", "b")},
			endpoints: []endpointHealth{{Endpoint: "a", Health: true}},
			removed:   "b", voting: 1, healthy: 1,
		},
		{
			name:      "unhealthy survivor does not count",
			members:   []etcdMember{member("a", "a"), member("b", "b"), member("c", "c")},
			endpoints: []endpointHealth{{Endpoint: "a", Health: true}, {Endpoint: "b", Health: false}, {Endpoint: "c", Health: true}},
			removed:   "c", voting: 2, healthy: 1,
		},
		{
			name:      "missing health and client URLs do not count",
			members:   []etcdMember{member("a", "a"), member("b", "b"), {Name: "unstarted"}, member("c", "c")},
			endpoints: []endpointHealth{{Endpoint: "a", Health: true}, {Endpoint: "unknown", Health: true}},
			removed:   "c", voting: 3, healthy: 1,
		},
		{
			name:      "learner does not vote",
			members:   []etcdMember{member("a", "a"), {Name: "learner", ClientURLs: []string{"l"}, IsLearner: true}, member("c", "c")},
			endpoints: []endpointHealth{{Endpoint: "a", Health: true}, {Endpoint: "l", Health: true}},
			removed:   "c", voting: 1, healthy: 1,
		},
		{
			name:      "multiple URLs count as one member",
			members:   []etcdMember{{Name: "a", ClientURLs: []string{"a/", "a2", "a3"}}, member("c", "c")},
			endpoints: []endpointHealth{{Endpoint: "a", Health: false}, {Endpoint: "a2", Health: true}, {Endpoint: "a3", Health: true}},
			removed:   "c", voting: 1, healthy: 1,
		},
		{
			name:    "empty report",
			members: []etcdMember{member("a", "a"), member("c", "c")},
			removed: "c", voting: 1, healthy: 0,
		},
		{
			name:      "last member",
			members:   []etcdMember{member("c", "c")},
			endpoints: []endpointHealth{{Endpoint: "c", Health: true}},
			removed:   "c", voting: 0, healthy: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			voting, healthy := etcdQuorumBeforeRemoval(tt.members, tt.endpoints, tt.removed)
			require.Equal(t, tt.voting, voting)
			require.Equal(t, tt.healthy, healthy)
		})
	}
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

	unhealthy, checked := unhealthyEndpoints(endpoints, nil)

	require.Equal(t, 2, checked)
	require.Len(t, unhealthy, 1)
	require.Contains(t, unhealthy[0], "https://10.12.1.11:2379")
	require.Contains(t, unhealthy[0], "context deadline exceeded")
}

func TestUnhealthyEndpointsReportsSilentFailure(t *testing.T) {
	unhealthy, _ := unhealthyEndpoints([]endpointHealth{
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

	unhealthy, checked := unhealthyEndpoints(endpoints, memberEndpoints(members, "cluster-master-2"))
	require.Empty(t, unhealthy)
	require.Equal(t, 1, checked)

	unhealthy, _ = unhealthyEndpoints(endpoints, memberEndpoints(members, "cluster-master-0"))
	require.Len(t, unhealthy, 1)
}

// An empty report is not a healthy cluster: nothing answered, so nothing was checked.
func TestUnhealthyEndpointsCountsNothingWhenAllAreIgnored(t *testing.T) {
	members := []etcdMember{
		{Name: "cluster-master-0", ClientURLs: []string{"https://10.12.1.10:2379"}},
	}
	endpoints := []endpointHealth{{Endpoint: "https://10.12.1.10:2379", Health: true}}

	unhealthy, checked := unhealthyEndpoints(endpoints, memberEndpoints(members, "cluster-master-0"))
	require.Empty(t, unhealthy)
	require.Zero(t, checked)

	unhealthy, checked = unhealthyEndpoints(nil, nil)
	require.Empty(t, unhealthy)
	require.Zero(t, checked)
}

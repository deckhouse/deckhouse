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

package layout

import (
	"testing"

	"github.com/stretchr/testify/assert"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

func reportedAt(node, address string) registryv1alpha1.StorageReplicaStatus {
	return registryv1alpha1.StorageReplicaStatus{Node: node, Address: address}
}

// TestStorageAddressesTakeThePortEachReplicaReported covers the update to the store's current
// ports: a replica on the previous pod spec serves on 5001 until it is replaced, and the agents have
// to keep reaching it there, because the images of its replacement come through them from it.
func TestStorageAddressesTakeThePortEachReplicaReported(t *testing.T) {
	cases := []struct {
		name      string
		addresses []string
		replicas  []registryv1alpha1.StorageReplicaStatus
		want      []string
	}{
		{
			name:      "a replica on the previous pod spec keeps its port",
			addresses: []string{"10.0.0.1"},
			replicas:  []registryv1alpha1.StorageReplicaStatus{reportedAt("master-0", "10.0.0.1:5001")},
			want:      []string{"10.0.0.1:5001"},
		},
		{
			name:      "a replaced replica is reached on the current port",
			addresses: []string{"10.0.0.1"},
			replicas:  []registryv1alpha1.StorageReplicaStatus{reportedAt("master-0", "10.0.0.1:4281")},
			want:      []string{"10.0.0.1:4281"},
		},
		{
			name:      "without a report the current port",
			addresses: []string{"10.0.0.1", "10.0.0.2"},
			want:      []string{"10.0.0.1:4281", "10.0.0.2:4281"},
		},
		{
			name:      "mixed: each replica on its own port, one not reported yet",
			addresses: []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
			replicas: []registryv1alpha1.StorageReplicaStatus{
				reportedAt("master-1", "10.0.0.2:4281"),
				reportedAt("master-0", "10.0.0.1:5001"),
			},
			want: []string{"10.0.0.1:5001", "10.0.0.2:4281", "10.0.0.3:4281"},
		},
		{
			name:      "IPv6, bare or bracketed, matched to a report however it is spelled",
			addresses: []string{"fd00::1", "[fd00::2]", "fd00::3"},
			replicas: []registryv1alpha1.StorageReplicaStatus{
				reportedAt("master-0", "[fd00:0:0::1]:5001"),
				reportedAt("master-1", "[fd00::2]:4281"),
			},
			want: []string{"[fd00::1]:5001", "[fd00::2]:4281", "[fd00::3]:4281"},
		},
		{
			name:      "a report wins over a port written into the address",
			addresses: []string{"10.0.0.1:4281", "10.0.0.2:4281"},
			replicas:  []registryv1alpha1.StorageReplicaStatus{reportedAt("master-0", "10.0.0.1:5001")},
			want:      []string{"10.0.0.1:5001", "10.0.0.2:4281"},
		},
		{
			name:      "an unusable report is ignored rather than taking the replica out",
			addresses: []string{"10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4"},
			replicas: []registryv1alpha1.StorageReplicaStatus{
				reportedAt("master-0", "10.0.0.1"),
				reportedAt("master-1", "10.0.0.2:http"),
				reportedAt("master-2", "10.0.0.3:0"),
				reportedAt("master-3", ""),
			},
			want: []string{"10.0.0.1:4281", "10.0.0.2:4281", "10.0.0.3:4281", "10.0.0.4:4281"},
		},
		{
			name:      "a report from a host that is not a replica address adds nothing",
			addresses: []string{"10.0.0.1"},
			replicas:  []registryv1alpha1.StorageReplicaStatus{reportedAt("gone", "10.9.9.9:5001")},
			want:      []string{"10.0.0.1:4281"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, StorageAddresses(tc.addresses, tc.replicas))
		})
	}
}

// TestTheLeaderIsFoundByHostWhateverPortItReported: the leader's address is its own report, and the
// addresses are computed from the secret, so the two meet on the host. A leader on the previous pod
// spec must still go first, or agents ask a follower that may be behind and take its 404 as final.
func TestTheLeaderIsFoundByHostWhateverPortItReported(t *testing.T) {
	replicas := []registryv1alpha1.StorageReplicaStatus{
		reportedAt("master-0", "10.0.0.1:4281"),
		reportedAt("master-2", "10.0.0.3:5001"),
	}

	t.Run("a leader on the previous pod spec among replaced replicas", func(t *testing.T) {
		access := testAccess()
		access.Addresses = StorageAddresses([]string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}, replicas)
		access.Leader = "10.0.0.3:5001"

		backend := storageBackend(access)

		assert.Equal(t, "10.0.0.3:5001", backend.Endpoint.Host)
		hosts := make([]string, 0, len(backend.Mirrors))
		for _, mirror := range backend.Mirrors {
			hosts = append(hosts, mirror.Host)
		}
		assert.Equal(t, []string{"10.0.0.1:4281", "10.0.0.2:4281"}, hosts)
	})

	t.Run("an address whose port differs from the leader's report", func(t *testing.T) {
		access := testAccess()
		access.Addresses = []string{"10.0.0.1:4281", "10.0.0.3:4281"}
		access.Leader = "10.0.0.3:5001"

		backend := storageBackend(access)

		assert.Equal(t, "10.0.0.3:4281", backend.Endpoint.Host,
			"the leader goes first even where the port it is addressed by was decided elsewhere")
		assert.Len(t, backend.Mirrors, 1)
	})

	t.Run("IPv6 spelled differently in the report", func(t *testing.T) {
		access := testAccess()
		access.Addresses = StorageAddresses([]string{"fd00::1", "fd00::3"},
			[]registryv1alpha1.StorageReplicaStatus{reportedAt("master-2", "[fd00:0::3]:5001")})
		access.Leader = "[fd00:0::3]:5001"

		backend := storageBackend(access)

		assert.Equal(t, "[fd00::3]:5001", backend.Endpoint.Host)
		assert.Len(t, backend.Mirrors, 1)
		assert.Equal(t, "[fd00::1]:4281", backend.Mirrors[0].Host)
	})

	t.Run("a prefix of another address is not the same host", func(t *testing.T) {
		access := testAccess()
		access.Addresses = []string{"10.0.0.10:4281", "10.0.0.1:4281"}
		access.Leader = "10.0.0.1:5001"

		assert.Equal(t, "10.0.0.1:4281", storageBackend(access).Endpoint.Host)
	})
}

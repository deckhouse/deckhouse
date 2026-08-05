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

package run

import (
	"testing"

	"github.com/stretchr/testify/assert"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

func passThrough(needSync bool) *registryv1alpha1.RegistryStorageSpec {
	return &registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io", Path: "/deckhouse/ee"},
		},
		Source:   &registryv1alpha1.StorageSource{ExpectedDigests: 459},
		NeedSync: needSync,
	}
}

func TestDecide(t *testing.T) {
	tests := []struct {
		name     string
		spec     *registryv1alpha1.RegistryStorageSpec
		isLeader bool
		leader   *Leader
		want     Action
	}{
		{
			name:     "the leader was asked to fill",
			spec:     passThrough(true),
			isLeader: true,
			want:     ActionFill,
		},
		{
			name: "the leader was not asked to fill",
			spec: passThrough(false),
			// Counting the catalogue here would be wrong: with an upstream configured
			// the storage may hold images it fetched on a cache miss, which says
			// nothing about whether it holds the whole expected set.
			isLeader: true,
			want:     ActionNone,
		},
		{
			name: "an air-gapped leader",
			spec: &registryv1alpha1.RegistryStorageSpec{
				Source: &registryv1alpha1.StorageSource{ExpectedDigests: 459},
			},
			// The content arrives through the write endpoint and the syncer never sees
			// it, so reading the catalogue is the only honest accounting left.
			isLeader: true,
			want:     ActionCountCatalogue,
		},
		{
			name: "an air-gapped leader that was asked to fill anyway",
			spec: &registryv1alpha1.RegistryStorageSpec{
				Source:   &registryv1alpha1.StorageSource{ExpectedDigests: 459},
				NeedSync: true,
			},
			isLeader: true,
			// There is nowhere to fill from. Trying would fail every pass and report a
			// failure that no operator can act on.
			want: ActionCountCatalogue,
		},
		{
			name:     "a follower with no leader to copy from yet",
			spec:     passThrough(true),
			isLeader: false,
			// A follower never spends the upstream credentials, so with nothing to
			// replicate from it just reports what it already holds.
			want: ActionCountCatalogue,
		},
		{
			name:     "a follower with a complete leader",
			spec:     passThrough(true),
			isLeader: false,
			leader:   &Leader{Node: "master-0", Address: "10.0.0.1:5001", Full: true},
			// Ahead of time, not on a cache miss: this is what makes a leader change
			// cheap, and in air-gap it is the only way a follower gets the content.
			want: ActionReplicate,
		},
		{
			name:     "a follower whose leader is still filling",
			spec:     passThrough(true),
			isLeader: false,
			leader:   &Leader{Node: "master-0", Address: "10.0.0.1:5001", Full: false},
			// Copying a partial set is work that has to be redone.
			want: ActionCountCatalogue,
		},
		{
			name:     "a follower whose leader reported no address",
			spec:     passThrough(true),
			isLeader: false,
			leader:   &Leader{Node: "master-0", Full: true},
			want:     ActionCountCatalogue,
		},
		{
			name:     "an air-gapped follower with a complete leader",
			spec:     &registryv1alpha1.RegistryStorageSpec{},
			isLeader: false,
			leader:   &Leader{Node: "master-0", Address: "10.0.0.1:5001", Full: true},
			// The only way content reaches a follower in air-gap: there is no upstream
			// for it to fall back on, ever.
			want: ActionReplicate,
		},
		{
			name:     "no spec at all",
			spec:     nil,
			isLeader: true,
			want:     ActionNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Decide(tt.spec, tt.isLeader, tt.leader))
		})
	}
}

// TestOnlyTheLeaderFills is the property stated separately because it is what keeps
// several replicas from all hammering the upstream with the same credentials.
func TestOnlyTheLeaderFills(t *testing.T) {
	spec := passThrough(true)

	assert.Equal(t, ActionFill, Decide(spec, true, nil))
	assert.NotEqual(t, ActionFill, Decide(spec, false, nil))
}

func TestExpectedDigests(t *testing.T) {
	assert.EqualValues(t, 459, ExpectedDigests(passThrough(true)))
	assert.EqualValues(t, 0, ExpectedDigests(&registryv1alpha1.RegistryStorageSpec{}))
	assert.EqualValues(t, 0, ExpectedDigests(nil))
}

func TestRole(t *testing.T) {
	assert.Equal(t, registryv1alpha1.ReplicaRoleLeader, Role(true))
	assert.Equal(t, registryv1alpha1.ReplicaRoleFollower, Role(false))
}

func TestLeaderUsable(t *testing.T) {
	tests := []struct {
		name   string
		leader *Leader
		want   bool
	}{
		{name: "none", leader: nil, want: false},
		{
			name:   "complete with an address",
			leader: &Leader{Node: "master-0", Address: "10.0.0.1:5001", Full: true},
			want:   true,
		},
		{
			name:   "still filling",
			leader: &Leader{Node: "master-0", Address: "10.0.0.1:5001"},
			want:   false,
		},
		{
			name: "no address to reach it at",
			// Nothing to connect to, so a follower would fail every pass.
			leader: &Leader{Node: "master-0", Full: true},
			want:   false,
		},
		{
			name:   "no node name",
			leader: &Leader{Address: "10.0.0.1:5001", Full: true},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.leader.Usable())
		})
	}
}

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

	"github.com/deckhouse/registry-syncer/internal/fill"
	"github.com/deckhouse/registry-syncer/internal/gc"
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
			leader:   &Leader{Node: "master-1", Address: "10.0.0.1:5001", Full: false},
			// Replicated from anyway: whatever the leader holds is worth having, and what it does
			// not hold yet comes back as pending rather than as a failure. Waiting for completeness
			// left followers idle for as long as a fill takes — and holding nothing if the leader
			// died meanwhile, which is the one thing ahead-of-time replication exists to prevent.
			want: ActionReplicate,
		}, {
			name:     "a follower whose leader reported no address",
			spec:     passThrough(true),
			isLeader: false,
			leader:   &Leader{Node: "master-0", Full: true},
			want:     ActionCountCatalogue,
		},
		{
			name:     "an air-gapped follower with a complete leader",
			spec:     &registryv1alpha1.RegistryStorageSpec{NeedSync: true},
			isLeader: false,
			leader:   &Leader{Node: "master-0", Address: "10.0.0.1:5001", Full: true},
			// The only way content reaches a follower in air-gap: there is no upstream
			// for it to fall back on, ever.
			want: ActionReplicate,
		},
		{
			// The steady state: every replica holds the set, so the controller asks for nothing.
			// Replicating anyway found every reference already present, every half a minute.
			name:     "a follower when no replica is behind",
			spec:     passThrough(false),
			isLeader: false,
			leader:   &Leader{Node: "master-0", Address: "10.0.0.1:5001", Full: true},
			want:     ActionNone,
		},
		{
			name:     "an air-gapped follower when no replica is behind",
			spec:     &registryv1alpha1.RegistryStorageSpec{},
			isLeader: false,
			leader:   &Leader{Node: "master-0", Address: "10.0.0.1:5001", Full: true},
			want:     ActionNone,
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
			// Usable while still filling: what it already holds is worth copying, and what it does
			// not hold yet is pending rather than failed. Requiring completeness here is what kept
			// followers idle for the whole of a fill.
			name:   "still filling",
			leader: &Leader{Node: "master-0", Address: "10.0.0.1:5001"},
			want:   true,
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

// TestStoreIsAuthority pins who gets to say how much the storage holds.
//
// The transition window is the case that matters: air-gap asked for, the write
// endpoint open, and the upstream still held so nothing stops working. There the
// count has to come from the store, because `d8 mirror push` writes straight into
// it and the syncer never sees those writes.
func TestStoreIsAuthority(t *testing.T) {
	publishing := &registryv1alpha1.RegistryStorageSpec{
		// The window, not the endpoint. Publication is now on every managed cluster and says nothing
		// about how completeness is measured; this field is what still means "air-gap asked for,
		// upstream still held".
		Publish:         true,
		AirGapRequested: true,
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{Host: "registry.example.com"},
		},
		NeedSync: true,
	}

	tests := []struct {
		name     string
		spec     *registryv1alpha1.RegistryStorageSpec
		isLeader bool
		want     bool
	}{
		{
			name: "the leader in the transition window, upstream still held",
			spec: publishing, isLeader: true, want: true,
		},
		{
			// A follower's store is a copy of the leader's, so what it holds
			// authorizes nothing and must not be mistaken for evidence.
			name: "a follower while the endpoint is open",
			spec: publishing, isLeader: false, want: false,
		},
		{
			// An ordinary cache: pull-through is on, so the catalogue counts what
			// the storage can fetch rather than what it holds.
			name: "the leader of a cache that is not publishing",
			spec: &registryv1alpha1.RegistryStorageSpec{
				Upstream: &registryv1alpha1.Upstream{
					Endpoint: registryv1alpha1.Endpoint{Host: "registry.example.com"},
				},
				NeedSync: true,
			},
			isLeader: true, want: false,
		},
		{name: "no storage at all", spec: nil, isLeader: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, StoreIsAuthority(tt.spec, tt.isLeader))
		})
	}
}

func TestSetKeyMovesExactlyWithTheSet(t *testing.T) {
	modules := []fill.ModuleRef{
		{Name: "console", Version: "v1.2.0", Source: fill.ModuleSource{Repository: "registry.d8-system.svc:5001/system/deckhouse/modules"}},
		{Name: "pushgateway", Version: "v1.0.2", Source: fill.ModuleSource{Repository: "registry.deckhouse.io/deckhouse/ce/modules"}},
	}
	base := SetKey(gc.Releases{Deployed: "v1.70.1", Previous: "v1.69.3"}, modules, nil)

	reordered := []fill.ModuleRef{modules[1], modules[0]}
	assert.Equal(t, base, SetKey(gc.Releases{Deployed: "v1.70.1", Previous: "v1.69.3"}, reordered, nil), "the order modules are listed in is not the set")

	moved := func(mutate func([]fill.ModuleRef) []fill.ModuleRef) string {
		copied := append([]fill.ModuleRef(nil), modules...)
		return SetKey(gc.Releases{Deployed: "v1.70.1", Previous: "v1.69.3"}, mutate(copied), nil)
	}
	assert.NotEqual(t, base, SetKey(gc.Releases{Deployed: "v1.71.0", Previous: "v1.70.1"}, modules, nil), "the platform updated")
	assert.NotEqual(t, base, moved(func(m []fill.ModuleRef) []fill.ModuleRef { m[1].Version = "v1.0.3"; return m }),
		"a module updated")
	assert.NotEqual(t, base, moved(func(m []fill.ModuleRef) []fill.ModuleRef {
		m[1].Source.Repository = "registry.d8-system.svc:5001/system/deckhouse/modules"
		return m
	}), "a module repointed at the store ahead of air-gap")
	assert.NotEqual(t, base, moved(func(m []fill.ModuleRef) []fill.ModuleRef { return m[:1] }), "a module removed")
	assert.NotEqual(t, base, moved(func(m []fill.ModuleRef) []fill.ModuleRef {
		return append(m, fill.ModuleRef{Name: "stronghold", Version: "v1.0.0"})
	}), "a module installed")
}

func TestReverify(t *testing.T) {
	withUpstream := &registryv1alpha1.RegistryStorageSpec{Upstream: &registryv1alpha1.Upstream{}}
	airGap := &registryv1alpha1.RegistryStorageSpec{}

	for _, tc := range []struct {
		name              string
		action            Action
		leader            bool
		spec              *registryv1alpha1.RegistryStorageSpec
		reported, current string
		want              Action
		withdraw          bool
	}{
		{"a full leader, the set moved", ActionNone, true, withUpstream, "old", "new", ActionFill, true},
		{"a full leader, the same set", ActionNone, true, withUpstream, "same", "same", ActionNone, false},
		{"after an upgrade from before fingerprints", ActionNone, true, withUpstream, "", "new", ActionFill, true},
		{"the set could not be read", ActionNone, true, withUpstream, "old", "", ActionNone, false},
		{"already filling", ActionFill, true, withUpstream, "old", "new", ActionFill, false},
		{"a follower", ActionReplicate, false, withUpstream, "old", "new", ActionReplicate, false},
		{"air-gapped: nowhere to fill from", ActionCountCatalogue, true, airGap, "old", "new", ActionCountCatalogue, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			action, withdraw := Reverify(tc.action, tc.leader, tc.spec, tc.reported, tc.current)
			assert.Equal(t, tc.want, action)
			assert.Equal(t, tc.withdraw, withdraw)
		})
	}
}

// TestSetKeyIsTheSetAndAheadKeyWhatIsAboutToArrive: a module pushed into the catalogue changes the
// set, so it takes the leader's full report away and raises needSync. An update waiting to be deployed
// does not: it is ahead of the set, and moves AheadKey instead — the next version only, and every
// module update. Nothing offered, the set's fingerprint is the one it always was; nothing pending, there
// is nothing ahead.
func TestSetKeyIsTheSetAndAheadKeyWhatIsAboutToArrive(t *testing.T) {
	deployed := gc.Releases{Deployed: "v1.76.6", Previous: "v1.75.0"}
	base := SetKey(deployed, nil, nil)
	assert.Empty(t, AheadKey(deployed, nil), "nothing pending, nothing ahead")

	pending := deployed
	pending.Pending = []string{"v1.77.0"}
	assert.Equal(t, base, SetKey(pending, nil, nil), "an update is not the set the cluster runs")
	next := AheadKey(pending, nil)
	assert.NotEmpty(t, next, "an update arrived")

	further := deployed
	further.Pending = []string{"v1.77.0", "v1.78.0"}
	assert.Equal(t, next, AheadKey(further, nil), "only the next version is copied ahead, so only it counts")

	update := []fill.ModuleRef{{Name: "console", Version: "v1.2.0", Pending: true}}
	assert.Equal(t, base, SetKey(deployed, update, nil), "nor is a module update")
	assert.NotEqual(t, next, AheadKey(pending, update), "but it is ahead of the set")
	assert.NotEmpty(t, AheadKey(deployed, update))

	withModule := SetKey(deployed, nil, []string{"deckhouse/console", "deckhouse/virtualization"})
	assert.NotEqual(t, base, withModule, "a module was pushed into the catalogue")
	assert.NotEqual(t, withModule, SetKey(deployed, nil, []string{"deckhouse/console"}))
	assert.Equal(t, base, SetKey(deployed, nil, []string{}), "nothing offered: as before")
}

func TestAlreadyHeld(t *testing.T) {
	leader := &Leader{Node: "master-1"}

	tests := []struct {
		name           string
		action         Action
		reportedSet    string
		reportedSource string
		leader         *Leader
		want           bool
	}{
		{name: "a leader found full against this set", action: ActionFill, reportedSet: "a", want: true},
		{name: "a leader found full against another set", action: ActionFill, reportedSet: "b", want: false},
		{name: "a follower that replicated this set from the leader", action: ActionReplicate,
			reportedSet: "a", reportedSource: "master-1", leader: leader, want: true},
		// The lease moved: copying once from the new leader is what needSync was raised for.
		{name: "a follower that replicated this set from the previous leader", action: ActionReplicate,
			reportedSet: "a", reportedSource: "master-0", leader: leader, want: false},
		{name: "a former leader, which replicated from nobody", action: ActionReplicate,
			reportedSet: "a", reportedSource: "", leader: leader, want: false},
		{name: "a follower with no leader to name", action: ActionReplicate,
			reportedSet: "a", reportedSource: "master-1", want: false},
		{name: "nothing to copy anyway", action: ActionNone, reportedSet: "a", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AlreadyHeld(tt.action, "a", tt.reportedSet, tt.reportedSource, tt.leader))
		})
	}
	assert.False(t, AlreadyHeld(ActionFill, "", "", "", nil), "a set that could not be read is never held")
}

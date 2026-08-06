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
	"github.com/stretchr/testify/require"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

func testUpstream(host string) *registryv1alpha1.Upstream {
	return &registryv1alpha1.Upstream{
		Endpoint: registryv1alpha1.Endpoint{
			Scheme: registryv1alpha1.SchemeHTTPS,
			Host:   host,
			Path:   "/deckhouse/ee",
			Auth:   &registryv1alpha1.Auth{Username: "license-token", Password: "key"},
		},
	}
}

// heldUpstream is an upstream as the CLUSTER records it: the addresses, and no credentials.
//
// The distinction that hid a defect through a whole run of these tests. A held upstream comes
// from RegistryConfig.status, which records addresses deliberately without credentials — while
// every test here built it with `testUpstream`, credentials included, and so described a
// cluster that cannot exist. What reaches a real reconciliation is this shape.
func heldUpstream(host string) *registryv1alpha1.Upstream {
	out := testUpstream(host)
	out.Endpoint.Auth = nil
	return out
}

// persistedUpstreamAuth is what this module wrote into its own auth Secret last time, in the
// pre-encoded form that is the only form it writes.
func persistedUpstreamAuth() map[string]registryv1alpha1.Auth {
	return map[string]registryv1alpha1.Auth{
		constant.AuthKeyUpstream: {Auth: "bGljZW5zZS10b2tlbjprZXk="},
	}
}

func testAccess() Access {
	return Access{
		// Node addresses: what a client can actually dial. The Service name is what
		// image references are built from, and nothing resolves it.
		Addresses: []string{"10.0.0.1:5001", "10.0.0.2:5001"},
		CA:        "-----BEGIN CERTIFICATE-----storage",
		Auth:      &registryv1alpha1.Auth{Username: "ro", Password: "secret"},
	}
}

func testSource() *registryv1alpha1.StorageSource {
	return &registryv1alpha1.StorageSource{BundleRef: "d8-mirror-bundle", ExpectedDigests: 459}
}

// TestComputeDirect covers cache off: the agent talks to the upstream and no
// storage exists at all.
func TestComputeDirect(t *testing.T) {
	got := Compute(Inputs{
		Config: registryv1alpha1.RegistryConfigSpec{
			Mode:    registryv1alpha1.ModeManaged,
			Primary: registryv1alpha1.PrimarySource{Upstream: testUpstream("registry.deckhouse.io")},
			Storage: registryv1alpha1.StorageConfig{Cache: false},
		},
		Nodes: []string{"master-0", "worker-1"},
	})

	assert.Nil(t, got.Storage, "no storage object may exist when the cache is off")
	assert.False(t, got.DropUpstream)
	require.Len(t, got.Nodes, 2)

	for name, node := range got.Nodes {
		assert.Falsef(t, node.Cache, "node %s must not route through a cache", name)
		require.Len(t, node.Backends, 1)
		assert.Equal(t, registryv1alpha1.BackendUpstream, node.Backends[0].Name)
		assert.Equal(t, "registry.deckhouse.io/deckhouse/ee", node.Backends[0].Address())
	}
}

// TestComputePassThroughCache covers cache on with an upstream: the storage is
// filled from the upstream, and nodes keep the upstream as a fallback while it
// fills.
func TestComputePassThroughCache(t *testing.T) {
	got := Compute(Inputs{
		Config: registryv1alpha1.RegistryConfigSpec{
			Mode:    registryv1alpha1.ModeManaged,
			Primary: registryv1alpha1.PrimarySource{Upstream: testUpstream("registry.deckhouse.io")},
			Storage: registryv1alpha1.StorageConfig{Cache: true, Size: "50Gi", Source: testSource()},
		},
		Nodes:         []string{"master-0"},
		StorageAccess: testAccess(),
	})

	require.NotNil(t, got.Storage)
	require.NotNil(t, got.Storage.Upstream)
	assert.Equal(t, "registry.deckhouse.io", got.Storage.Upstream.Host)
	assert.Equal(t, constant.StorePath, got.Storage.Store.Path)
	assert.Equal(t, "50Gi", got.Storage.Store.Size)
	assert.True(t, got.Storage.NeedSync, "the leader is not full yet, so filling it is a task")
	assert.False(t, got.Storage.Publish,
		"a pass-through cache needs no write endpoint; publishing one would add write surface for nothing")
	assert.False(t, got.DropUpstream)

	node := got.Nodes["master-0"]
	assert.True(t, node.Cache)
	require.Len(t, node.Backends, 2)

	// Order is priority: the cache first, the upstream only as a fallback.
	assert.Equal(t, registryv1alpha1.BackendStorage, node.Backends[0].Name)
	// An address rather than constant.Host: that name identifies the image set and is
	// what a request is matched against, but a node agent runs in the host network and
	// cannot resolve it.
	assert.Equal(t, "10.0.0.1:5001", node.Backends[0].Host)
	assert.Equal(t, constant.Path, node.Backends[0].Path)
	assert.Equal(t, testAccess().CA, node.Backends[0].CA)
	// The layout names where the credentials are, never what they are: this resource is
	// cluster-scoped and every node's kubelet can read it.
	require.NotNil(t, node.Backends[0].Auth)
	require.NotNil(t, node.Backends[0].Auth.SecretRef)
	assert.Equal(t, constant.AuthSecretName, node.Backends[0].Auth.SecretRef.Name)
	assert.Equal(t, constant.AuthKeyStorage, node.Backends[0].Auth.SecretRef.Key)
	assert.Empty(t, node.Backends[0].Auth.Username)
	assert.Empty(t, node.Backends[0].Auth.Password)
	// And the credentials come back separately, for the caller to put in that Secret.
	assert.Equal(t, "ro", got.Credentials[constant.AuthKeyStorage].Username)

	assert.Equal(t, registryv1alpha1.BackendUpstream, node.Backends[1].Name)
}

// TestComputeAirGapWaitsForTheLeader is the safety property of the whole design:
// removing the upstream from the configuration must not take effect until the
// cache can stand alone.
func TestComputeAirGapWaitsForTheLeader(t *testing.T) {
	applied := testUpstream("registry.deckhouse.io")

	// The user has removed the upstream, so the configuration asks for air-gap.
	config := registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Storage: registryv1alpha1.StorageConfig{Cache: true, Size: "50Gi", Source: testSource()},
	}

	t.Run("the leader is not full yet", func(t *testing.T) {
		got := Compute(Inputs{
			Config: config,
			// As the cluster records it — without credentials — plus the copy this
			// module keeps of them. That is the whole of what a hold has to work with.
			AppliedUpstream:      heldUpstream("registry.deckhouse.io"),
			PersistedCredentials: persistedUpstreamAuth(),
			LeaderFull:           false,
			Nodes:                []string{"master-0", "worker-1"},
			StorageAccess:        testAccess(),
		})

		assert.False(t, got.DropUpstream)

		require.NotNil(t, got.Storage)
		require.NotNil(t, got.Storage.Upstream,
			"the storage must keep the last applied upstream: dropping it now would cut every node off")
		assert.Equal(t, "registry.deckhouse.io", got.Storage.Upstream.Host)
		assert.True(t, got.Storage.Publish, "an air-gapped cache is filled through the write endpoint")

		// Held WITH its credentials, which is the difference between a fallback and a
		// fallback nobody can use.
		//
		// The upstream is private, so a backend without credentials turns a cache miss
		// from something slower into a failure, and answers any reference naming the
		// upstream by its own name with a 401. Measured on a cluster in exactly this
		// state: asked for air-gap, cache incomplete, and every node holding an
		// upstream it could not authenticate to.
		require.Contains(t, got.Credentials, constant.AuthKeyUpstream,
			"the credentials of a held upstream must survive into the auth Secret")

		for name, node := range got.Nodes {
			require.Lenf(t, node.Backends, 2, "node %s must keep the upstream as a fallback", name)
			upstream := node.Backends[1]
			assert.Equal(t, registryv1alpha1.BackendUpstream, upstream.Name)
			require.NotNilf(t, upstream.Endpoint.Auth,
				"node %s holds an upstream with no credentials, so its fallback cannot authenticate", name)
			require.NotNil(t, upstream.Endpoint.Auth.SecretRef)
			assert.Equal(t, constant.AuthKeyUpstream, upstream.Endpoint.Auth.SecretRef.Key)
		}
	})

	// Nothing is invented when there is nothing to reattach: the fallback goes out without
	// credentials, which is honest, and is what a cluster whose upstream never had any looks
	// like.
	t.Run("nothing was persisted to hold over", func(t *testing.T) {
		got := Compute(Inputs{
			Config:          config,
			AppliedUpstream: heldUpstream("registry.deckhouse.io"),
			LeaderFull:      false,
			Nodes:           []string{"master-0"},
			StorageAccess:   testAccess(),
		})

		assert.NotContains(t, got.Credentials, constant.AuthKeyUpstream)
		assert.Nil(t, got.Nodes["master-0"].Backends[1].Endpoint.Auth)
	})

	t.Run("the leader is full", func(t *testing.T) {
		got := Compute(Inputs{
			Config:          config,
			AppliedUpstream: applied,
			LeaderFull:      true,
			Nodes:           []string{"master-0", "worker-1"},
			StorageAccess:   testAccess(),
		})

		assert.True(t, got.DropUpstream, "the cache is complete, so this is the moment of the transition")
		assert.Nil(t, got.HeldUpstream)

		require.NotNil(t, got.Storage)
		assert.Nil(t, got.Storage.Upstream, "the cache is now authoritative")
		assert.False(t, got.Storage.NeedSync, "there is nothing left to fill the leader from")

		// The upstream disappears from the storage and from every node in the same
		// reconciliation, so no node is ever left pointing at an upstream the
		// storage no longer knows about.
		for name, node := range got.Nodes {
			require.Lenf(t, node.Backends, 1, "node %s must have only the cache left", name)
			assert.Equal(t, registryv1alpha1.BackendStorage, node.Backends[0].Name)
		}
	})
}

// TestComputeDoesNotDropAConfiguredUpstream guards against the gate firing in the
// wrong direction: a full cache with an upstream still configured must keep it.
func TestComputeDoesNotDropAConfiguredUpstream(t *testing.T) {
	got := Compute(Inputs{
		Config: registryv1alpha1.RegistryConfigSpec{
			Mode:    registryv1alpha1.ModeManaged,
			Primary: registryv1alpha1.PrimarySource{Upstream: testUpstream("registry.deckhouse.io")},
			Storage: registryv1alpha1.StorageConfig{Cache: true, Source: testSource()},
		},
		AppliedUpstream: testUpstream("registry.deckhouse.io"),
		LeaderFull:      true,
		Nodes:           []string{"master-0"},
		StorageAccess:   testAccess(),
	})

	assert.False(t, got.DropUpstream)
	require.NotNil(t, got.Storage.Upstream)
	assert.Len(t, got.Nodes["master-0"].Backends, 2)
	assert.False(t, got.Storage.NeedSync, "the leader is already full")
}

// TestComputeHoldsLastKnownGoodOnAFailedProbe is the "broke the license, kept the
// cluster" property. The configured upstream is new and does not work, so the
// cluster must stay on the one that does.
func TestComputeHoldsLastKnownGoodOnAFailedProbe(t *testing.T) {
	working := testUpstream("registry.deckhouse.io")
	broken := testUpstream("registry.typo.example.com")

	t.Run("with a cache", func(t *testing.T) {
		got := Compute(Inputs{
			Config: registryv1alpha1.RegistryConfigSpec{
				Mode:    registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: broken},
				Storage: registryv1alpha1.StorageConfig{Cache: true, Source: testSource()},
			},
			AppliedUpstream:     working,
			UpstreamProbeFailed: true,
			Nodes:               []string{"master-0"},
			StorageAccess:       testAccess(),
		})

		require.NotNil(t, got.Storage.Upstream)
		assert.Equal(t, "registry.deckhouse.io", got.Storage.Upstream.Host,
			"the storage must stay on the upstream that works")
		assert.False(t, got.DropUpstream)

		node := got.Nodes["master-0"]
		require.Len(t, node.Backends, 2)
		assert.Equal(t, "registry.deckhouse.io/deckhouse/ee", node.Backends[1].Address())
	})

	t.Run("without a cache", func(t *testing.T) {
		// There is no storage here to hold anything, so the hold has to work off the
		// recorded effective upstream alone.
		got := Compute(Inputs{
			Config: registryv1alpha1.RegistryConfigSpec{
				Mode:    registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: broken},
				Storage: registryv1alpha1.StorageConfig{Cache: false},
			},
			AppliedUpstream:     working,
			UpstreamProbeFailed: true,
			Nodes:               []string{"worker-1"},
		})

		assert.Nil(t, got.Storage)
		node := got.Nodes["worker-1"]
		require.Len(t, node.Backends, 1)
		assert.Equal(t, "registry.deckhouse.io/deckhouse/ee", node.Backends[0].Address(),
			"a broken license must not stop the nodes pulling")
	})
}

// TestComputeFirstUpstreamFailingItsProbe covers the case with nothing to fall back
// on: a fresh cluster configured with an upstream that does not work.
func TestComputeFirstUpstreamFailingItsProbe(t *testing.T) {
	got := Compute(Inputs{
		Config: registryv1alpha1.RegistryConfigSpec{
			Mode:    registryv1alpha1.ModeManaged,
			Primary: registryv1alpha1.PrimarySource{Upstream: testUpstream("registry.typo.example.com")},
			Storage: registryv1alpha1.StorageConfig{Cache: true, Source: testSource()},
		},
		AppliedUpstream:     nil,
		UpstreamProbeFailed: true,
		Nodes:               []string{"master-0"},
		StorageAccess:       testAccess(),
	})

	// Nothing good to hold, so nothing is used. Honest and inert: the condition on
	// RegistryConfig is what tells the operator why, rather than a layout that
	// looks configured and silently fails on every pull.
	require.NotNil(t, got.Storage)
	assert.Nil(t, got.Storage.Upstream)
	assert.False(t, got.DropUpstream)
	assert.Len(t, got.Nodes["master-0"].Backends, 1)
}

// TestComputeAirGapFromScratch covers a cluster that never had an upstream: there
// is nothing to hold, and nothing to drop.
func TestComputeAirGapFromScratch(t *testing.T) {
	got := Compute(Inputs{
		Config: registryv1alpha1.RegistryConfigSpec{
			Mode:    registryv1alpha1.ModeManaged,
			Storage: registryv1alpha1.StorageConfig{Cache: true, Size: "50Gi", Source: testSource()},
		},
		AppliedUpstream: nil,
		LeaderFull:      false,
		Nodes:           []string{"master-0"},
		StorageAccess:   testAccess(),
	})

	require.NotNil(t, got.Storage)
	assert.Nil(t, got.Storage.Upstream)
	assert.True(t, got.Storage.Publish)
	assert.False(t, got.Storage.NeedSync, "in air-gap the syncer cannot fill the leader from anywhere")

	// DropUpstream describes a transition, and there was none: the cluster was
	// already air-gapped.
	assert.False(t, got.DropUpstream)

	node := got.Nodes["master-0"]
	require.Len(t, node.Backends, 1)
	assert.Equal(t, registryv1alpha1.BackendStorage, node.Backends[0].Name)
}

func TestComputeUnmanagedLaysOutNothing(t *testing.T) {
	got := Compute(Inputs{
		Config: registryv1alpha1.RegistryConfigSpec{
			Mode:    registryv1alpha1.ModeUnmanaged,
			Primary: registryv1alpha1.PrimarySource{Upstream: testUpstream("registry.deckhouse.io")},
			Storage: registryv1alpha1.StorageConfig{Cache: true},
		},
		Nodes:         []string{"master-0"},
		StorageAccess: testAccess(),
	})

	assert.Nil(t, got.Storage)
	assert.Empty(t, got.Nodes)
	assert.False(t, got.DropUpstream)
}

// TestComputeCompilesAdditionalRoutes checks the transit routes reach every node
// and stay out of the cache.
func TestComputeCompilesAdditionalRoutes(t *testing.T) {
	routes := []registryv1alpha1.Route{{
		Match: "images.virtualization.example.com",
		Endpoint: registryv1alpha1.Endpoint{
			Scheme: registryv1alpha1.SchemeHTTPS,
			Host:   "registry-vendor.example.com",
			Path:   "/virt",
		},
	}}

	got := Compute(Inputs{
		Config: registryv1alpha1.RegistryConfigSpec{
			Mode:    registryv1alpha1.ModeManaged,
			Primary: registryv1alpha1.PrimarySource{Upstream: testUpstream("registry.deckhouse.io")},
			Storage: registryv1alpha1.StorageConfig{Cache: true, Source: testSource()},
		},
		Nodes:         []string{"master-0", "worker-1"},
		Routes:        routes,
		StorageAccess: testAccess(),
	})

	for name, node := range got.Nodes {
		require.Lenf(t, node.AdditionalRoutes, 1, "node %s is missing the route", name)
		assert.Equal(t, "images.virtualization.example.com", node.AdditionalRoutes[0].Match)
	}

	// Additional upstreams are transit only: nothing about them may end up in the
	// storage, whose space is reserved for Deckhouse components.
	assert.NotContains(t, got.Storage.Upstream.Host, "vendor")
}

// TestComputeGivesEachNodeItsOwnCopy guards against nodes sharing backing arrays.
// They are identical today, so a shared slice would go unnoticed until the first
// per-node difference silently rewrote every other node's layout.
func TestComputeGivesEachNodeItsOwnCopy(t *testing.T) {
	got := Compute(Inputs{
		Config: registryv1alpha1.RegistryConfigSpec{
			Mode:    registryv1alpha1.ModeManaged,
			Primary: registryv1alpha1.PrimarySource{Upstream: testUpstream("registry.deckhouse.io")},
			Storage: registryv1alpha1.StorageConfig{Cache: true, Source: testSource()},
		},
		Nodes: []string{"master-0", "worker-1"},
		Routes: []registryv1alpha1.Route{{
			Match:    "images.example.com",
			Endpoint: registryv1alpha1.Endpoint{Host: "vendor.example.com"},
		}},
		StorageAccess: testAccess(),
	})

	first := got.Nodes["master-0"]
	first.Backends[0].Host = "mutated"
	first.AdditionalRoutes[0].Match = "mutated"

	second := got.Nodes["worker-1"]
	assert.Equal(t, "10.0.0.1:5001", second.Backends[0].Host)
	assert.Equal(t, "images.example.com", second.AdditionalRoutes[0].Match)
}

// TestComputeDoesNotMutateItsInputs matters because the inputs come straight out
// of the informer cache, which is shared process-wide.
func TestComputeDoesNotMutateItsInputs(t *testing.T) {
	applied := testUpstream("registry.deckhouse.io")
	config := registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Primary: registryv1alpha1.PrimarySource{Upstream: testUpstream("registry.deckhouse.io")},
		Storage: registryv1alpha1.StorageConfig{Cache: true, Source: testSource()},
	}
	configCopy := *config.DeepCopy()
	appliedCopy := *applied.DeepCopy()

	got := Compute(Inputs{
		Config:          config,
		AppliedUpstream: applied,
		Nodes:           []string{"master-0"},
		StorageAccess:   testAccess(),
	})

	node := got.Nodes["master-0"]
	node.Backends[1].Host = "mutated"
	node.Backends[1].Auth.Username = "mutated"

	assert.Equal(t, configCopy, config)
	assert.Equal(t, appliedCopy, *applied)
}

func TestComputeWithoutNodes(t *testing.T) {
	// A cluster with no nodes yet still needs its storage spec: the object is what
	// the syncer reads once the storage pod comes up.
	got := Compute(Inputs{
		Config: registryv1alpha1.RegistryConfigSpec{
			Mode:    registryv1alpha1.ModeManaged,
			Primary: registryv1alpha1.PrimarySource{Upstream: testUpstream("registry.deckhouse.io")},
			Storage: registryv1alpha1.StorageConfig{Cache: true, Source: testSource()},
		},
		StorageAccess: testAccess(),
	})

	require.NotNil(t, got.Storage)
	assert.Empty(t, got.Nodes)
}

// TestComputeAddressesTheStorageByReplica: the cache is one source of images reachable
// in several places, so the replicas are mirrors of one backend rather than several
// backends. The agent then fails over between them without treating a dead replica as a
// different registry.
func TestComputeAddressesTheStorageByReplica(t *testing.T) {
	desired := Compute(Inputs{
		Config: registryv1alpha1.RegistryConfigSpec{
			Mode:    registryv1alpha1.ModeManaged,
			Primary: registryv1alpha1.PrimarySource{Upstream: testUpstream("registry.deckhouse.io")},
			Storage: registryv1alpha1.StorageConfig{Cache: true, Source: testSource()},
		},
		Nodes:         []string{"worker-1"},
		StorageAccess: testAccess(),
	})

	spec := desired.Nodes["worker-1"]
	storage := spec.Backend(registryv1alpha1.BackendStorage)
	require.NotNil(t, storage)
	assert.Equal(t, "10.0.0.1:5001", storage.Host)
	require.Len(t, storage.Mirrors, 1)
	assert.Equal(t, "10.0.0.2:5001", storage.Mirrors[0].Host)
	// Every replica needs the same authority and credentials, since any of them may be
	// the one that answers.
	assert.Equal(t, storage.CA, storage.Mirrors[0].CA)
	assert.Equal(t, storage.Auth, storage.Mirrors[0].Auth)
	// And the path is the module's own prefix on every replica, because they all hold
	// the same content under it.
	assert.Equal(t, constant.Path, storage.Mirrors[0].Path)
}

// TestComputeWithoutAStorageAddressFallsBackToTheUpstream is a guard, not a feature. The
// caller refuses to build a layout from an incomplete access secret, so this is
// unreachable — but the alternative to handling it is indexing an empty slice, and a
// controller that panics stops reconciling everything else too.
func TestComputeWithoutAStorageAddressFallsBackToTheUpstream(t *testing.T) {
	access := testAccess()
	access.Addresses = nil

	desired := Compute(Inputs{
		Config: registryv1alpha1.RegistryConfigSpec{
			Mode:    registryv1alpha1.ModeManaged,
			Primary: registryv1alpha1.PrimarySource{Upstream: testUpstream("registry.deckhouse.io")},
			Storage: registryv1alpha1.StorageConfig{Cache: true, Source: testSource()},
		},
		Nodes:         []string{"worker-1"},
		StorageAccess: access,
	})

	spec := desired.Nodes["worker-1"]
	assert.False(t, spec.Cache, "a cache nobody can reach was laid out anyway")
	assert.Nil(t, spec.Backend(registryv1alpha1.BackendStorage))
	require.NotNil(t, spec.Backend(registryv1alpha1.BackendUpstream),
		"the nodes were left with no source of images at all")
	assert.Nil(t, desired.Storage)
}

// TestComputeCompilesTheCollectionSchedule: resolved once, centrally, so that every replica
// agrees on when its turn comes round. Three replicas reading one instruction behind one lease
// and three replicas each picking their own hour are different arrangements.
func TestComputeCompilesTheCollectionSchedule(t *testing.T) {
	desired := Compute(Inputs{
		Config: registryv1alpha1.RegistryConfigSpec{
			Mode:    registryv1alpha1.ModeManaged,
			Primary: registryv1alpha1.PrimarySource{Upstream: testUpstream("registry.deckhouse.io")},
			Storage: registryv1alpha1.StorageConfig{Cache: true, Source: testSource()},
		},
		Nodes:              []string{"master-0"},
		StorageAccess:      testAccess(),
		MaintenanceWindows: []MaintenanceWindow{{From: "04:00", To: "06:00", Days: []string{"Sun"}}},
	})

	require.NotNil(t, desired.Storage)
	require.NotNil(t, desired.Storage.GarbageCollection)
	assert.True(t, desired.Storage.GarbageCollection.Enabled)
	assert.Equal(t, "0 4 * * 0", desired.Storage.GarbageCollection.Schedule)
}

// TestComputeEnablesCollectionByOmission: a store nothing ever reclaims fills and then stops
// being able to pull, which is not a state to arrive at by leaving a field out.
func TestComputeEnablesCollectionByOmission(t *testing.T) {
	desired := Compute(Inputs{
		Config: registryv1alpha1.RegistryConfigSpec{
			Mode:    registryv1alpha1.ModeManaged,
			Primary: registryv1alpha1.PrimarySource{Upstream: testUpstream("registry.deckhouse.io")},
			Storage: registryv1alpha1.StorageConfig{Cache: true, Source: testSource()},
		},
		Nodes:         []string{"master-0"},
		StorageAccess: testAccess(),
	})

	require.NotNil(t, desired.Storage.GarbageCollection)
	assert.True(t, desired.Storage.GarbageCollection.Enabled)
	assert.Equal(t, DefaultGarbageCollectionSchedule, desired.Storage.GarbageCollection.Schedule)
}

// TestComputeRespectsCollectionTurnedOff is the one way to end up with a store that grows: an
// explicit decision.
func TestComputeRespectsCollectionTurnedOff(t *testing.T) {
	desired := Compute(Inputs{
		Config: registryv1alpha1.RegistryConfigSpec{
			Mode:    registryv1alpha1.ModeManaged,
			Primary: registryv1alpha1.PrimarySource{Upstream: testUpstream("registry.deckhouse.io")},
			Storage: registryv1alpha1.StorageConfig{
				Cache:             true,
				Source:            testSource(),
				GarbageCollection: &registryv1alpha1.GarbageCollection{Enabled: false},
			},
		},
		Nodes:         []string{"master-0"},
		StorageAccess: testAccess(),
	})

	require.NotNil(t, desired.Storage.GarbageCollection)
	assert.False(t, desired.Storage.GarbageCollection.Enabled)
	// A schedule is still compiled, so that turning it back on does not need one supplied.
	assert.NotEmpty(t, desired.Storage.GarbageCollection.Schedule)
}

// TestComputeLeavesNoCredentialInAnyResource is the property the reference model exists
// for, asserted over the whole result rather than field by field.
//
// The resources below are cluster-scoped and the node agent's role is bound to the
// `system:nodes` group, so one credential left inline in any of them is readable through
// the API by every kubelet in the cluster — including credentials for registries that node
// never pulls from. Walking everything is deliberate: the earlier version of this
// transformation was applied at each return of Compute, and there are four of them.
func TestComputeLeavesNoCredentialInAnyResource(t *testing.T) {
	upstream := testUpstream("registry.deckhouse.io")
	upstream.Mirrors = []registryv1alpha1.Endpoint{{
		Scheme: registryv1alpha1.SchemeHTTPS,
		Host:   "mirror.example.com",
		// Its own credentials, which must not be folded into the primary's key.
		Auth: &registryv1alpha1.Auth{Username: "mirror-user", Password: "mirror-pass"},
	}}

	got := Compute(Inputs{
		Config: registryv1alpha1.RegistryConfigSpec{
			Mode:    registryv1alpha1.ModeManaged,
			Primary: registryv1alpha1.PrimarySource{Upstream: upstream},
			Storage: registryv1alpha1.StorageConfig{Cache: true},
		},
		Nodes:         []string{"master-0", "worker-0"},
		StorageAccess: testAccess(),
		Routes: []registryv1alpha1.Route{{
			Match: "images.example.com",
			Endpoint: registryv1alpha1.Endpoint{
				Host: "images.example.com",
				Auth: &registryv1alpha1.Auth{Username: "route-user", Password: "route-pass"},
			},
		}},
	})

	assertReferenceOnly := func(where string, auth *registryv1alpha1.Auth) {
		if auth == nil {
			return
		}
		assert.Empty(t, auth.Username, where)
		assert.Empty(t, auth.Password, where)
		assert.Empty(t, auth.Auth, where)
		if !auth.SecretRef.IsEmpty() {
			assert.Equal(t, constant.AuthSecretName, auth.SecretRef.Name, where)
			assert.Contains(t, got.Credentials, auth.SecretRef.Key,
				"%s references a key nothing was collected under, so the agent would "+
					"authenticate with nothing", where)
		}
	}

	for node, spec := range got.Nodes {
		for _, backend := range spec.Backends {
			assertReferenceOnly(node+" backend "+string(backend.Name), backend.Endpoint.Auth)
			for i := range backend.Mirrors {
				assertReferenceOnly(node+" mirror", backend.Mirrors[i].Auth)
			}
		}
		for _, route := range spec.AdditionalRoutes {
			assertReferenceOnly(node+" route "+route.Match, route.Endpoint.Auth)
			for i := range route.Mirrors {
				assertReferenceOnly(node+" route mirror", route.Mirrors[i].Auth)
			}
		}
	}

	require.NotNil(t, got.Storage)
	require.NotNil(t, got.Storage.Upstream)
	assertReferenceOnly("storage upstream", got.Storage.Upstream.Endpoint.Auth)
	for i := range got.Storage.Upstream.Mirrors {
		assertReferenceOnly("storage upstream mirror", got.Storage.Upstream.Mirrors[i].Auth)
	}

	// The recorded upstream keeps no reference either: it is there for an operator to
	// read, and nothing resolves it.
	require.NotNil(t, got.HeldUpstream)
	assert.Nil(t, got.HeldUpstream.Endpoint.Auth)

	// Every credential that went in came back out, so nothing was dropped on the way.
	assert.Equal(t, "license-token", got.Credentials[constant.AuthKeyUpstream].Username)
	assert.Equal(t, "ro", got.Credentials[constant.AuthKeyStorage].Username)
	assert.Equal(t, "mirror-user", got.Credentials["upstream-mirror-0"].Username)
	assert.Equal(t, "route-user", got.Credentials["route-images.example.com"].Username)
}

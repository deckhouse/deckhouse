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
	"context"
	"io"
	"log"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/registry-syncer/internal/distribution"
	"github.com/deckhouse/registry-syncer/internal/fill"
	"github.com/deckhouse/registry-syncer/internal/report"
)

type fixedLeadership bool

func (f fixedLeadership) IsLeader() bool { return bool(f) }

type noopRestarter struct{ restarts int }

func (n *noopRestarter) Restart() error {
	n.restarts++
	return nil
}

// startRegistry runs a real registry in memory, so the fill and the catalogue read
// are exercised against actual registry behaviour.
func startRegistry(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
	t.Cleanup(server.Close)

	parsed, err := url.Parse(server.URL)
	require.NoError(t, err)
	return parsed.Host
}

func pushImage(t *testing.T, address, reference string) {
	t.Helper()

	image, err := random.Image(128, 1)
	require.NoError(t, err)

	tag, err := name.NewTag(address+"/"+reference, name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(tag, image))
}

func newLoop(
	t *testing.T, isLeader bool, storage *registryv1alpha1.RegistryStorage,
) (*Loop, client.Client, *noopRestarter) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))

	builder := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&registryv1alpha1.RegistryStorage{})
	if storage != nil {
		builder = builder.WithObjects(storage)
	}
	fakeClient := builder.Build()

	dir := t.TempDir()
	restarter := &noopRestarter{}

	loop := &Loop{
		Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Client: fakeClient,
		Node:   "master-0",
		Applier: &distribution.Applier{
			ConfigPath:     filepath.Join(dir, "config.yaml"),
			UpstreamCAPath: filepath.Join(dir, "upstream-ca.crt"),
			Restarter:      restarter,
			Options: distribution.Options{
				ListenAddress: "10.0.0.1",
				HTTPSecret:    "secret",
				AuthRealm:     "https://10.0.0.1:5051/auth",
				TokenIssuer:   "Registry server",
			},
		},
		Publisher:  &report.Publisher{Client: fakeClient},
		Leadership: fixedLeadership(isLeader),
	}
	return loop, fakeClient, restarter
}

func storageWith(spec registryv1alpha1.RegistryStorageSpec) *registryv1alpha1.RegistryStorage {
	return &registryv1alpha1.RegistryStorage{
		ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName},
		Spec:       spec,
	}
}

func replicaOf(t *testing.T, c client.Client, node string) registryv1alpha1.StorageReplicaStatus {
	t.Helper()

	storage := &registryv1alpha1.RegistryStorage{}
	require.NoError(t, c.Get(context.Background(),
		types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage))

	for _, replica := range storage.Status.Replicas {
		if replica.Node == node {
			return replica
		}
	}
	t.Fatalf("no report from %s", node)
	return registryv1alpha1.StorageReplicaStatus{}
}

// TestOnceFillsAndReportsFull walks a leader through a complete fill, which is what
// eventually authorizes the controller to go air-gap.
func TestOnceFillsAndReportsFull(t *testing.T) {
	upstream := startRegistry(t)
	local := startRegistry(t)

	pushImage(t, upstream, "deckhouse/ee/one:v1")
	pushImage(t, upstream, "deckhouse/ee/two:v1")

	loop, c, restarter := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP, Host: upstream, Path: "/deckhouse/ee",
			},
		},
		Source:   &registryv1alpha1.StorageSource{ExpectedDigests: 2},
		NeedSync: true,
	}))
	loop.LocalAddress = local

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.Equal(t, registryv1alpha1.ReplicaRoleLeader, replica.Role)
	assert.True(t, replica.Full)
	assert.EqualValues(t, 2, replica.VerifiedDigests)
	assert.Empty(t, replica.Error)

	// The configuration is applied on every pass, regardless of role or of what the
	// fill did.
	assert.Equal(t, 1, restarter.restarts)
	config, err := os.ReadFile(loop.Applier.ConfigPath)
	require.NoError(t, err)
	assert.Contains(t, string(config), upstream)
}

// TestOnceReportsAPartialFill checks a fill that did not reach the expectation is
// not reported as complete, since that is what gates cutting the cluster off its
// upstream.
func TestOnceReportsAPartialFill(t *testing.T) {
	upstream := startRegistry(t)
	local := startRegistry(t)
	pushImage(t, upstream, "deckhouse/ee/one:v1")

	loop, c, _ := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP, Host: upstream, Path: "/deckhouse/ee",
			},
		},
		Source:   &registryv1alpha1.StorageSource{ExpectedDigests: 459},
		NeedSync: true,
	}))
	loop.LocalAddress = local

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.False(t, replica.Full)
	assert.EqualValues(t, 1, replica.VerifiedDigests)
}

// TestOnceAirGapCountsWhatIsHeld covers content that arrived through the write
// endpoint: the syncer never copied it, so the only honest accounting is to read
// what the storage holds.
func TestOnceAirGapCountsWhatIsHeld(t *testing.T) {
	local := startRegistry(t)
	for _, reference := range []string{"one:v1", "two:v1", "three:v1"} {
		pushImage(t, local, "system/deckhouse/"+reference)
	}

	loop, c, _ := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Source: &registryv1alpha1.StorageSource{ExpectedDigests: 3},
	}))
	loop.LocalAddress = local

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.True(t, replica.Full)
	assert.EqualValues(t, 3, replica.VerifiedDigests)

	// And the configuration has no pull-through section, so the cache cannot reach out.
	//
	// Checked on the parsed document: `auth.token.proxy` is a different thing with the
	// same name — how the registry fetches a token on a client's behalf — and it must
	// stay.
	raw, err := os.ReadFile(loop.Applier.ConfigPath)
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &config))
	assert.NotContains(t, config, "proxy")
}

// TestOnceFollowerDoesNotTouchTheUpstream is what keeps every replica from spending
// the upstream credentials at once.
func TestOnceFollowerDoesNotTouchTheUpstream(t *testing.T) {
	local := startRegistry(t)
	pushImage(t, local, "system/deckhouse/one:v1")

	loop, c, _ := newLoop(t, false, storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				// Unreachable on purpose: a follower that touched it would fail here.
				Scheme: registryv1alpha1.SchemeHTTP, Host: "upstream.invalid", Path: "/deckhouse/ee",
			},
		},
		Source:   &registryv1alpha1.StorageSource{ExpectedDigests: 1},
		NeedSync: true,
	}))
	loop.LocalAddress = local

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.Equal(t, registryv1alpha1.ReplicaRoleFollower, replica.Role)
	assert.Empty(t, replica.Error)
	assert.EqualValues(t, 1, replica.VerifiedDigests)
}

// TestOnceReportsAFailedFill matters because the controller distinguishes "still
// filling" from "broken", and only the latter is actionable.
func TestOnceReportsAFailedFill(t *testing.T) {
	local := startRegistry(t)

	loop, c, _ := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP, Host: "upstream.invalid", Path: "/deckhouse/ee",
			},
		},
		Source:   &registryv1alpha1.StorageSource{ExpectedDigests: 459},
		NeedSync: true,
	}))
	loop.LocalAddress = local

	// The pass itself succeeds: a broken upstream is reported, not raised. The
	// replica keeps serving what it holds.
	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.False(t, replica.Full)
	assert.NotEmpty(t, replica.Error)
}

// TestOnceIdleKeepsThePreviousCount guards against an idle pass looking like the
// storage emptied, which would revoke an already granted air-gap gate.
func TestOnceIdleKeepsThePreviousCount(t *testing.T) {
	local := startRegistry(t)

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{Scheme: registryv1alpha1.SchemeHTTP, Host: local},
		},
		Source: &registryv1alpha1.StorageSource{ExpectedDigests: 459},
		// The controller has not asked for a fill.
		NeedSync: false,
	})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 459,
	}}

	loop, c, _ := newLoop(t, true, storage)
	loop.LocalAddress = local

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.EqualValues(t, 459, replica.VerifiedDigests, "an idle pass must not zero the count")
	assert.True(t, replica.Full)
}

// TestOnceAppliesACredentialChange is the story the whole component exists for: the
// upstream credentials change in the custom resource, and the serving process picks
// them up without the Deckhouse operator being involved.
func TestOnceAppliesACredentialChange(t *testing.T) {
	local := startRegistry(t)

	loop, c, restarter := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP, Host: local, Path: "/deckhouse/ee",
				Auth: &registryv1alpha1.Auth{Username: "license-token", Password: "the-expired-key"},
			},
		},
		Source: &registryv1alpha1.StorageSource{ExpectedDigests: 1},
	}))
	loop.LocalAddress = local
	ctx := context.Background()

	require.NoError(t, loop.once(ctx))
	require.Equal(t, 1, restarter.restarts)

	live := &registryv1alpha1.RegistryStorage{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, live))
	live.Spec.Upstream.Auth.Password = "the-renewed-key"
	require.NoError(t, c.Update(ctx, live))

	require.NoError(t, loop.once(ctx))

	assert.Equal(t, 2, restarter.restarts)
	config, err := os.ReadFile(loop.Applier.ConfigPath)
	require.NoError(t, err)
	assert.Contains(t, string(config), "the-renewed-key")
}

// TestOnceWithoutAStorageObject covers the cache being turned off: nothing to
// configure and nothing to report, and certainly nothing to crash over.
func TestOnceWithoutAStorageObject(t *testing.T) {
	loop, _, restarter := newLoop(t, true, nil)
	loop.LocalAddress = startRegistry(t)

	require.NoError(t, loop.once(context.Background()))
	assert.Equal(t, 0, restarter.restarts)
}

func TestOnceIsIdempotent(t *testing.T) {
	local := startRegistry(t)
	pushImage(t, local, "system/deckhouse/one:v1")

	loop, c, restarter := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Source: &registryv1alpha1.StorageSource{ExpectedDigests: 1},
	}))
	loop.LocalAddress = local
	ctx := context.Background()

	require.NoError(t, loop.once(ctx))

	storage := &registryv1alpha1.RegistryStorage{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage))
	version := storage.ResourceVersion

	require.NoError(t, loop.once(ctx))

	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage))
	assert.Equal(t, version, storage.ResourceVersion, "an unchanged report must not be rewritten")
	assert.Equal(t, 1, restarter.restarts, "an unchanged configuration must not restart the registry")
}

func TestTrimSlashes(t *testing.T) {
	assert.Equal(t, "deckhouse/ee", trimSlashes("/deckhouse/ee"))
	assert.Equal(t, "deckhouse/ee", trimSlashes("deckhouse/ee/"))
	assert.Equal(t, "deckhouse/ee", trimSlashes("//deckhouse/ee//"))
	assert.Equal(t, "", trimSlashes("/"))
	assert.Equal(t, "system/deckhouse", trimSlashes(constant.Path))
}

// TestOnceReplicatesFromTheLeader is the "self-healing" property: a follower is
// filled ahead of time, so whichever replica takes over already holds the set
// instead of refilling from the upstream — which in an air-gapped cluster it could
// not do at all.
func TestOnceReplicatesFromTheLeader(t *testing.T) {
	leaderStorage := startRegistry(t)
	followerStorage := startRegistry(t)

	for _, reference := range []string{"one:v1", "two:v1", "three:v1"} {
		pushImage(t, leaderStorage, "system/deckhouse/"+reference)
	}

	// Air-gapped on purpose: there is no upstream for the follower to fall back on,
	// so replication is the only way the content can reach it.
	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Source: &registryv1alpha1.StorageSource{ExpectedDigests: 3},
	})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader,
		Full: true, VerifiedDigests: 3, Address: leaderStorage,
	}}

	loop, c, _ := newLoop(t, false, storage)
	loop.LocalAddress = followerStorage
	loop.ReportedAddress = followerStorage

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.Equal(t, registryv1alpha1.ReplicaRoleFollower, replica.Role)
	assert.Equal(t, "master-1", replica.Source, "the report has to say where it was filled from")
	assert.True(t, replica.Full)
	assert.EqualValues(t, 3, replica.VerifiedDigests)
	assert.Empty(t, replica.Error)

	// And the content really is there, so a leader change costs nothing.
	assert.EqualValues(t, 3, mustCount(t, followerStorage))
}

// TestOnceDoesNotReplicateFromAnIncompleteLeader keeps a follower from copying a
// partial set that would only have to be redone.
func TestOnceDoesNotReplicateFromAnIncompleteLeader(t *testing.T) {
	leaderStorage := startRegistry(t)
	followerStorage := startRegistry(t)
	pushImage(t, leaderStorage, "system/deckhouse/one:v1")

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Source: &registryv1alpha1.StorageSource{ExpectedDigests: 459},
	})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader,
		Full: false, VerifiedDigests: 1, Address: leaderStorage,
	}}

	loop, c, _ := newLoop(t, false, storage)
	loop.LocalAddress = followerStorage

	require.NoError(t, loop.once(context.Background()))

	assert.EqualValues(t, 0, mustCount(t, followerStorage), "nothing may be copied from a partial leader")
	assert.Empty(t, replicaOf(t, c, "master-0").Source)
}

// TestOnceIgnoresALeaderClaimingFullWithAnError covers a leader that both claims
// completeness and reports a failure: not trustworthy enough to copy a whole set
// from.
func TestOnceIgnoresALeaderClaimingFullWithAnError(t *testing.T) {
	leaderStorage := startRegistry(t)
	followerStorage := startRegistry(t)
	pushImage(t, leaderStorage, "system/deckhouse/one:v1")

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Source: &registryv1alpha1.StorageSource{ExpectedDigests: 1},
	})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader,
		Full: true, VerifiedDigests: 1, Address: leaderStorage,
		Error: "verification failed",
	}}

	loop, _, _ := newLoop(t, false, storage)
	loop.LocalAddress = followerStorage

	require.NoError(t, loop.once(context.Background()))
	assert.EqualValues(t, 0, mustCount(t, followerStorage))
}

// TestOnceLeaderDoesNotReplicateFromItself guards against the leader finding its own
// entry and copying from itself, which would be an endless no-op at best.
func TestOnceLeaderDoesNotReplicateFromItself(t *testing.T) {
	local := startRegistry(t)
	pushImage(t, local, "system/deckhouse/one:v1")

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Source: &registryv1alpha1.StorageSource{ExpectedDigests: 1},
	})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader,
		Full: true, VerifiedDigests: 1, Address: local,
	}}

	loop, c, _ := newLoop(t, true, storage)
	loop.LocalAddress = local

	require.NoError(t, loop.once(context.Background()))
	assert.Empty(t, replicaOf(t, c, "master-0").Source)
}

func TestOnceReportsItsOwnAddress(t *testing.T) {
	local := startRegistry(t)

	loop, c, _ := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Source: &registryv1alpha1.StorageSource{ExpectedDigests: 1},
	}))
	loop.LocalAddress = local
	loop.ReportedAddress = "10.0.0.1:5001"

	require.NoError(t, loop.once(context.Background()))

	// Reported by the replica itself, so a follower can reach the leader without
	// resolving a node name and without reading Node objects.
	assert.Equal(t, "10.0.0.1:5001", replicaOf(t, c, "master-0").Address)
}

func mustCount(t *testing.T, address string) int32 {
	t.Helper()

	options, err := fill.RegistryOptions("", "", "")
	require.NoError(t, err)

	count, err := fill.CountCatalogue(context.Background(), fill.Registry{
		Address: address, Repository: "system/deckhouse", Insecure: true, Options: options,
	})
	require.NoError(t, err)
	return count
}

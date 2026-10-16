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
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/registry-syncer/internal/distribution"
	"github.com/deckhouse/registry-syncer/internal/fill"
	"github.com/deckhouse/registry-syncer/internal/gc"
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

// pushDigest puts an image in with no tag, the way every image of an embedded module lives
// in the registry: the release names them by digest and nothing else does.
func pushDigest(t *testing.T, address, repository string) string {
	t.Helper()

	image, err := random.Image(128, 1)
	require.NoError(t, err)
	digest, err := image.Digest()
	require.NoError(t, err)

	reference, err := name.NewDigest(
		fmt.Sprintf("%s/%s@%s", address, repository, digest.String()), name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(reference, image))

	return digest.String()
}

// pushInstaller puts in the image a release declares its own image set in — where the fill
// now reads that set from, instead of listing the upstream.
func pushInstaller(t *testing.T, address, reference string, digests []string) {
	t.Helper()

	byImage := make(map[string]string, len(digests))
	for i, digest := range digests {
		byImage[fmt.Sprintf("image%d", i)] = digest
	}
	encoded, err := json.Marshal(map[string]map[string]string{"registry": byImage})
	require.NoError(t, err)

	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	require.NoError(t, writer.WriteHeader(&tar.Header{
		Name: "deckhouse/candi/images_digests.json", Mode: 0o644, Size: int64(len(encoded)),
	}))
	_, err = writer.Write(encoded)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	layer, err := tarball.LayerFromReader(bytes.NewReader(archive.Bytes()))
	require.NoError(t, err)
	image, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	tag, err := name.NewTag(address+"/"+reference, name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(tag, image))
}

// deployedRelease is what the cluster says it is running. The fill enumerates from it, so
// without one there is nothing to fill towards — which is a refusal, not an empty set.
func deployedRelease(version string) *unstructured.Unstructured {
	release := &unstructured.Unstructured{Object: map[string]any{
		"spec":   map[string]any{"version": version},
		"status": map[string]any{"phase": "Deployed"},
	}}
	release.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "deckhouse.io", Version: "v1alpha1", Kind: "DeckhouseRelease",
	})
	release.SetName(version)
	return release
}

func newLoop(
	t *testing.T, isLeader bool, storage *registryv1alpha1.RegistryStorage,
	extra ...client.Object,
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
	if len(extra) > 0 {
		builder = builder.WithObjects(extra...)
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
				StoreCapacity: 50 << 30,
			},
		},
		Publisher:  &report.Publisher{Client: fakeClient},
		Leadership: fixedLeadership(isLeader),
		// A test registry is up before the pass starts, and one that is not is meant to stay down.
		RegistryWait: 200 * time.Millisecond,
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

	// The image set the release declares, addressed by digest under the repository root,
	// which is how the platform addresses its own images.
	one := pushDigest(t, upstream, "deckhouse/ee")
	two := pushDigest(t, upstream, "deckhouse/ee")
	pushImage(t, upstream, "deckhouse/ee:v1.70.1")
	pushInstaller(t, upstream, "deckhouse/ee/install:v1.70.1", []string{one, two})

	// And something the release says nothing about, which must not be copied: the fill
	// enumerates the release, not the registry.
	pushImage(t, upstream, "deckhouse/ee/unrelated:v1")

	loop, c, restarter := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP, Host: upstream, Path: "/deckhouse/ee",
			},
		},
		// Two declared digests, plus the release and its installer.
		Source:   &registryv1alpha1.StorageSource{ExpectedDigests: 4},
		NeedSync: true,
	}), deployedRelease("v1.70.1"))
	loop.LocalAddress = local
	loop.WriteAddress = local

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.Equal(t, registryv1alpha1.ReplicaRoleLeader, replica.Role)
	assert.True(t, replica.Full)
	assert.EqualValues(t, 3, replica.VerifiedDigests)
	assert.Empty(t, replica.Error)

	// The configuration is applied on every pass, regardless of role or of what the
	// fill did.
	assert.Equal(t, 1, restarter.restarts)
	// The upstream is in the registry image's own half of the configuration, written by the same
	// pass: the registry's own file is upstream's vocabulary and says nothing about where a miss
	// goes.
	config, err := os.ReadFile(loop.Applier.WrapperPath())
	require.NoError(t, err)
	assert.Contains(t, string(config), upstream)
}

// TestOnceReportsAPartialFill checks a fill that did not put the whole set in place is not reported as
// complete, since that is what gates cutting the cluster off its upstream.
//
// The set is short by an image the release declares and the upstream does not hold, which is what a
// partial fill actually looks like. It used to be expressed instead by stating an expectation the fill
// could not reach — and that mechanism had to go: an operator's number counts the manifests in a bundle,
// while a fill counts what the releases and modules declare, so comparing the two made completeness
// unreachable on an air-gapped cluster that held everything it needed. What still guarantees this case
// is the fill's own account of what it failed to copy.
func TestOnceReportsAPartialFill(t *testing.T) {
	upstream := startRegistry(t)
	local := startRegistry(t)

	one := pushDigest(t, upstream, "deckhouse/ee")
	pushImage(t, upstream, "deckhouse/ee:v1.70.1")
	// Declared by the release, absent from the upstream: nothing can copy it.
	missing := "sha256:" + strings.Repeat("a", 64)
	pushInstaller(t, upstream, "deckhouse/ee/install:v1.70.1", []string{one, missing})

	loop, c, _ := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP, Host: upstream, Path: "/deckhouse/ee",
			},
		},
		NeedSync: true,
	}), deployedRelease("v1.70.1"))
	loop.LocalAddress = local
	loop.WriteAddress = local

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.False(t, replica.Full, "a set with an image missing is not a complete set")
	assert.EqualValues(t, 2, replica.VerifiedDigests,
		"two of the three references the release declares, which is what short means")

	// Not reported as an error, and that is deliberate: an image the source does not hold yet is
	// pending rather than failed, so a follower copying from a leader that is still filling does not
	// look broken — and, through the eligibility rules, is not disqualified from leading.
	assert.Empty(t, replica.Error)
}

// TestOnceAirGapCountsWhatIsHeld covers content that arrived through the write
// endpoint: the syncer never copied it, so the only honest accounting is to read
// what the storage holds.
func TestOnceAirGapCountsWhatIsHeld(t *testing.T) {
	local := startRegistry(t)

	// What `d8 mirror push` left in the store: the release's own images, addressed by digest, plus the
	// release image and the installer that declares the set. The accounting reads the disk — asking the
	// registry would be unsound wherever a pass-through cache is configured — but it counts only what
	// the release declares, so that this number means the same thing as the one a copy reports.
	one := pushDigest(t, local, "system/deckhouse")
	two := pushDigest(t, local, "system/deckhouse")
	pushImage(t, local, "system/deckhouse:v1.70.1")
	pushInstaller(t, local, "system/deckhouse/install:v1.70.1", []string{one, two})

	loop, c, _ := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Source: &registryv1alpha1.StorageSource{ExpectedDigests: 4},
	}), deployedRelease("v1.70.1"))
	loop.LocalAddress = local
	loop.WriteAddress = local
	loop.DataDir = t.TempDir()
	release := resolveTag(t, local, "system/deckhouse", "v1.70.1")
	heldOnDisk(t, loop.DataDir, one, two, release,
		resolveTag(t, local, "system/deckhouse/install", "v1.70.1"))
	// And the tag, without which the release cannot be resolved from this store and no follower can
	// enumerate the set from it.
	tagOnDisk(t, loop.DataDir, "v1.70.1", release)

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
	// The section survives, carrying `skipmodecleanup` — without it the registry wipes the store on a
	// start in another mode, which is exactly what an air-gapped cluster cannot afford. The address is
	// what has to be gone.
	proxy, _ := config["proxy"].(map[string]any)
	assert.NotContains(t, proxy, "remoteurl")
}

// TestOnceFollowerDoesNotTouchTheUpstream is what keeps every replica from spending
// the upstream credentials at once.
func TestOnceFollowerDoesNotTouchTheUpstream(t *testing.T) {
	local := startRegistry(t)

	// The set this replica is judged against, in its own store: the accounting counts declared
	// digests present, so a release to enumerate has to exist even for a replica that copies nothing.
	one := pushDigest(t, local, "system/deckhouse")
	pushImage(t, local, "system/deckhouse:v1.70.1")
	pushInstaller(t, local, "system/deckhouse/install:v1.70.1", []string{one})

	loop, c, _ := newLoop(t, false, storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				// Unreachable on purpose: a follower that touched it would fail here.
				Scheme: registryv1alpha1.SchemeHTTP, Host: "upstream.invalid", Path: "/deckhouse/ee",
			},
		},
		Source:   &registryv1alpha1.StorageSource{ExpectedDigests: 1},
		NeedSync: true,
	}), deployedRelease("v1.70.1"))
	loop.LocalAddress = local
	loop.WriteAddress = local
	loop.DataDir = t.TempDir()
	heldOnDisk(t, loop.DataDir, one)

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
	loop.WriteAddress = local

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
	loop.WriteAddress = local

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.EqualValues(t, 459, replica.VerifiedDigests, "an idle pass must not zero the count")
	assert.True(t, replica.Full)
}

// TestOnceIdleKeepsTheDenominatorToo is the same guard for the other half of the fraction, and it is
// here because the first version of the denominator did not have it.
//
// A restarted process whose first pass does not read the store publishes the counts it carried over
// from the object, while the denominator — carried nowhere — comes out absent, so a complete store
// reports no progress at all. Six places set the numerator; the denominator has to travel with it in
// all of them, and a fresh process starts with nothing in memory, so the object is the only source.
func TestOnceIdleKeepsTheDenominatorToo(t *testing.T) {
	local := startRegistry(t)

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{Scheme: registryv1alpha1.SchemeHTTP, Host: local},
		},
		NeedSync: false,
	})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true,
		VerifiedDigests: 405, DeclaredDigests: 405, TotalDigests: 517,
	}}

	// A loop with nothing remembered, which is what a restarted syncer is.
	loop, c, _ := newLoop(t, true, storage)
	loop.LocalAddress = local
	loop.WriteAddress = local

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.EqualValues(t, 405, replica.VerifiedDigests)
	assert.EqualValues(t, 405, replica.DeclaredDigests,
		"a pass that did not count the set must keep the size it was told, or the status loses its denominator")
}

// TestTheDenominatorAppearsOnACachingClusterWithNothingToFill is the configuration the first two
// versions of this fix both missed, and the one most clusters are in.
//
// With an upstream, a cache and no air-gap declaration, a steady-state pass computes nobody's set: the
// fill path knows only what it wrote, the survey that keeps the store's size up to date is asked
// without a set deliberately, and a freshly started process has nothing in memory to fall back on.
// `totalDigests` then climbs pass after pass with `declaredDigests` absent, so the status shows no
// `fill` at all — the very thing this was supposed to fix, surviving two fixes.
func TestTheDenominatorAppearsOnACachingClusterWithNothingToFill(t *testing.T) {
	local := startRegistry(t)

	one := pushDigest(t, local, "system/deckhouse")
	two := pushDigest(t, local, "system/deckhouse")
	pushImage(t, local, "system/deckhouse:v1.70.1")
	pushInstaller(t, local, "system/deckhouse/install:v1.70.1", []string{one, two})

	loop, c, _ := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP,
				Host:   local,
				Path:   "/system/deckhouse",
			},
		},
		// Nothing asked of it: no fill, no air-gap declaration. The ordinary caching cluster.
		NeedSync: false,
	}), deployedRelease("v1.70.1"))
	loop.LocalAddress = local
	loop.WriteAddress = local
	loop.DataDir = t.TempDir()

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.NotZero(t, replica.DeclaredDigests,
		"a cluster with an upstream must still learn how big the set is, or its status has no denominator")
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
	loop.WriteAddress = local
	ctx := context.Background()

	require.NoError(t, loop.once(ctx))
	require.Equal(t, 1, restarter.restarts)

	live := &registryv1alpha1.RegistryStorage{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, live))
	live.Spec.Upstream.Auth.Password = "the-renewed-key"
	require.NoError(t, c.Update(ctx, live))

	require.NoError(t, loop.once(ctx))

	assert.Equal(t, 2, restarter.restarts)
	config, err := os.ReadFile(loop.Applier.WrapperPath())
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
	loop.WriteAddress = local
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

	// What the leader holds, in the shape the release declares it: images addressed by digest, plus
	// the installer that names them. The follower enumerates the release rather than listing the
	// leader — listing would return the leader's UPSTREAM contents while a pull-through is
	// configured, and copying those is neither the set nor a bounded amount of work.
	one := pushDigest(t, leaderStorage, "system/deckhouse")
	two := pushDigest(t, leaderStorage, "system/deckhouse")
	pushImage(t, leaderStorage, "system/deckhouse:v1.76.6")
	pushInstaller(t, leaderStorage, "system/deckhouse/install:v1.76.6", []string{one, two})

	// Air-gapped on purpose: there is no upstream for the follower to fall back on,
	// so replication is the only way the content can reach it.
	// Two declared digests, plus the release and its installer — the same four things the leader's
	// own fill puts in, which is what makes a takeover free.
	// NeedSync, because this follower holds nothing yet: the controller raises it while any replica
	// is short of what the leader holds.
	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Source:   &registryv1alpha1.StorageSource{ExpectedDigests: 4},
		NeedSync: true,
	})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader,
		Full: true, VerifiedDigests: 4, Address: leaderStorage,
	}}

	loop, c, _ := newLoop(t, false, storage, deployedRelease("v1.76.6"))
	loop.LocalAddress = followerStorage
	loop.WriteAddress = followerStorage
	loop.ReportedAddress = followerStorage

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.Equal(t, registryv1alpha1.ReplicaRoleFollower, replica.Role)
	assert.Equal(t, "master-1", replica.Source, "the report has to say where it was filled from")
	assert.True(t, replica.Full)
	assert.EqualValues(t, 3, replica.VerifiedDigests)
	assert.Empty(t, replica.Error)

	// And the content really is there, so a leader change costs nothing.
	assert.True(t, holdsDigest(t, followerStorage, "system/deckhouse", one))
	assert.True(t, holdsDigest(t, followerStorage, "system/deckhouse", two))
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
	loop.WriteAddress = followerStorage

	require.NoError(t, loop.once(context.Background()))

	assert.False(t, holdsAnything(t, followerStorage), "nothing may be copied from a partial leader")
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
	loop.WriteAddress = followerStorage

	require.NoError(t, loop.once(context.Background()))
	assert.False(t, holdsAnything(t, followerStorage))
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
	loop.WriteAddress = local

	require.NoError(t, loop.once(context.Background()))
	assert.Empty(t, replicaOf(t, c, "master-0").Source)
}

func TestOnceReportsItsOwnAddress(t *testing.T) {
	local := startRegistry(t)

	loop, c, _ := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Source: &registryv1alpha1.StorageSource{ExpectedDigests: 1},
	}))
	loop.LocalAddress = local
	loop.WriteAddress = local
	loop.ReportedAddress = "10.0.0.1:5001"

	require.NoError(t, loop.once(context.Background()))

	// Reported by the replica itself, so a follower can reach the leader without
	// resolving a node name and without reading Node objects.
	assert.Equal(t, "10.0.0.1:5001", replicaOf(t, c, "master-0").Address)
}

// holdsAnything reports whether a test registry received any repository at all, which is how "the
// follower copied nothing" is asserted without naming what it would have copied.
func holdsAnything(t *testing.T, address string) bool {
	t.Helper()

	parsed, err := name.NewRegistry(address, name.Insecure)
	require.NoError(t, err)

	repositories, err := remote.Catalog(context.Background(), parsed)
	require.NoError(t, err)
	return len(repositories) > 0
}

// holdsDigest asks a test registry whether one manifest reached it.
//
// A direct question about a named digest, and not a count of a listing: the images the platform
// replicates carry no tags, and a listing of a registry is in any case the question the product
// deliberately stopped asking — see fill.CountHeld.
func holdsDigest(t *testing.T, address, repository, digest string) bool {
	t.Helper()

	reference, err := name.NewDigest(
		fmt.Sprintf("%s/%s@%s", address, repository, digest), name.Insecure)
	require.NoError(t, err)

	_, err = remote.Head(reference)
	return err == nil
}

// resolveTag asks a registry what a tag points at, so a test can write the matching revision link on
// disk — the accounting counts declared digests present, and a tag is declared by what it resolves to.
func resolveTag(t *testing.T, address, repository, tag string) string {
	t.Helper()

	reference, err := name.NewTag(fmt.Sprintf("%s/%s:%s", address, repository, tag), name.Insecure)
	require.NoError(t, err)

	descriptor, err := remote.Head(reference)
	require.NoError(t, err)
	return descriptor.Digest.String()
}

// heldOnDisk puts images in the store the way a finished `d8 mirror push` leaves them: the revision
// link, the manifest itself, and every blob the manifest names. The layout is the product's contract
// with its own filesystem, and writing it out here is what makes the count testable without a
// registry.
//
// All three, because held means servable. A revision link alone is what a pull-through cache writes
// the moment it has SERVED a manifest, before fetching anything it points at — so a store built out
// of links only can answer questions about images it cannot hand over, call itself full, and
// authorize an air-gap in which no node can pull at all.
func heldOnDisk(t *testing.T, root string, digests ...string) {
	t.Helper()

	// Shared between the images here exactly as they are between the images of one release.
	const (
		config = "sha256:c0f11111111111111111111111111111111111111111111111111111111111ff"
		layer  = "sha256:1a4e2222222222222222222222222222222222222222222222222222222222ff"
	)
	blobOnDisk(t, root, config, []byte(`{}`))
	blobOnDisk(t, root, layer, []byte("layer"))

	for i, digest := range digests {
		algorithm, hex, found := strings.Cut(digest, ":")
		require.True(t, found)

		dir := filepath.Join(root, "docker", "registry", "v2", "repositories",
			"system", "deckhouse", fmt.Sprintf("image-%d", i),
			"_manifests", "revisions", algorithm, hex)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "link"), []byte(digest), 0o644))

		blobOnDisk(t, root, digest, []byte(
			`{"mediaType":"application/vnd.oci.image.manifest.v1+json",`+
				`"config":{"digest":"`+config+`"},"layers":[{"digest":"`+layer+`"}]}`))
	}
}

// blobOnDisk writes a blob's content where the registry keeps it.
func blobOnDisk(t *testing.T, root, digest string, content []byte) {
	t.Helper()

	algorithm, hex, found := strings.Cut(digest, ":")
	require.True(t, found)

	dir := filepath.Join(root, "docker", "registry", "v2", "blobs", algorithm, hex[:2], hex)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data"), content, 0o644))
}

// TestAFullSetThatCannotResolveTheReleaseIsNotComplete is the check the count alone did not make.
//
// A store can hold the whole declared set by digest and still be unable to resolve the release by tag,
// and it then authorizes a transition it cannot honour: a follower asking it for the release gets
// `MANIFEST_UNKNOWN`, the node agent gets `NAME_UNKNOWN: repository name not known to registry`, and
// replication — which enumerates the set by reading the release BY TAG — propagates nothing, an update
// included.
//
// The check does not depend on how a store came to be in that state: either way it cannot resolve the
// release, and either way it must not authorize dropping the upstream.
func TestAFullSetThatCannotResolveTheReleaseIsNotComplete(t *testing.T) {
	local := startRegistry(t)

	one := pushDigest(t, local, "system/deckhouse")
	two := pushDigest(t, local, "system/deckhouse")
	pushImage(t, local, "system/deckhouse:v1.70.1")
	pushInstaller(t, local, "system/deckhouse/install:v1.70.1", []string{one, two})

	loop, c, _ := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Publish:         true,
		AirGapRequested: true,
	}), deployedRelease("v1.70.1"))
	loop.LocalAddress = local
	loop.WriteAddress = local
	loop.DataDir = t.TempDir()

	// Every manifest the release declares, and no tag link. The count is satisfied; the store is not
	// usable as a source.
	heldOnDisk(t, loop.DataDir, one, two, resolveTag(t, local, "system/deckhouse", "v1.70.1"))

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.EqualValues(t, 3, replica.VerifiedDigests,
		"the count is honest: the manifests really are there")
	assert.False(t, replica.Full,
		"but a store the release cannot be resolved from must not authorize dropping the upstream")

	// Not an error of the pass, either: nothing failed, the store is simply not ready to be a source.
	// Reported as an error it would disqualify this replica from leading, which is not the intent.
	assert.Empty(t, replica.Error)
}

// TestTotalDigestsSurvivesAPassThatDoesNotReadTheStore pins the field against both of its failure modes.
//
// It was introduced because a status that omits it reads as an emptied store. First it was answered by
// walking the store on every pass, which cost 72% of a CPU continuously on an air-gapped master — six
// walks a minute for a number that changes when somebody pushes. Then it was moved to the one path that
// already reads the store, and promptly vanished everywhere else: `verified=396` beside `total: null` on
// a caching cluster.
//
// So: filled on a fill pass, which does not read the store itself, and filled from a survey that is
// taken at most once a minute.
func TestTotalDigestsSurvivesAPassThatDoesNotReadTheStore(t *testing.T) {
	local := startRegistry(t)

	one := pushDigest(t, local, "system/deckhouse")
	two := pushDigest(t, local, "system/deckhouse")
	pushImage(t, local, "system/deckhouse:v1.70.1")
	pushInstaller(t, local, "system/deckhouse/install:v1.70.1", []string{one, two})

	loop, c, _ := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTPS,
				Host:   local,
				Path:   "/system/deckhouse",
			},
		},
		NeedSync: true,
	}), deployedRelease("v1.70.1"))
	loop.LocalAddress = local
	loop.WriteAddress = local
	loop.DataDir = t.TempDir()

	// Something on the disk for the survey to find, put there by no pass of this loop.
	heldOnDisk(t, loop.DataDir, one, two)

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.NotZero(t, replica.TotalDigests,
		"a pass that fills rather than reads the store must still report how much the store holds")

	// And the denominator with it. On a cluster with an upstream there is no `expectedDigests` for the
	// controller to divide by, so a replica that reports how full it is without reporting how big the
	// set is leaves the status at 0/0 — which is what an operator saw while gigabytes moved.
	assert.NotZero(t, replica.DeclaredDigests,
		"the set's size travels with the count of it, or the status has nothing to divide by")
	assert.GreaterOrEqual(t, replica.DeclaredDigests, replica.VerifiedDigests,
		"a set cannot be smaller than the part of it that is present")
}

// tagOnDisk lays out the tag link distribution writes, which is what makes a release RESOLVABLE from
// the store rather than merely present in it.
//
// Its own helper because the two are different facts and a store can have one without the other — the
// case that cost a three-master cluster its transition: the leader counted a full set of manifests while
// its followers got `MANIFEST_UNKNOWN` asking it for the release by tag.
func tagOnDisk(t *testing.T, root, tag, digest string) {
	t.Helper()

	dir := filepath.Join(root, "docker", "registry", "v2", "repositories",
		"system", "deckhouse", "_manifests", "tags", tag, "current")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "link"), []byte(digest), 0o644))
}

// TestOnceCountsAPushedBundleWhileTheUpstreamIsStillHeld is the transition window of
// the air-gap story, and the shape of the defect it guards against.
//
// Air-gap has been asked for, so the write endpoint is open and `d8 mirror push` has
// put the whole set in the store. The upstream is still HELD, deliberately: the
// cluster has to keep working until the leader is complete. With the count taken from
// what the fill copied, a leader in exactly this state reports nothing at all — the
// fill cannot even start, and the push that filled the store goes unseen — so the
// upstream is held forever, which is the one outcome the transition exists to avoid.
//
// The cluster here can name no version at all — neither a DeckhouseRelease nor a running
// image with a tag — so the fill cannot start on any pass. Note that a cluster installed
// from a tag is no longer in this state: the version is then read from the image the
// cluster runs (see gc.FromCluster). What is left here is the narrower case where nothing
// answers, which is the one that must still count a pushed bundle.
func TestOnceCountsAPushedBundleWhileTheUpstreamIsStillHeld(t *testing.T) {
	local := startRegistry(t)

	// What `d8 mirror push` left in the store: the release set, as the release declares it. The
	// upstream is unreachable on purpose, so nothing here could have been copied — the whole point is
	// that a push the syncer never saw is still accounted for.
	one := pushDigest(t, local, "system/deckhouse")
	two := pushDigest(t, local, "system/deckhouse")
	pushImage(t, local, "system/deckhouse:v1.70.1")
	pushInstaller(t, local, "system/deckhouse/install:v1.70.1", []string{one, two})

	loop, c, _ := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP,
				Host:   "held.example.invalid",
				Path:   "/deckhouse/ee",
			},
		},
		Source:          &registryv1alpha1.StorageSource{ExpectedDigests: 4},
		Publish:         true,
		AirGapRequested: true,
		NeedSync:        true,
	}), deployedRelease("v1.70.1"))
	loop.LocalAddress = local
	loop.WriteAddress = local
	// What `d8 mirror push` left on disk, which is where the count now reads from: the registry
	// API would answer a listing with its upstream's contents, and this number decides whether the
	// cluster may be cut off from that upstream.
	loop.DataDir = t.TempDir()
	release := resolveTag(t, local, "system/deckhouse", "v1.70.1")
	heldOnDisk(t, loop.DataDir, one, two, release,
		resolveTag(t, local, "system/deckhouse/install", "v1.70.1"))
	// A push leaves the tag as well as the manifests, and completeness requires it: a store the release
	// cannot be resolved from is not a store anything can be replicated or updated from.
	tagOnDisk(t, loop.DataDir, "v1.70.1", release)

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.EqualValues(t, 3, replica.VerifiedDigests,
		"the count must come from the store, which is where the push wrote")
	assert.True(t, replica.Full,
		"the store holds the whole expected set, so the leader is complete")
	assert.Empty(t, replica.Error,
		"a fill that could not run must not veto a transition whose evidence it no longer is")
}

// TestOnceWhilePublishingWithAnUnreadableStore is the other half of the same rule:
// the fill's complaint is dropped because the store answers instead, so a store that
// does NOT answer leaves no evidence — and no evidence is not permission.
func TestOnceWhilePublishingWithAnUnreadableStore(t *testing.T) {
	loop, c, _ := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP,
				Host:   "held.example.invalid",
				Path:   "/deckhouse/ee",
			},
		},
		Source:   &registryv1alpha1.StorageSource{ExpectedDigests: 3},
		Publish:  true,
		NeedSync: true,
	}))
	// A registry that answers, so that the pass gets as far as the store: one that is down is said as
	// itself — see TestAPassReportsARegistryThatStaysDown — and is not what this is about.
	loop.LocalAddress = startRegistry(t)
	loop.WriteAddress = loop.LocalAddress
	// A store that cannot be read at all: the path is a file where a directory has to be. Distinct
	// from a store that is merely empty, which is an honest zero — this one is an absence of
	// evidence, and an absence of evidence must not read as an empty store that happens to satisfy
	// nothing.
	unreadable := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(unreadable, []byte("x"), 0o644))
	loop.DataDir = unreadable

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.False(t, replica.Full)
	assert.NotEmpty(t, replica.Error, "an unreadable store must be reported, not silently treated as empty")
}

// TestAFillWaitsForACollectionInsteadOfRacingIt is the writer side of the rule: inside the storage
// pod the syncer is the only thing that writes to the cache, so "the store must not be written to
// while it is collected" is this process refraining — not the registry being made read-only.
//
// What it replaces mattered: read-only was applied by restarting the serving process, which the
// kubelet counts as a crash, twice per collection whether or not anything was deletable. Those
// restarts accumulate into kubelet backoff, and the store answers `connection refused` for minutes at
// a time. Serving images is not something housekeeping may interrupt.
func TestAFillWaitsForACollectionInsteadOfRacingIt(t *testing.T) {
	local := startRegistry(t)
	loop, _, _ := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Source: &registryv1alpha1.StorageSource{ExpectedDigests: 1},
	}))
	loop.LocalAddress = local
	loop.WriteAddress = local
	loop.DataDir = t.TempDir()

	// A collection is under way.
	release := loop.PauseWrites()

	started := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		close(started)
		done := loop.PauseWrites()
		done()
		close(finished)
	}()

	<-started
	select {
	case <-finished:
		t.Fatal("a write went ahead while the collection held the store")
	case <-time.After(100 * time.Millisecond):
	}

	// And once the collection lets go, the write proceeds.
	release()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the write never resumed after the collection finished")
	}
}

// TestAnIdlePassDoesNotErasePreviousCompleteness is the cycle that reads as flapping leader election.
//
// A leader fills the store and reports full. The controller sees the storage converged and clears
// `needSync`. With nothing left to do the next pass goes idle — and it used to recompute fullness from
// `expectedDigests`, which this configuration does not state, so the recomputation could only answer
// "not full". The controller then saw a store that was not converged and asked for a fill again. Every
// few seconds. And because eligibility to lead depends on being full, the lease travels with it, which
// is what makes the symptom look like an election problem — though no fix to leadership can cure a
// status that erases its own evidence.
func TestAnIdlePassDoesNotErasePreviousCompleteness(t *testing.T) {
	local := startRegistry(t)

	// The state a finished fill leaves behind: this replica holds the set and says so.
	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP, Host: "upstream.invalid", Path: "/deckhouse/ee",
			},
		},
		// No `source`, so nothing states an expectation — which is the ordinary shape of a
		// cache-with-an-upstream installation, and where the recomputation went wrong.
		NeedSync: false,
	})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader,
		Full: true, VerifiedDigests: 333, Source: "master-1",
	}}

	loop, c, _ := newLoop(t, true, storage)
	loop.LocalAddress = local
	loop.WriteAddress = local
	loop.DataDir = t.TempDir()

	// A pass with nothing to do: the controller has not asked for a fill.
	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.True(t, replica.Full,
		"a pass that learned nothing must not contradict the one that established completeness")
	assert.EqualValues(t, 333, replica.VerifiedDigests)
	assert.Equal(t, "master-1", replica.Source, "and must not forget where it was filled from")
}

// TestAStoreWithoutLayersIsNotComplete is the guard on the one action that cannot be taken back.
//
// The fill and the replication decide completeness from their own copier's report, and a copier that
// finds a manifest already present counts it as done. A pull-through cache writes that manifest the
// moment it SERVES it and fetches the layers only when somebody asks, so "already present" can mean
// a store holding nothing anybody can pull — while every replica reports `full` with
// `safeToDropUpstream: true`.
//
// So the store is asked instead, and a set whose images are not servable is not a complete set —
// whatever the pass that ran before it thought.
func TestAStoreWithoutLayersIsNotComplete(t *testing.T) {
	local := startRegistry(t)

	one := pushDigest(t, local, "system/deckhouse")
	two := pushDigest(t, local, "system/deckhouse")
	pushImage(t, local, "system/deckhouse:v1.70.1")
	pushInstaller(t, local, "system/deckhouse/install:v1.70.1", []string{one, two})

	loop, c, _ := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP,
				Host:   "held.example.invalid",
				Path:   "/deckhouse/ee",
			},
		},
		Source:          &registryv1alpha1.StorageSource{ExpectedDigests: 4},
		Publish:         true,
		AirGapRequested: true,
		NeedSync:        true,
	}), deployedRelease("v1.70.1"))
	loop.LocalAddress = local
	loop.WriteAddress = local
	loop.DataDir = t.TempDir()

	// What a pull-through cache leaves: the revision links and the manifests, and not one layer.
	release := resolveTag(t, local, "system/deckhouse", "v1.70.1")
	for i, digest := range []string{one, two, release} {
		algorithm, hex, found := strings.Cut(digest, ":")
		require.True(t, found)
		dir := filepath.Join(loop.DataDir, "docker", "registry", "v2", "repositories",
			"system", "deckhouse", fmt.Sprintf("image-%d", i), "_manifests", "revisions", algorithm, hex)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "link"), []byte(digest), 0o644))
		blobOnDisk(t, loop.DataDir, digest, []byte(
			`{"mediaType":"application/vnd.oci.image.manifest.v1+json",`+
				`"config":{"digest":"sha256:c0f1111111111111111111111111111111111111111111111111111111111fff"},`+
				`"layers":[{"digest":"sha256:1a4e222222222222222222222222222222222222222222222222222222222fff"}]}`))
	}
	tagOnDisk(t, loop.DataDir, "v1.70.1", release)

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.False(t, replica.Full,
		"a store whose images have no layers must not authorize dropping the upstream")
	assert.EqualValues(t, 0, replica.VerifiedDigests,
		"and it holds none of the set, however many manifests it has touched")
}

// TestTheFillWritesToTheNonProxyingInstance pins which of the two registry instances a fill writes to.
//
// The storage pod runs two of them over one data directory: the serving one, which is a pull-through
// cache, and the write endpoint, which never proxies. Filling through the serving one fills nothing —
// before uploading a layer the client asks whether the destination already has that blob, and a cache
// answers yes by fetching it from the upstream on the spot. The upload is skipped, the manifest is
// written, and the store ends up holding manifests naming blobs it does not have. Such a fill reports
// the whole set written while the store gains nothing, and the registry answers 200 for layers that
// are not on disk — because the upstream is still there to answer.
func TestTheFillWritesToTheNonProxyingInstance(t *testing.T) {
	upstream := startRegistry(t)
	serving := startRegistry(t)
	writeEndpoint := startRegistry(t)

	one := pushDigest(t, upstream, "deckhouse/ee")
	two := pushDigest(t, upstream, "deckhouse/ee")
	pushImage(t, upstream, "deckhouse/ee:v1.70.1")
	pushInstaller(t, upstream, "deckhouse/ee/install:v1.70.1", []string{one, two})

	loop, _, _ := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP, Host: upstream, Path: "/deckhouse/ee",
			},
		},
		Source:   &registryv1alpha1.StorageSource{ExpectedDigests: 4},
		NeedSync: true,
	}), deployedRelease("v1.70.1"))
	loop.LocalAddress = serving
	loop.WriteAddress = writeEndpoint
	loop.ReportedAddress = serving

	require.NoError(t, loop.once(context.Background()))

	// The images are in the write endpoint, which is the same store on disk.
	assert.True(t, holdsDigest(t, writeEndpoint, "system/deckhouse", one),
		"the fill did not write through the non-proxying instance")
	assert.True(t, holdsDigest(t, writeEndpoint, "system/deckhouse", two))

	// And nothing was written through the serving one, which would have swallowed the layers.
	assert.False(t, holdsDigest(t, serving, "system/deckhouse", one),
		"a fill through the pull-through cache uploads no layers, so it must not go there")
}

// closedAddress is an address nothing listens on: taken and released at once.
func closedAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	return address
}

// TestOnceWaitsForItsOwnRegistryToStart is the start of every store pod.
//
// The syncer and the registry start together, and a pass that runs before the registry listens
// reads the declared set through it and gets "connection refused". Published, that turned the
// storage Failed and safeToDropUpstream false on every start of the pod, until the next clean pass
// — on an Engine cluster at the first start and again after an update replaced the replica.
func TestOnceWaitsForItsOwnRegistryToStart(t *testing.T) {
	follower := func(t *testing.T) (*Loop, client.Client) {
		loop, c, _ := newLoop(t, false, storageWith(registryv1alpha1.RegistryStorageSpec{
			Upstream: &registryv1alpha1.Upstream{
				Endpoint: registryv1alpha1.Endpoint{
					Scheme: registryv1alpha1.SchemeHTTP, Host: "upstream.invalid", Path: "/deckhouse/ee",
				},
			},
			Source:   &registryv1alpha1.StorageSource{ExpectedDigests: 1},
			NeedSync: true,
		}), deployedRelease("v1.70.1"))
		loop.LocalAddress = closedAddress(t)
		loop.WriteAddress = loop.LocalAddress
		loop.DataDir = t.TempDir()
		return loop, c
	}

	t.Run("just started: the pass waits and reports nothing", func(t *testing.T) {
		loop, c := follower(t)
		loop.started = time.Now()

		require.NoError(t, loop.once(context.Background()))

		storage := &registryv1alpha1.RegistryStorage{}
		require.NoError(t, c.Get(context.Background(),
			types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage))
		assert.Empty(t, storage.Status.Replicas, "a store still starting is not a failed store")
	})

	t.Run("past the grace: a registry that never came up is a failure", func(t *testing.T) {
		loop, c := follower(t)
		loop.started = time.Now().Add(-3 * time.Minute)

		require.NoError(t, loop.once(context.Background()))

		assert.NotEmpty(t, replicaOf(t, c, "master-0").Error)
	})
}

// TestTheStoreIsReportedWhileAPassRuns: a fill over a slow link is one pass of an hour, and the
// store it fills has to show in the status before that hour is over.
func TestTheStoreIsReportedWhileAPassRuns(t *testing.T) {
	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Store: registryv1alpha1.StorageStore{Size: "1Gi"},
	})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Full: true, VerifiedDigests: 459},
	}
	loop, c, _ := newLoop(t, true, storage)
	loop.DataDir = t.TempDir()
	loop.Publisher = &report.Publisher{Client: c}
	loop.desired.Store(&storage.Spec)

	blob := filepath.Join(loop.DataDir, "docker", "registry", "v2", "blobs", "sha256", "ab", "abcdef", "data")
	require.NoError(t, os.MkdirAll(filepath.Dir(blob), 0o755))
	require.NoError(t, os.WriteFile(blob, make([]byte, 40<<20), 0o644))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go loop.watchStore(ctx, 20*time.Millisecond)

	require.Eventually(t, func() bool {
		return replicaOf(t, c, "master-0").Store != nil
	}, 5*time.Second, 20*time.Millisecond)

	replica := replicaOf(t, c, "master-0")
	assert.Equal(t, int64(48<<20), replica.Store.UsedBytes, "40 MiB, rounded up to 16 MiB")
	assert.Equal(t, int64(1<<30), replica.Store.BudgetBytes)
	assert.True(t, replica.Full, "a store report must leave the fill's fields alone")
	assert.EqualValues(t, 459, replica.VerifiedDigests)
}

// TestAPassUnderWayReportsItselfAndClearsTheOldError: published at the first reference and then at
// most every progressEvery, so a long fill neither goes silent nor floods the API server.
func TestAPassUnderWayReportsItselfAndClearsTheOldError(t *testing.T) {
	storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader,
		Error: "reading the image set of main: MANIFEST_UNKNOWN",
	}}
	loop, c, _ := newLoop(t, true, storage)
	loop.Publisher = &report.Publisher{Client: c}

	state := report.State{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader}
	onProgress := loop.progress(context.Background(), state, "fill")

	onProgress(1, 459)
	replica := replicaOf(t, c, "master-0")
	assert.Empty(t, replica.Error)
	assert.EqualValues(t, 1, replica.VerifiedDigests)
	assert.EqualValues(t, 459, replica.DeclaredDigests)

	// Within the interval: logged, not published.
	onProgress(2, 459)
	assert.EqualValues(t, 1, replicaOf(t, c, "master-0").VerifiedDigests)
}

// releaseAwareClient is a fake API server that also serves what the set is read from: platform
// releases, module releases and module sources.
func releaseAwareClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))
	for _, kind := range []string{"DeckhouseRelease", "ModuleRelease", "ModuleSource"} {
		gvk := schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: kind}
		scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
		scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind(kind+"List"), &unstructured.UnstructuredList{})
	}
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&registryv1alpha1.RegistryStorage{}).
		WithObjects(objects...).
		Build()
}

// TestAFullLeaderReverifiesWhenTheSetMoves is the defect a live cluster found: a module installed
// after the leader reported full, and the store kept saying "full" — with the permission to cut the
// cluster off from its upstream standing on it — for a set it had never seen.
func TestAFullLeaderReverifiesWhenTheSetMoves(t *testing.T) {
	local := startRegistry(t)

	for _, tc := range []struct {
		name        string
		verifiedSet string
		wantFull    bool
	}{
		// Found full against another set: withdrawn, then verified — and the verification fails
		// here, because the upstream holds nothing of v1.70.1.
		{name: "the set moved", verifiedSet: "a set from before", wantFull: false},
		// Found full against this very set: nothing to do, nothing withdrawn.
		{name: "the set is the same", verifiedSet: SetKey(gc.Releases{Deployed: "v1.70.1", Previous: ""}, nil, nil), wantFull: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storage := storageWith(registryv1alpha1.RegistryStorageSpec{
				Upstream: &registryv1alpha1.Upstream{
					Endpoint: registryv1alpha1.Endpoint{Scheme: registryv1alpha1.SchemeHTTP, Host: local},
				},
				// The controller does not ask for a fill: the leader reported full.
				NeedSync: false,
			})
			storage.Status.SafeToDropUpstream = true
			storage.Status.AllReplicasFull = true
			storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
				Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true,
				VerifiedDigests: 459, VerifiedSet: tc.verifiedSet,
			}}

			loop, _, _ := newLoop(t, true, nil)
			c := releaseAwareClient(t, storage, deployedRelease("v1.70.1"))
			loop.Client = c
			loop.Publisher = &report.Publisher{Client: c}
			loop.LocalAddress = local
			loop.WriteAddress = local
			if tc.wantFull {
				// A leader that is full holds the release it was found full against; an idle pass reads
				// the store again and would rightly find one without it incomplete.
				pushImage(t, local, "system/deckhouse:v1.70.1")
			}

			require.NoError(t, loop.once(context.Background()))

			replica := replicaOf(t, c, "master-0")
			assert.Equal(t, tc.wantFull, replica.Full)
			if tc.wantFull {
				assert.Equal(t, tc.verifiedSet, replica.VerifiedSet, "an unchanged set keeps its fingerprint")
				return
			}
			assert.Empty(t, replica.VerifiedSet)
			assert.NotEmpty(t, replica.Error, "the verification ran, and says why it did not pass")

			current := &registryv1alpha1.RegistryStorage{}
			require.NoError(t, c.Get(context.Background(),
				types.NamespacedName{Name: registryv1alpha1.SingletonName}, current))
			assert.False(t, current.Status.SafeToDropUpstream, "the permission went with the report it rested on")
			assert.False(t, current.Status.AllReplicasFull)
		})
	}
}

// TestAFollowerSaysWhichSetItIsFullFor: eligibility compares reports about the same set, so a follower
// reports the set it replicated against along with its full — see MayLead.
func TestAFollowerSaysWhichSetItIsFullFor(t *testing.T) {
	leaderStorage := startRegistry(t)
	followerStorage := startRegistry(t)

	one := pushDigest(t, leaderStorage, "system/deckhouse")
	pushImage(t, leaderStorage, "system/deckhouse:v1.76.6")
	pushInstaller(t, leaderStorage, "system/deckhouse/install:v1.76.6", []string{one})

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{NeedSync: true})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, Address: leaderStorage,
	}}

	loop, _, _ := newLoop(t, false, nil)
	c := releaseAwareClient(t, storage, deployedRelease("v1.76.6"))
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.LocalAddress = followerStorage
	loop.WriteAddress = followerStorage
	loop.ReportedAddress = followerStorage

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	require.True(t, replica.Full)
	assert.Equal(t, SetKey(gc.Releases{Deployed: "v1.76.6", Previous: ""}, nil, nil), replica.VerifiedSet)
}

// TestAFailingPassKeepsTheErrorItInherited: the copier reports after every reference, failed ones too,
// and a progress report takes the previous pass's error back. On a leader whose writes all fail, every
// pass cleared the error at its first reference and wrote it again at its end — the storage read
// Filling rather than Failed, and the leader looked healthy to the election for the whole pass.
func TestAFailingPassKeepsTheErrorItInherited(t *testing.T) {
	storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Error: "3 of 3 references could not be copied",
	}}
	loop, c, _ := newLoop(t, true, storage)
	loop.Publisher.Name = registryv1alpha1.SingletonName

	progress := loop.progress(context.Background(), report.State{Node: "master-0"}, "fill")

	progress(0, 3) // the first reference failed
	progress(0, 3) // and the second
	assert.Equal(t, "3 of 3 references could not be copied", replicaOf(t, c, "master-0").Error,
		"nothing has landed, so nothing says the failure is behind this pass")

	progress(1, 3) // something landed
	assert.Empty(t, replicaOf(t, c, "master-0").Error)
}

// TestAPassWaitsForItsOwnRegistry is the start of a fill on a fresh cluster: the syncer restarts the
// registry and begins a pass before it listens again. That pass used to fail every reference on
// "connection refused" and publish it as the replica's error. It now waits for the registry and fills.
func TestAPassWaitsForItsOwnRegistry(t *testing.T) {
	upstream := startRegistry(t)
	local := startRegistry(t)
	one := pushDigest(t, upstream, "deckhouse/ee")
	pushImage(t, upstream, "deckhouse/ee:v1.70.1")
	pushInstaller(t, upstream, "deckhouse/ee/install:v1.70.1", []string{one})

	// The replica's registry, opening its port a moment after the pass begins.
	address := delayedProxy(t, local, 300*time.Millisecond)

	loop, c, _ := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP, Host: upstream, Path: "/deckhouse/ee",
			},
		},
		NeedSync: true,
	}), deployedRelease("v1.70.1"))
	loop.LocalAddress = address
	loop.WriteAddress = address
	loop.RegistryWait = 5 * time.Second

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.Empty(t, replica.Error, "a registry still starting is not a failed copy")
	assert.True(t, replica.Full)
}

// TestAPassReportsARegistryThatStaysDown is the other half: a registry that never listens is said once,
// as itself, rather than as every reference of the set failing.
func TestAPassReportsARegistryThatStaysDown(t *testing.T) {
	upstream := startRegistry(t)
	one := pushDigest(t, upstream, "deckhouse/ee")
	pushImage(t, upstream, "deckhouse/ee:v1.70.1")
	pushInstaller(t, upstream, "deckhouse/ee/install:v1.70.1", []string{one})

	loop, c, _ := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP, Host: upstream, Path: "/deckhouse/ee",
			},
		},
		NeedSync: true,
	}), deployedRelease("v1.70.1"))
	loop.LocalAddress = closedAddress(t)
	loop.WriteAddress = loop.LocalAddress

	// The first pass that finds it down says nothing: the wrapper walks the whole store before it opens
	// a port, after every restart the configuration causes, and that is not a failure.
	require.NoError(t, loop.once(context.Background()))
	assert.Empty(t, replicaOf(t, c, "master-0").Error, "a registry that has just gone quiet may be starting")

	// Down for longer than a start takes, it is.
	loop.registryDownSince.Store(time.Now().Add(-registryDownReported).UnixNano())
	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.False(t, replica.Full)
	assert.Contains(t, replica.Error, "registry is not listening")
	assert.NotContains(t, replica.Error, "references could not be copied")
}

// closedAddress is an address nothing listens on: taken, then released.
// delayedProxy forwards a fresh address to target, opening it only after delay.
func delayedProxy(t *testing.T, target string, delay time.Duration) string {
	t.Helper()
	address := closedAddress(t)
	go func() {
		time.Sleep(delay)
		listener, err := net.Listen("tcp", address)
		if err != nil {
			return
		}
		t.Cleanup(func() { _ = listener.Close() })
		for {
			in, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer in.Close()
				out, err := net.Dial("tcp", target)
				if err != nil {
					return
				}
				defer out.Close()
				go func() { _, _ = io.Copy(out, in) }()
				_, _ = io.Copy(in, out)
			}()
		}
	}()
	return address
}

// TestAFullReplicaWaitsOutTheOthers: needSync is up because some other replica is behind, and this
// one already holds the set it was last found full against. It does not go to the leader — the
// leader here answers nothing, so a pass would show up as an error — and keeps reporting itself full.
func TestAFullReplicaWaitsOutTheOthers(t *testing.T) {
	followerStorage := startRegistry(t)
	set := SetKey(gc.Releases{Deployed: "v1.76.6", Previous: ""}, nil, nil)

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{NeedSync: true})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedSet: set,
			Address: closedAddress(t)},
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleFollower, Full: true, VerifiedSet: set,
			VerifiedDigests: 3, Source: "master-1"},
	}

	loop, _, _ := newLoop(t, false, nil)
	c := releaseAwareClient(t, storage, deployedRelease("v1.76.6"))
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.LocalAddress = followerStorage
	loop.WriteAddress = followerStorage
	loop.ReportedAddress = followerStorage
	// Checked a moment ago. Otherwise this pass is the first idle one and reads the store — and a store
	// whose set cannot be read leaves the report alone, which made this test pass whatever the pass did.
	loop.lastStoreCheck.Store(time.Now().UnixNano())

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.Empty(t, replica.Error, "a full replica does not copy, so the unreachable leader is never asked")
	assert.True(t, replica.Full)
	assert.Equal(t, set, replica.VerifiedSet)
}

// TestAnIdleReplicaFindsWhatItsStoreLost: every replica holds the set, so needSync is down and
// nothing is copied. This one's disk has since lost an image. The idle pass reads the store again,
// reports it as not full and no longer full for the set — which is what makes the controller raise
// needSync — and does it once per recheck, not on every pass.
func TestAnIdleReplicaFindsWhatItsStoreLost(t *testing.T) {
	local := startRegistry(t)
	one := pushDigest(t, local, "system/deckhouse")
	two := pushDigest(t, local, "system/deckhouse")
	pushImage(t, local, "system/deckhouse:v1.76.6")
	pushInstaller(t, local, "system/deckhouse/install:v1.76.6", []string{one, two})
	set := SetKey(gc.Releases{Deployed: "v1.76.6", Previous: ""}, nil, nil)

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedSet: set,
			Address: closedAddress(t)},
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleFollower, Full: true, VerifiedSet: set,
			VerifiedDigests: 3, Source: "master-1"},
	}

	loop, _, _ := newLoop(t, false, nil)
	c := releaseAwareClient(t, storage, deployedRelease("v1.76.6"))
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.LocalAddress = local
	loop.WriteAddress = local
	loop.ReportedAddress = local
	loop.DataDir = t.TempDir()

	// The release resolves, and `two` is gone from the disk.
	release := resolveTag(t, local, "system/deckhouse", "v1.76.6")
	heldOnDisk(t, loop.DataDir, one, release)
	tagOnDisk(t, loop.DataDir, "v1.76.6", release)

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.False(t, replica.Full, "the store lost an image, and the replica says so")
	assert.Empty(t, replica.VerifiedSet, "so it is no longer full for the set, and the controller sees it behind")
	assert.Empty(t, replica.Error, "a store short of an image is not a failure of the pass")

	// Put back, and not read again until the recheck is due.
	heldOnDisk(t, loop.DataDir, two)
	require.NoError(t, loop.once(context.Background()))
	assert.False(t, replicaOf(t, c, "master-0").Full, "an idle pass between rechecks reads nothing")

	loop.lastStoreCheck.Store(time.Now().Add(-storeRecheck).UnixNano())
	loop.surveyedAt = time.Time{}
	require.NoError(t, loop.once(context.Background()))
	replica = replicaOf(t, c, "master-0")
	assert.True(t, replica.Full, "the recheck finds it complete again")
	// But it gives no fingerprint back: what it reads is the set the cluster runs, not what is pending,
	// so only a pass that copies may say "full for this set". The replica stays behind for the controller,
	// which raises needSync, and the replication that follows verifies the whole set and stamps it.
	assert.Empty(t, replica.VerifiedSet, "a recheck takes a fingerprint away and never awards one")
}

// TestARecheckDoesNotVouchForASetItDidNotRead is the review's finding on the recheck: an idle follower
// that last replicated an older set, while the leader holds the current one. Its disk happens to hold
// everything the current set needs, and the recheck finds it so; stamped with the current fingerprint,
// the follower would look caught up without a replication ever saying so, and needSync would stay
// down. Only a copying pass gives a fingerprint.
func TestARecheckDoesNotVouchForASetItDidNotRead(t *testing.T) {
	storage := startRegistry(t)
	running := pushDigest(t, storage, "system/deckhouse")
	pushImage(t, storage, "system/deckhouse:v1.76.6")
	pushInstaller(t, storage, "system/deckhouse/install:v1.76.6", []string{running})

	before := SetKey(gc.Releases{Deployed: "v1.76.5"}, nil, nil)

	state := storageWith(registryv1alpha1.RegistryStorageSpec{})
	state.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader, Full: true,
			VerifiedSet: SetKey(gc.Releases{Deployed: "v1.76.6"}, nil, nil),
			Address:     closedAddress(t)},
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleFollower, Full: true, VerifiedSet: before,
			Source: "master-1"},
	}

	loop, _, _ := newLoop(t, false, nil)
	c := releaseAwareClient(t, state, deployedRelease("v1.76.6"))
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.LocalAddress = storage
	loop.WriteAddress = storage
	loop.ReportedAddress = storage
	loop.DataDir = t.TempDir()

	// Everything the running set needs is on the disk.
	release := resolveTag(t, storage, "system/deckhouse", "v1.76.6")
	heldOnDisk(t, loop.DataDir, running, release)
	tagOnDisk(t, loop.DataDir, "v1.76.6", release)

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	require.True(t, replica.Full, "the recheck found the running set complete")
	assert.Equal(t, before, replica.VerifiedSet,
		"the follower stays full for the set it verified, so the controller sees it behind the leader")
}

// TestAnUpdateReachesTheFollowersBeforeItIsDeployed is the air-gapped update. `d8 mirror push` put
// the next release onto the leader and the cluster lists it as pending. The leader holds it — its
// AheadSet says so — and the follower, full for the set and idle, copies it from the leader and says
// it holds it too, before anything was deployed. Counted only once deployed, the update stayed on the
// leader alone through the whole update; counted in the set, a follower that had copied the set and
// not yet the update reported both.
func TestAnUpdateReachesTheFollowersBeforeItIsDeployed(t *testing.T) {
	leaderStorage := startRegistry(t)
	followerStorage := startRegistry(t)

	running := pushDigest(t, leaderStorage, "system/deckhouse")
	pushImage(t, leaderStorage, "system/deckhouse:v1.76.6")
	pushInstaller(t, leaderStorage, "system/deckhouse/install:v1.76.6", []string{running})

	update := pushDigest(t, leaderStorage, "system/deckhouse")
	pushImage(t, leaderStorage, "system/deckhouse:v1.77.0")
	pushInstaller(t, leaderStorage, "system/deckhouse/install:v1.77.0", []string{update})

	pending := deployedRelease("v1.77.0")
	pending.Object["status"] = map[string]any{"phase": "Pending"}

	set := SetKey(gc.Releases{Deployed: "v1.76.6"}, nil, nil)
	ahead := AheadKey(gc.Releases{Deployed: "v1.76.6", Pending: []string{"v1.77.0"}}, nil)

	// Nobody asks for a copy: the set is held everywhere.
	storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedSet: set,
			AheadSet: ahead, Address: leaderStorage},
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleFollower, Full: true, VerifiedSet: set,
			Source: "master-1"},
	}

	loop, _, _ := newLoop(t, false, nil)
	c := releaseAwareClient(t, storage, deployedRelease("v1.76.6"), pending)
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.LocalAddress = followerStorage
	loop.WriteAddress = followerStorage
	loop.ReportedAddress = followerStorage
	// The store check is not what this is about.
	loop.lastStoreCheck.Store(time.Now().UnixNano())

	gotSet, gotAhead, err := CurrentKeys(context.Background(), c)
	require.NoError(t, err)
	require.Equal(t, set, gotSet, "the pending release is not part of the set")
	require.Equal(t, ahead, gotAhead, "it is what is ahead of it")

	require.NoError(t, loop.once(context.Background()))
	// The copy ahead runs in the background, and the next pass says what it found.
	loop.waitAhead()
	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.True(t, replica.Full)
	assert.Equal(t, set, replica.VerifiedSet)
	assert.Equal(t, ahead, replica.AheadSet, "the follower holds what is ahead, as the leader does")
	assert.Equal(t, resolveTag(t, leaderStorage, "system/deckhouse", "v1.77.0"),
		resolveTag(t, followerStorage, "system/deckhouse", "v1.77.0"),
		"the update is on the follower, not only on the leader")
}

// TestAFollowerWaitsForALeaderThatHoldsTheUpdate: a leader that does not hold what is ahead yet has
// nothing whole to give, and the follower does not copy from it — it would only miss what the leader
// misses — nor claim to hold it.
func TestAFollowerWaitsForALeaderThatHoldsTheUpdate(t *testing.T) {
	leaderStorage := startRegistry(t)
	followerStorage := startRegistry(t)

	running := pushDigest(t, leaderStorage, "system/deckhouse")
	pushImage(t, leaderStorage, "system/deckhouse:v1.76.6")
	pushInstaller(t, leaderStorage, "system/deckhouse/install:v1.76.6", []string{running})

	pending := deployedRelease("v1.77.0")
	pending.Object["status"] = map[string]any{"phase": "Pending"}
	set := SetKey(gc.Releases{Deployed: "v1.76.6"}, nil, nil)

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedSet: set,
			Address: leaderStorage},
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleFollower, Full: true, VerifiedSet: set,
			Source: "master-1"},
	}

	loop, _, _ := newLoop(t, false, nil)
	c := releaseAwareClient(t, storage, deployedRelease("v1.76.6"), pending)
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.LocalAddress = followerStorage
	loop.WriteAddress = followerStorage
	loop.ReportedAddress = followerStorage
	loop.lastStoreCheck.Store(time.Now().UnixNano())

	require.NoError(t, loop.once(context.Background()))
	assert.False(t, loop.Filling(), "no copy ahead from a leader short of the update")
	assert.Empty(t, replicaOf(t, c, "master-0").AheadSet)
}

// TestAnAirGappedLeaderHoldsWhatWasPushedOntoIt: with no upstream there is nothing to copy ahead from,
// and the leader checks its own store instead. Pushed, the update is held, and the followers can take
// it from there.
func TestAnAirGappedLeaderHoldsWhatWasPushedOntoIt(t *testing.T) {
	store := startRegistry(t)

	running := pushDigest(t, store, "system/deckhouse")
	pushImage(t, store, "system/deckhouse:v1.76.6")
	pushInstaller(t, store, "system/deckhouse/install:v1.76.6", []string{running})

	pending := deployedRelease("v1.77.0")
	pending.Object["status"] = map[string]any{"phase": "Pending"}
	set := SetKey(gc.Releases{Deployed: "v1.76.6"}, nil, nil)
	ahead := AheadKey(gc.Releases{Deployed: "v1.76.6", Pending: []string{"v1.77.0"}}, nil)

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedSet: set},
	}

	loop, _, _ := newLoop(t, true, nil)
	c := releaseAwareClient(t, storage, deployedRelease("v1.76.6"), pending)
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.LocalAddress = store
	loop.WriteAddress = store
	loop.ReportedAddress = store
	loop.DataDir = t.TempDir()
	loop.lastStoreCheck.Store(time.Now().UnixNano())

	// The set the cluster runs is on the disk, which is what an air-gapped leader is counted full by.
	release := resolveTag(t, store, "system/deckhouse", "v1.76.6")
	heldOnDisk(t, loop.DataDir, running, release)
	tagOnDisk(t, loop.DataDir, "v1.76.6", release)

	pass := func() {
		require.NoError(t, loop.once(context.Background()))
		loop.waitAhead()
		require.NoError(t, loop.once(context.Background()))
	}

	// Not pushed yet: missed, and not tried again at once.
	pass()
	assert.Empty(t, replicaOf(t, c, "master-0").AheadSet, "an update that is not there is not held")

	update := pushDigest(t, store, "system/deckhouse")
	pushImage(t, store, "system/deckhouse:v1.77.0")
	pushInstaller(t, store, "system/deckhouse/install:v1.77.0", []string{update})

	pass()
	assert.Empty(t, replicaOf(t, c, "master-0").AheadSet, "a miss waits before the store is checked again")

	loop.aheadMu.Lock()
	loop.aheadMissedAt = time.Now().Add(-aheadRetry)
	loop.aheadMu.Unlock()
	pass()
	assert.Equal(t, ahead, replicaOf(t, c, "master-0").AheadSet, "pushed, it is held")
}

// TestAPendingUpdateTheSourceDoesNotHoldDoesNotKeepTheFillGoing is what a live cluster showed once
// pending releases joined the set: a module update listed as pending, its package not in the source
// (the platform itself could not download it). Counted in the set, it left every replica one short
// for good — needSync up forever, the leader refilling and the followers replicating every pass. It is
// copied ahead of time now, outside completeness: the leader is full, and the miss is only logged.
func TestAPendingUpdateTheSourceDoesNotHoldDoesNotKeepTheFillGoing(t *testing.T) {
	upstream := startRegistry(t)
	local := startRegistry(t)

	one := pushDigest(t, upstream, "deckhouse/ee")
	pushImage(t, upstream, "deckhouse/ee:v1.70.1")
	pushInstaller(t, upstream, "deckhouse/ee/install:v1.70.1", []string{one})

	pendingModule := &unstructured.Unstructured{Object: map[string]any{
		"spec":   map[string]any{"moduleName": "snapshot-controller", "version": "0.1.21"},
		"status": map[string]any{"phase": "Pending"},
	}}
	pendingModule.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleRelease",
	})
	pendingModule.SetName("snapshot-controller-v0.1.21")

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP, Host: upstream, Path: "/deckhouse/ee",
			},
		},
		NeedSync: true,
	})

	loop, _, _ := newLoop(t, true, nil)
	c := releaseAwareClient(t, storage, deployedRelease("v1.70.1"), pendingModule)
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.LocalAddress = local
	loop.WriteAddress = local

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.True(t, replica.Full, "an update the source does not hold yet does not hold back the set")
	assert.Empty(t, replica.Error)

	key, err := CurrentSetKey(context.Background(), c)
	require.NoError(t, err)
	assert.Equal(t, key, replica.VerifiedSet, "full against the set, which the update is not part of")

	// And the update is not claimed: the copy ahead missed it, so nobody takes it for held.
	loop.waitAhead()
	require.NoError(t, loop.once(context.Background()))
	assert.Empty(t, replicaOf(t, c, "master-0").AheadSet, "a missed update is not held")
}

// TestAReplicaThatLostTheReleaseIsNotFull: the release is what the set is read out of, so a store
// without it cannot even enumerate the set. That used to keep the last report — full — for good, and
// nothing asked for the release to be copied back. Absent is an answer: not full, so the controller
// sees the replica behind.
func TestAReplicaThatLostTheReleaseIsNotFull(t *testing.T) {
	empty := startRegistry(t)
	set := SetKey(gc.Releases{Deployed: "v1.76.6"}, nil, nil)

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedSet: set,
			Address: closedAddress(t)},
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleFollower, Full: true, VerifiedSet: set,
			VerifiedDigests: 3, Source: "master-1"},
	}

	loop, _, _ := newLoop(t, false, nil)
	c := releaseAwareClient(t, storage, deployedRelease("v1.76.6"))
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.LocalAddress = empty
	loop.WriteAddress = empty
	loop.ReportedAddress = empty
	loop.DataDir = t.TempDir()

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.False(t, replica.Full, "a store without the release is not full")
	assert.Empty(t, replica.VerifiedSet, "so the controller sees it behind and raises needSync")
	assert.Empty(t, replica.Error, "and it is not a failure of the pass")
}

// TestALeaderSaysItIsFullBeforeItCopiesAhead is the review's finding on the copy ahead: run inside the
// fill, a pending release copied at the background pace held the report of a store already complete for
// the set — the storage read Filling and the upstream could not be dropped for as long as that took. The
// pass now publishes what it found first, and copies ahead after.
//
// Told apart by the upstream itself: the first request it gets for the pending release looks at what
// the leader has published by then.
func TestALeaderSaysItIsFullBeforeItCopiesAhead(t *testing.T) {
	var c client.Client
	var asked atomic.Bool
	var fullWhenAsked atomic.Bool
	inner := registry.New(registry.Logger(log.New(io.Discard, "", 0)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "v1.70.2") && c != nil && !asked.Swap(true) {
			storage := &registryv1alpha1.RegistryStorage{}
			if err := c.Get(r.Context(), types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage); err == nil {
				for _, replica := range storage.Status.Replicas {
					if replica.Node == "master-0" && replica.Full {
						fullWhenAsked.Store(true)
					}
				}
			}
		}
		inner.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	parsed, err := url.Parse(server.URL)
	require.NoError(t, err)
	upstream := parsed.Host
	local := startRegistry(t)

	running := pushDigest(t, upstream, "deckhouse/ee")
	pushImage(t, upstream, "deckhouse/ee:v1.70.1")
	pushInstaller(t, upstream, "deckhouse/ee/install:v1.70.1", []string{running})
	update := pushDigest(t, upstream, "deckhouse/ee")
	pushImage(t, upstream, "deckhouse/ee:v1.70.2")
	pushInstaller(t, upstream, "deckhouse/ee/install:v1.70.2", []string{update})

	pending := deployedRelease("v1.70.2")
	pending.Object["status"] = map[string]any{"phase": "Pending"}

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP, Host: upstream, Path: "/deckhouse/ee",
			},
		},
		NeedSync: true,
	})

	loop, _, _ := newLoop(t, true, nil)
	c = releaseAwareClient(t, storage, deployedRelease("v1.70.1"), pending)
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.LocalAddress = local
	loop.WriteAddress = local

	require.NoError(t, loop.once(context.Background()))
	loop.waitAhead()

	require.True(t, asked.Load(), "the pending release was never copied ahead")
	assert.True(t, fullWhenAsked.Load(), "the copy ahead started before the leader said it is full for the set")
	assert.True(t, replicaOf(t, c, "master-0").Full)
}

// setThenAhead discovers one reference as the set and another as ahead of it.
type setThenAhead struct{ set, ahead name.Reference }

func (d setThenAhead) Discover(context.Context, fill.Registry, *remote.Puller) ([]name.Reference, error) {
	return []name.Reference{d.set}, nil
}

func (d setThenAhead) Optional(context.Context, fill.Registry, *remote.Puller) ([]name.Reference, error) {
	return []name.Reference{d.ahead}, nil
}

// TestACopyAheadGivesWayToTheSetAndResumes is the A/B against main on harbor: the copy ahead, run inside
// the pass, held the next one back for minutes on an upstream slow to answer, and the cluster converged a
// quarter of an hour later. In the background now, it stops for a pass that has to copy the set, and
// resumes afterwards from where it can.
func TestACopyAheadGivesWayToTheSetAndResumes(t *testing.T) {
	release := make(chan struct{})
	var held atomic.Int32
	var armed atomic.Bool // only once the images are in: pushing them goes through the same handler
	inner := registry.New(registry.Logger(log.New(io.Discard, "", 0)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if armed.Load() && strings.Contains(r.URL.Path, "/pending/") {
			held.Add(1)
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		inner.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	parsed, err := url.Parse(server.URL)
	require.NoError(t, err)
	source, destination := parsed.Host, startRegistry(t)
	pushImage(t, source, "platform:v1")
	pushImage(t, source, "pending:v2")

	set, err := name.ParseReference(source+"/platform:v1", name.Insecure)
	require.NoError(t, err)
	ahead, err := name.ParseReference(source+"/pending:v2", name.Insecure)
	require.NoError(t, err)

	copier := &fill.Copier{
		Source:        fill.Registry{Address: source, Insecure: true},
		Destination:   fill.Registry{Address: destination, Insecure: true},
		Discover:      setThenAhead{set: set, ahead: ahead},
		SeparateAhead: true,
	}
	_, err = copier.Run(context.Background())
	require.NoError(t, err)

	armed.Store(true)
	loop := &Loop{Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	loop.startAhead(context.Background(), copier.RunAhead, "pending", "leader")
	require.Eventually(t, func() bool { return held.Load() > 0 }, 5*time.Second, 10*time.Millisecond,
		"the copy ahead never reached the slow upstream")
	assert.True(t, loop.Filling(), "a copy ahead in progress holds the collection off like a fill")

	stopped := make(chan struct{})
	go func() { loop.stopAhead(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("a pass that has to copy the set waited for the copy ahead")
	}
	assert.False(t, loop.Filling())

	// An idle pass starts it again, finding it not held.
	close(release)
	loop.startAhead(context.Background(), copier.RunAhead, "pending", "leader")
	loop.waitAhead()

	tag, err := name.NewTag(destination+"/pending:v2", name.Insecure)
	require.NoError(t, err)
	_, err = remote.Head(tag)
	assert.NoError(t, err, "the copy ahead was started again and went through")
	assert.False(t, loop.Filling())
}

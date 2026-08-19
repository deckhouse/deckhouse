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
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
	config, err := os.ReadFile(loop.Applier.ConfigPath)
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
// Measured on the static stand: the process restarted, its first pass was one that does not read the
// store, so it published the counts it had carried over from the object — and the denominator, which
// was carried nowhere, came out absent. The status then reported no progress at all on a complete
// store, which is precisely the defect the denominator was added to fix, reappearing through a path
// nobody had covered. Six places set the numerator; the denominator has to travel with it in all of
// them, and a fresh process starts with nothing in memory, so the object is the only source.
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
// Measured on the static stand: `totalDigests` climbing pass after pass, `declaredDigests` absent, and
// therefore no `fill` in the status — the very thing this was supposed to fix, surviving two fixes.
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
	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Source: &registryv1alpha1.StorageSource{ExpectedDigests: 4},
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

// heldOnDisk lays out manifest revisions the way distribution does, so a test can give a replica a
// store to count. The layout is the product's contract with its own filesystem; writing it out here
// is what makes the count testable without a registry that has one.
// heldOnDisk puts images in the store the way a finished `d8 mirror push` leaves them: the revision
// link, the manifest itself, and every blob the manifest names.
//
// All three, because held means servable. A revision link alone is what a pull-through cache writes
// the moment it has SERVED a manifest, before fetching anything it points at — so a store built out
// of links only is a store that can answer questions about images it cannot hand over. Measured on
// `ly-mmc`: 332 manifests, 61 layer links, 333 MB, three replicas calling themselves full and
// authorizing an air-gap in which no node could pull at all.
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
// Measured on a three-master cluster: the leader reported full, and its followers asking it for the
// release got `MANIFEST_UNKNOWN` for `:pr21788` while the node agent got `NAME_UNKNOWN: repository name
// not known to registry`. The transition had been authorized by a store that could not hand the release
// to anybody — replication enumerates the set by reading the release BY TAG, so a set whose tag is
// missing propagates to nothing, and neither does an update.
//
// Whether that particular store was short a tag or short everything was never established (the cluster
// was gone before its disk could be read, and the owner's reading is that the wrong bundle had been
// poured in). This check does not depend on which: either way the store cannot resolve the release, and
// either way it must not authorize dropping the upstream.
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
// the air-gap story, and the shape of a defect measured on a live cluster.
//
// Air-gap has been asked for, so the write endpoint is open and `d8 mirror push` has
// put the whole set in the store. The upstream is still HELD, deliberately: the
// cluster has to keep working until the leader is complete. Before this was fixed the
// leader in exactly this state reported nothing at all — the count came from what the
// fill copied, the fill could not even start, and the push it was pushed by went
// unseen. The upstream was then held forever, which is the one outcome the transition
// exists to avoid.
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
	loop.LocalAddress = "127.0.0.1:1"
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
// kubelet counts as a crash, twice per collection whether or not anything was deletable. Measured on
// a cluster: seven restarts per replica, exponential backoff, and a store answering `connection
// refused` for minutes at a time. Serving images is not something housekeeping may interrupt.
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

// TestAnIdlePassDoesNotErasePreviousCompleteness is the cycle a live cluster ran for an hour, and the
// reason it looked like flapping leader election.
//
// A leader fills the store and reports full. The controller sees the storage converged and clears
// `needSync`. With nothing left to do the next pass goes idle — and it used to recompute fullness from
// `expectedDigests`, which this configuration does not state, so the recomputation could only answer
// "not full". The controller then saw a store that was not converged and asked for a fill again. Every
// few seconds. And because eligibility to lead depends on being full, the lease travelled with it, which
// is what made the symptom look like an election problem: three fixes to leadership could not have
// cured a status that erased its own evidence.
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
// a store holding nothing anybody can pull. Measured on `ly-mmc`: 333 MB, twenty manifests sampled
// and twenty missing their layers, three replicas reporting `full` with `safeToDropUpstream: true`.
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
// written, and the store ends up holding manifests naming blobs it does not have.
//
// Measured on `ly-mmc`: a fill of the whole set reporting `written=400, skipped=0` that left the store
// at the same 333 MB and the same 450 blobs, with every layer of every sampled manifest absent from
// disk while the registry answered 200 for it — because the upstream was still there to answer.
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

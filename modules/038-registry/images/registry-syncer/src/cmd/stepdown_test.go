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

package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-syncer/internal/report"
	"github.com/deckhouse/registry-syncer/internal/run"
)

const (
	lossNamespace = "d8-system"
	lossLease     = "registry-storage"
)

// lossStand is defect 52 as the stand showed it: master-0 holds the lease and reports the whole set,
// but its local_data was wiped; master-1 holds the whole set. master-2 holds part of it.
type lossStand struct {
	kube      client.Client
	clientset *k8sfake.Clientset
	set       string
}

func newLossStand(t *testing.T, upstream bool, interceptors interceptor.Funcs, lostNodes ...string) *lossStand {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))
	for _, kind := range []string{"DeckhouseRelease", "ModuleRelease", "ModuleSource"} {
		gvk := schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: kind}
		scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
		scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind(kind+"List"), &unstructured.UnstructuredList{})
	}
	release := &unstructured.Unstructured{Object: map[string]any{
		"spec":   map[string]any{"version": "v1.76.6"},
		"status": map[string]any{"phase": "Deployed"},
	}}
	release.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "DeckhouseRelease"})
	release.SetName("v1.76.6")

	storage := &registryv1alpha1.RegistryStorage{ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName}}
	if upstream {
		storage.Spec.Upstream = &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{Scheme: registryv1alpha1.SchemeHTTPS, Host: "registry.example.com"},
		}
	}
	kube := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&registryv1alpha1.RegistryStorage{}).
		WithObjects(storage, release).
		WithInterceptorFuncs(interceptors).
		Build()

	set, err := run.CurrentSetKey(context.Background(), kube)
	require.NoError(t, err)
	require.NotEmpty(t, set)

	stored := &registryv1alpha1.RegistryStorage{}
	require.NoError(t, kube.Get(context.Background(), client.ObjectKeyFromObject(storage), stored))
	stored.Status.SafeToDropUpstream = true
	stored.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true,
			VerifiedDigests: 453, DeclaredDigests: 453, VerifiedSet: set},
		{Node: "master-1", Role: registryv1alpha1.ReplicaRoleFollower, Full: true,
			VerifiedDigests: 453, DeclaredDigests: 453, VerifiedSet: set, Source: "master-0"},
		{Node: "master-2", Role: registryv1alpha1.ReplicaRoleFollower,
			VerifiedDigests: 300, DeclaredDigests: 453, Source: "master-0"},
	}
	require.NoError(t, kube.Status().Update(context.Background(), stored))

	now := time.Now()
	objects := []runtime.Object{&coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{Name: lossLease, Namespace: lossNamespace},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       ptr.To("master-0"),
			RenewTime:            &metav1.MicroTime{Time: now},
			AcquireTime:          &metav1.MicroTime{Time: now.Add(-time.Hour)},
			LeaseDurationSeconds: ptr.To[int32](60),
		},
	}}
	lost := map[string]bool{}
	for _, node := range lostNodes {
		lost[node] = true
	}
	for _, name := range []string{"master-0", "master-1", "master-2"} {
		status := corev1.ConditionTrue
		if lost[name] {
			status = corev1.ConditionUnknown
		}
		node := nodeIn(status, now.Add(-2*time.Minute))
		node.Name = name
		objects = append(objects, node, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "registry-storage-" + name, Namespace: lossNamespace,
				Labels: map[string]string{"app": "registry-storage"}},
			Spec:   corev1.PodSpec{NodeName: name},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		})
	}
	return &lossStand{kube: kube, clientset: k8sfake.NewClientset(objects...), set: set}
}

// replica is a candidacy of one replica of the stand, its store an empty directory.
func (s *lossStand) replica(t *testing.T, self string) *election {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &election{
		log:        log,
		kubeClient: s.kube,
		clientset:  s.clientset,
		namespace:  lossNamespace,
		leaseName:  lossLease,
		self:       self,
		loop: &run.Loop{
			Log: log, Client: s.kube, Node: self, DataDir: t.TempDir(),
			Publisher: &report.Publisher{Client: s.kube},
		},
		tracker: &leadership{},
		waiting: &run.Waiting{For: passOverAfter, Log: log},
		seen:    &renewals{},
	}
}

// holdsTheRelease puts the deployed release on a replica's disk — its tag, and the manifest it names —
// which is what the full replica's store has and the wiped one's has not.
func holdsTheRelease(t *testing.T, candidacy *election) *election {
	t.Helper()
	const manifest = "sha256:5e1ea5e000000000000000000000000000000000000000000000000000000000"
	v2 := filepath.Join(candidacy.loop.DataDir, "docker", "registry", "v2")
	tag := filepath.Join(v2, "repositories", "system", "deckhouse", "_manifests", "tags", "v1.76.6", "current")
	blob := filepath.Join(v2, "blobs", "sha256", "5e", strings.TrimPrefix(manifest, "sha256:"))
	require.NoError(t, os.MkdirAll(tag, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tag, "link"), []byte(manifest), 0o644))
	require.NoError(t, os.MkdirAll(blob, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(blob, "data"), []byte(`{}`), 0o644))
	return candidacy
}

func (s *lossStand) holder(t *testing.T) string {
	t.Helper()
	lease, err := s.clientset.CoordinationV1().Leases(lossNamespace).Get(context.Background(), lossLease, metav1.GetOptions{})
	require.NoError(t, err)
	return ptr.Deref(lease.Spec.HolderIdentity, "")
}

func (s *lossStand) status(t *testing.T) registryv1alpha1.RegistryStorageStatus {
	t.Helper()
	storage := &registryv1alpha1.RegistryStorage{}
	require.NoError(t, s.kube.Get(context.Background(), client.ObjectKey{Name: registryv1alpha1.SingletonName}, storage))
	return storage.Status
}

// TestALeaderThatLostItsStoreHandsTheLeaseToTheFullReplica: with an upstream the incumbent rule keeps the
// leader beside a full follower, its report being clean; its store, read, holds nothing of the set. It
// takes its report back, gives the lease up, and stays aside; the full replica takes over. The same in
// air-gap, where the leader answered 404 for every image.
func TestALeaderThatLostItsStoreHandsTheLeaseToTheFullReplica(t *testing.T) {
	for _, upstream := range []bool{true, false} {
		stand := newLossStand(t, upstream, interceptor.Funcs{})
		ctx := context.Background()

		if upstream {
			lease := run.Lease{Known: true, Holder: "master-0", Live: true, CanFill: true}
			require.True(t, run.MayLead("master-0", stand.status(t).Replicas, stand.set, lease),
				"by the reports alone the incumbent rule keeps it: the gap this closes")
		}

		leader := stand.replica(t, "master-0")
		ok, why, err := leader.eligible(ctx)
		require.NoError(t, err)
		assert.False(t, ok, "upstream %v", upstream)
		assert.Contains(t, why, "master-1 holds the whole image set and this replica's store does not hold it")
		assert.Empty(t, stand.holder(t), "the lease is given up, not left to run out")

		status := stand.status(t)
		assert.False(t, status.Replicas[0].Full)
		assert.Zero(t, status.Replicas[0].VerifiedDigests)
		assert.False(t, status.SafeToDropUpstream, "nothing is left for the air-gap transition to stand on")

		full := holdsTheRelease(t, stand.replica(t, "master-1"))
		ok, why, err = full.eligible(ctx)
		require.NoError(t, err)
		assert.True(t, ok, "the full replica stands: %s", why)

		ok, _, err = stand.replica(t, "master-2").eligible(ctx)
		require.NoError(t, err)
		assert.False(t, ok, "the replica short of the set does not")

		ok, _, err = leader.eligible(ctx)
		require.NoError(t, err)
		assert.False(t, ok, "and the ex-leader stays aside while the other is full")
	}
}

// TestTheHandoverDoesNotWaitForTheDemotion: the status may refuse the demotion; the ex-leader still
// stays aside — while the other is full, by its step-down standing, which WhyStepAside answers without
// looking at the disk again — and the full replica stands: it reads the ex-leader's stale Full as one
// more full replica, which does not keep it aside, and its own store holds the set.
//
// With the full replica's report no longer counting, there is nobody to step down for and WhyStepAside
// says nothing; the ex-leader's own entry, still Full in the status, is then judged as its step-down
// left it — see WithOwnLoss — and the fullest of the rest is the choice, not the empty store. The
// others read that stale Full until the demotion lands; the empty store does not lead on it.
func TestTheHandoverDoesNotWaitForTheDemotion(t *testing.T) {
	refuse := interceptor.Funcs{SubResourcePatch: func(
		context.Context, client.Client, string, client.Object, client.Patch, ...client.SubResourcePatchOption,
	) error {
		return errors.New("the server is currently unable to handle the request")
	}}
	stand := newLossStand(t, true, refuse)
	ctx := context.Background()

	leader := stand.replica(t, "master-0")
	ok, _, err := leader.eligible(ctx)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.True(t, stand.status(t).Replicas[0].Full, "the demotion was refused")
	assert.Empty(t, stand.holder(t))

	ok, why, err := holdsTheRelease(t, stand.replica(t, "master-1")).eligible(ctx)
	require.NoError(t, err)
	assert.True(t, ok, why)

	ok, _, err = leader.eligible(ctx)
	require.NoError(t, err)
	assert.False(t, ok, "the incumbent rule does not hand it back")

	t.Run("nobody full beside it", func(t *testing.T) {
		// master-1's node goes unheard from, so its report no longer counts.
		gone := nodeIn(corev1.ConditionUnknown, time.Now().Add(-2*time.Minute))
		gone.Name = "master-1"
		_, err := stand.clientset.CoreV1().Nodes().Update(ctx, gone, metav1.UpdateOptions{})
		require.NoError(t, err)
		require.True(t, stand.status(t).Replicas[0].Full, "the demotion is still not in the status")

		ok, why, err := leader.eligible(ctx)
		require.NoError(t, err)
		assert.False(t, ok, "an empty store does not lead by a report it took back")
		assert.Contains(t, why, "master-2", "the fullest of the rest is the choice, not the empty store")
	})
}

// TestNoStepDownWithoutAFullReplicaThatIsThere: the only other full replica on a node unheard from is
// not one to hand over to, and neither is a replica that holds only part of the set. The leader keeps
// the lease; its report, over an empty store, goes back — see run.Loop.TakeBackFull.
func TestNoStepDownWithoutAFullReplicaThatIsThere(t *testing.T) {
	stand := newLossStand(t, true, interceptor.Funcs{}, "master-1")

	ok, why, err := stand.replica(t, "master-0").eligible(context.Background())
	require.NoError(t, err)
	assert.True(t, ok, why)
	assert.Equal(t, "master-0", stand.holder(t))
	status := stand.status(t)
	assert.False(t, status.Replicas[0].Full, "its report goes back all the same")
	assert.Equal(t, registryv1alpha1.ReplicaRoleLeader, status.Replicas[0].Role)
	assert.False(t, status.SafeToDropUpstream)
}

// nobodyElseFull makes master-1 a replica still copying: master-0 is the only one whose report says it
// holds the set — a single master, or followers that have not finished.
func (s *lossStand) nobodyElseFull(t *testing.T) *lossStand {
	t.Helper()
	storage := &registryv1alpha1.RegistryStorage{}
	require.NoError(t, s.kube.Get(context.Background(), client.ObjectKey{Name: registryv1alpha1.SingletonName}, storage))
	storage.Status.Replicas[1].Full = false
	storage.Status.Replicas[1].VerifiedSet = ""
	storage.Status.Replicas[1].VerifiedDigests = 400
	require.NoError(t, s.kube.Status().Update(context.Background(), storage))
	return s
}

// TestALeaderWithNobodyElseFullTakesItsReportBackAndKeepsTheLease is the gap the controller guard left
// open: a store wiped under a running leader while no other replica holds the set. Its report, full and
// stamped by the running syncer, stood until the recheck six hours on, and a request for air-gap dropped
// the upstream on it. Now the report goes back — Full, the count, the permission — while the leader keeps
// the lease and its role, and keeps standing: there is nobody to hand over to, and it is the one that
// fills. Once master-1 holds the whole set, the leader steps down in its favour.
func TestALeaderWithNobodyElseFullTakesItsReportBackAndKeepsTheLease(t *testing.T) {
	for _, upstream := range []bool{true, false} {
		stand := newLossStand(t, upstream, interceptor.Funcs{}).nobodyElseFull(t)
		ctx := context.Background()

		leader := stand.replica(t, "master-0")
		ok, why, err := leader.eligible(ctx)
		require.NoError(t, err)
		assert.True(t, ok, "upstream %v: %s", upstream, why)
		assert.Equal(t, "master-0", stand.holder(t), "the lease is kept")

		status := stand.status(t)
		assert.False(t, status.Replicas[0].Full)
		assert.Zero(t, status.Replicas[0].VerifiedDigests)
		assert.Equal(t, registryv1alpha1.ReplicaRoleLeader, status.Replicas[0].Role)
		assert.False(t, status.SafeToDropUpstream, "nothing is left for the air-gap transition to stand on")
		assert.EqualValues(t, 400, status.Replicas[1].VerifiedDigests, "the other replica is left as it was")
		assert.True(t, leader.loop.Lost())

		ok, why, err = leader.eligible(ctx)
		require.NoError(t, err)
		assert.True(t, ok, "it keeps standing on the lease it holds: %s", why)

		ok, _, err = stand.replica(t, "master-1").eligible(ctx)
		require.NoError(t, err)
		assert.False(t, ok, "nobody is full and the lease stays where the fill is")

		// master-1 finishes copying.
		storage := &registryv1alpha1.RegistryStorage{}
		require.NoError(t, stand.kube.Get(ctx, client.ObjectKey{Name: registryv1alpha1.SingletonName}, storage))
		storage.Status.Replicas[1].Full = true
		storage.Status.Replicas[1].VerifiedSet = stand.set
		storage.Status.Replicas[1].VerifiedDigests = 453
		require.NoError(t, stand.kube.Status().Update(ctx, storage))

		// With an upstream the incumbent rule would keep it and the step-down says otherwise; in air-gap
		// the rule says so itself. Either way the step-down it owed is made: the role and the lease.
		ok, why, err = leader.eligible(ctx)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, why, "master-1 holds the whole image set and this replica")
		assert.Empty(t, stand.holder(t), "the lease is given up, not left to run out")
		assert.Equal(t, registryv1alpha1.ReplicaRoleFollower, stand.status(t).Replicas[0].Role)
		assert.False(t, leader.loop.TookBack(), "the step-down is made once")

		ok, _, err = holdsTheRelease(t, stand.replica(t, "master-1")).eligible(ctx)
		require.NoError(t, err)
		assert.True(t, ok, "the full replica stands")
	}
}

// TestALeaderWithNobodyElseFullAndTheSetOnItsDiskKeepsItsReport: the release on the disk — the check
// finds nothing, and nothing goes back.
func TestALeaderWithNobodyElseFullAndTheSetOnItsDiskKeepsItsReport(t *testing.T) {
	stand := newLossStand(t, true, interceptor.Funcs{}).nobodyElseFull(t)

	leader := holdsTheRelease(t, stand.replica(t, "master-0"))
	ok, why, err := leader.eligible(context.Background())
	require.NoError(t, err)
	assert.True(t, ok, why)
	assert.True(t, stand.status(t).Replicas[0].Full)
	assert.True(t, stand.status(t).SafeToDropUpstream)
	assert.False(t, leader.loop.Lost())
}

// TestARestartedLeaderStepsDownBeforeItsRegistryStarts: the pod recreated over a wiped store, the lease
// still names it and its report from the previous process still says full. Before the loop's first
// pass — before the registry is configured — it takes the report back and gives the lease up, which
// shortens the time the controller can read that report to the pod's downtime and this check.
func TestARestartedLeaderStepsDownBeforeItsRegistryStarts(t *testing.T) {
	stand := newLossStand(t, false, interceptor.Funcs{})
	// Its last renewal from the previous process, past the lease's grace: the controller reads the record
	// all the same.
	lease, err := stand.clientset.CoordinationV1().Leases(lossNamespace).Get(context.Background(), lossLease, metav1.GetOptions{})
	require.NoError(t, err)
	lease.Spec.RenewTime = &metav1.MicroTime{Time: time.Now().Add(-10 * time.Minute)}
	_, err = stand.clientset.CoordinationV1().Leases(lossNamespace).Update(context.Background(), lease, metav1.UpdateOptions{})
	require.NoError(t, err)

	leader := stand.replica(t, "master-0")
	leader.stepDownAtStart(context.Background())

	assert.True(t, leader.loop.Lost())
	assert.False(t, stand.status(t).Replicas[0].Full)
	assert.False(t, stand.status(t).SafeToDropUpstream)
	assert.Empty(t, stand.holder(t))

	t.Run("a replica the lease does not name", func(t *testing.T) {
		stand := newLossStand(t, false, interceptor.Funcs{})
		other := stand.replica(t, "master-2")
		other.stepDownAtStart(context.Background())
		assert.False(t, other.loop.Lost())
		assert.Equal(t, "master-0", stand.holder(t))
	})

	t.Run("nobody else full", func(t *testing.T) {
		// With an upstream, where the transition can drop it: the report goes back before the registry
		// serves, and the lease stays — this replica is the one that fills.
		stand := newLossStand(t, true, interceptor.Funcs{}).nobodyElseFull(t)
		leader := stand.replica(t, "master-0")
		leader.stepDownAtStart(context.Background())

		assert.True(t, leader.loop.Lost())
		status := stand.status(t)
		assert.False(t, status.Replicas[0].Full)
		assert.Equal(t, registryv1alpha1.ReplicaRoleLeader, status.Replicas[0].Role)
		assert.False(t, status.SafeToDropUpstream)
		assert.Equal(t, "master-0", stand.holder(t), "the lease record still names it")
	})

	t.Run("nobody else full and the release on the disk", func(t *testing.T) {
		stand := newLossStand(t, true, interceptor.Funcs{}).nobodyElseFull(t)
		leader := holdsTheRelease(t, stand.replica(t, "master-0"))
		leader.stepDownAtStart(context.Background())

		assert.False(t, leader.loop.Lost())
		assert.True(t, stand.status(t).Replicas[0].Full)
		assert.Equal(t, "master-0", stand.holder(t))
	})
}

// TestReleaseLeaseOnlyGivesUpItsOwn: the lease is given up the way the elector releases it, and only
// while it carries this replica's name.
func TestReleaseLeaseOnlyGivesUpItsOwn(t *testing.T) {
	stand := newLossStand(t, true, interceptor.Funcs{})
	ctx := context.Background()

	require.NoError(t, releaseLease(ctx, stand.clientset, lossNamespace, lossLease, "master-1"))
	assert.Equal(t, "master-0", stand.holder(t), "not this replica's to give up")

	require.NoError(t, releaseLease(ctx, stand.clientset, lossNamespace, lossLease, "master-0"))
	lease, err := stand.clientset.CoordinationV1().Leases(lossNamespace).Get(ctx, lossLease, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Empty(t, ptr.Deref(lease.Spec.HolderIdentity, "x"))
	assert.EqualValues(t, 1, ptr.Deref(lease.Spec.LeaseDurationSeconds, 0))
	assert.Equal(t, run.Lease{Known: true}, leaseAt(lease, time.Now()), "nobody leads, and the next takes it at once")

	require.NoError(t, releaseLease(ctx, k8sfake.NewClientset(), lossNamespace, lossLease, "master-0"),
		"no lease, nothing to give up")
}

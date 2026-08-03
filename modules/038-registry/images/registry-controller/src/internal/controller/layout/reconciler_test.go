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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/registry-controller/internal/probe"
)

func upstream(host string) *registryv1alpha1.Upstream {
	return &registryv1alpha1.Upstream{
		Endpoint: registryv1alpha1.Endpoint{
			Scheme: registryv1alpha1.SchemeHTTPS,
			Host:   host,
			Path:   "/deckhouse/ee",
		},
	}
}

func source() *registryv1alpha1.StorageSource {
	return &registryv1alpha1.StorageSource{BundleRef: "d8-mirror-bundle", ExpectedDigests: 459}
}

func registryConfig(spec registryv1alpha1.RegistryConfigSpec) *registryv1alpha1.RegistryConfig {
	return &registryv1alpha1.RegistryConfig{
		ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName, Generation: 1},
		Spec:       spec,
	}
}

func node(name string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, UID: types.UID("uid-" + name)}}
}

func accessSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: Namespace, Name: StorageAccessSecretName},
		Data: map[string][]byte{
			storageAccessCAKey:       []byte("-----BEGIN CERTIFICATE-----storage"),
			storageAccessUsernameKey: []byte("ro"),
			storageAccessPasswordKey: []byte("secret"),
			// Node addresses, which is what a node agent can actually dial. Given
			// without a port, as the hook discovers them.
			storageAccessAddressesKey: []byte("10.0.0.1,10.0.0.2"),
		},
	}
}

// stubProber lets a test decide the probe outcome, and records what was probed so
// that "probed only on change" can be asserted.
type stubProber struct {
	err    error
	probed []string
}

func (s *stubProber) Probe(_ context.Context, upstream *registryv1alpha1.Upstream) error {
	if upstream == nil {
		return nil
	}
	s.probed = append(s.probed, upstream.Host)
	return s.err
}

func newReconciler(t *testing.T, objects ...client.Object) (*Reconciler, client.Client) {
	t.Helper()

	r, c, _ := newReconcilerWithProber(t, &stubProber{}, objects...)
	return r, c
}

func newReconcilerWithProber(
	t *testing.T, prober *stubProber, objects ...client.Object,
) (*Reconciler, client.Client, *stubProber) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithStatusSubresource(
			&registryv1alpha1.RegistryConfig{},
			&registryv1alpha1.RegistryStorage{},
			&registryv1alpha1.RegistryNode{},
			&registryv1alpha1.RegistryUpstream{},
		).
		Build()

	r := &Reconciler{Prober: prober}
	r.InjectClient(fakeClient)
	return r, fakeClient, prober
}

func getConfig(t *testing.T, c client.Client) *registryv1alpha1.RegistryConfig {
	t.Helper()

	cfg := &registryv1alpha1.RegistryConfig{}
	require.NoError(t, c.Get(context.Background(),
		types.NamespacedName{Name: registryv1alpha1.SingletonName}, cfg))
	return cfg
}

func getUpstream(t *testing.T, c client.Client, name string) *registryv1alpha1.RegistryUpstream {
	t.Helper()

	upstreamObj := &registryv1alpha1.RegistryUpstream{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: name}, upstreamObj))
	return upstreamObj
}

func additionalUpstream(name, match, host string) *registryv1alpha1.RegistryUpstream {
	return &registryv1alpha1.RegistryUpstream{
		ObjectMeta: metav1.ObjectMeta{Name: name, Generation: 1},
		Spec: registryv1alpha1.RegistryUpstreamSpec{
			Match: match,
			Upstream: registryv1alpha1.Upstream{
				Endpoint: registryv1alpha1.Endpoint{
					Scheme: registryv1alpha1.SchemeHTTPS, Host: host, Path: "/images",
				},
			},
		},
	}
}

func runReconcile(t *testing.T, r *Reconciler) {
	t.Helper()

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: registryv1alpha1.SingletonName},
	})
	require.NoError(t, err)
}

func getStorage(t *testing.T, c client.Client) *registryv1alpha1.RegistryStorage {
	t.Helper()

	storage := &registryv1alpha1.RegistryStorage{}
	require.NoError(t, c.Get(context.Background(),
		types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage))
	return storage
}

func listNodes(t *testing.T, c client.Client) map[string]registryv1alpha1.RegistryNode {
	t.Helper()

	list := &registryv1alpha1.RegistryNodeList{}
	require.NoError(t, c.List(context.Background(), list))

	out := make(map[string]registryv1alpha1.RegistryNode, len(list.Items))
	for i := range list.Items {
		out[list.Items[i].Name] = list.Items[i]
	}
	return out
}

func TestReconcileCreatesTheLayout(t *testing.T) {
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Primary: registryv1alpha1.PrimarySource{Upstream: upstream("registry.deckhouse.io")},
		Storage: registryv1alpha1.StorageConfig{Cache: true, Size: "50Gi", Source: source()},
	})

	r, c := newReconciler(t, cfg, accessSecret(), node("master-0"), node("worker-1"))
	runReconcile(t, r)

	storage := getStorage(t, c)
	require.NotNil(t, storage.Spec.Upstream)
	assert.Equal(t, "registry.deckhouse.io", storage.Spec.Upstream.Host)
	assert.Equal(t, constant.StorePath, storage.Spec.Store.Path)
	// No replica has reported yet, so nothing may look complete.
	assert.Equal(t, registryv1alpha1.StoragePhaseIdle, storage.Status.Phase)
	assert.False(t, storage.Status.SafeToDropUpstream)

	nodes := listNodes(t, c)
	require.Len(t, nodes, 2)
	for name, layoutObj := range nodes {
		assert.Truef(t, layoutObj.Spec.Cache, "node %s", name)
		require.Lenf(t, layoutObj.Spec.Backends, 2, "node %s", name)
		// An address, not `constant.Host`. That name identifies the image set and is
		// what a request is matched against, but a node agent cannot resolve it: it
		// runs in the host network, where cluster DNS does not exist.
		assert.Equal(t, "10.0.0.1:5001", layoutObj.Spec.Backends[0].Host)
		assert.Equal(t, "-----BEGIN CERTIFICATE-----storage", layoutObj.Spec.Backends[0].CA)

		// Owned by its Node, so a removed node takes its layout with it without the
		// controller having to notice.
		require.Len(t, layoutObj.OwnerReferences, 1)
		assert.Equal(t, "Node", layoutObj.OwnerReferences[0].Kind)
		assert.Equal(t, name, layoutObj.OwnerReferences[0].Name)
	}
}

func TestReconcileRemovesTheLayoutOfAGoneNode(t *testing.T) {
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Primary: registryv1alpha1.PrimarySource{Upstream: upstream("registry.deckhouse.io")},
		Storage: registryv1alpha1.StorageConfig{Cache: true, Source: source()},
	})
	stale := &registryv1alpha1.RegistryNode{ObjectMeta: metav1.ObjectMeta{Name: "worker-gone"}}

	r, c := newReconciler(t, cfg, accessSecret(), node("master-0"), stale)
	runReconcile(t, r)

	nodes := listNodes(t, c)
	assert.Contains(t, nodes, "master-0")
	assert.NotContains(t, nodes, "worker-gone")
}

func TestReconcileTurningTheCacheOffRemovesTheStorage(t *testing.T) {
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Primary: registryv1alpha1.PrimarySource{Upstream: upstream("registry.deckhouse.io")},
		Storage: registryv1alpha1.StorageConfig{Cache: false},
	})
	existing := &registryv1alpha1.RegistryStorage{
		ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName},
		Spec:       registryv1alpha1.RegistryStorageSpec{Upstream: upstream("registry.deckhouse.io")},
	}

	// No access secret on purpose: with the cache off the storage is not consulted
	// at all, so its credentials must not be required.
	r, c := newReconciler(t, cfg, existing, node("master-0"))
	runReconcile(t, r)

	err := c.Get(context.Background(),
		types.NamespacedName{Name: registryv1alpha1.SingletonName}, &registryv1alpha1.RegistryStorage{})
	assert.True(t, apierrors.IsNotFound(err), "no storage object may survive with the cache off")

	layoutObj := listNodes(t, c)["master-0"]
	assert.False(t, layoutObj.Spec.Cache)
	require.Len(t, layoutObj.Spec.Backends, 1)
	assert.Equal(t, registryv1alpha1.BackendUpstream, layoutObj.Spec.Backends[0].Name)
}

// TestReconcileAirGapTransition walks the transition end to end through the
// objects, which is the one sequence where a mistake cuts every node off from
// images.
func TestReconcileAirGapTransition(t *testing.T) {
	// The cluster runs a pass-through cache.
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Primary: registryv1alpha1.PrimarySource{Upstream: upstream("registry.deckhouse.io")},
		Storage: registryv1alpha1.StorageConfig{Cache: true, Size: "50Gi", Source: source()},
	})

	r, c := newReconciler(t, cfg, accessSecret(), node("master-0"))
	ctx := context.Background()
	runReconcile(t, r)

	// The user removes the upstream, asking for air-gap.
	live := &registryv1alpha1.RegistryConfig{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, live))
	live.Spec.Primary.Upstream = nil
	live.Generation = 2
	require.NoError(t, c.Update(ctx, live))

	runReconcile(t, r)

	// Nothing has reported, so the upstream is held and the nodes keep their
	// fallback: the request alone must not cut anyone off.
	storage := getStorage(t, c)
	require.NotNil(t, storage.Spec.Upstream, "the last applied upstream must be held until the cache is complete")
	assert.True(t, storage.Spec.Publish, "an air-gapped cache is filled through the write endpoint")
	assert.Len(t, listNodes(t, c)["master-0"].Spec.Backends, 2)

	// The leader reports it is filling.
	storage = getStorage(t, c)
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: false, VerifiedDigests: 312},
	}
	require.NoError(t, c.Status().Update(ctx, storage))

	runReconcile(t, r)

	storage = getStorage(t, c)
	require.NotNil(t, storage.Spec.Upstream, "a partially filled cache is not a reason to drop the upstream")
	assert.Equal(t, registryv1alpha1.StoragePhaseFilling, storage.Status.Phase)
	require.NotNil(t, storage.Status.Fill)
	assert.EqualValues(t, 312, storage.Status.Fill.Filled)
	assert.EqualValues(t, 459, storage.Status.Fill.Total)

	// The leader reports it is complete.
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 459},
	}
	require.NoError(t, c.Status().Update(ctx, storage))

	runReconcile(t, r)

	storage = getStorage(t, c)
	assert.Nil(t, storage.Spec.Upstream, "the cache is complete, so the upstream is dropped")
	assert.True(t, storage.Status.Authoritative)
	assert.True(t, storage.Status.SafeToDropUpstream)
	assert.Equal(t, registryv1alpha1.StoragePhaseReady, storage.Status.Phase)

	converged := apimeta.FindStatusCondition(
		storage.Status.Conditions, registryv1alpha1.ConditionStorageConverged)
	require.NotNil(t, converged)
	assert.Equal(t, metav1.ConditionTrue, converged.Status)

	// The upstream leaves the node layouts in the same reconciliation, so no node
	// is ever left pointing at an upstream the storage has forgotten.
	layoutObj := listNodes(t, c)["master-0"]
	require.Len(t, layoutObj.Spec.Backends, 1)
	assert.Equal(t, registryv1alpha1.BackendStorage, layoutObj.Spec.Backends[0].Name)
}

// TestReconcileKeepsTheUpstreamWhenTheLeaderFails is the failure counterpart: a
// broken fill must never look like a complete cache.
func TestReconcileKeepsTheUpstreamWhenTheLeaderFails(t *testing.T) {
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Storage: registryv1alpha1.StorageConfig{Cache: true, Source: source()},
	})
	storage := &registryv1alpha1.RegistryStorage{
		ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName},
		Spec:       registryv1alpha1.RegistryStorageSpec{Upstream: upstream("registry.deckhouse.io")},
		Status: registryv1alpha1.RegistryStorageStatus{
			Replicas: []registryv1alpha1.StorageReplicaStatus{{
				Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader,
				Full: true, Error: "verification failed: 12 digests missing",
			}},
		},
	}

	r, c := newReconciler(t, cfg, accessSecret(), storage, node("master-0"))
	runReconcile(t, r)

	got := getStorage(t, c)
	require.NotNil(t, got.Spec.Upstream, "a leader reporting a failure must not authorize going air-gap")
	assert.Equal(t, registryv1alpha1.StoragePhaseFailed, got.Status.Phase)
	assert.False(t, got.Status.SafeToDropUpstream)
	assert.Len(t, listNodes(t, c)["master-0"].Spec.Backends, 2)
}

func TestReconcileSkipsAnInvalidConfig(t *testing.T) {
	// Managed with neither an upstream nor a cache. Compiling this into the node
	// layouts would push the mistake all the way to containerd.
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{Mode: registryv1alpha1.ModeManaged})

	r, c := newReconciler(t, cfg, node("master-0"))
	runReconcile(t, r)

	assert.Empty(t, listNodes(t, c))
}

func TestReconcileUnmanagedTouchesNothing(t *testing.T) {
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{Mode: registryv1alpha1.ModeUnmanaged})

	r, c := newReconciler(t, cfg, node("master-0"))
	runReconcile(t, r)

	assert.Empty(t, listNodes(t, c))
	err := c.Get(context.Background(),
		types.NamespacedName{Name: registryv1alpha1.SingletonName}, &registryv1alpha1.RegistryStorage{})
	assert.True(t, apierrors.IsNotFound(err))
}

func TestReconcileWithoutAConfigLeavesEverythingAlone(t *testing.T) {
	// The data plane keeps serving from what it already has, which is the point of
	// the design, so a missing configuration must not tear the layout down.
	existing := &registryv1alpha1.RegistryNode{
		ObjectMeta: metav1.ObjectMeta{Name: "master-0"},
		Spec:       registryv1alpha1.RegistryNodeSpec{Cache: true},
	}

	r, c := newReconciler(t, existing, node("master-0"))
	runReconcile(t, r)

	assert.Contains(t, listNodes(t, c), "master-0")
}

func TestReconcileFailsLoudlyWithoutStorageAccess(t *testing.T) {
	// Writing a layout without the storage certificate authority would leave every
	// agent unable to pull from the cache. Better to retry than to publish it.
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Primary: registryv1alpha1.PrimarySource{Upstream: upstream("registry.deckhouse.io")},
		Storage: registryv1alpha1.StorageConfig{Cache: true, Source: source()},
	})

	r, c := newReconciler(t, cfg, node("master-0"))

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: registryv1alpha1.SingletonName},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), StorageAccessSecretName)
	assert.Empty(t, listNodes(t, c))
}

func TestReconcileIsIdempotent(t *testing.T) {
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Primary: registryv1alpha1.PrimarySource{Upstream: upstream("registry.deckhouse.io")},
		Storage: registryv1alpha1.StorageConfig{Cache: true, Size: "50Gi", Source: source()},
	})

	r, c := newReconciler(t, cfg, accessSecret(), node("master-0"))

	runReconcile(t, r)
	storageVersion := getStorage(t, c).ResourceVersion
	nodeVersion := listNodes(t, c)["master-0"].ResourceVersion

	runReconcile(t, r)

	assert.Equal(t, storageVersion, getStorage(t, c).ResourceVersion,
		"an unchanged storage must not be rewritten")
	assert.Equal(t, nodeVersion, listNodes(t, c)["master-0"].ResourceVersion,
		"an unchanged node layout must not be rewritten: every rewrite wakes an agent")
}

// TestReconcileCorrectsAHandEditedLayout covers the drift case: the agent applies
// whatever is in its object, so a hand edit has to be undone.
func TestReconcileCorrectsAHandEditedLayout(t *testing.T) {
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Primary: registryv1alpha1.PrimarySource{Upstream: upstream("registry.deckhouse.io")},
		Storage: registryv1alpha1.StorageConfig{Cache: true, Source: source()},
	})

	r, c := newReconciler(t, cfg, accessSecret(), node("master-0"))
	ctx := context.Background()
	runReconcile(t, r)

	tampered := listNodes(t, c)["master-0"]
	tampered.Spec.Cache = false
	tampered.Spec.Backends = nil
	require.NoError(t, c.Update(ctx, &tampered))

	runReconcile(t, r)

	restored := listNodes(t, c)["master-0"]
	assert.True(t, restored.Spec.Cache)
	assert.Len(t, restored.Spec.Backends, 2)
}

// TestReconcileHoldsLastKnownGoodOnAFailedProbe is the "changed the license to a
// broken one, kept the cluster running" path, end to end through the objects.
func TestReconcileHoldsLastKnownGoodOnAFailedProbe(t *testing.T) {
	working := upstream("registry.deckhouse.io")

	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Primary: registryv1alpha1.PrimarySource{Upstream: working},
		Storage: registryv1alpha1.StorageConfig{Cache: true, Source: source()},
	})

	prober := &stubProber{}
	r, c, _ := newReconcilerWithProber(t, prober, cfg, accessSecret(), node("master-0"))
	ctx := context.Background()

	// The first pass probes the new upstream and records it as effective.
	runReconcile(t, r)
	assert.Equal(t, []string{"registry.deckhouse.io"}, prober.probed)

	live := getConfig(t, c)
	require.NotNil(t, live.Status.EffectiveUpstream)
	assert.Equal(t, "registry.deckhouse.io", live.Status.EffectiveUpstream.Host)

	valid := apimeta.FindStatusCondition(live.Status.Conditions, registryv1alpha1.ConditionUpstreamValid)
	require.NotNil(t, valid)
	assert.Equal(t, metav1.ConditionTrue, valid.Status)

	// An unchanged upstream is not probed again: a transient outage of the registry
	// must not roll the configuration back.
	prober.probed = nil
	runReconcile(t, r)
	assert.Empty(t, prober.probed)

	// The operator switches to a license without access.
	prober.err = &probe.Failure{Kind: probe.FailureAuth, Message: "the credentials were rejected"}
	live = getConfig(t, c)
	live.Spec.Primary.Upstream = upstream("registry.wrong.example.com")
	live.Generation = 2
	require.NoError(t, c.Update(ctx, live))

	result, err := r.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: registryv1alpha1.SingletonName},
	})
	require.NoError(t, err, "a bad upstream must not fail the reconciliation; the rest of the layout still applies")
	assert.NotZero(t, result.RequeueAfter, "a registry that was merely down has to be picked up on its own")

	// The cluster stays on the upstream that works, in the storage and on the node.
	storage := getStorage(t, c)
	require.NotNil(t, storage.Spec.Upstream)
	assert.Equal(t, "registry.deckhouse.io", storage.Spec.Upstream.Host)

	layoutObj := listNodes(t, c)["master-0"]
	require.Len(t, layoutObj.Spec.Backends, 2)
	assert.Equal(t, "registry.deckhouse.io/deckhouse/ee", layoutObj.Spec.Backends[1].Address())

	live = getConfig(t, c)
	assert.Equal(t, "registry.deckhouse.io", live.Status.EffectiveUpstream.Host)

	valid = apimeta.FindStatusCondition(live.Status.Conditions, registryv1alpha1.ConditionUpstreamValid)
	require.NotNil(t, valid)
	assert.Equal(t, metav1.ConditionFalse, valid.Status)
	assert.Equal(t, registryv1alpha1.ReasonAuthFailed, valid.Reason,
		"the reason has to name what the operator must fix")
	assert.Contains(t, valid.Message, "rejected")
}

// TestReconcileSwitchesAfterAGoodProbe is the counterpart: a correct new upstream
// does take effect.
func TestReconcileSwitchesAfterAGoodProbe(t *testing.T) {
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Primary: registryv1alpha1.PrimarySource{Upstream: upstream("registry.deckhouse.io")},
		Storage: registryv1alpha1.StorageConfig{Cache: true, Source: source()},
	})

	r, c := newReconciler(t, cfg, accessSecret(), node("master-0"))
	ctx := context.Background()
	runReconcile(t, r)

	live := getConfig(t, c)
	live.Spec.Primary.Upstream = upstream("registry.internal.example.com")
	live.Generation = 2
	require.NoError(t, c.Update(ctx, live))

	runReconcile(t, r)

	assert.Equal(t, "registry.internal.example.com", getStorage(t, c).Spec.Upstream.Host)
	assert.Equal(t, "registry.internal.example.com", getConfig(t, c).Status.EffectiveUpstream.Host)
}

// TestReconcileAirGapIsNotProbed keeps the two gates apart: going air-gap is gated
// on cache completeness, not on probing an upstream that is being removed.
func TestReconcileAirGapIsNotProbed(t *testing.T) {
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Storage: registryv1alpha1.StorageConfig{Cache: true, Source: source()},
	})

	prober := &stubProber{err: &probe.Failure{Kind: probe.FailureUnreachable, Message: "boom"}}
	r, c, _ := newReconcilerWithProber(t, prober, cfg, accessSecret(), node("master-0"))
	runReconcile(t, r)

	assert.Empty(t, prober.probed)

	valid := apimeta.FindStatusCondition(
		getConfig(t, c).Status.Conditions, registryv1alpha1.ConditionUpstreamValid)
	require.NotNil(t, valid)
	assert.Equal(t, metav1.ConditionTrue, valid.Status)
	assert.Equal(t, registryv1alpha1.ReasonAirGap, valid.Reason)
}

func TestReconcileCompilesAcceptedUpstreams(t *testing.T) {
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Primary: registryv1alpha1.PrimarySource{Upstream: upstream("registry.deckhouse.io")},
		Storage: registryv1alpha1.StorageConfig{Cache: true, Source: source()},
	})
	vendor := additionalUpstream("virtualization", "images.virtualization.example.com", "vendor.example.com")

	r, c := newReconciler(t, cfg, accessSecret(), node("master-0"), vendor)
	runReconcile(t, r)

	layoutObj := listNodes(t, c)["master-0"]
	require.Len(t, layoutObj.Spec.AdditionalRoutes, 1)
	assert.Equal(t, "images.virtualization.example.com", layoutObj.Spec.AdditionalRoutes[0].Match)
	assert.Equal(t, "vendor.example.com/images", layoutObj.Spec.AdditionalRoutes[0].Address())

	accepted := getUpstream(t, c, "virtualization")
	assert.True(t, accepted.Status.Accepted)
	assert.Empty(t, accepted.Status.Conflict)

	condition := apimeta.FindStatusCondition(accepted.Status.Conditions, registryv1alpha1.ConditionAccepted)
	require.NotNil(t, condition)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
}

// TestReconcileRejectsAnUpstreamShadowingThePrimary checks the rejection reaches
// both the writer and the nodes: the status explains it, and no node routes it.
func TestReconcileRejectsAnUpstreamShadowingThePrimary(t *testing.T) {
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Primary: registryv1alpha1.PrimarySource{Upstream: upstream("registry.deckhouse.io")},
		Storage: registryv1alpha1.StorageConfig{Cache: true, Source: source()},
	})
	hijack := additionalUpstream("hijack", "registry.deckhouse.io", "attacker.example.com")

	r, c := newReconciler(t, cfg, accessSecret(), node("master-0"), hijack)
	runReconcile(t, r)

	assert.Empty(t, listNodes(t, c)["master-0"].Spec.AdditionalRoutes,
		"a rejected upstream must never reach a node")

	rejected := getUpstream(t, c, "hijack")
	assert.False(t, rejected.Status.Accepted)
	assert.Contains(t, rejected.Status.Conflict, "the primary upstream of Deckhouse components")

	condition := apimeta.FindStatusCondition(rejected.Status.Conditions, registryv1alpha1.ConditionAccepted)
	require.NotNil(t, condition)
	assert.Equal(t, metav1.ConditionFalse, condition.Status)
	assert.Equal(t, registryv1alpha1.ReasonShadowsPrimary, condition.Reason)
}

func TestReconcileReportsAMatchCollisionToTheLoser(t *testing.T) {
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Primary: registryv1alpha1.PrimarySource{Upstream: upstream("registry.deckhouse.io")},
		Storage: registryv1alpha1.StorageConfig{Cache: true, Source: source()},
	})

	incumbent := additionalUpstream("incumbent", "ghcr.io", "first.example.com")
	incumbent.CreationTimestamp = metav1.NewTime(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	newcomer := additionalUpstream("newcomer", "ghcr.io", "second.example.com")
	newcomer.CreationTimestamp = metav1.NewTime(time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC))

	r, c := newReconciler(t, cfg, accessSecret(), node("master-0"), incumbent, newcomer)
	runReconcile(t, r)

	routes := listNodes(t, c)["master-0"].Spec.AdditionalRoutes
	require.Len(t, routes, 1)
	assert.Equal(t, "first.example.com/images", routes[0].Address())

	assert.True(t, getUpstream(t, c, "incumbent").Status.Accepted)

	loser := getUpstream(t, c, "newcomer")
	assert.False(t, loser.Status.Accepted)
	assert.Contains(t, loser.Status.Conflict, "RegistryUpstream/incumbent")
}

// TestReconcileAddressesTheStorageByNodeAddress is the fix for a layout that looked
// right and could not work: a node agent runs in the host network and resolves through
// the host's resolver, where `registry.d8-system.svc` does not exist. The Service name
// stays what image references are built from; what gets dialled has to be an address.
func TestReconcileAddressesTheStorageByNodeAddress(t *testing.T) {
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Primary: registryv1alpha1.PrimarySource{Upstream: upstream("registry.deckhouse.io")},
		Storage: registryv1alpha1.StorageConfig{Cache: true, Source: source()},
	})

	r, c := newReconciler(t, cfg, accessSecret(), node("master-0"))
	runReconcile(t, r)

	applied := listNodes(t, c)["master-0"]
	storage := applied.Spec.Backend(registryv1alpha1.BackendStorage)
	require.NotNil(t, storage)

	assert.Equal(t, "10.0.0.1:5001", storage.Host,
		"the layout names something no node can resolve")
	// The second replica is a mirror of the same backend, not a second backend: it
	// holds the same content, so it is one source reachable in two places.
	require.Len(t, storage.Mirrors, 1)
	assert.Equal(t, "10.0.0.2:5001", storage.Mirrors[0].Host)

	// The credentials and authority travel to every one of them, since any may answer.
	assert.Equal(t, "ro", storage.Auth.Username)
	assert.Equal(t, "ro", storage.Mirrors[0].Auth.Username)
	assert.Equal(t, storage.CA, storage.Mirrors[0].CA)
}

// TestReconcileRefusesAStorageWithNoAddress: without an address the cache cannot be
// reached at all, and writing the layout anyway would produce a cluster where every
// node fails to pull with an error that says nothing about a missing secret key.
func TestReconcileRefusesAStorageWithNoAddress(t *testing.T) {
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Primary: registryv1alpha1.PrimarySource{Upstream: upstream("registry.deckhouse.io")},
		Storage: registryv1alpha1.StorageConfig{Cache: true, Source: source()},
	})

	secret := accessSecret()
	delete(secret.Data, storageAccessAddressesKey)

	r, c := newReconciler(t, cfg, secret, node("master-0"))

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: registryv1alpha1.SingletonName},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), storageAccessAddressesKey)
	assert.Empty(t, listNodes(t, c), "a layout was written from an unusable input")
}

// TestReconcileAddressesTolerateAnExplicitPort keeps the discovered form and the
// configured form interchangeable, since one of them comes from a Node object and the
// other from whoever edits the secret.
func TestReconcileAddressesTolerateAnExplicitPort(t *testing.T) {
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Primary: registryv1alpha1.PrimarySource{Upstream: upstream("registry.deckhouse.io")},
		Storage: registryv1alpha1.StorageConfig{Cache: true, Source: source()},
	})

	secret := accessSecret()
	secret.Data[storageAccessAddressesKey] = []byte(" 10.0.0.1:5001 , 10.0.0.2 ")

	r, c := newReconciler(t, cfg, secret, node("master-0"))
	runReconcile(t, r)

	applied := listNodes(t, c)["master-0"]
	storage := applied.Spec.Backend(registryv1alpha1.BackendStorage)
	require.NotNil(t, storage)
	assert.Equal(t, "10.0.0.1:5001", storage.Host)
	require.Len(t, storage.Mirrors, 1)
	assert.Equal(t, "10.0.0.2:5001", storage.Mirrors[0].Host)
}

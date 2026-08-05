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

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/registry-agent/internal/containerd"
	"github.com/deckhouse/registry-agent/internal/layout"
	"github.com/deckhouse/registry-agent/internal/pki"
	"github.com/deckhouse/registry-agent/internal/status"
)

func nodeLayout() *registryv1alpha1.RegistryNode {
	return &registryv1alpha1.RegistryNode{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-1", Generation: 7},
		Spec: registryv1alpha1.RegistryNodeSpec{
			Cache: true,
			Backends: []registryv1alpha1.Backend{
				{
					Name:     registryv1alpha1.BackendStorage,
					Endpoint: registryv1alpha1.Endpoint{Host: constant.Host, Path: constant.Path},
				},
				{
					Name:     registryv1alpha1.BackendUpstream,
					Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io", Path: "/deckhouse/ee"},
				},
			},
		},
	}
}

// materialise writes the agent's own certificate material, as the bashible step does at
// bootstrap.
func materialise(t *testing.T) *pki.OnDisk {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, pki.CAFile), []byte("-----BEGIN CERTIFICATE-----agent"), 0o644))
	return &pki.OnDisk{Dir: dir}
}

func newLoop(t *testing.T, kubeClient client.Client) *Loop {
	t.Helper()

	root := filepath.Join(t.TempDir(), "registry.d")
	return &Loop{
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Source: &layout.Source{
			Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
			Client: kubeClient,
			Node:   "worker-1",
			Cache:  &layout.Cache{Path: filepath.Join(t.TempDir(), "layout.json")},
		},
		Writer:        &containerd.Writer{Root: root},
		PKI:           materialise(t),
		Publisher:     &status.Publisher{Client: kubeClient, Node: "worker-1"},
		ProxyEndpoint: "127.0.0.1:5001",
		Serving:       func() bool { return true },
	}
}

func newClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithStatusSubresource(&registryv1alpha1.RegistryNode{}).
		Build()
}

func failingClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithStatusSubresource(&registryv1alpha1.RegistryNode{}).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
				return assert.AnError
			},
		}).
		Build()
}

func statusOf(t *testing.T, c client.Client) registryv1alpha1.RegistryNodeStatus {
	t.Helper()

	object := &registryv1alpha1.RegistryNode{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "worker-1"}, object))
	return object.Status
}

func TestOnceConfiguresTheRuntimeAndReports(t *testing.T) {
	kubeClient := newClient(t, nodeLayout())
	loop := newLoop(t, kubeClient)

	require.NoError(t, loop.once(context.Background()))

	// The runtime is pointed at the agent.
	config, err := os.ReadFile(filepath.Join(loop.Writer.Root, containerd.DefaultHost, "hosts.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(config), "127.0.0.1:5001")

	// The proxy has the layout to route by.
	require.NotNil(t, loop.Current())
	assert.Len(t, loop.Current().Backends, 2)

	got := statusOf(t, kubeClient)
	assert.EqualValues(t, 7, got.ObservedGeneration)
	assert.True(t, got.Reconciled)
	assert.True(t, got.ProxyListening)
	assert.Equal(t, []string{
		string(registryv1alpha1.BackendStorage),
		string(registryv1alpha1.BackendUpstream),
	}, got.ActiveBackends)
}

// TestOnceWithoutTheAPIStillConfiguresTheRuntime is the requirement the whole design
// turns on, seen from the top: the node keeps pulling with no control plane at all.
func TestOnceWithoutTheAPIStillConfiguresTheRuntime(t *testing.T) {
	loop := newLoop(t, newClient(t, nodeLayout()))
	ctx := context.Background()
	require.NoError(t, loop.once(ctx))

	// Everything the agent talks to goes away.
	loop.Source.Client = failingClient(t)
	loop.Publisher = &status.Publisher{Client: failingClient(t), Node: "worker-1"}

	require.NoError(t, loop.once(ctx))

	config, err := os.ReadFile(filepath.Join(loop.Writer.Root, containerd.DefaultHost, "hosts.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(config), "127.0.0.1:5001")

	// And the routing rules are still there, credentials included.
	require.NotNil(t, loop.Current())
	assert.Len(t, loop.Current().Backends, 2)
}

// TestOnceReportsRunningFromTheCopy is what tells an operator that a node is configured
// but possibly behind: without it, "configured" and "running on what it remembers" look
// identical from the outside.
func TestOnceReportsRunningFromTheCopy(t *testing.T) {
	loop := newLoop(t, newClient(t, nodeLayout()))
	ctx := context.Background()
	require.NoError(t, loop.once(ctx))

	// The layout cannot be read, but the status still can: a partial outage.
	reporting := newClient(t, nodeLayout())
	loop.Source.Client = failingClient(t)
	loop.Publisher = &status.Publisher{Client: reporting, Node: "worker-1"}

	require.NoError(t, loop.once(ctx))

	condition := apimeta.FindStatusCondition(
		statusOf(t, reporting).Conditions, registryv1alpha1.ConditionReconciled)
	require.NotNil(t, condition)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, registryv1alpha1.ReasonAppliedFromCache, condition.Reason)
}

// TestOnceWithdrawsWhatItOwnsWhenUnmanaged covers a node dropping out of the cluster's
// management: what the agent put in the runtime configuration goes with it.
func TestOnceWithdrawsWhatItOwnsWhenUnmanaged(t *testing.T) {
	kubeClient := newClient(t, nodeLayout())
	loop := newLoop(t, kubeClient)
	ctx := context.Background()

	require.NoError(t, loop.once(ctx))
	require.DirExists(t, filepath.Join(loop.Writer.Root, containerd.DefaultHost))

	require.NoError(t, kubeClient.Delete(ctx, nodeLayout()))
	require.NoError(t, loop.once(ctx))

	assert.NoDirExists(t, filepath.Join(loop.Writer.Root, containerd.DefaultHost))
	assert.Nil(t, loop.Current(), "the proxy must stop routing for a node it no longer has a layout for")
}

// TestOnceWithoutCertificateMaterialKeepsThePreviousConfiguration covers a node where
// the bashible step has not run yet, or a half-finished rotation. Writing a drop-in that
// names an authority the agent cannot serve with would make every pull fail
// verification.
func TestOnceWithoutCertificateMaterialKeepsThePreviousConfiguration(t *testing.T) {
	kubeClient := newClient(t, nodeLayout())
	loop := newLoop(t, kubeClient)
	loop.PKI = &pki.OnDisk{Dir: filepath.Join(t.TempDir(), "absent")}

	err := loop.once(context.Background())
	require.Error(t, err)

	assert.NoDirExists(t, filepath.Join(loop.Writer.Root, containerd.DefaultHost))

	// And it says so, rather than failing silently.
	condition := apimeta.FindStatusCondition(
		statusOf(t, kubeClient).Conditions, registryv1alpha1.ConditionReconciled)
	require.NotNil(t, condition)
	assert.Equal(t, metav1.ConditionFalse, condition.Status)
}

// TestOnceReportsOnlyUsableBackends checks the narrowing reaches the status, so a
// backend the agent has found broken stops being advertised.
func TestOnceReportsOnlyUsableBackends(t *testing.T) {
	kubeClient := newClient(t, nodeLayout())
	loop := newLoop(t, kubeClient)
	loop.Usable = func(names []string) []string {
		usable := make([]string, 0, len(names))
		for _, name := range names {
			if name != string(registryv1alpha1.BackendStorage) {
				usable = append(usable, name)
			}
		}
		return usable
	}

	require.NoError(t, loop.once(context.Background()))
	assert.Equal(t, []string{string(registryv1alpha1.BackendUpstream)}, statusOf(t, kubeClient).ActiveBackends)
}

// TestOnceIsIdempotent keeps a scheduled pass from rewriting the runtime configuration
// or the status for nothing.
func TestOnceIsIdempotent(t *testing.T) {
	kubeClient := newClient(t, nodeLayout())
	loop := newLoop(t, kubeClient)
	ctx := context.Background()

	require.NoError(t, loop.once(ctx))
	configPath := filepath.Join(loop.Writer.Root, containerd.DefaultHost, "hosts.toml")
	before, err := os.Stat(configPath)
	require.NoError(t, err)

	object := &registryv1alpha1.RegistryNode{}
	require.NoError(t, kubeClient.Get(ctx, types.NamespacedName{Name: "worker-1"}, object))
	version := object.ResourceVersion

	require.NoError(t, loop.once(ctx))

	after, err := os.Stat(configPath)
	require.NoError(t, err)
	assert.Equal(t, before.ModTime(), after.ModTime())

	require.NoError(t, kubeClient.Get(ctx, types.NamespacedName{Name: "worker-1"}, object))
	assert.Equal(t, version, object.ResourceVersion)
}

// TestOnceWithoutAPublisher covers a node that cannot reach the API at all: there is
// nobody to report to, and that must not stop the runtime from being configured.
func TestOnceWithoutAPublisher(t *testing.T) {
	loop := newLoop(t, newClient(t, nodeLayout()))
	loop.Publisher = nil

	require.NoError(t, loop.once(context.Background()))
	assert.FileExists(t, filepath.Join(loop.Writer.Root, containerd.DefaultHost, "hosts.toml"))
}

// TestOnceWithoutCredentialsStillConfiguresTheRuntime is the bootstrap requirement, end
// to end: a node with no kubeconfig yet must still end up with a container runtime
// pointed at the agent, or nothing on that node can be pulled and the node never joins.
func TestOnceWithoutCredentialsStillConfiguresTheRuntime(t *testing.T) {
	loop := newLoop(t, nil)
	loop.Publisher = nil

	seed := filepath.Join(t.TempDir(), "bootstrap-layout.json")
	content, err := json.Marshal(nodeLayout().Spec)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(seed, content, 0o600))
	loop.Source.Bootstrap = &layout.Bootstrap{Path: seed}

	require.NoError(t, loop.once(context.Background()))

	assert.NotNil(t, loop.Current(), "the proxy has nothing to route by")
	hosts, err := (&containerd.Writer{Root: loop.Writer.Root}).HostsOf()
	require.NoError(t, err)
	assert.Equal(t, []string{containerd.DefaultHost}, hosts)
}

// TestConnectPicksUpCredentialsLater covers the handover. The agent starts before the
// node has a kubeconfig, and once the kubelet has one the cluster's layout has to take
// over from the one the node was installed with — including for the status, which is
// written with the same credentials.
func TestConnectPicksUpCredentialsLater(t *testing.T) {
	loop := newLoop(t, nil)

	seed := filepath.Join(t.TempDir(), "bootstrap-layout.json")
	content, err := json.Marshal(registryv1alpha1.RegistryNodeSpec{
		Backends: []registryv1alpha1.Backend{{
			Name:     registryv1alpha1.BackendUpstream,
			Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io", Path: "/deckhouse/ee"},
		}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(seed, content, 0o600))
	loop.Source.Bootstrap = &layout.Bootstrap{Path: seed}

	ctx := context.Background()
	require.NoError(t, loop.once(ctx))
	require.False(t, loop.Current().Cache, "the seed does not name a cache")

	// The kubelet completes its TLS bootstrap.
	appeared := newClient(t, nodeLayout())
	loop.Connect = func() (client.Client, error) { return appeared, nil }

	require.NoError(t, loop.once(ctx))
	require.True(t, loop.Current().Cache, "the cluster's layout did not take over")

	// And the status is now published, which it could not be a moment ago.
	applied := &registryv1alpha1.RegistryNode{}
	require.NoError(t, appeared.Get(ctx, types.NamespacedName{Name: "worker-1"}, applied))
	assert.EqualValues(t, 7, applied.Status.ObservedGeneration)
	assert.True(t, applied.Status.Reconciled)
}

// TestConnectKeepsRunningWhenCredentialsNeverArrive: a failure to build a client is not a
// failure of the pass. The node keeps the configuration it has.
func TestConnectKeepsRunningWhenCredentialsNeverArrive(t *testing.T) {
	loop := newLoop(t, nil)
	loop.Connect = func() (client.Client, error) { return nil, errors.New("no kubeconfig") }

	seed := filepath.Join(t.TempDir(), "bootstrap-layout.json")
	content, err := json.Marshal(nodeLayout().Spec)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(seed, content, 0o600))
	loop.Source.Bootstrap = &layout.Bootstrap{Path: seed}

	require.NoError(t, loop.once(context.Background()))
	assert.Nil(t, loop.Source.Client)
	assert.NotNil(t, loop.Current())
}

// TestOnceReportsStorageDataLeftBehind is the whole reason the agent measures a directory
// at all: the blobs are kept on purpose when the cache is turned off, and nothing else in
// the module would ever mention the disk they occupy.
func TestOnceReportsStorageDataLeftBehind(t *testing.T) {
	noCache := nodeLayout()
	noCache.Spec.Cache = false
	noCache.Spec.Backends = []registryv1alpha1.Backend{{
		Name:     registryv1alpha1.BackendUpstream,
		Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io", Path: "/deckhouse/ee"},
	}}

	c := newClient(t, noCache)
	loop := newLoop(t, c)

	store := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(store, "blob"), make([]byte, 8192), 0o644))
	loop.StorePath = store

	require.NoError(t, loop.once(context.Background()))

	applied := &registryv1alpha1.RegistryNode{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "worker-1"}, applied))
	assert.EqualValues(t, 8192, applied.Status.StaleStorageDataBytes)
}

// TestOnceReportsNoStaleDataWhileTheCacheIsOn draws the line the number depends on: with a
// cache configured this is the store in use, not data left behind, and "how big is the
// cache" is a different question with a different owner.
func TestOnceReportsNoStaleDataWhileTheCacheIsOn(t *testing.T) {
	c := newClient(t, nodeLayout())
	loop := newLoop(t, c)

	store := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(store, "blob"), make([]byte, 8192), 0o644))
	loop.StorePath = store

	require.NoError(t, loop.once(context.Background()))

	applied := &registryv1alpha1.RegistryNode{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "worker-1"}, applied))
	assert.Zero(t, applied.Status.StaleStorageDataBytes)
}

// TestStaleStorageIsRateLimited: the walk covers a directory that can hold tens of
// gigabytes, and a stale number costs nothing — nothing acts on it automatically.
func TestStaleStorageIsRateLimited(t *testing.T) {
	loop := newLoop(t, nil)
	spec := &registryv1alpha1.RegistryNodeSpec{Cache: false}

	store := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(store, "blob"), make([]byte, 1024), 0o644))
	loop.StorePath = store
	loop.StoreReview = time.Hour

	assert.EqualValues(t, 1024, loop.staleStorage(spec))

	// The directory grows, but the measurement is not due again.
	require.NoError(t, os.WriteFile(filepath.Join(store, "another"), make([]byte, 4096), 0o644))
	assert.EqualValues(t, 1024, loop.staleStorage(spec))

	// Until it is.
	loop.StoreReview = time.Nanosecond
	assert.EqualValues(t, 5120, loop.staleStorage(spec))
}

// TestStaleStorageSurvivesAnUnwalkableDirectory keeps a disk problem from being reported as
// a broken registry: the node pulls images perfectly well either way.
func TestStaleStorageSurvivesAnUnwalkableDirectory(t *testing.T) {
	loop := newLoop(t, nil)
	loop.StorePath = filepath.Join(t.TempDir(), "absent")

	assert.Zero(t, loop.staleStorage(&registryv1alpha1.RegistryNodeSpec{Cache: false}))
}

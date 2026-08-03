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
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// errUnreachable stands in for an API server that is down, which is the situation the
// on-disk copy exists for.
var errUnreachable = errors.New("dial tcp 10.0.0.1:6443: connect: connection refused")

func nodeLayout() *registryv1alpha1.RegistryNode {
	return &registryv1alpha1.RegistryNode{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-1", Generation: 7},
		Spec: registryv1alpha1.RegistryNodeSpec{
			Cache: true,
			Backends: []registryv1alpha1.Backend{
				{
					Name: registryv1alpha1.BackendStorage,
					Endpoint: registryv1alpha1.Endpoint{
						Host: constant.Host, Path: constant.Path,
						CA:   "-----BEGIN CERTIFICATE-----storage",
						Auth: &registryv1alpha1.Auth{Username: "registry-ro", Password: "the-read-secret"},
					},
				},
				{
					Name: registryv1alpha1.BackendUpstream,
					Endpoint: registryv1alpha1.Endpoint{
						Host: "registry.deckhouse.io", Path: "/deckhouse/ee",
						Auth: &registryv1alpha1.Auth{Username: "license-token", Password: "the-license-key"},
					},
				},
			},
			AdditionalRoutes: []registryv1alpha1.Route{{
				Match: "images.virtualization.example.com",
				Endpoint: registryv1alpha1.Endpoint{
					Host: "vendor.example.com", Path: "/virt",
					Auth: &registryv1alpha1.Auth{Username: "vendor", Password: "the-vendor-secret"},
				},
			}},
		},
	}
}

func newSource(t *testing.T, kubeClient client.Client) *Source {
	t.Helper()

	return &Source{
		Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Client: kubeClient,
		Node:   "worker-1",
		Cache:  &Cache{Path: filepath.Join(t.TempDir(), "layout.json")},
	}
}

func newClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

// failingClient answers every read as an unreachable API server.
func failingClient(t *testing.T) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
				return errUnreachable
			},
		}).
		Build()
}

func TestGetFromTheAPIStoresACopy(t *testing.T) {
	source := newSource(t, newClient(t, nodeLayout()))

	snapshot, err := source.Get(context.Background())
	require.NoError(t, err)
	assert.Equal(t, OriginAPI, snapshot.Origin)
	require.NotNil(t, snapshot.Spec)
	assert.True(t, snapshot.Spec.Cache)

	stored, err := source.Cache.Load()
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, snapshot.Spec, stored.Spec)
}

// TestGetWithoutTheAPIKeepsRouting is the requirement the whole on-disk copy exists
// for: the agent is what lets a node pull images, including the images of the control
// plane, so it cannot need the control plane to be up.
func TestGetWithoutTheAPIKeepsRouting(t *testing.T) {
	source := newSource(t, newClient(t, nodeLayout()))
	ctx := context.Background()

	_, err := source.Get(ctx)
	require.NoError(t, err)

	// The API server goes away.
	source.Client = failingClient(t)

	snapshot, err := source.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, OriginCache, snapshot.Origin)
	spec := snapshot.Spec
	require.NotNil(t, spec)

	// And crucially the credentials are there, because routing a pull means
	// authenticating to a registry and there is nothing left to dereference.
	storage := spec.Backend(registryv1alpha1.BackendStorage)
	require.NotNil(t, storage)
	assert.Equal(t, "the-read-secret", storage.Auth.Password)
	assert.Equal(t, "-----BEGIN CERTIFICATE-----storage", storage.CA)

	upstream := spec.Backend(registryv1alpha1.BackendUpstream)
	require.NotNil(t, upstream)
	assert.Equal(t, "the-license-key", upstream.Auth.Password)

	require.Len(t, spec.AdditionalRoutes, 1)
	assert.Equal(t, "the-vendor-secret", spec.AdditionalRoutes[0].Auth.Password)
}

// TestGetWithoutTheAPISurvivesARestart covers the harder half: not merely losing the
// API while running, but starting up while it is already down.
func TestGetWithoutTheAPISurvivesARestart(t *testing.T) {
	first := newSource(t, newClient(t, nodeLayout()))
	_, err := first.Get(context.Background())
	require.NoError(t, err)

	restarted := &Source{
		Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Client: failingClient(t),
		Node:   "worker-1",
		Cache:  first.Cache,
	}

	snapshot, err := restarted.Get(context.Background())
	require.NoError(t, err)
	assert.Equal(t, OriginCache, snapshot.Origin)
	require.NotNil(t, snapshot.Spec)
	assert.Len(t, snapshot.Spec.Backends, 2)
	// The generation travels with the copy, so the status can still say which layout
	// was applied after a restart during an outage.
	assert.EqualValues(t, 7, snapshot.Generation)
}

// TestGetWithoutTheAPIAndWithoutACopy is the one case that genuinely cannot work: a
// node that has never reached the API has no layout, and there is none to invent.
func TestGetWithoutTheAPIAndWithoutACopy(t *testing.T) {
	source := newSource(t, failingClient(t))

	_, err := source.Get(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no layout has ever been stored")
}

// TestGetDeletedLayoutIsNotRevived draws the line between "unreachable" and
// "removed". A layout that was deleted means the node is no longer managed, and
// serving it from the copy would keep it configured after the cluster decided
// otherwise.
func TestGetDeletedLayoutIsNotRevived(t *testing.T) {
	source := newSource(t, newClient(t, nodeLayout()))
	ctx := context.Background()

	_, err := source.Get(ctx)
	require.NoError(t, err)
	require.NoError(t, source.Client.Delete(ctx, nodeLayout()))

	snapshot, err := source.Get(ctx)
	require.NoError(t, err)
	assert.Nil(t, snapshot)
}

// TestGetToleratesAnUnwritableCache keeps a disk problem from becoming an outage: the
// layout in hand is good, and refusing to use it would trade a working pull path for
// a full disk.
func TestGetToleratesAnUnwritableCache(t *testing.T) {
	source := newSource(t, newClient(t, nodeLayout()))
	source.Cache = &Cache{Path: filepath.Join(t.TempDir(), "missing-parent", "\x00", "layout.json")}

	snapshot, err := source.Get(context.Background())
	require.NoError(t, err)
	assert.Equal(t, OriginAPI, snapshot.Origin)
	require.NotNil(t, snapshot.Spec)
}

func TestGetWithACorruptCopy(t *testing.T) {
	source := newSource(t, failingClient(t))
	require.NoError(t, os.MkdirAll(filepath.Dir(source.Cache.Path), 0o700))
	require.NoError(t, os.WriteFile(source.Cache.Path, []byte("this is not json"), 0o600))

	// Applying half a layout could point the runtime at a backend that is not there,
	// so a corrupt copy is worse than none and has to be reported.
	_, err := source.Get(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unusable")
}

func TestGetRequiresANodeName(t *testing.T) {
	source := newSource(t, newClient(t))
	source.Node = ""

	_, err := source.Get(context.Background())
	assert.Error(t, err)
}

func TestIsNotFound(t *testing.T) {
	assert.True(t, isNotFound(apierrors.NewNotFound(
		schema.GroupResource{Group: "deckhouse.io", Resource: "registrynodes"}, "worker-1")))
	assert.False(t, isNotFound(errUnreachable))
	assert.False(t, isNotFound(nil))
}

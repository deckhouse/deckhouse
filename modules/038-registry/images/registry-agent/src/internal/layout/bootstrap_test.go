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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// seedSpec is the layout a node is installed with: reach the upstream directly, with
// no cache to fall back on, because no cache exists yet.
func seedSpec() *registryv1alpha1.RegistryNodeSpec {
	return &registryv1alpha1.RegistryNodeSpec{
		Backends: []registryv1alpha1.Backend{{
			Name: registryv1alpha1.BackendUpstream,
			Endpoint: registryv1alpha1.Endpoint{
				Host: "registry.deckhouse.io", Path: "/deckhouse/ee",
				Auth: &registryv1alpha1.Auth{Username: "license-token", Password: "the-license-key"},
			},
		}},
	}
}

func writeSeed(t *testing.T, spec *registryv1alpha1.RegistryNodeSpec) *Bootstrap {
	t.Helper()

	path := filepath.Join(t.TempDir(), "bootstrap-layout.json")
	content, err := json.Marshal(spec)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, content, 0o600))

	return &Bootstrap{Path: path}
}

func TestBootstrapLoadAbsentIsNotAnError(t *testing.T) {
	seed := &Bootstrap{Path: filepath.Join(t.TempDir(), "absent.json")}

	spec, err := seed.Load()
	require.NoError(t, err)
	assert.Nil(t, spec)
}

func TestBootstrapLoadRejectsAnUnusableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bootstrap-layout.json")
	require.NoError(t, os.WriteFile(path, []byte("this is not json"), 0o600))

	_, err := (&Bootstrap{Path: path}).Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unusable")
}

// TestBootstrapLoadRejectsALayoutWithNoBackend keeps the agent from being pointed at by
// the runtime while having nowhere to forward to, which turns every pull on the node
// into an error with no explanation.
func TestBootstrapLoadRejectsALayoutWithNoBackend(t *testing.T) {
	seed := writeSeed(t, &registryv1alpha1.RegistryNodeSpec{Cache: true})

	_, err := seed.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "names no backend")
}

// TestGetFallsBackToTheBootstrapLayout is the bootstrap case that has no other way out:
// the first master must pull the control plane, and there is no control plane yet to ask
// how.
func TestGetFallsBackToTheBootstrapLayout(t *testing.T) {
	source := newSource(t, failingClient(t))
	source.Bootstrap = writeSeed(t, seedSpec())

	snapshot, err := source.Get(context.Background())
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	assert.Equal(t, OriginBootstrap, snapshot.Origin)
	assert.EqualValues(t, 0, snapshot.Generation)

	upstream := snapshot.Spec.Backend(registryv1alpha1.BackendUpstream)
	require.NotNil(t, upstream)
	assert.Equal(t, "the-license-key", upstream.Auth.Password)
}

// TestGetMissingLayoutUsesTheBootstrapLayoutOnce covers the other half of bootstrap: the
// API server is up, but the controller has not compiled this node's layout yet. Treating
// that as "not managed" would withdraw the runtime configuration in the middle of a
// cluster coming up.
func TestGetMissingLayoutUsesTheBootstrapLayout(t *testing.T) {
	source := newSource(t, newClient(t))
	source.Bootstrap = writeSeed(t, seedSpec())

	snapshot, err := source.Get(context.Background())
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	assert.Equal(t, OriginBootstrap, snapshot.Origin)
}

// TestGetDeletedLayoutIsNotRevivedFromTheBootstrapLayout is the line the cache draws.
// Once a node has had a layout, a layout that is gone means the node is no longer
// managed — and the file it was installed with must not bring it back.
func TestGetDeletedLayoutIsNotRevivedFromTheBootstrapLayout(t *testing.T) {
	source := newSource(t, newClient(t, nodeLayout()))
	source.Bootstrap = writeSeed(t, seedSpec())
	ctx := context.Background()

	_, err := source.Get(ctx)
	require.NoError(t, err)
	require.NoError(t, source.Client.Delete(ctx, nodeLayout()))

	snapshot, err := source.Get(ctx)
	require.NoError(t, err)
	assert.Nil(t, snapshot)
}

// TestGetPrefersTheCacheOverTheBootstrapLayout: the cache is what the cluster last said,
// the seed is what the node was installed with, and the cluster's answer is the newer
// of the two.
func TestGetPrefersTheCacheOverTheBootstrapLayout(t *testing.T) {
	source := newSource(t, newClient(t, nodeLayout()))
	source.Bootstrap = writeSeed(t, seedSpec())
	ctx := context.Background()

	_, err := source.Get(ctx)
	require.NoError(t, err)

	source.Client = failingClient(t)

	snapshot, err := source.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	assert.Equal(t, OriginCache, snapshot.Origin)
	assert.EqualValues(t, 7, snapshot.Generation)
}

// TestGetWithoutTheAPIAndAnUnusableBootstrapLayout reports the file rather than the
// outage: an operator who wrote a malformed seed needs to be told that, not that the API
// server is down.
func TestGetWithoutTheAPIAndAnUnusableBootstrapLayout(t *testing.T) {
	source := newSource(t, failingClient(t))
	path := filepath.Join(t.TempDir(), "bootstrap-layout.json")
	require.NoError(t, os.WriteFile(path, []byte("{"), 0o600))
	source.Bootstrap = &Bootstrap{Path: path}

	_, err := source.Get(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bootstrap layout is unusable")
}

// TestBootstrapLayoutIsNotPromotedIntoTheCache keeps the two files distinguishable. If
// the seed were stored as though the cluster had said it, the node could never be
// unmanaged again.
func TestBootstrapLayoutIsNotPromotedIntoTheCache(t *testing.T) {
	source := newSource(t, failingClient(t))
	source.Bootstrap = writeSeed(t, seedSpec())

	_, err := source.Get(context.Background())
	require.NoError(t, err)

	stored, err := source.Cache.Load()
	require.NoError(t, err)
	assert.Nil(t, stored)
}

// TestGetWithoutCredentialsUsesTheBootstrapLayout is the state a node starts in: the
// kubelet has not completed its TLS bootstrap, so there is no kubeconfig to read the
// layout with — and the agent still has to answer the container runtime.
func TestGetWithoutCredentialsUsesTheBootstrapLayout(t *testing.T) {
	source := newSource(t, nil)
	source.Bootstrap = writeSeed(t, seedSpec())

	snapshot, err := source.Get(context.Background())
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	assert.Equal(t, OriginBootstrap, snapshot.Origin)
}

func TestGetWithoutCredentialsAndWithoutAnythingOnDisk(t *testing.T) {
	source := newSource(t, nil)

	_, err := source.Get(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no credentials")
}

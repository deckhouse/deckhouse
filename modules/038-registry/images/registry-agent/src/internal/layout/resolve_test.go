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
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

const testNamespace = "d8-system"

func authSecret(data map[string]string) *corev1.Secret {
	encoded := map[string][]byte{}
	for k, v := range data {
		encoded[k] = []byte(v)
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: constant.AuthSecretName, Namespace: testNamespace},
		Data:       encoded,
	}
}

func clientWithSecrets(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func ref(key string) *registryv1alpha1.Auth {
	return &registryv1alpha1.Auth{
		SecretRef: &registryv1alpha1.AuthSecretRef{Name: constant.AuthSecretName, Key: key},
	}
}

func referencingLayout() *registryv1alpha1.RegistryNodeSpec {
	return &registryv1alpha1.RegistryNodeSpec{
		Cache: true,
		Backends: []registryv1alpha1.Backend{
			{
				Name: registryv1alpha1.BackendStorage,
				Endpoint: registryv1alpha1.Endpoint{
					Host: "10.0.0.1:5001",
					Auth: ref(constant.AuthKeyStorage),
				},
				// Every replica takes the same credentials, so they share one key.
				Mirrors: []registryv1alpha1.Endpoint{{
					Host: "10.0.0.2:5001",
					Auth: ref(constant.AuthKeyStorage),
				}},
			},
			{
				Name: registryv1alpha1.BackendUpstream,
				Endpoint: registryv1alpha1.Endpoint{
					Host: "registry.deckhouse.io",
					Auth: ref(constant.AuthKeyUpstream),
				},
			},
		},
	}
}

func TestResolveFillsInEveryReference(t *testing.T) {
	secret := authSecret(map[string]string{
		constant.AuthKeyStorage:  "cm86c2VjcmV0",
		constant.AuthKeyUpstream: "bGljZW5zZS10b2tlbjprZXk=",
	})
	kubeClient := clientWithSecrets(t, secret)
	resolver := &Resolver{Namespace: testNamespace}

	spec := referencingLayout()
	require.NoError(t, resolver.Resolve(context.Background(), kubeClient, spec))

	storage := spec.Backends[0]
	assert.Equal(t, "cm86c2VjcmV0", storage.Endpoint.Auth.Auth)
	assert.Equal(t, "cm86c2VjcmV0", storage.Mirrors[0].Auth.Auth)
	assert.Equal(t, "bGljZW5zZS10b2tlbjprZXk=", spec.Backends[1].Endpoint.Auth.Auth)

	// The reference is gone once it is resolved, so nothing downstream can mistake a
	// resolved credential for one still needing a lookup.
	assert.Nil(t, storage.Endpoint.Auth.SecretRef)
	assert.False(t, storage.Endpoint.Auth.NeedsResolution())
}

// TestResolveRefusesAMissingKey: an endpoint whose credentials could not be found still
// resolves to a syntactically fine layout, and the pull then fails with a 401 that says
// nothing about a missing key. Refusing here is what turns that into a legible error.
func TestResolveRefusesAMissingKey(t *testing.T) {
	secret := authSecret(map[string]string{constant.AuthKeyStorage: "cm86c2VjcmV0"})
	kubeClient := clientWithSecrets(t, secret)
	resolver := &Resolver{Namespace: testNamespace}

	err := resolver.Resolve(context.Background(), kubeClient, referencingLayout())
	require.Error(t, err)
	assert.Contains(t, err.Error(), constant.AuthKeyUpstream)
}

func TestResolveWithoutAReaderRefusesRatherThanSilentlySkipping(t *testing.T) {
	resolver := &Resolver{Namespace: testNamespace}

	require.Error(t, resolver.Resolve(context.Background(), nil, referencingLayout()))

	// A layout with nothing to resolve is fine without a client, which is what a node
	// running from its bootstrap layout has.
	inline := &registryv1alpha1.RegistryNodeSpec{Backends: []registryv1alpha1.Backend{{
		Name:     registryv1alpha1.BackendUpstream,
		Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io"},
	}}}
	assert.NoError(t, resolver.Resolve(context.Background(), nil, inline))
}

// TestTheCachedLayoutIsTheResolvedOne is the property that makes the reference model safe
// for this component.
//
// The agent has to keep serving images when the API server is gone — that is why it keeps a
// copy on disk at all. A copy holding references would be useless at exactly that moment,
// because there is nothing left to dereference them against.
func TestTheCachedLayoutIsTheResolvedOne(t *testing.T) {
	node := &registryv1alpha1.RegistryNode{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-1", Generation: 7},
		Spec:       *referencingLayout(),
	}
	secret := authSecret(map[string]string{
		constant.AuthKeyStorage:  "cm86c2VjcmV0",
		constant.AuthKeyUpstream: "bGljZW5zZS10b2tlbjprZXk=",
	})

	cachePath := filepath.Join(t.TempDir(), "layout.json")
	kubeClient := clientWithSecrets(t, node, secret)
	source := &Source{
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Client:   kubeClient,
		Node:     "worker-1",
		Cache:    &Cache{Path: cachePath},
		Resolver: &Resolver{Namespace: testNamespace},
	}

	got, err := source.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, OriginAPI, got.Origin)
	assert.Equal(t, "cm86c2VjcmV0", got.Spec.Backends[0].Endpoint.Auth.Auth)

	// Now read the copy back the way a restarted agent with no API would.
	offline := &Source{
		Log:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Node:  "worker-1",
		Cache: &Cache{Path: cachePath},
	}
	fromDisk, err := offline.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, OriginCache, fromDisk.Origin)

	storage := fromDisk.Spec.Backends[0]
	assert.Equal(t, "cm86c2VjcmV0", storage.Endpoint.Auth.Auth,
		"the copy on disk cannot authenticate, so a node that restarts during an "+
			"API outage cannot pull")
	assert.False(t, storage.Endpoint.Auth.NeedsResolution())
}

// TestTheResolverFollowsTheSourcesClient is the regression this signature exists for.
//
// The agent constructs everything before a node has credentials — that is the whole point of
// it — and the client appears later, when the kubelet's kubeconfig does. While the resolver
// carried a client of its own, handed to it at construction, that copy stayed nil forever: the
// Source's was replaced on connect and the resolver's was not. Every layout naming a
// credential was refused for the life of the node, and refused in the words of an unreachable
// API server. There is now one client, and it is the Source's.
func TestTheResolverFollowsTheSourcesClient(t *testing.T) {
	node := &registryv1alpha1.RegistryNode{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-1", Generation: 3},
		Spec:       *referencingLayout(),
	}
	secret := authSecret(map[string]string{
		constant.AuthKeyStorage:  "cm86c2VjcmV0",
		constant.AuthKeyUpstream: "bGljZW5zZS10b2tlbjprZXk=",
	})

	// As the agent starts on a node that has not finished its TLS bootstrap: no client yet.
	source := &Source{
		Log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Node:      "worker-1",
		Cache:     &Cache{Path: filepath.Join(t.TempDir(), "layout.json")},
		Bootstrap: &Bootstrap{Path: filepath.Join(t.TempDir(), "absent.json")},
		Resolver:  &Resolver{Namespace: testNamespace},
	}

	_, err := source.Get(context.Background())
	require.Error(t, err, "with no credentials and nothing on disk there is nothing to apply")

	// And then the credentials appear, which is what the loop does on connect.
	source.Client = clientWithSecrets(t, node, secret)

	got, err := source.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, OriginAPI, got.Origin,
		"the layout from the API was refused, so the resolver is not using the client the "+
			"Source was given")
	assert.Equal(t, "cm86c2VjcmV0", got.Spec.Backends[0].Endpoint.Auth.Auth)
}

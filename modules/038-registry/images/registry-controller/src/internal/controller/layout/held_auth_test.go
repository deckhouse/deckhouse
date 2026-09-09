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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// TestReconcileKeepsTheCredentialsOfAHeldUpstream walks the whole hold through the objects,
// which is the only level at which this defect is visible.
//
// The sequence is the documented way to go air-gapped: the operator removes the upstream, and
// the controller keeps serving through it until the cache can stand alone. Nothing about the
// addresses is lost in that hold — they are recorded in the status. The credentials are not, on
// purpose, so the configuration removing them takes the last copy the controller can see, and
// the only one left is the Secret this module wrote itself.
//
// Without reading it back, the hold keeps the address and loses the ability to use it: every
// node ends up with a fallback it cannot authenticate to. On the cluster this was measured on,
// that turned a cache miss from something slower into a failure, and any image reference naming
// the upstream by its own name into a relayed 401 — while every address, scheme and authority
// in the layout was correct.
func TestReconcileKeepsTheCredentialsOfAHeldUpstream(t *testing.T) {
	// Air-gap requested: the configuration has no upstream, and so no credentials either.
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Storage: registryv1alpha1.StorageConfig{Cache: true, Source: source()},
	})
	// What the controller recorded while the upstream was still configured: addresses only.
	cfg.Status.EffectiveUpstream = upstream("registry.deckhouse.io")
	cfg.Status.EffectiveUpstream.Endpoint.Auth = nil

	// And the credentials it was working with, where this module keeps them.
	persisted := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: Namespace, Name: constant.AuthSecretName},
		Data: map[string][]byte{
			constant.AuthKeyUpstream: []byte("bGljZW5zZS10b2tlbjprZXk="),
		},
	}

	// The cache is not complete, so the upstream must be held rather than dropped.
	storage := &registryv1alpha1.RegistryStorage{
		ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName},
		Spec:       registryv1alpha1.RegistryStorageSpec{Upstream: upstream("registry.deckhouse.io")},
		Status: registryv1alpha1.RegistryStorageStatus{
			Replicas: []registryv1alpha1.StorageReplicaStatus{{
				Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader,
				Full: false, VerifiedDigests: 120,
			}},
		},
	}

	r, c := newReconciler(t, cfg, accessSecret(), persisted, storage,
		node("master-0"), node("worker-1"), storageLease("master-0"))
	runReconcile(t, r)

	// Held, as the transition requires.
	got := getStorage(t, c)
	require.NotNil(t, got.Spec.Upstream, "the upstream must be held while the cache is incomplete")
	require.NotNil(t, got.Spec.Upstream.Auth, "the storage cannot fill from an upstream it cannot authenticate to")
	require.NotNil(t, got.Spec.Upstream.Auth.SecretRef)
	assert.Equal(t, constant.AuthKeyUpstream, got.Spec.Upstream.Auth.SecretRef.Key)

	// And every node's fallback can authenticate.
	for name, layout := range listNodes(t, c) {
		require.Lenf(t, layout.Spec.Backends, 2, "node %s must keep the upstream as a fallback", name)
		fallback := layout.Spec.Backends[1]
		require.Equal(t, registryv1alpha1.BackendUpstream, fallback.Name)
		require.NotNilf(t, fallback.Endpoint.Auth,
			"node %s holds an upstream with no credentials: a cache miss there fails instead of being slower", name)
		require.NotNil(t, fallback.Endpoint.Auth.SecretRef)
		assert.Equal(t, constant.AuthKeyUpstream, fallback.Endpoint.Auth.SecretRef.Key)
	}

	// The key is still there for the reference to resolve to, and holds what it held.
	secret := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{
		Namespace: Namespace, Name: constant.AuthSecretName,
	}, secret))
	assert.Equal(t, "bGljZW5zZS10b2tlbjprZXk=", string(secret.Data[constant.AuthKeyUpstream]),
		"the credentials of a held upstream were dropped from the Secret, so the references above resolve to nothing")
}

// TestReconcileDropsTheCredentialsWithTheUpstream is the other half: once the cache stands
// alone the upstream is gone, and its credentials must not be left readable.
func TestReconcileDropsTheCredentialsWithTheUpstream(t *testing.T) {
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Storage: registryv1alpha1.StorageConfig{Cache: true, Source: source()},
	})
	cfg.Status.EffectiveUpstream = upstream("registry.deckhouse.io")
	cfg.Status.EffectiveUpstream.Endpoint.Auth = nil

	persisted := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: Namespace, Name: constant.AuthSecretName},
		Data: map[string][]byte{
			constant.AuthKeyUpstream: []byte("bGljZW5zZS10b2tlbjprZXk="),
		},
	}

	// The leader holds the whole expected set, so this reconciliation is the transition.
	storage := &registryv1alpha1.RegistryStorage{
		ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName},
		Spec:       registryv1alpha1.RegistryStorageSpec{Upstream: upstream("registry.deckhouse.io")},
		Status: registryv1alpha1.RegistryStorageStatus{
			Replicas: []registryv1alpha1.StorageReplicaStatus{{
				Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader,
				Full: true, VerifiedDigests: 459,
			}},
		},
	}

	r, c := newReconciler(t, cfg, accessSecret(), persisted, storage,
		node("master-0"), storageLease("master-0"))
	runReconcile(t, r)

	got := getStorage(t, c)
	assert.Nil(t, got.Spec.Upstream, "the cache is complete, so the upstream goes")
	assert.Len(t, listNodes(t, c)["master-0"].Spec.Backends, 1)

	secret := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{
		Namespace: Namespace, Name: constant.AuthSecretName,
	}, secret))
	assert.NotContains(t, secret.Data, constant.AuthKeyUpstream,
		"a credential nothing references any more must not stay readable")
}

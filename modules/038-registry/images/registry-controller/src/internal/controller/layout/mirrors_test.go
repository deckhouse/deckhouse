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
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

const nodeGroupLabel = "node.deckhouse.io/group"

func groupNode(name, group string) *corev1.Node {
	n := node(name)
	n.Labels = map[string]string{nodeGroupLabel: group}
	return n
}

func registryMirror(name, group, host string, created time.Time) *registryv1alpha1.RegistryMirror {
	return &registryv1alpha1.RegistryMirror{
		ObjectMeta: metav1.ObjectMeta{Name: name, Generation: 1, CreationTimestamp: metav1.NewTime(created)},
		Spec: registryv1alpha1.RegistryMirrorSpec{
			NodeSelector: metav1.LabelSelector{MatchLabels: map[string]string{nodeGroupLabel: group}},
			Source: registryv1alpha1.Upstream{
				Endpoint: registryv1alpha1.Endpoint{
					Scheme: registryv1alpha1.SchemeHTTPS,
					Host:   host,
					Path:   "/d8-proxy/deckhouse/ee",
					Auth:   &registryv1alpha1.Auth{Auth: "dXNlcm5hbWU6cGFzc3dvcmQ="},
				},
			},
		},
	}
}

func getMirror(t *testing.T, c client.Client, name string) *registryv1alpha1.RegistryMirror {
	t.Helper()

	mirror := &registryv1alpha1.RegistryMirror{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: name}, mirror))
	return mirror
}

func managedCachedConfig() *registryv1alpha1.RegistryConfig {
	return registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Primary: registryv1alpha1.PrimarySource{Upstream: upstream("registry.deckhouse.io")},
		Storage: registryv1alpha1.StorageConfig{Cache: true, Source: source()},
	})
}

// The mirror reaches the nodes it selects and no other, with its credentials moved into the auth
// Secret like every other credential the layouts name.
func TestReconcileLaysOutAMirrorForTheNodesItSelects(t *testing.T) {
	mirror := registryMirror("gpu-eu", "gpu-eu", "harbor.eu.example.com", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))

	r, c := newReconciler(t, managedCachedConfig(), accessSecret(),
		groupNode("master-0", "master"), groupNode("gpu-eu-0", "gpu-eu"), mirror)
	runReconcile(t, r)

	layouts := listNodes(t, c)
	assert.Nil(t, layouts["master-0"].Spec.PrimaryMirror, "a node the selector does not pick has no mirror")

	laidOut := layouts["gpu-eu-0"].Spec.PrimaryMirror
	require.NotNil(t, laidOut)
	assert.Equal(t, "gpu-eu", laidOut.Name)
	assert.Equal(t, "harbor.eu.example.com/d8-proxy/deckhouse/ee", laidOut.Address())
	require.NotNil(t, laidOut.Auth)
	assert.True(t, laidOut.Auth.IsEmpty(), "the layout carries a reference, never the credentials")
	require.NotNil(t, laidOut.Auth.SecretRef)
	assert.Equal(t, constant.AuthSecretName, laidOut.Auth.SecretRef.Name)

	// The backends are what they would be without a mirror: it is asked ahead of them, not instead.
	require.Len(t, layouts["gpu-eu-0"].Spec.Backends, 2)
	assert.Equal(t, registryv1alpha1.BackendStorage, layouts["gpu-eu-0"].Spec.Backends[0].Name)

	secret := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{
		Namespace: Namespace, Name: constant.AuthSecretName,
	}, secret))
	assert.Equal(t, "dXNlcm5hbWU6cGFzc3dvcmQ=", string(secret.Data[laidOut.Auth.SecretRef.Key]))

	status := getMirror(t, c, "gpu-eu").Status
	assert.Equal(t, int32(1), status.MatchedNodes)
	assert.Equal(t, int64(1), status.ObservedGeneration)
	condition := apimeta.FindStatusCondition(status.Conditions, registryv1alpha1.ConditionAccepted)
	require.NotNil(t, condition)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, "The mirror is laid out for 1 node(s).", condition.Message)
}

// The second mirror on a node is refused whole, and says why on its Accepted condition.
func TestReconcileRefusesTheNewerMirrorOnANode(t *testing.T) {
	older := registryMirror("gpu-eu", "gpu-eu", "harbor.eu.example.com", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	newer := registryMirror("gpu-eu-2", "gpu-eu", "harbor-2.eu.example.com", time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC))

	r, c := newReconciler(t, managedCachedConfig(), accessSecret(), groupNode("gpu-eu-0", "gpu-eu"), older, newer)
	runReconcile(t, r)

	assert.Equal(t, "gpu-eu", listNodes(t, c)["gpu-eu-0"].Spec.PrimaryMirror.Name)

	status := getMirror(t, c, "gpu-eu-2").Status
	assert.Zero(t, status.MatchedNodes)
	condition := apimeta.FindStatusCondition(status.Conditions, registryv1alpha1.ConditionAccepted)
	require.NotNil(t, condition)
	assert.Equal(t, metav1.ConditionFalse, condition.Status)
	assert.Equal(t, registryv1alpha1.ReasonNodeConflict, condition.Reason)
	assert.Contains(t, condition.Message, `node "gpu-eu-0" is already served by RegistryMirror/gpu-eu`)
}

// A node relabelled out of the selector loses the mirror on the next pass, and a deleted mirror
// leaves no node pointing at it.
func TestReconcileFollowsTheSelectorAndTheObject(t *testing.T) {
	mirror := registryMirror("gpu-eu", "gpu-eu", "harbor.eu.example.com", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	gpu := groupNode("gpu-eu-0", "gpu-eu")

	r, c := newReconciler(t, managedCachedConfig(), accessSecret(), gpu, mirror)
	runReconcile(t, r)
	require.NotNil(t, listNodes(t, c)["gpu-eu-0"].Spec.PrimaryMirror)

	relabelled := &corev1.Node{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "gpu-eu-0"}, relabelled))
	relabelled.Labels[nodeGroupLabel] = "gpu-us"
	require.NoError(t, c.Update(context.Background(), relabelled))
	runReconcile(t, r)

	assert.Nil(t, listNodes(t, c)["gpu-eu-0"].Spec.PrimaryMirror)
	assert.Zero(t, getMirror(t, c, "gpu-eu").Status.MatchedNodes)

	relabelled.Labels[nodeGroupLabel] = "gpu-eu"
	require.NoError(t, c.Update(context.Background(), relabelled))
	runReconcile(t, r)
	require.NotNil(t, listNodes(t, c)["gpu-eu-0"].Spec.PrimaryMirror)

	require.NoError(t, c.Delete(context.Background(), getMirror(t, c, "gpu-eu")))
	runReconcile(t, r)
	assert.Nil(t, listNodes(t, c)["gpu-eu-0"].Spec.PrimaryMirror)
}

// Without the cache the agent goes to the upstream, and the mirror is asked ahead of it the same.
func TestReconcileLaysOutAMirrorWithoutTheCache(t *testing.T) {
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Primary: registryv1alpha1.PrimarySource{Upstream: upstream("registry.deckhouse.io")},
	})
	mirror := registryMirror("gpu-eu", "gpu-eu", "harbor.eu.example.com", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))

	r, c := newReconciler(t, cfg, groupNode("gpu-eu-0", "gpu-eu"), mirror)
	runReconcile(t, r)

	spec := listNodes(t, c)["gpu-eu-0"].Spec
	require.NotNil(t, spec.PrimaryMirror)
	require.Len(t, spec.Backends, 1)
	assert.Equal(t, registryv1alpha1.BackendUpstream, spec.Backends[0].Name)
}

// A mirror for a node group scaled to zero, and one with an empty selector, select no node: both
// are accepted, laid out nowhere, and say so in the status.
func TestReconcileAcceptsAMirrorSelectingNoNode(t *testing.T) {
	created := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	scaledToZero := registryMirror("gpu-eu", "gpu-eu", "harbor.eu.example.com", created)
	empty := registryMirror("empty", "", "harbor.example.com", created.Add(time.Hour))
	empty.Spec.NodeSelector = metav1.LabelSelector{}

	r, c := newReconciler(t, managedCachedConfig(), accessSecret(), groupNode("master-0", "master"),
		scaledToZero, empty)
	runReconcile(t, r)

	assert.Nil(t, listNodes(t, c)["master-0"].Spec.PrimaryMirror)

	for _, name := range []string{"gpu-eu", "empty"} {
		status := getMirror(t, c, name).Status
		assert.Zero(t, status.MatchedNodes, name)
		condition := apimeta.FindStatusCondition(status.Conditions, registryv1alpha1.ConditionAccepted)
		require.NotNil(t, condition, name)
		assert.Equal(t, metav1.ConditionTrue, condition.Status, name)
		assert.Equal(t, "The mirror is laid out for 0 node(s).", condition.Message, name)
	}
}

// The own credentials of a spare address must not share a key with another mirror's: `eu` with its
// first spare address and the mirror `eu-mirror-0` once both keyed `registrymirror-eu-mirror-0`.
func TestReconcileKeysSpareAddressCredentialsApartFromOtherMirrors(t *testing.T) {
	created := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	eu := registryMirror("eu", "gpu-eu", "harbor.eu.example.com", created)
	eu.Spec.Source.Mirrors = []registryv1alpha1.Endpoint{{
		Scheme: registryv1alpha1.SchemeHTTPS, Host: "harbor-2.eu.example.com", Path: "/d8",
		Auth: &registryv1alpha1.Auth{Username: "spare", Password: "dh-test-secret-0123456789abcdef"},
	}}
	other := registryMirror("eu-mirror-0", "gpu-us", "harbor.us.example.com", created.Add(time.Hour))

	r, c := newReconciler(t, managedCachedConfig(), accessSecret(),
		groupNode("gpu-eu-0", "gpu-eu"), groupNode("gpu-us-0", "gpu-us"), eu, other)
	runReconcile(t, r)

	layouts := listNodes(t, c)
	spare := layouts["gpu-eu-0"].Spec.PrimaryMirror.Mirrors[0].Auth.SecretRef
	require.NotNil(t, spare)
	otherKey := layouts["gpu-us-0"].Spec.PrimaryMirror.Auth.SecretRef
	require.NotNil(t, otherKey)
	assert.NotEqual(t, otherKey.Key, spare.Key)

	secret := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{
		Namespace: Namespace, Name: constant.AuthSecretName,
	}, secret))
	assert.Equal(t, eu.Spec.Source.Mirrors[0].Auth.Encoded(), string(secret.Data[spare.Key]))
	assert.Equal(t, "dXNlcm5hbWU6cGFzc3dvcmQ=", string(secret.Data[otherKey.Key]))
}

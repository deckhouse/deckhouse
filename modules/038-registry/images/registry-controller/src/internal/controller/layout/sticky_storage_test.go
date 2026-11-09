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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// layoutPointedAt is a RegistryNode as an earlier pass left it, naming host as the Storage endpoint.
func layoutPointedAt(name, host string) *registryv1alpha1.RegistryNode {
	return &registryv1alpha1.RegistryNode{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: registryv1alpha1.RegistryNodeSpec{
			Cache: true,
			Backends: []registryv1alpha1.Backend{{
				Name:     registryv1alpha1.BackendStorage,
				Endpoint: registryv1alpha1.Endpoint{Scheme: registryv1alpha1.SchemeHTTPS, Host: host},
			}},
		},
	}
}

// storageEndpoints is the Storage endpoint of every node's layout, by node.
func storageEndpoints(t *testing.T, c client.Client) map[string]string {
	t.Helper()

	out := map[string]string{}
	for name, applied := range listNodes(t, c) {
		storage := applied.Spec.Backend(registryv1alpha1.BackendStorage)
		require.NotNil(t, storage, "node %s has no storage backend", name)
		out[name] = storage.Host
	}
	return out
}

func setLeaseHolder(t *testing.T, c client.Client, holder string) {
	t.Helper()

	lease := &coordinationv1.Lease{}
	require.NoError(t, c.Get(context.Background(),
		types.NamespacedName{Namespace: Namespace, Name: StorageLeaseName}, lease))
	lease.Spec.HolderIdentity = &holder
	require.NoError(t, c.Update(context.Background(), lease))
}

// TestTheStorageEndpointStaysPutWithoutALeaseHolder walks a planned handover: the leader releases the
// lease on its way out, and for a few seconds nobody holds it.
//
// The endpoint stays on the replica that led, rather than going to whichever address sorts first:
// that rewrote every node's layout twice for one change of leader, and in an air-gapped cluster the
// first address can be a follower that answers 404 for an image the cluster holds. It moves only when
// the new leader takes the lease. A push, meanwhile, is refused — it is not kept the way a pull is.
func TestTheStorageEndpointStaysPutWithoutALeaseHolder(t *testing.T) {
	r, c := newReconciler(t, cachedConfig(), accessSecret(), node("master-0"), node("master-1"),
		storageWithReplicas(), storageLease("master-1"))
	runReconcile(t, r)
	require.Equal(t, map[string]string{"master-0": "10.0.0.2:5001", "master-1": "10.0.0.2:5001"},
		storageEndpoints(t, c), "the leader is the endpoint")

	setLeaseHolder(t, c, "")
	runReconcile(t, r)
	assert.Equal(t, map[string]string{"master-0": "10.0.0.2:5001", "master-1": "10.0.0.2:5001"},
		storageEndpoints(t, c), "a released lease must not move the endpoint to the first address")
	assert.Empty(t, pushSlice(t, c).Endpoints, "a push is not kept on the replica that led: it is refused")

	setLeaseHolder(t, c, "master-0")
	runReconcile(t, r)
	assert.Equal(t, map[string]string{"master-0": "10.0.0.1:5001", "master-1": "10.0.0.1:5001"},
		storageEndpoints(t, c), "the new leader becomes the endpoint as soon as it holds the lease")
}

// TestTheStorageEndpointWithoutAnyLeaderToKeep: with nothing to keep the endpoint falls back to the
// first address, as it always did — a cache turned on a moment ago, or an endpoint whose replica left
// the cluster along with its master.
func TestTheStorageEndpointWithoutAnyLeaderToKeep(t *testing.T) {
	t.Run("no layouts yet", func(t *testing.T) {
		r, c := newReconciler(t, cachedConfig(), accessSecret(), node("master-0"), node("master-1"),
			storageWithReplicas(), storageLease(""))
		runReconcile(t, r)

		assert.Equal(t, map[string]string{"master-0": "10.0.0.1:5001", "master-1": "10.0.0.1:5001"},
			storageEndpoints(t, c))
	})

	t.Run("the endpoint the layouts carry is no longer a replica", func(t *testing.T) {
		r, c := newReconciler(t, cachedConfig(), accessSecret(), node("master-0"), node("master-1"),
			storageWithReplicas(), storageLease(""),
			layoutPointedAt("master-0", "10.0.0.9:5001"), layoutPointedAt("master-1", "10.0.0.9:5001"))
		runReconcile(t, r)

		assert.Equal(t, map[string]string{"master-0": "10.0.0.1:5001", "master-1": "10.0.0.1:5001"},
			storageEndpoints(t, c), "a replica that left must not stay the endpoint")
	})
}

// TestTheStorageEndpointKeptIsTheOneMostLayoutsCarry: layouts disagree only when a pass was
// interrupted halfway or someone edited one by hand. Whichever endpoint most of them carry is kept, on
// every node, so the disagreement ends with this pass instead of being preserved by it.
func TestTheStorageEndpointKeptIsTheOneMostLayoutsCarry(t *testing.T) {
	secret := accessSecret()
	secret.Data[storageAccessAddressesKey] = []byte("10.0.0.1,10.0.0.2,10.0.0.3")

	r, c := newReconciler(t, cachedConfig(), secret,
		node("master-0"), node("master-1"), node("master-2"),
		storageWithReplicas(), storageLease(""),
		layoutPointedAt("master-0", "10.0.0.3:5001"),
		layoutPointedAt("master-1", "10.0.0.2:5001"),
		layoutPointedAt("master-2", "10.0.0.3:5001"))
	runReconcile(t, r)

	assert.Equal(t, map[string]string{
		"master-0": "10.0.0.3:5001", "master-1": "10.0.0.3:5001", "master-2": "10.0.0.3:5001",
	}, storageEndpoints(t, c))
}

// failsNodeListOnce answers the first list of RegistryNode objects with an error and every later one
// from the client it wraps, the way a list that times out once and then recovers does.
type failsNodeListOnce struct {
	client.Client
	failed bool
}

func (c *failsNodeListOnce) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if _, isNodes := list.(*registryv1alpha1.RegistryNodeList); isNodes && !c.failed {
		c.failed = true
		return errors.New("the server was unable to return a response in the time allotted")
	}
	return c.Client.List(ctx, list, opts...)
}

// TestTheStorageEndpointIsNotMovedByAFailedList: the endpoint to keep is read from the RegistryNode
// objects, and a list that fails says nothing about it. Taken as "no endpoint to keep", it would send
// every node to the first address — the rewrite keeping the endpoint exists to avoid — because the
// later list of the same objects, the one the layouts are written from, may well succeed. The pass
// fails instead, leaves every layout where it was, and the next one keeps the endpoint.
func TestTheStorageEndpointIsNotMovedByAFailedList(t *testing.T) {
	r, c := newReconciler(t, cachedConfig(), accessSecret(), node("master-0"), node("master-1"),
		storageWithReplicas(), storageLease(""),
		layoutPointedAt("master-0", "10.0.0.2:5001"), layoutPointedAt("master-1", "10.0.0.2:5001"))
	r.InjectClient(&failsNodeListOnce{Client: c})

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: registryv1alpha1.SingletonName},
	})
	require.ErrorContains(t, err, "listing RegistryNode objects")
	assert.Equal(t, map[string]string{"master-0": "10.0.0.2:5001", "master-1": "10.0.0.2:5001"},
		storageEndpoints(t, c), "a failed list must not move the endpoint to the first address")

	runReconcile(t, r)
	assert.Equal(t, map[string]string{"master-0": "10.0.0.2:5001", "master-1": "10.0.0.2:5001"},
		storageEndpoints(t, c), "the pass after the failure keeps the endpoint")
}

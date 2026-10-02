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
	coordinationv1 "k8s.io/api/coordination/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

func storageWithReplicas() *registryv1alpha1.RegistryStorage {
	return &registryv1alpha1.RegistryStorage{
		ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName},
		Status: registryv1alpha1.RegistryStorageStatus{
			Replicas: []registryv1alpha1.StorageReplicaStatus{
				{Node: "master-0", Role: registryv1alpha1.ReplicaRoleFollower, Address: "10.0.0.1:5001", Full: true},
				{Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader, Address: "10.0.0.2:5001", Full: true},
			},
		},
	}
}

func pushSlice(t *testing.T, c client.Client) *discoveryv1.EndpointSlice {
	t.Helper()
	slice := &discoveryv1.EndpointSlice{}
	require.NoError(t, c.Get(context.Background(),
		types.NamespacedName{Namespace: Namespace, Name: pushEndpointSlice}, slice))
	return slice
}

func cachedConfig() *registryv1alpha1.RegistryConfig {
	return registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Primary: registryv1alpha1.PrimarySource{Upstream: upstream("registry.deckhouse.io")},
		Storage: registryv1alpha1.StorageConfig{Cache: true, Source: source()},
	})
}

// TestAPushGoesToTheLeader: a push lands where the cluster reads from. A Service over every replica
// sent it wherever the client's address hashed, and a follower's store is one nobody reads.
func TestAPushGoesToTheLeader(t *testing.T) {
	r, c := newReconciler(t, cachedConfig(), accessSecret(), node("master-0"), node("master-1"),
		storageWithReplicas(), storageLease("master-1"))
	runReconcile(t, r)

	slice := pushSlice(t, c)
	assert.Equal(t, PushService, slice.Labels[discoveryv1.LabelServiceName])
	assert.Equal(t, pushManagedBy, slice.Labels[discoveryv1.LabelManagedBy],
		"the EndpointSlice controller must leave it alone")
	require.Len(t, slice.Endpoints, 1, "exactly one replica takes a push")
	assert.Equal(t, []string{"10.0.0.2"}, slice.Endpoints[0].Addresses, "the one the lease names")
	require.Len(t, slice.Ports, 1)
	assert.Equal(t, pushPortName, *slice.Ports[0].Name)
	assert.EqualValues(t, 5003, *slice.Ports[0].Port, "the write listener, not the cache")

	require.Len(t, slice.OwnerReferences, 1)
	assert.Equal(t, "RegistryStorage", slice.OwnerReferences[0].Kind,
		"goes with the storage when the cache is turned off")
}

// TestThePushFollowsTheLease: the leader changes, and so does where a push lands — to the replica the
// agents are pointed at, from the same lease.
func TestThePushFollowsTheLease(t *testing.T) {
	r, c := newReconciler(t, cachedConfig(), accessSecret(), node("master-0"), node("master-1"),
		storageWithReplicas(), storageLease("master-1"))
	runReconcile(t, r)
	require.Equal(t, []string{"10.0.0.2"}, pushSlice(t, c).Endpoints[0].Addresses)

	lease := &coordinationv1.Lease{}
	require.NoError(t, c.Get(context.Background(),
		types.NamespacedName{Namespace: Namespace, Name: StorageLeaseName}, lease))
	holder := "master-0"
	lease.Spec.HolderIdentity = &holder
	require.NoError(t, c.Update(context.Background(), lease))

	runReconcile(t, r)
	slice := pushSlice(t, c)
	require.Len(t, slice.Endpoints, 1)
	assert.Equal(t, []string{"10.0.0.1"}, slice.Endpoints[0].Addresses)
}

// TestWithoutALeaderNothingTakesAPush: refused at the ingress rather than written where the cluster
// would not read it.
func TestWithoutALeaderNothingTakesAPush(t *testing.T) {
	r, c := newReconciler(t, cachedConfig(), accessSecret(), node("master-0"), node("master-1"),
		storageWithReplicas())
	runReconcile(t, r)

	assert.Empty(t, pushSlice(t, c).Endpoints)
}

func TestPushEndpointsKeepsToOneAddressFamily(t *testing.T) {
	kind, endpoints := pushEndpoints("[fd00::2]:5001")
	assert.Equal(t, discoveryv1.AddressTypeIPv6, kind)
	require.Len(t, endpoints, 1)
	assert.Equal(t, []string{"fd00::2"}, endpoints[0].Addresses)

	kind, endpoints = pushEndpoints("")
	assert.Equal(t, discoveryv1.AddressTypeIPv4, kind)
	assert.Empty(t, endpoints)

	_, endpoints = pushEndpoints("master-1:5001")
	assert.Empty(t, endpoints, "a name is not an endpoint address")
}

// TestAPushSliceChangesFamilyByBeingReplaced: the address type of a slice is immutable, so a leader on
// the other family replaces it — while an empty slice keeps the family it has.
func TestAPushSliceChangesFamilyByBeingReplaced(t *testing.T) {
	storage := storageWithReplicas()
	storage.Status.Replicas[1].Address = "[fd00::2]:5001"
	r, c := newReconciler(t, cachedConfig(), accessSecret(), node("master-0"), node("master-1"),
		storage, storageLease("master-1"),
		&discoveryv1.EndpointSlice{
			ObjectMeta:  metav1.ObjectMeta{Namespace: Namespace, Name: pushEndpointSlice},
			AddressType: discoveryv1.AddressTypeIPv4,
		})
	runReconcile(t, r)

	slice := pushSlice(t, c)
	assert.Equal(t, discoveryv1.AddressTypeIPv6, slice.AddressType)
	require.Len(t, slice.Endpoints, 1)
	assert.Equal(t, []string{"fd00::2"}, slice.Endpoints[0].Addresses)

	// The lease is lost: the slice empties and stays IPv6.
	require.NoError(t, c.Delete(context.Background(), storageLease("master-1")))
	runReconcile(t, r)
	slice = pushSlice(t, c)
	assert.Equal(t, discoveryv1.AddressTypeIPv6, slice.AddressType)
	assert.Empty(t, slice.Endpoints)
}

// TestOnlyAChangeOfHolderWakesTheReconciler: the lease is renewed every few seconds, and a reconcile
// on each renewal would be the whole layout recomputed for nothing.
func TestOnlyAChangeOfHolderWakesTheReconciler(t *testing.T) {
	filter := storageLeaseHolderChanged()
	lease := func(name, holder string) *coordinationv1.Lease {
		l := storageLease(holder)
		l.Name = name
		return l
	}

	assert.True(t, filter.Update(event.UpdateEvent{
		ObjectOld: lease(StorageLeaseName, "master-0"), ObjectNew: lease(StorageLeaseName, "master-1"),
	}), "the leader changed")
	assert.False(t, filter.Update(event.UpdateEvent{
		ObjectOld: lease(StorageLeaseName, "master-0"), ObjectNew: lease(StorageLeaseName, "master-0"),
	}), "a renewal")
	assert.False(t, filter.Update(event.UpdateEvent{
		ObjectOld: lease("registry-controller-leader", "a"), ObjectNew: lease("registry-controller-leader", "b"),
	}), "somebody else's lease")
	assert.True(t, filter.Create(event.CreateEvent{Object: lease(StorageLeaseName, "master-0")}))
	assert.True(t, filter.Delete(event.DeleteEvent{Object: lease(StorageLeaseName, "master-0")}))
}

// TestThePushSlicesOfTheSelectorAreRemoved: the Service had a selector before the publication endpoint
// was pointed at the leader, and the EndpointSlice controller does not take back what it wrote once the
// selector is gone. Those slices still name every replica, and the ingress spread a push across all of
// them — BLOB_UPLOAD_UNKNOWN when one chunk reached another master than the last.
func TestThePushSlicesOfTheSelectorAreRemoved(t *testing.T) {
	stale := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{Name: PushService + "-abcde", Namespace: Namespace, Labels: map[string]string{
			discoveryv1.LabelServiceName: PushService,
			discoveryv1.LabelManagedBy:   "endpointslice-controller.k8s.io",
		}},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints:   []discoveryv1.Endpoint{{Addresses: []string{"10.0.0.1"}}, {Addresses: []string{"10.0.0.2"}}},
	}
	unrelated := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{Name: "registry-abcde", Namespace: Namespace, Labels: map[string]string{
			discoveryv1.LabelServiceName: "registry",
		}},
		AddressType: discoveryv1.AddressTypeIPv4,
	}
	r, c := newReconciler(t, cachedConfig(), accessSecret(), node("master-0"), node("master-1"),
		storageWithReplicas(), storageLease("master-1"), stale, unrelated)
	runReconcile(t, r)

	slices := &discoveryv1.EndpointSliceList{}
	require.NoError(t, c.List(context.Background(), slices, client.InNamespace(Namespace)))
	var names []string
	for _, slice := range slices.Items {
		names = append(names, slice.Name)
	}
	assert.ElementsMatch(t, []string{pushEndpointSlice, "registry-abcde"}, names,
		"only the leader's slice is left for the publication endpoint, and nothing else is touched")
}

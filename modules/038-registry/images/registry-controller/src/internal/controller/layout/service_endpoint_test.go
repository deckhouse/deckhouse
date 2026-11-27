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
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// storeService is the Service the template renders for the cache: the store's, without a selector.
func storeService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: constant.ServiceName, Namespace: Namespace, Labels: map[string]string{"app": "registry-storage"}},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "registry", Port: 5001}, {Name: "storage", Port: 4281}}},
	}
}

func serviceSlice(t *testing.T, c client.Client) *discoveryv1.EndpointSlice {
	t.Helper()
	slice := &discoveryv1.EndpointSlice{}
	require.NoError(t, c.Get(t.Context(),
		types.NamespacedName{Namespace: Namespace, Name: serviceEndpointSlice}, slice))
	return slice
}

// A pod reads the key from the leader: what `d8 mirror push` publishes beside the image set, such as
// the security databases, is on the leader alone, and a follower of an air-gapped cache answers 404.
func TestAPodReadsFromTheLeader(t *testing.T) {
	r, c := newReconciler(t, cachedConfig(), accessSecret(), node("master-0"), node("master-1"),
		storageWithReplicas(), storageLease("master-1"), storeService())
	runReconcile(t, r)

	slice := serviceSlice(t, c)
	assert.Equal(t, constant.ServiceName, slice.Labels[discoveryv1.LabelServiceName])
	assert.Equal(t, pushManagedBy, slice.Labels[discoveryv1.LabelManagedBy],
		"the EndpointSlice controller must leave it alone")
	require.Len(t, slice.Endpoints, 1)
	assert.Equal(t, []string{"10.0.0.2"}, slice.Endpoints[0].Addresses, "the one the lease names")

	ports := map[string]int32{}
	for _, port := range slice.Ports {
		ports[*port.Name] = *port.Port
	}
	assert.Equal(t, map[string]int32{"registry": 4281, "storage": 4281}, ports,
		"both of the Service's ports, to the cache's serving listener")
	require.Len(t, slice.OwnerReferences, 1, "goes with the cache")

	t.Run("and follows the lease", func(t *testing.T) {
		lease := &coordinationv1.Lease{}
		require.NoError(t, c.Get(t.Context(),
			types.NamespacedName{Namespace: Namespace, Name: StorageLeaseName}, lease))
		holder := "master-0"
		lease.Spec.HolderIdentity = &holder
		require.NoError(t, c.Update(t.Context(), lease))

		runReconcile(t, r)
		assert.Equal(t, []string{"10.0.0.1"}, serviceSlice(t, c).Endpoints[0].Addresses)
	})
}

// With the lease empty a pod keeps reading where the nodes do, rather than nowhere: a read, unlike a
// push, lands correctly on the replica that served a moment ago.
func TestWithoutALeaderAPodKeepsReadingWhereTheNodesDo(t *testing.T) {
	r, c := newReconciler(t, cachedConfig(), accessSecret(), node("master-0"), node("master-1"),
		storageWithReplicas(), storageLease("master-1"), storeService())
	runReconcile(t, r)

	lease := &coordinationv1.Lease{}
	require.NoError(t, c.Get(t.Context(),
		types.NamespacedName{Namespace: Namespace, Name: StorageLeaseName}, lease))
	lease.Spec.HolderIdentity = nil
	require.NoError(t, c.Update(t.Context(), lease))
	runReconcile(t, r)

	assert.Empty(t, pushSlice(t, c).Endpoints, "a push is refused without a leader")
	reads := serviceSlice(t, c)
	require.Len(t, reads.Endpoints, 1)
	assert.Equal(t, []string{"10.0.0.2"}, reads.Endpoints[0].Addresses, "the endpoint the node layouts carry")
}

// With neither a leader nor an endpoint the nodes have, the nodes are given the first address rather than
// none, and a pod reads there too, rather than from an empty Service: a cache turned on a moment ago, or
// the leader's master replaced.
func TestWithNeitherAPodReadsWhereTheNodesDo(t *testing.T) {
	r, c := newReconciler(t, cachedConfig(), accessSecret(), node("master-0"), node("master-1"),
		storageWithReplicas(), storeService())
	runReconcile(t, r)

	applied := listNodes(t, c)["master-0"]
	storage := applied.Spec.Backend(registryv1alpha1.BackendStorage)
	require.NotNil(t, storage)
	host, _, err := net.SplitHostPort(storage.Endpoint.Host)
	require.NoError(t, err)

	reads := serviceSlice(t, c)
	require.Len(t, reads.Endpoints, 1, "not an empty Service while the nodes read from a replica")
	assert.Equal(t, []string{host}, reads.Endpoints[0].Addresses)
	assert.Equal(t, "10.0.0.1", host, "the first address, with no lease taken")
}

func TestServicePortFollowsTheReportedServingPort(t *testing.T) {
	assert.EqualValues(t, 4281, servicePort("10.0.0.1:4281"))
	assert.EqualValues(t, 5001, servicePort("10.0.0.1:5001"), "a replica still on the previous pod spec")
	assert.EqualValues(t, 4281, servicePort(""))
}

// The slices the EndpointSlice controller wrote while the Service selected every replica outlive the
// selector; left, they would keep sending a pod to a follower.
func TestTheServiceSlicesOfTheSelectorAreRemoved(t *testing.T) {
	stale := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{Name: "registry-abcde", Namespace: Namespace, Labels: map[string]string{
			discoveryv1.LabelServiceName: constant.ServiceName,
			discoveryv1.LabelManagedBy:   "endpointslice-controller.k8s.io",
		}},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints:   []discoveryv1.Endpoint{{Addresses: []string{"10.0.0.1"}}, {Addresses: []string{"10.0.0.2"}}},
	}
	r, c := newReconciler(t, cachedConfig(), accessSecret(), node("master-0"), node("master-1"),
		storageWithReplicas(), storageLease("master-1"), stale, storeService())
	runReconcile(t, r)

	slices := &discoveryv1.EndpointSliceList{}
	require.NoError(t, c.List(t.Context(), slices, client.InNamespace(Namespace),
		client.MatchingLabels{discoveryv1.LabelServiceName: constant.ServiceName}))
	require.Len(t, slices.Items, 1)
	assert.Equal(t, serviceEndpointSlice, slices.Items[0].Name)
}

// During the move from the previous implementation the name is its Service's, whose slices are the pull
// path of a node not on the agent yet: neither pointed elsewhere nor cleaned until legacy_cleanup.go
// removes it, and no more while the store's Service still has a selector.
func TestTheServiceIsLeftAloneUntilItIsTheSelectorlessStoreOne(t *testing.T) {
	for name, service := range map[string]*corev1.Service{
		"the previous implementation's": {
			ObjectMeta: metav1.ObjectMeta{Name: constant.ServiceName, Namespace: Namespace, Labels: map[string]string{"app": "registry"}},
			Spec:       corev1.ServiceSpec{ClusterIP: corev1.ClusterIPNone, Selector: map[string]string{"component": "registry-service"}},
		},
		"the store's, still with a selector": {
			ObjectMeta: metav1.ObjectMeta{Name: constant.ServiceName, Namespace: Namespace, Labels: map[string]string{"app": "registry-storage"}},
			Spec:       corev1.ServiceSpec{ClusterIP: corev1.ClusterIPNone, Selector: map[string]string{"app": "registry-storage"}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			legacy := &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{Name: "registry-abcde", Namespace: Namespace, Labels: map[string]string{
					discoveryv1.LabelServiceName: constant.ServiceName,
					discoveryv1.LabelManagedBy:   "endpointslice-controller.k8s.io",
				}},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints:   []discoveryv1.Endpoint{{Addresses: []string{"10.111.0.7"}}},
			}
			r, c := newReconciler(t, cachedConfig(), accessSecret(), node("master-0"), node("master-1"),
				storageWithReplicas(), storageLease("master-1"), service, legacy)
			runReconcile(t, r)

			slices := &discoveryv1.EndpointSliceList{}
			require.NoError(t, c.List(t.Context(), slices, client.InNamespace(Namespace),
				client.MatchingLabels{discoveryv1.LabelServiceName: constant.ServiceName}))
			require.Len(t, slices.Items, 1)
			assert.Equal(t, "registry-abcde", slices.Items[0].Name, "the existing pull path stays as it is")
		})
	}
}

// The Service is not watched: while it is not yet the store's selectorless one, as right after the
// previous implementation's goes and before Helm creates it, the reconciler asks again by itself.
func TestTheReconcilerAsksAgainUntilTheServiceIsSelectorless(t *testing.T) {
	r, c := newReconciler(t, cachedConfig(), accessSecret(), node("master-0"), node("master-1"),
		storageWithReplicas(), storageLease("master-1"))
	result, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: types.NamespacedName{Name: registryv1alpha1.SingletonName}})
	require.NoError(t, err)
	assert.Equal(t, serviceRetryInterval, result.RequeueAfter)

	require.NoError(t, c.Create(t.Context(), storeService()))
	result, err = r.Reconcile(t.Context(), ctrl.Request{NamespacedName: types.NamespacedName{Name: registryv1alpha1.SingletonName}})
	require.NoError(t, err)
	assert.Zero(t, result.RequeueAfter, "pointed, nothing to wait for")
	assert.NotEmpty(t, serviceSlice(t, c).Endpoints)
}

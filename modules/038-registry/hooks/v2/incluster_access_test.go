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

package v2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1core "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"
)

func TestInClusterAccess(t *testing.T) {
	routable := &registryService{Name: "registry", Storage: true, Routes: true}
	headless := &registryService{Name: "registry", Headless: true, Storage: true, Routes: true}
	legacy := &registryService{Name: "registry", Headless: true, Legacy: true, Routes: true}

	cases := []struct {
		name           string
		state          inClusterAccessState
		access         bool
		deleteHeadless bool
		legacy         bool
	}{{
		name:  "no Service: Unmanaged, or the store's Service not rendered yet",
		state: inClusterAccessState{},
	}, {
		// The Service from before this change: replaced, and nothing is published through it, since a
		// pod dialling the key at a headless name reaches the masters' 5001, where nothing listens.
		name:           "the store's headless Service",
		state:          inClusterAccessState{Service: headless, Ready: 3},
		deleteHeadless: true,
	}, {
		// The previous implementation's: neither replaced here nor taken over by the template.
		name:   "the previous implementation's Service",
		state:  inClusterAccessState{Service: legacy, Ready: 1},
		legacy: true,
	}, {
		name:  "routable, no replica serves yet",
		state: inClusterAccessState{Service: routable},
	}, {
		name:   "routable, a replica serves",
		state:  inClusterAccessState{Service: routable, Ready: 1},
		access: true,
	}, {
		// Sticky: a replica restarting must not converge every module twice.
		name:   "published, no replica serves at the moment",
		state:  inClusterAccessState{Service: routable, Published: true},
		access: true,
	}, {
		name:  "routable, but without the key's port",
		state: inClusterAccessState{Service: &registryService{Name: "registry", Storage: true}, Ready: 1},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.access, tc.state.inClusterAccess(), "access")
			assert.Equal(t, tc.deleteHeadless, tc.state.deleteHeadless(), "delete headless")
			assert.Equal(t, tc.legacy, tc.state.legacyService(), "legacy")
		})
	}
}

func TestFilterRegistryService(t *testing.T) {
	cases := []struct {
		name    string
		service v1core.Service
		want    registryService
	}{{
		name: "the store's headless Service",
		service: v1core.Service{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "registry-storage"}}, Spec: v1core.ServiceSpec{
			ClusterIP: v1core.ClusterIPNone,
			Selector:  map[string]string{"app": "registry-storage"},
			Ports:     []v1core.ServicePort{{Name: "registry", Port: 5001}},
		}},
		want: registryService{Name: "registry", Headless: true, Storage: true, Routes: true},
	}, {
		name: "the store's routable Service with the cache: no selector, the controller points it at the leader",
		service: v1core.Service{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "registry-storage"}}, Spec: v1core.ServiceSpec{
			ClusterIP: "10.222.0.15",
			Ports:     []v1core.ServicePort{{Name: "registry", Port: 5001}, {Name: "storage", Port: 4281}},
		}},
		want: registryService{Name: "registry", Storage: true, Routes: true, Selectorless: true},
	}, {
		name: "the routable Service without the cache, over the proxy for pods",
		service: v1core.Service{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "registry-storage"}}, Spec: v1core.ServiceSpec{
			ClusterIP: "10.222.0.15",
			Selector:  map[string]string{"app": "registry-storage-proxy"},
			Ports:     []v1core.ServicePort{{Name: "registry", Port: 5001}, {Name: "storage", Port: 4281}},
		}},
		want: registryService{Name: "registry", Storage: true, Routes: true},
	}, {
		name: "the previous implementation's Service",
		service: v1core.Service{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "registry"}}, Spec: v1core.ServiceSpec{
			ClusterIP: v1core.ClusterIPNone,
			Selector:  map[string]string{"component": "registry-service", "type": "Direct"},
			Ports:     []v1core.ServicePort{{Name: "registry", Port: 5001}},
		}},
		want: registryService{Name: "registry", Headless: true, Legacy: true, Routes: true},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.service.Name = "registry"
			got, err := filterRegistryService(toUnstructured(t, &tc.service))
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestFilterReadyStorageEndpoints(t *testing.T) {
	slice := func(port int32, ready ...*bool) *discoveryv1.EndpointSlice {
		result := &discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{Name: "registry-abcde"},
			Ports:      []discoveryv1.EndpointPort{{Name: ptr.To("registry"), Port: ptr.To(port)}},
		}
		for _, r := range ready {
			result.Endpoints = append(result.Endpoints, discoveryv1.Endpoint{
				Addresses:  []string{"192.168.199.10"},
				Conditions: discoveryv1.EndpointConditions{Ready: r},
			})
		}
		return result
	}

	cases := []struct {
		name  string
		slice *discoveryv1.EndpointSlice
		want  int
	}{
		{"two of three ready", slice(4281, ptr.To(true), ptr.To(false), ptr.To(true)), 2},
		{"readiness unknown counts as ready", slice(4281, nil), 1},
		{"a replica on another port does not serve the store", slice(5001, ptr.To(true)), 0},
		{"no endpoints", slice(4281), 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := filterReadyStorageEndpoints(toUnstructured(t, tc.slice))
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.(storageSlice).Ready)
		})
	}
}

// Only the slices of the Service as it is now count: the controller's for the selectorless one with the
// cache. Kubernetes' slices of the headless Service it replaced, not yet collected, name every replica
// and would open the key to pods before the controller names the one the nodes read from.
func TestReadyForCountsOnlyTheSlicesOfTheServiceAsItIsNow(t *testing.T) {
	controllers := storageSlice{Ready: 1, ManagedBy: "registry-controller.deckhouse.io"}
	kubernetes := storageSlice{Ready: 3, ManagedBy: "endpointslice-controller.k8s.io"}

	selectorless := &registryService{Name: "registry", Storage: true, Routes: true, Selectorless: true}
	assert.Equal(t, 0, readyFor(selectorless, []storageSlice{kubernetes}), "the replaced Service's leftovers")
	assert.Equal(t, 1, readyFor(selectorless, []storageSlice{kubernetes, controllers}))

	selecting := &registryService{Name: "registry", Storage: true, Routes: true}
	assert.Equal(t, 3, readyFor(selecting, []storageSlice{kubernetes, controllers}), "the proxy pods, without the cache")
	assert.Equal(t, 0, readyFor(nil, []storageSlice{kubernetes}))
}

func toUnstructured(t *testing.T, obj runtime.Object) *unstructured.Unstructured {
	t.Helper()
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	require.NoError(t, err)
	return &unstructured.Unstructured{Object: content}
}

// The headless Service is deleted exactly as the snapshot saw it: one Helm has recreated since, routable,
// stays, and a Service already gone is not an error.
func TestDeleteHeadlessServiceDeletesOnlyWhatTheSnapshotSaw(t *testing.T) {
	headless := &v1core.Service{ObjectMeta: metav1.ObjectMeta{
		Name: "registry", Namespace: "d8-system", UID: "headless-uid", ResourceVersion: "7",
	}}

	t.Run("the object the snapshot saw", func(t *testing.T) {
		client := fake.NewClientset(headless.DeepCopy())
		deleted, err := deleteHeadlessService(t.Context(), client.CoreV1(),
			registryService{Name: "registry", UID: "headless-uid", ResourceVersion: "7"})
		require.NoError(t, err)
		assert.True(t, deleted)
		_, err = client.CoreV1().Services("d8-system").Get(t.Context(), "registry", metav1.GetOptions{})
		assert.True(t, apierrors.IsNotFound(err))
	})

	t.Run("one recreated since", func(t *testing.T) {
		recreated := headless.DeepCopy()
		recreated.UID, recreated.ResourceVersion = "routable-uid", "9"
		client := fake.NewClientset(recreated)
		client.PrependReactor("delete", "services", preconditionReactor(recreated))
		deleted, err := deleteHeadlessService(t.Context(), client.CoreV1(),
			registryService{Name: "registry", UID: "headless-uid", ResourceVersion: "7"})
		require.NoError(t, err)
		assert.False(t, deleted)
		_, err = client.CoreV1().Services("d8-system").Get(t.Context(), "registry", metav1.GetOptions{})
		require.NoError(t, err, "the routable Service stays")
	})

	t.Run("already gone", func(t *testing.T) {
		deleted, err := deleteHeadlessService(t.Context(), fake.NewClientset().CoreV1(),
			registryService{Name: "registry", UID: "headless-uid", ResourceVersion: "7"})
		require.NoError(t, err)
		assert.False(t, deleted)
	})
}

// preconditionReactor refuses a delete whose UID precondition does not name the object, as the API
// server does; the fake clientset ignores preconditions.
func preconditionReactor(current *v1core.Service) k8stesting.ReactionFunc {
	return func(action k8stesting.Action) (bool, runtime.Object, error) {
		del, ok := action.(k8stesting.DeleteActionImpl)
		if !ok || del.DeleteOptions.Preconditions == nil || del.DeleteOptions.Preconditions.UID == nil {
			return false, nil, nil
		}
		if *del.DeleteOptions.Preconditions.UID != current.UID {
			return true, nil, apierrors.NewConflict(v1core.Resource("services"), current.Name, nil)
		}
		return false, nil, nil
	}
}

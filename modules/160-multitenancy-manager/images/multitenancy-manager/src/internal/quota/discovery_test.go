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

package quota

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	fakediscovery "k8s.io/client-go/discovery/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestAPIDiscoveryLookup(t *testing.T) {
	all := []string{"create", "delete", "get", "list", "watch"}
	fake := &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{Resources: []*metav1.APIResourceList{
		{GroupVersion: "v1", APIResources: []metav1.APIResource{
			{Name: "pods", Namespaced: true, Verbs: all},
			{Name: "namespaces", Verbs: all},
		}},
		{GroupVersion: "deckhouse.io/v1alpha3", APIResources: []metav1.APIResource{
			{Name: "projectnamespaces", Namespaced: true, Verbs: all},
			{Name: "projectnamespaces/status", Namespaced: true, Verbs: []string{"get", "update"}},
		}},
		{GroupVersion: "events.k8s.io/v1", APIResources: []metav1.APIResource{
			{Name: "events", Namespaced: true, Verbs: []string{"create", "get", "list"}},
		}},
	}}}
	d := &APIDiscovery{Client: memory.NewMemCacheClient(fake)}

	tests := []struct {
		group, resource string
		expected        Served
	}{
		{"", "pods", ServedNamespaced},
		{"", "namespaces", ServedClusterScoped},
		{"deckhouse.io", "projectnamespaces", ServedNamespaced},
		{"deckhouse.io", "projectnamespace", ServedResourceMissing},
		{"events.k8s.io", "events", ServedNotCountable},
		{"example.com", "widgets", ServedGroupNotServed},
	}
	for _, tt := range tests {
		t.Run(tt.resource+"."+tt.group, func(t *testing.T) {
			got, err := d.Lookup(tt.group, tt.resource)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// countingDiscovery counts the invalidations of a cached discovery.
type countingDiscovery struct {
	discovery.CachedDiscoveryInterface
	invalidations int
}

func (c *countingDiscovery) Invalidate() {
	c.invalidations++
	c.CachedDiscoveryInterface.Invalidate()
}

// A key of a group that is not served misses on every reconcile of its quota; the cached discovery of
// the controller is dropped for it at most once in ReconcileInvalidateEvery, while a webhook drops its
// own on every miss.
func TestAPIDiscoveryInvalidatesSeldom(t *testing.T) {
	fake := &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{Resources: []*metav1.APIResourceList{
		{GroupVersion: "v1", APIResources: []metav1.APIResource{{Name: "pods", Namespaced: true}}},
	}}}
	cached := &countingDiscovery{CachedDiscoveryInterface: memory.NewMemCacheClient(fake)}
	clock := time.Now()
	d := &APIDiscovery{Client: cached, InvalidateEvery: ReconcileInvalidateEvery, Now: func() time.Time { return clock }}

	for range 5 {
		served, err := d.Lookup("example.com", "widgets")
		require.NoError(t, err)
		assert.Equal(t, ServedGroupNotServed, served)
	}
	assert.Equal(t, 1, cached.invalidations)

	clock = clock.Add(ReconcileInvalidateEvery)
	_, err := d.Lookup("example.com", "widgets")
	require.NoError(t, err)
	assert.Equal(t, 2, cached.invalidations)

	webhook := &APIDiscovery{Client: cached, Now: func() time.Time { return clock }}
	for range 3 {
		_, err := webhook.Lookup("example.com", "widgets")
		require.NoError(t, err)
	}
	assert.Equal(t, 5, cached.invalidations, "a webhook invalidates on every miss")
}

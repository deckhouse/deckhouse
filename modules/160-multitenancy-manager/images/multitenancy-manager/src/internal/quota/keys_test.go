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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// fakeDiscovery serves the resources it lists, keyed "<resource>.<group>"; a group it has no
// resource of is not served.
type fakeDiscovery map[string]Served

func (d fakeDiscovery) Lookup(group, resource string) (Served, error) {
	if served, ok := d[resource+"."+group]; ok {
		return served, nil
	}
	for key := range d {
		if len(key) > len(group) && key[len(key)-len(group)-1:] == "."+group {
			return ServedResourceMissing, nil
		}
	}
	return ServedGroupNotServed, nil
}

var testDiscovery = fakeDiscovery{
	"pods.":                                       ServedNamespaced,
	"projectnamespaces.deckhouse.io":              ServedNamespaced,
	"projects.deckhouse.io":                       ServedClusterScoped,
	"namespaces.":                                 ServedClusterScoped,
	"ingresses.networking.k8s.io":                 ServedNamespaced,
	"events.events.k8s.io":                        ServedNotCountable,
	"virtualmachines.virtualization.deckhouse.io": ServedNamespaced,
}

func list(keys ...string) corev1.ResourceList {
	out := corev1.ResourceList{}
	for _, key := range keys {
		out[corev1.ResourceName(key)] = resource.MustParse("1")
	}
	return out
}

func TestCheckKeys(t *testing.T) {
	tests := []struct {
		name         string
		hard         corev1.ResourceList
		old          corev1.ResourceList
		denied       int
		warnings     int
		notServed    []string
		deniedSubstr string
	}{
		{name: "standard keys", hard: list("cpu", "memory", "requests.cpu", "limits.memory", "pods", "services.loadbalancers", "requests.storage", "configmaps")},
		{name: "class and extended keys", hard: list("fast.storageclass.storage.k8s.io/requests.storage", "fast.storageclass.storage.k8s.io/persistentvolumeclaims",
			"gpu.deviceclass.resource.k8s.io/devices", "requests.nvidia.com/gpu", "requests.hugepages-2Mi")},
		{name: "count of a namespaced resource", hard: list("count/pods", "count/ingresses.networking.k8s.io", "count/projectnamespaces.deckhouse.io")},
		{name: "the alias", hard: list("projectNamespaces")},
		{name: "the alias next to its key", hard: list("projectNamespaces", "count/projectnamespaces.deckhouse.io"), denied: 1, deniedSubstr: "same limit"},
		{name: "resourcequotas", hard: list("resourcequotas"), denied: 1, deniedSubstr: "d8-project-quota"},
		{name: "count of resourcequotas", hard: list("count/resourcequotas"), denied: 1},
		{name: "count of a cluster-scoped resource", hard: list("count/namespaces", "count/projects.deckhouse.io"), denied: 2, deniedSubstr: "cluster-scoped"},
		{name: "a typo in a served group", hard: list("count/projectnamespace.deckhouse.io"), denied: 1, deniedSubstr: "spelling"},
		{name: "a resource the controller cannot count", hard: list("count/events.events.k8s.io"), denied: 1, deniedSubstr: "verbs"},
		{name: "a group not served yet", hard: list("count/dexclients.deckhouse.example"), warnings: 1, notServed: []string{"count/dexclients.deckhouse.example"}},
		{name: "the reserved prefix", hard: list("projectquota.deckhouse.io/anything"), denied: 1, deniedSubstr: "reserved"},
		{name: "a family not counted yet", hard: list("virtualmachines.virtualization.deckhouse.io/cores", "default.loadbalancerclass.network.deckhouse.io/services"),
			denied: 2, deniedSubstr: "family"},
		{name: "count of virtual machines is a plain count key", hard: list("count/virtualmachines.virtualization.deckhouse.io")},
		{name: "an unknown key", hard: list("example.com/widgets"), denied: 1, deniedSubstr: "does not count"},
		{name: "a limit of an extended resource", hard: list("limits.nvidia.com/gpu"), denied: 1},
		{name: "a kept key passes with a warning", hard: list("example.com/widgets", "cpu"), old: list("example.com/widgets"), warnings: 1},
		{name: "a kept key does not let a new one in", hard: list("example.com/widgets", "example.com/gadgets"), old: list("example.com/widgets"), denied: 1, warnings: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check, err := CheckKeys(tt.hard, tt.old, testDiscovery)
			require.NoError(t, err)
			assert.Len(t, check.Denied, tt.denied, "denied: %v", check.Denied)
			assert.Len(t, check.Warnings, tt.warnings, "warnings: %v", check.Warnings)
			assert.Equal(t, tt.notServed, check.NotServed)
			if tt.deniedSubstr != "" {
				require.NotEmpty(t, check.Denied)
				assert.Contains(t, check.Denied[0], tt.deniedSubstr)
			}
		})
	}
}

func TestCheckKeysWithoutDiscovery(t *testing.T) {
	check, err := CheckKeys(list("count/anything.example.com"), nil, nil)
	require.NoError(t, err)
	assert.Empty(t, check.Denied)
	assert.Empty(t, check.Warnings)
}

func TestCanonical(t *testing.T) {
	hard := corev1.ResourceList{"projectNamespaces": resource.MustParse("3"), "cpu": resource.MustParse("2")}
	got := Canonical(hard)
	assert.Equal(t, corev1.ResourceList{ProjectNamespacesKey: resource.MustParse("3"), "cpu": resource.MustParse("2")}, got)
	assert.Contains(t, hard, ProjectNamespacesAlias, "the input is left as written")
}

func TestResourceQuotaHard(t *testing.T) {
	got := ResourceQuotaHard(list("cpu", "projectquota.deckhouse.io/x", "virtualmachines.virtualization.deckhouse.io/cores",
		"count/virtualmachines.virtualization.deckhouse.io", "example.com/kept"))
	assert.Equal(t, list("cpu", "count/virtualmachines.virtualization.deckhouse.io", "example.com/kept"), got)
}

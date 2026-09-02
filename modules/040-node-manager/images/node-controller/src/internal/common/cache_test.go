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

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

// configMapByObject finds the ConfigMap entry. The map is keyed by client.Object values, so a
// freshly constructed &corev1.ConfigMap{} is a different key than the one stored in the map and a
// plain index expression would always miss.
func configMapByObject(t *testing.T, opts cache.Options) cache.ByObject {
	t.Helper()

	for obj, byObject := range opts.ByObject {
		if _, ok := obj.(*corev1.ConfigMap); ok {
			return byObject
		}
	}
	t.Fatal("no ByObject entry for ConfigMap")
	return cache.ByObject{}
}

// TestKubeSystemConfigMapsAreCachedUnfiltered guards a failure mode that no behavioural test can
// see: a cached Get for an object outside the configured scope does not error, it returns NotFound,
// and the caller degrades or requeues forever waiting for an object the informer will never hold.
// That bit the derived-status service once, while kube-system ConfigMaps were pinned by name to
// d8-cluster-uuid and d8-cluster-kubernetes — the source of the target Kubernetes version — was
// never found.
//
// Two objects are read here, d8-cluster-uuid and d8-cluster-kubernetes, and a name field selector
// matches exactly one — field selectors have no OR. Hence no selector at all, the same trade the
// kube-system Secrets entry already makes: the set is bounded by the platform and small.
func TestKubeSystemConfigMapsAreCachedUnfiltered(t *testing.T) {
	opts, _ := CacheOptions()

	configMaps := configMapByObject(t, opts)

	kubeSystem, ok := configMaps.Namespaces["kube-system"]
	require.True(t, ok, "kube-system must be in the ConfigMap cache scope: it is where "+
		"d8-cluster-uuid and d8-cluster-kubernetes live")

	assert.Nil(t, kubeSystem.FieldSelector,
		"a name field selector pins exactly one object, and this binary reads two kube-system "+
			"ConfigMaps through the cached client")
	assert.Nil(t, kubeSystem.LabelSelector,
		"d8-cluster-uuid is created by dhctl without the heritage label, so a label selector "+
			"would silently drop it")
}

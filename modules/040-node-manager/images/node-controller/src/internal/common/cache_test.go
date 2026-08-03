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

// TestCacheScopeCoversKubeSystemConfigMaps guards a failure mode that no behavioural test can see:
// a cached Get for an object outside the configured scope does not error, it returns NotFound. The
// caller then requeues forever waiting for a ConfigMap the informer will never see.
//
// This bit the derived-status service: kube-system ConfigMaps were pinned to d8-cluster-uuid by a
// name FieldSelector, so d8-cluster-kubernetes — the source of the cluster's target Kubernetes
// version — was never found and the ConfigMap watch never fired.
//
// A name FieldSelector can only ever match one object, so as long as two different kube-system
// ConfigMaps are read there must be no field or label filter on that namespace.
func TestCacheScopeCoversKubeSystemConfigMaps(t *testing.T) {
	opts, _ := CacheOptions()

	configMaps := configMapByObject(t, opts)

	kubeSystem, ok := configMaps.Namespaces["kube-system"]
	require.True(t, ok, "kube-system must be in the ConfigMap cache scope: the derived-status "+
		"service reads d8-cluster-uuid and d8-cluster-kubernetes from it")

	assert.Nil(t, kubeSystem.FieldSelector,
		"kube-system ConfigMaps must not be filtered by name: d8-cluster-uuid and "+
			"d8-cluster-kubernetes are both read, and a name FieldSelector matches only one")
	assert.Nil(t, kubeSystem.LabelSelector,
		"kube-system ConfigMaps must not be filtered by label: d8-cluster-uuid is created by "+
			"dhctl without any labels")
}

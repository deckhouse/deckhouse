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
// caller then requeues forever waiting for a ConfigMap the informer will never see. That bit the
// derived-status service once, when kube-system ConfigMaps were pinned to d8-cluster-uuid and
// d8-cluster-kubernetes — the source of the target Kubernetes version — was never found.
//
// The scope is one object, not the whole namespace: kube-system is a namespace users write to, so an
// unscoped informer would watch and hold arbitrary third-party ConfigMaps of unbounded size. A name
// field selector matches exactly one object and field selectors have no OR, so the pin is spent on
// the one ConfigMap that needs a *watch*, and d8-cluster-uuid — immutable, watch-free — is read
// through the uncached reader by common.ClusterUUID instead.
//
// Anything that adds a second cached kube-system ConfigMap read has to fail here rather than
// silently requeue in production: either route it through the uncached reader too, or change the
// scope deliberately.
func TestCacheScopeCoversKubeSystemConfigMaps(t *testing.T) {
	opts, _ := CacheOptions()

	configMaps := configMapByObject(t, opts)

	kubeSystem, ok := configMaps.Namespaces["kube-system"]
	require.True(t, ok, "kube-system must be in the ConfigMap cache scope: it is where "+
		"d8-cluster-kubernetes lives and the nodegroup status controller watches it")

	require.NotNil(t, kubeSystem.FieldSelector,
		"kube-system ConfigMaps must stay pinned to one object: users create ConfigMaps in this "+
			"namespace and an unscoped informer would cache all of them")
	assert.Equal(t, "metadata.name="+ClusterKubernetesConfigMapName, kubeSystem.FieldSelector.String(),
		"the pin must name the ConfigMap that needs a watch; every other kube-system ConfigMap "+
			"read in this binary must go through the uncached reader (see common.ClusterUUID)")
}

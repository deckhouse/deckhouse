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

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
)

// TestSecretsAreNotCached guards a decision that fails in a cluster and nowhere else.
//
// The controller's access to secrets is granted by name, and RBAC cannot authorize list or
// watch that way — a list is not a request for a name. So a cached secret read is refused,
// the informer never starts, and with it neither does any controller in this manager: no
// RegistryStorage, no RegistryNode, and nodes that cannot pull. Nothing in the module's own
// status says so. It took a three-master install to notice the first time, and a second
// install to notice that scoping the cache to one namespace does not help.
func TestSecretsAreNotCached(t *testing.T) {
	opts := clientOptions()

	require.NotNil(t, opts.Cache, "no cache options at all means secrets are cached")
	require.NotEmpty(t, opts.Cache.DisableFor)

	var found bool
	for _, object := range opts.Cache.DisableFor {
		if _, ok := object.(*corev1.Secret); ok {
			found = true
		}
	}
	assert.True(t, found, "secrets must be read uncached, or their name-scoped grant cannot work")
}

// TestLeasesAreNotCached is the same failure through a different door, and the third time this
// cache has cost a cluster.
//
// The storage election lease is granted by a Role in one namespace. This cache is cluster-scoped,
// so its informer lists leases at the cluster scope, and that is refused — the informer never
// syncs, the read waiting on it never returns, and the reconciliation never finishes. The
// symptom is not a permission error anywhere an operator would look: it is zero RegistryNode
// objects on a cluster with the cache enabled, every node without a layout, and one
// `Failed to watch` line in this controller's log.
func TestLeasesAreNotCached(t *testing.T) {
	opts := clientOptions()

	require.NotNil(t, opts.Cache)

	var found bool
	for _, object := range opts.Cache.DisableFor {
		if _, ok := object.(*coordinationv1.Lease); ok {
			found = true
		}
	}
	assert.True(t, found,
		"the lease must be read uncached: a cluster-scoped informer cannot be authorized by a namespaced Role")
}

// TestManagerElectsALeader guards the line whose absence made every replica act on its own.
//
// Nothing was electing anything, so each reconciler ran in every controller pod. Measured on a
// three-master cluster: the storage update controller replaced TWO cache replicas at once, two
// instances having each chosen "the next stale pod" seven milliseconds apart. The refusal to move
// while a replica is missing or not yet serving is what keeps a replacement safe, and it holds only
// within one process — so a second process undoes it and there is no error anywhere to say so.
//
// The symptom cannot appear on a single-master cluster, where there is one replica to begin with.
// That is why this is pinned by a test rather than left to the next multimaster run to rediscover.
func TestManagerElectsALeader(t *testing.T) {
	opts := managerOptions(":0", ":0")

	require.True(t, opts.LeaderElection,
		"with election off every replica reconciles, and every safeguard that counts replicas is undone by the other process")
	assert.Equal(t, leaderElectionLease, opts.LeaderElectionID)
	assert.Equal(t, registryNamespace, opts.LeaderElectionNamespace,
		"the lease belongs in the module's own namespace, which is the only one the Role covers")

	// A controller that keeps the lease after it stops leaves the cache unattended for the rest of
	// the lease's life, which on a rolling update of this deployment is exactly when it is needed.
	assert.True(t, opts.LeaderElectionReleaseOnCancel,
		"the lease must be handed over on shutdown rather than waited out")
}

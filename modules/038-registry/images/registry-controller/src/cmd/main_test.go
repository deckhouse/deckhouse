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

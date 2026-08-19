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
)

// TestTheCleanupDeletesOnlyWhatHasNoOwnerAndNoReader is a test about the list, not about the loop.
//
// The loop is three lines and cannot really fail; the list can, and the way it fails is by growing.
// Every name below that is spared is spared for a reason that costs a cluster if forgotten, and a
// future reader adding "the obviously dead registry-* secret" to the list is exactly who this is for.
func TestTheCleanupDeletesOnlyWhatHasNoOwnerAndNoReader(t *testing.T) {
	deleted := legacyLeftovers()

	assert.Contains(t, deleted, InitSecretName,
		"the installer's PKI hand-off has no reader once a cluster runs, and holds credentials")
	assert.Contains(t, deleted, LegacyStateSecretName,
		"the previous implementation's state machine is what the handover left behind")

	spared := map[string]string{
		BashibleConfigSecretName: "this implementation writes it: deleting it un-tells every node " +
			"which registry to use",
		"deckhouse-registry": "it is the imagePullSecret of this module's own storage and controller, " +
			"so deleting it stops the pods that serve the registry",
		"registry-config": "module 002 renders it on every cluster and dhctl reads it to resolve the " +
			"upstream when pushing a bundle",
	}

	for name, why := range spared {
		assert.NotContainsf(t, deleted, name, "%s must not be deleted: %s", name, why)
	}
}

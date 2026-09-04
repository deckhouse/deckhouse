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

// TestNothingIsDeletedBeforeTheHandoverIsRecorded is about the order of two irreversible things.
//
// Every object below is the previous implementation's own state, written to the API by the installer
// and by bashible, and deleting it cannot be undone. What justifies the deletion is the handover
// having happened — and "having happened" is a fact about the cluster, recorded by the marker secret,
// not the decision this run has just taken.
//
// The two used to be conflated: the hook keyed on the decision, at OnBeforeHelm, so the deletion
// went out before Helm had applied anything at all — including the marker. A release that failed on
// that pass (this module has failed a render for want of cert-manager, and does not control quota,
// admission or the apiserver either) left a cluster whose previous implementation's state was gone
// while nothing of the current one had been applied, and the gate's own input — the legacy state
// secret it reads to decide — was among what went.
func TestNothingIsDeletedBeforeTheHandoverIsRecorded(t *testing.T) {
	assert.False(t, deletionJustified(false, false),
		"the previous implementation still owns the cluster; every object here is alive")
	assert.False(t, deletionJustified(false, true),
		"a marker without an active implementation is a cluster on its way back, not one to clean up")
	assert.False(t, deletionJustified(true, false),
		"the decision to switch is not the switch: nothing has been applied yet, and this pass may fail")
	assert.True(t, deletionJustified(true, true),
		"the handover is recorded in the cluster, so what the previous implementation left has no reader")
}

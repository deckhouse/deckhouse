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

package cleanup

import (
	"testing"

	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

func TestShouldClean(t *testing.T) {
	if shouldClean(helpers.PhaseLegacy) {
		t.Error("must not clean in Legacy")
	}
	if shouldClean(helpers.PhaseTakingOver) {
		t.Error("must not clean in TakingOver")
	}
	if shouldClean(helpers.PhaseNew) {
		t.Error("must not clean in New (only CleanupPending)")
	}
	if !shouldClean(helpers.PhaseCleanupPending) {
		t.Error("must clean in CleanupPending")
	}
}

func TestCaDurable(t *testing.T) {
	if caDurable(false) {
		t.Error("must not delete PKI when CA not durable")
	}
	if !caDurable(true) {
		t.Error("must delete PKI when CA is durable")
	}
}

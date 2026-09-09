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

package fill

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCountDeclaredHeldIgnoresWhatNoReleaseDeclares is the one definition of "how much does this replica
// hold", and it exists because there used to be two.
//
// A store legitimately holds more than any release declares: the pass-through cache settles whatever the
// cluster pulls through it. Counting all of that answered a different question, and mixing the two
// answers into one field is what made completeness flap — a replica said 333 after a copying pass and
// 348 after a counting one. `full` followed whichever ran last; eligibility follows `full`, so the lease
// moved; and a fill restarts on every move. Measured on a cluster: the lease travelling between three
// replicas every twenty seconds, none of them ever finishing.
func TestCountDeclaredHeldIgnoresWhatNoReleaseDeclares(t *testing.T) {
	root := store(t)

	declared := map[string]struct{}{
		digestOne: {},
		digestTwo: {},
	}

	// One declared image is in, one is not — and something the cache settled on its own is in too.
	revisionLink(t, root, "system/deckhouse", digestOne)
	revisionLink(t, root, "system/deckhouse/module", digestThree)

	held, err := CountDeclaredHeld(root, "system/deckhouse", declared)
	require.NoError(t, err)
	assert.EqualValues(t, 1, held,
		"only what a kept release declares counts towards holding the set")

	// The second declared image arrives.
	revisionLink(t, root, "system/deckhouse/module", digestTwo)
	held, err = CountDeclaredHeld(root, "system/deckhouse", declared)
	require.NoError(t, err)
	assert.EqualValues(t, 2, held)

	// The same digest linked from two repositories is one digest held, exactly as the expectation
	// counts it once.
	revisionLink(t, root, "system/deckhouse/other", digestTwo)
	held, err = CountDeclaredHeld(root, "system/deckhouse", declared)
	require.NoError(t, err)
	assert.EqualValues(t, 2, held)
}

// TestCountDeclaredHeldWithNothingDeclared: an unknown set is not a full store, and zero is the honest
// answer rather than "everything I have".
func TestCountDeclaredHeldWithNothingDeclared(t *testing.T) {
	root := store(t)
	revisionLink(t, root, "system/deckhouse", digestOne)

	held, err := CountDeclaredHeld(root, "system/deckhouse", nil)
	require.NoError(t, err)
	assert.Zero(t, held)
}

// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controlplane

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/lib-dhctl/pkg/retry"
)

// Scaling a multi-master down keeps whichever master the state lists first, healthy or
// not, so the check runs before the first role is stripped, not after.
func TestBeforeActionStopsWhenCheckFails(t *testing.T) {
	wasInTestEnvironment := retry.InTestEnvironment
	retry.InTestEnvironment = true
	t.Cleanup(func() { retry.InTestEnvironment = wasInTestEnvironment })

	hook := NewHookForDestroyPipeline(
		unreachableKubeGetter{},
		nil,
		"cluster-master-2",
		map[string]string{"cluster-master-0": ""},
		false,
		true,
		true,
	)

	_, err := hook.BeforeAction(t.Context(), recreatedMasterRunner{})

	require.ErrorContains(t, err, "not all nodes are ready")
	require.ErrorContains(t, err, "cluster-master-0")
}

func TestBeforeActionRunsWithoutCheck(t *testing.T) {
	hook := &HookForDestroyPipeline{nodeToDestroy: "cluster-master-2"}

	_, err := hook.BeforeAction(t.Context(), destroyedMasterRunner{})

	require.NoError(t, err)
}

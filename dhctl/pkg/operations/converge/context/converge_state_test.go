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

package context

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The node list is the only record of converge-user accounts nobody has taken off the
// masters yet, and a cleanup skipped in commander or sshless mode reports no error to say
// so. Dropped with the state, those accounts live to their expiry date untracked.
func TestConvergeStateOutlivesNodesNobodyCleanedUp(t *testing.T) {
	store := &fakeStateStore{state: &State{ConvergeUserNodes: []string{"cluster-master-2"}}}

	ctx := NewContext(t.Context(), Params{})
	ctx.stateStore = store

	require.NoError(t, ctx.DeleteConvergeState())
	require.False(t, store.deleted, "the state was deleted while it still named a node carrying the converge user")

	store.state = &State{}

	require.NoError(t, ctx.DeleteConvergeState())
	require.True(t, store.deleted, "a converge that cleaned up after itself must leave no state behind")
}

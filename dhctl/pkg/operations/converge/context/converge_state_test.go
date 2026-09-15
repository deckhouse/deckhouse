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
	"time"

	"github.com/stretchr/testify/require"
)

// The node list is the only record of converge-user accounts nobody has taken off the
// masters yet. It stops being one at the expiry those accounts were created with: past it
// the names only send the next converge logging in as a user that is gone.
func TestConvergeStateOutlivesNodesNobodyCleanedUpUntilTheirExpiry(t *testing.T) {
	deleteWith := func(t *testing.T, state *State) bool {
		t.Helper()

		store := &fakeStateStore{state: state}

		ctx := NewContext(t.Context(), Params{})
		ctx.stateStore = store

		require.NoError(t, ctx.DeleteConvergeStateIfUserGone())

		return store.deleted
	}

	t.Run("a node still carrying a live account keeps the whole state", func(t *testing.T) {
		require.False(t, deleteWith(t, &State{
			ConvergeUserNodes:  []string{"cluster-master-2"},
			ConvergeUserExpiry: time.Now().Add(time.Hour),
		}))
	})

	t.Run("accounts past their expiry take the record with them", func(t *testing.T) {
		require.True(t, deleteWith(t, &State{
			ConvergeUserNodes:  []string{"cluster-master-2"},
			ConvergeUserExpiry: time.Now().Add(-time.Minute),
		}))
	})

	// A state written before the expiry was recorded is older than any account it names:
	// nothing that lives at most two days survives an upgrade of dhctl.
	t.Run("a list with no expiry at all is an expired one", func(t *testing.T) {
		require.True(t, deleteWith(t, &State{ConvergeUserNodes: []string{"cluster-master-2"}}))
	})

	t.Run("a converge that cleaned up after itself leaves no state behind", func(t *testing.T) {
		require.True(t, deleteWith(t, &State{}))
	})
}

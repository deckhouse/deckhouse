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

func TestSelectMasterStates(t *testing.T) {
	first := &NodeState{Name: "cluster-master-0", State: []byte("s0")}
	others := []*NodeState{
		{Name: "cluster-master-1", State: []byte("s1")},
		{Name: "cluster-master-2", State: []byte("s2")},
	}

	t.Run("only first", func(t *testing.T) {
		got := selectMasterStates(first, others, func(name string) bool {
			return name == "cluster-master-0"
		})
		require.Equal(t, map[string][]byte{"cluster-master-0": []byte("s0")}, got)
	})

	t.Run("all but first", func(t *testing.T) {
		got := selectMasterStates(first, others, func(name string) bool {
			return name != "cluster-master-0"
		})
		require.Len(t, got, 2)
		require.NotContains(t, got, "cluster-master-0")
	})

	t.Run("excluding deleted", func(t *testing.T) {
		deleted := map[string]struct{}{"cluster-master-1": {}}
		got := selectMasterStates(first, others, func(name string) bool {
			_, ok := deleted[name]
			return !ok
		})
		require.Len(t, got, 2)
		require.NotContains(t, got, "cluster-master-1")
	})

	t.Run("nil first is skipped", func(t *testing.T) {
		got := selectMasterStates(nil, others, func(string) bool { return true })
		require.Len(t, got, 2)
	})
}

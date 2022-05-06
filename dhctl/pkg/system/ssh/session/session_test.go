// Copyright 2021 Flant JSC
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

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/stringsutil"
)

func TestCreatingNewSShSession(t *testing.T) {
	const host = "a"

	ses := NewSession(Input{
		AvailableHosts: []string{host},
	})

	t.Run("Create settings with not empty AvailableHosts returns session struct without errors", func(t *testing.T) {
		require.NotNil(t, ses)
	})

	t.Run("Create settings with not empty AvailableHosts sets hosts field", func(t *testing.T) {
		require.Equal(t, ses.host, host)
	})

	t.Run("Create settings with not empty AvailableHosts choices host from remainingHosts (not contains host in remainingHosts)", func(t *testing.T) {
		require.NotContains(t, ses.remainingHosts, host)
	})
}

func TestSession_SetNewAvailableHosts(t *testing.T) {
	const oldHost = "a"
	const newHost = "b"

	oldHostsList := []string{oldHost}
	newHostsList := []string{newHost}

	tests := []struct {
		name   string
		assert func(t *testing.T, s *Session)
	}{
		{
			name: "Set new available hosts sets new host",
			assert: func(t *testing.T, s *Session) {
				require.Equal(t, s.host, newHost)
			},
		},

		{
			name: "Set new available sets new available list",
			assert: func(t *testing.T, s *Session) {
				require.Equal(t, s.availableHosts, newHostsList)
			},
		},

		{
			name: "Set new available hosts choices host from remainingHosts (not contains host in remainingHosts)",
			assert: func(t *testing.T, s *Session) {
				require.NotContains(t, s.remainingHosts, oldHost)
				require.NotContains(t, s.remainingHosts, newHost)
			},
		},
	}

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			s := NewSession(Input{
				AvailableHosts: oldHostsList,
			})

			s.SetAvailableHosts(newHostsList)

			tst.assert(t, s)
		})
	}
}

func TestSession_ChoiceNewHost(t *testing.T) {
	t.Run("ChoiceNewHost should always return one host when setting contains one host", func(t *testing.T) {
		const host = "a"
		ses := NewSession(Input{
			AvailableHosts: []string{host},
		})

		for i := 0; i < 3; i++ {
			ses.ChoiceNewHost()
			require.Equal(t, ses.host, host)
		}
	})

	t.Run("With multiple hosts ChoiceNewHost does not repeat hosts for calling count - 1 times", func(t *testing.T) {
		availableHosts := []string{"a", "b", "c"}
		ses := NewSession(Input{
			AvailableHosts: availableHosts,
		})

		choicedHosts := make(map[string]bool)
		choicedHosts[ses.host] = true

		for i := 0; i < len(availableHosts)-1; i++ {
			ses.ChoiceNewHost()

			require.NotContains(t, choicedHosts, ses.host)

			choicedHosts[ses.host] = true
		}
	})

	t.Run("With multiple hosts ChoiceNewHost should resets remainingHosts", func(t *testing.T) {
		availableHosts := []string{"a", "b", "c"}
		ses := NewSession(Input{
			AvailableHosts: availableHosts,
		})

		for i := 0; i < len(availableHosts); i++ {
			ses.ChoiceNewHost()
		}

		remainedHosts := stringsutil.ExcludeElementFromSlice(availableHosts, ses.host)
		var expectedRemainedHosts []string
		expectedRemainedHosts = append(expectedRemainedHosts, remainedHosts...)

		require.Len(t, ses.remainingHosts, len(availableHosts)-1)
		require.NotContains(t, ses.remainingHosts, ses.host)

		for _, h := range expectedRemainedHosts {
			require.Contains(t, ses.remainingHosts, h)
		}
	})
}

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

package phase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubState struct {
	calls []string
}

type stubPhase struct {
	name string
	err  error
}

func (s *stubPhase) Name() string { return s.name }
func (s *stubPhase) Run(_ context.Context, st *stubState) error {
	st.calls = append(st.calls, s.name)
	return s.err
}

func TestRunnerRunsPhasesInOrder(t *testing.T) {
	state := &stubState{}
	err := NewRunner[*stubState]().Run(t.Context(), state, []Phase[*stubState]{
		&stubPhase{name: "first"},
		&stubPhase{name: "second"},
		&stubPhase{name: "third"},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"first", "second", "third"}, state.calls)
}

func TestRunnerStopsOnPhaseError(t *testing.T) {
	state := &stubState{}
	boom := errors.New("boom")

	err := NewRunner[*stubState]().Run(t.Context(), state, []Phase[*stubState]{
		&stubPhase{name: "first"},
		&stubPhase{name: "explodes", err: boom},
		&stubPhase{name: "never"},
	})
	require.ErrorIs(t, err, boom)
	require.ErrorContains(t, err, "explodes:", "phase name must be in error")
	require.Equal(t, []string{"first", "explodes"}, state.calls)
}

func TestRunnerEmptyListIsNoop(t *testing.T) {
	state := &stubState{}
	require.NoError(t, NewRunner[*stubState]().Run(t.Context(), state, nil))
	require.Empty(t, state.calls)
}

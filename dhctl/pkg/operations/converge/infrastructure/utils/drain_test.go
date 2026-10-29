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

package utils

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	kubedrain "github.com/deckhouse/deckhouse/go_lib/dependency/k8s/drain"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func init() {
	retry.InTestEnvironment = true
}

func answers(values ...bool) (func(string) bool, *int) {
	asked := 0
	return func(string) bool {
		asked++
		if asked > len(values) {
			panic("asked more than expected")
		}
		return values[asked-1]
	}, &asked
}

// A timed-out drain is offered again without forcing anything, as many times as the
// operator asks. Only a refused retry leads to the question of skipping the drain.
func TestDrainWithPrompts(t *testing.T) {
	timeout := fmt.Errorf("failed to drain node 'n': %w", kubedrain.ErrDrainTimeout)

	cases := []struct {
		name       string
		results    []error
		retry      []bool
		skip       []bool
		wantErr    bool
		wantDrains int
	}{
		{name: "drained", results: []error{nil}, wantDrains: 1},
		{name: "other error is not offered for retry", results: []error{errors.New("forbidden")}, wantErr: true, wantDrains: 1},
		{name: "retried until drained", results: []error{timeout, timeout, nil}, retry: []bool{true, true}, wantDrains: 3},
		{name: "retry refused, drain skipped", results: []error{timeout}, retry: []bool{false}, skip: []bool{true}, wantDrains: 1},
		{name: "retry and skip refused", results: []error{timeout}, retry: []bool{false}, skip: []bool{false}, wantErr: true, wantDrains: 1},
		{name: "retried, then skipped", results: []error{timeout, timeout}, retry: []bool{true, false}, skip: []bool{true}, wantDrains: 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			retryPrompt, retried := answers(tc.retry...)
			skipPrompt, skipped := answers(tc.skip...)

			drains := 0
			err := drainWithPrompts(context.Background(), "n", DrainPrompts{Retry: retryPrompt, SkipDrain: skipPrompt}, func() error {
				drains++
				return tc.results[drains-1]
			})

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantDrains, drains)
			require.Equal(t, len(tc.retry), *retried)
			require.Equal(t, len(tc.skip), *skipped)
		})
	}
}

// A drain cut short by a cancelled converge looks like a timeout to kubedrain. It must
// not be offered for retry, nor skipped: commander would delete the node.
func TestCancelledDrainAsksNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	never := func(string) bool { panic("must not ask") }
	err := drainWithPrompts(ctx, "n", DrainPrompts{Retry: never, SkipDrain: never}, func() error {
		return fmt.Errorf("failed to drain node 'n': %w", kubedrain.ErrDrainTimeout)
	})

	require.ErrorIs(t, err, kubedrain.ErrDrainTimeout)
}

// Commander has no one to answer, so a timed-out drain must not be retried forever.
func TestCommanderPromptsNeverRetry(t *testing.T) {
	require.False(t, GetDrainPrompts(true).Retry("Retry draining?"))
}

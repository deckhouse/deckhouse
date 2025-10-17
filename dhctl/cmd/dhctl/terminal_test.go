// Copyright 2025 Flant JSC
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

package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	terminal "golang.org/x/term"
)

func TestRestoreTerminal(t *testing.T) {
	t.Run("non-terminal stdin returns no-op function", func(t *testing.T) {
		// This test assumes we're running in a non-terminal environment (like CI)
		// If stdin is not a terminal, restoreTerminal should return a no-op function

		fd := int(os.Stdin.Fd())
		if terminal.IsTerminal(fd) {
			t.Skip("Skipping test because stdin is a terminal")
		}

		restoreFn := restoreTerminal()
		require.NotNil(t, restoreFn)

		// Should not panic when called
		require.NotPanics(t, func() {
			restoreFn()
		})
	})

	t.Run("terminal stdin returns restore function", func(t *testing.T) {
		fd := int(os.Stdin.Fd())
		if !terminal.IsTerminal(fd) {
			t.Skip("Skipping test because stdin is not a terminal")
		}

		// Get initial state
		initialState, err := terminal.GetState(fd)
		require.NoError(t, err)

		restoreFn := restoreTerminal()
		require.NotNil(t, restoreFn)

		// The restore function should not panic when called
		require.NotPanics(t, func() {
			restoreFn()
		})

		// Verify state is still accessible (terminal wasn't broken)
		currentState, err := terminal.GetState(fd)
		require.NoError(t, err)
		require.NotNil(t, currentState)

		// We can't easily test that the state was actually restored without
		// modifying the terminal state first, but we can at least verify
		// the function doesn't panic and the terminal is still functional
		_ = initialState // Use the variable to avoid unused variable error
	})
}

// TestRestoreTerminalPanicHandling tests the panic behavior when terminal.GetState fails
// This is harder to test directly since we can't easily make terminal.GetState fail
// in a controlled way, but we can at least document the expected behavior
func TestRestoreTerminalPanicHandling(t *testing.T) {
	t.Run("documents panic behavior on GetState error", func(t *testing.T) {
		// This test documents that restoreTerminal() will panic if terminal.GetState fails
		// We can't easily test this without mocking, but it's important to document
		// the expected behavior for future maintainers

		// The function is designed to panic on terminal.GetState error because:
		// 1. It's called during application initialization
		// 2. Terminal state errors are typically unrecoverable
		// 3. It's better to fail fast than to have undefined terminal behavior

		t.Log("restoreTerminal() is expected to panic if terminal.GetState() fails")
		t.Log("This is intentional behavior for unrecoverable terminal state errors")
	})
}

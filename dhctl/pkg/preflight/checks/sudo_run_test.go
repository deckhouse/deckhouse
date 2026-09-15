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

package checks

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// TestSudoOverTheDefaultBackend is the case the old tests could not reach.
//
// They handed the check an *exec.ExitError, which is what the legacy clissh backend returns. The
// default gossh backend returns x/crypto's *ssh.ExitError, whose status is behind ExitStatus()
// rather than ExitCode() — so in production the branch that reads the status never matched, every
// refusal fell through to the generic wrapper, and the stderr sudo had written was dropped. The
// fake answers the way the default backend does.
func TestSudoOverTheDefaultBackend(t *testing.T) {
	tests := []struct {
		name     string
		node     *fakeNode
		wantErr  string
		wantPerm bool
	}{
		{
			name: "sudo works",
			node: newFakeNode().on("true").succeeds(),
		},
		{
			// What sudo itself said, rather than dhctl's paraphrase of it.
			name:     "a password is required",
			node:     newFakeNode().on("true").exits(1).stderr("sudo: a password is required"),
			wantErr:  "sudo: a password is required",
			wantPerm: true,
		},
		{
			name:     "the user is not in sudoers",
			node:     newFakeNode().on("true").exits(1).stderr("user is not in the sudoers file"),
			wantErr:  "user is not in the sudoers file",
			wantPerm: true,
		},
		{
			// 255 is the SSH transport itself, which says nothing about sudo.
			name:    "the transport failed",
			node:    newFakeNode().on("true").exits(255),
			wantErr: "the command did not complete",
		},
		{
			name:    "the session died",
			node:    newFakeNode().on("true").fails(errors.New("ssh: connection lost")),
			wantErr: "the command did not complete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := SudoAllowedCheck{NodeInterface: FixedNodeInterface(tt.node)}
			detail, err := check.Run(t.Context())

			if tt.wantErr == "" {
				require.NoError(t, err)
				assert.Contains(t, detail, "can sudo")
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)

			// A sudoers rule does not change between two attempts; a dropped connection might.
			var permanent *preflight.Failure
			require.ErrorAs(t, err, &permanent)
			if tt.wantPerm {
				assert.Contains(t, permanent.Fix, "sudoers",
					"a refusal by sudo points at sudoers, and at --ask-become-pass")
			}
		})
	}
}

// TestSudoInstalledOverTheDefaultBackend covers the other half of the split the same way.
func TestSudoInstalledOverTheDefaultBackend(t *testing.T) {
	t.Run("sudo is on PATH", func(t *testing.T) {
		node := newFakeNode().on("command -v sudo").prints("/usr/bin/sudo")

		check := SudoInstalledCheck{NodeInterface: FixedNodeInterface(node)}
		detail, err := check.Run(t.Context())

		require.NoError(t, err)
		assert.Contains(t, detail, "sudo is installed")
	})

	t.Run("sudo is not installed", func(t *testing.T) {
		// The default answer of the fake is exit 127, which is what `command -v` does for a
		// binary that is not there.
		check := SudoInstalledCheck{NodeInterface: FixedNodeInterface(newFakeNode())}
		_, err := check.Run(t.Context())

		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Equal(t, "sudo is not installed", failure.Observed)
		assert.Contains(t, failure.Fix, "Connecting as root does not avoid this")
	})
}

// TestTimeDriftOverTheDefaultBackend: the error from `date` used to be discarded and the check
// reported as passed, so a node whose clock could not be read at all looked like a node whose
// clock is right.
func TestTimeDriftOverTheDefaultBackend(t *testing.T) {
	tests := []struct {
		name    string
		node    *fakeNode
		wantErr string
	}{
		{
			name:    "date could not be run",
			node:    newFakeNode().on("date +%s").fails(errors.New("session closed")),
			wantErr: "cannot read the clock on",
		},
		{
			name:    "date printed something else",
			node:    newFakeNode().on("date +%s").prints("not-a-timestamp"),
			wantErr: "cannot read the clock on",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := TimeDriftCheck{NodeInterface: FixedNodeInterface(tt.node)}
			_, err := check.Run(t.Context())

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

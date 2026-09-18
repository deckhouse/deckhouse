// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package checks

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks/mocks"
)

func TestCheckSudo(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(
			*mocks.MockNodeInterface,
			*mocks.MockCommand,
			*mocks.MockCommand,
		)
		expectedError string
	}{
		{
			name: "sudo is allowed",
			setupMock: func(
				mni *mocks.MockNodeInterface,
				_ *mocks.MockCommand,
				sudoCmd *mocks.MockCommand,
			) {
				mni.On(
					"Command",
					"true",
					[]string(nil),
				).Return(sudoCmd)

				sudoCmd.On("Sudo", mock.Anything)
				sudoCmd.
					On("Run", mock.Anything).
					Return(nil)
			},
		},
		{
			name: "sudo is not allowed",
			setupMock: func(
				mni *mocks.MockNodeInterface,
				_ *mocks.MockCommand,
				sudoCmd *mocks.MockCommand,
			) {
				mni.On(
					"Command",
					"true",
					[]string(nil),
				).Return(sudoCmd)

				exitErr := &exec.ExitError{}

				sudoCmd.On("Sudo", mock.Anything)
				sudoCmd.
					On("Run", mock.Anything).
					Return(exitErr)
				sudoCmd.
					On("StderrBytes").
					Return([]byte("sudo: a password is required"))
			},
			// What sudo itself said, rather than dhctl's paraphrase of it.
			expectedError: "sudo: a password is required",
		},
		{
			name: "unexpected error during sudo check",
			setupMock: func(
				mni *mocks.MockNodeInterface,
				_ *mocks.MockCommand,
				sudoCmd *mocks.MockCommand,
			) {
				mni.On(
					"Command",
					"true",
					[]string(nil),
				).Return(sudoCmd)

				sudoCmd.On("Sudo", mock.Anything)
				sudoCmd.
					On("Run", mock.Anything).
					Return(errors.New("connection timeout"))

				sudoCmd.
					On("StderrBytes").
					Return([]byte("timeout"))
			},
			expectedError: "timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNode := &mocks.MockNodeInterface{}
			mockCheckInstalledCmd := &mocks.MockCommand{}
			mockSudoCmd := &mocks.MockCommand{}

			tt.setupMock(
				mockNode,
				mockCheckInstalledCmd,
				mockSudoCmd,
			)

			err := checkSudo(t.Context(), mockNode)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockNode.AssertExpectations(t)
			mockCheckInstalledCmd.AssertExpectations(t)
			mockSudoCmd.AssertExpectations(t)
		})
	}
}

// TestSudoInstalled covers the half of the old check that asks whether the command exists. It is
// its own check because the fix is a package, not a sudoers rule, and because asking whether the
// user may sudo is meaningless while sudo is absent.
func TestSudoInstalled(t *testing.T) {
	t.Run("sudo is installed", func(t *testing.T) {
		mockNode := &mocks.MockNodeInterface{}
		mockCmd := &mocks.MockCommand{}
		mockNode.On("Command", "command", []string{"-v", "sudo"}).Return(mockCmd)
		mockCmd.On("Run", mock.Anything).Return(nil)

		check := SudoInstalledCheck{NodeInterface: FixedNodeInterface(mockNode)}
		_, err := check.Run(t.Context())

		assert.NoError(t, err)
		mockNode.AssertExpectations(t)
		mockCmd.AssertExpectations(t)
	})

	t.Run("sudo is not installed", func(t *testing.T) {
		mockNode := &mocks.MockNodeInterface{}
		mockCmd := &mocks.MockCommand{}
		mockNode.On("Command", "command", []string{"-v", "sudo"}).Return(mockCmd)
		mockCmd.On("Run", mock.Anything).Return(errors.New("exit status 127"))

		check := SudoInstalledCheck{NodeInterface: FixedNodeInterface(mockNode)}
		_, err := check.Run(t.Context())

		assert.ErrorContains(t, err, "sudo is not installed")
		mockNode.AssertExpectations(t)
		mockCmd.AssertExpectations(t)
	})
}

// TestSudoIsRequiredForRootToo pins a decision that is easy to undo by accident: there is no
// "skip sudo when the user is root" path anywhere. Both lib-connection backends build every
// privileged command as `sudo -p SudoPassword -H -S -i bash -c …` without looking at the user, so
// a root account on a node with no sudo binary fails at the first such command. Exempting root
// here would turn that into a failure discovered during bootstrap instead of before it.
func TestSudoIsRequiredForRootToo(t *testing.T) {
	mockNode := &mocks.MockNodeInterface{}
	mockCmd := &mocks.MockCommand{}
	mockNode.On("Command", "command", []string{"-v", "sudo"}).Return(mockCmd)
	mockCmd.On("Run", mock.Anything).Return(errors.New("exit status 127"))

	// The node interface carries no user here, which is the local case; the point is that the
	// check body has no branch on the user at all.
	check := SudoInstalledCheck{NodeInterface: FixedNodeInterface(mockNode)}
	_, err := check.Run(t.Context())

	// Error() is the observation; the advice is in `fix:`, which the report prints under it.
	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, "sudo is not installed", failure.Observed)
	assert.Contains(t, failure.Fix, "Connecting as root does not avoid this")

	mockNode.AssertExpectations(t)
	mockCmd.AssertExpectations(t)
}

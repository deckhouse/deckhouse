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

	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks/mocks"
)

func TestCheckSudo(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(
			*mocks.MockNodeInterface,
			*mocks.MockCommand,
			*mocks.MockCommand,
		)
		expectedError string
	}{
		{
			name: "sudo installed and allowed",
			setupMock: func(
				mni *mocks.MockNodeInterface,
				checkInstalledCmd *mocks.MockCommand,
				sudoCmd *mocks.MockCommand,
			) {
				mni.On(
					"Command",
					"command -v sudo >/dev/null 2>&1",
					[]string(nil),
				).Return(checkInstalledCmd)

				checkInstalledCmd.
					On("Run", mock.Anything).
					Return(nil)

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
			name: "sudo is not installed",
			setupMock: func(
				mni *mocks.MockNodeInterface,
				checkInstalledCmd *mocks.MockCommand,
				_ *mocks.MockCommand,
			) {
				mni.On(
					"Command",
					"command -v sudo >/dev/null 2>&1",
					[]string(nil),
				).Return(checkInstalledCmd)

				checkInstalledCmd.
					On("Run", mock.Anything).
					Return(errors.New("exit status 127"))
			},
			expectedError: `required command "sudo" is not installed`,
		},
		{
			name: "sudo is not allowed",
			setupMock: func(
				mni *mocks.MockNodeInterface,
				checkInstalledCmd *mocks.MockCommand,
				sudoCmd *mocks.MockCommand,
			) {
				mni.On(
					"Command",
					"command -v sudo >/dev/null 2>&1",
					[]string(nil),
				).Return(checkInstalledCmd)

				checkInstalledCmd.
					On("Run", mock.Anything).
					Return(nil)

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
			},
			expectedError: "Provided SSH user is not allowed to sudo",
		},
		{
			name: "unexpected error during sudo check",
			setupMock: func(
				mni *mocks.MockNodeInterface,
				checkInstalledCmd *mocks.MockCommand,
				sudoCmd *mocks.MockCommand,
			) {
				mni.On(
					"Command",
					"command -v sudo >/dev/null 2>&1",
					[]string(nil),
				).Return(checkInstalledCmd)

				checkInstalledCmd.
					On("Run", mock.Anything).
					Return(nil)

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
			expectedError: "Unexpected error when checking sudoers permissions for SSH user:",
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

// Copyright 2025 Flant JSC
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
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new/checks/mocks"
)

func TestChecker_CheckSudoIsAllowedForUser(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*mocks.MockNodeInterface, *mocks.MockCommand)
		expectedError string
	}{
		{
			name: "sudo allowed successfully",
			setupMock: func(mni *mocks.MockNodeInterface, mc *mocks.MockCommand) {
				mc.On("Sudo", mock.Anything)
				mc.On("Run", mock.Anything).Return(nil)
				mni.On("Command", "echo", []string(nil)).Return(mc)
			},
		},
		{
			name: "sudo not allowed - exit error",
			setupMock: func(mni *mocks.MockNodeInterface, mc *mocks.MockCommand) {
				// Create ExitError with exit code != 255 to test "sudo not allowed" path
				// Note: &exec.ExitError{} has exit code 0 by default, which != 255
				exitErr := &exec.ExitError{}
				mc.On("Sudo", mock.Anything)
				mc.On("Run", mock.Anything).Return(exitErr)
				mni.On("Command", "echo", []string(nil)).Return(mc)
			},
			expectedError: "Provided SSH user is not allowed to sudo",
		},
		{
			name: "unexpected error during sudo check",
			setupMock: func(mni *mocks.MockNodeInterface, mc *mocks.MockCommand) {
				// Use a regular error (not ExitError) to test the unexpected error path
				mc.On("Sudo", mock.Anything)
				mc.On("Run", mock.Anything).Return(errors.New("connection timeout"))
				mni.On("Command", "echo", []string(nil)).Return(mc)
			},
			expectedError: "Unexpected error when checking sudoers permissions for SSH user:",
		},
		{
			name: "generic error during sudo check",
			setupMock: func(mni *mocks.MockNodeInterface, mc *mocks.MockCommand) {
				mc.On("Sudo", mock.Anything)
				mc.On("Run", mock.Anything).Return(errors.New("network error"))
				mni.On("Command", "echo", []string(nil)).Return(mc)
			},
			expectedError: "Unexpected error when checking sudoers permissions for SSH user:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNode := &mocks.MockNodeInterface{}
			mockCmd := &mocks.MockCommand{}
			tt.setupMock(mockNode, mockCmd)

			check := SudoAllowedCheck{Node: mockNode}
			err := check.Run(context.Background())

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockNode.AssertExpectations(t)
			mockCmd.AssertExpectations(t)
		})
	}
}

type mockProcessState struct {
	exitCode int
}

func (m *mockProcessState) ExitCode() int {
	return m.exitCode
}

func (m *mockProcessState) String() string {
	return ""
}

func (m *mockProcessState) Success() bool {
	return m.exitCode == 0
}

func (m *mockProcessState) Sys() interface{} {
	return nil
}

func (m *mockProcessState) SysUsage() interface{} {
	return nil
}

func TestCallSudo(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*mocks.MockNodeInterface, *mocks.MockCommand)
		expectedError string
	}{
		{
			name: "sudo successful",
			setupMock: func(mni *mocks.MockNodeInterface, mc *mocks.MockCommand) {
				mc.On("Sudo", mock.Anything)
				mc.On("Run", mock.Anything).Return(nil)
				mni.On("Command", "echo", []string(nil)).Return(mc)
			},
		},
		{
			name: "sudo not allowed",
			setupMock: func(mni *mocks.MockNodeInterface, mc *mocks.MockCommand) {
				// Create ExitError with exit code != 255 to test "sudo not allowed" path
				// Note: &exec.ExitError{} has exit code 0 by default, which != 255
				exitErr := &exec.ExitError{}
				mc.On("Sudo", mock.Anything)
				mc.On("Run", mock.Anything).Return(exitErr)
				mni.On("Command", "echo", []string(nil)).Return(mc)
			},
			expectedError: "Provided SSH user is not allowed to sudo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNode := &mocks.MockNodeInterface{}
			mockCmd := &mocks.MockCommand{}
			tt.setupMock(mockNode, mockCmd)

			check := SudoAllowedCheck{Node: mockNode}
			err := check.Run(context.Background())

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockNode.AssertExpectations(t)
			mockCmd.AssertExpectations(t)
		})
	}
}

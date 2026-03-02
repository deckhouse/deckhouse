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

package preflight

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
)

func TestChecker_CheckAvailabilityPorts(t *testing.T) {
	tests := []struct {
		name          string
		skipFlag      bool
		executeError  error
		executeOutput []byte
		expectedError string
		setupMock     func(*mockNodeInterface, *mockScript)
	}{
		{
			name:     "skip flag enabled",
			skipFlag: true,
			setupMock: func(mni *mockNodeInterface, msc *mockScript) {
				// No mock setup needed as function should return early
			},
		},
		{
			name:          "successful port check",
			skipFlag:      false,
			executeError:  nil,
			executeOutput: []byte("All ports are available\n"),
			setupMock: func(mni *mockNodeInterface, msc *mockScript) {
				mni.On("UploadScript", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(msc)
				msc.On("Execute", mock.Anything).Return([]byte("All ports are available\n"), nil)
			},
		},
		{
			name:          "port check failed with exit error",
			skipFlag:      false,
			executeError:  &exec.ExitError{Stderr: []byte("Port 6443 is already in use")},
			executeOutput: []byte("Port check failed"),
			expectedError: "Required ports check failed:",
			setupMock: func(mni *mockNodeInterface, msc *mockScript) {
				mni.On("UploadScript", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(msc)
				exitErr := &exec.ExitError{Stderr: []byte("Port 6443 is already in use")}
				msc.On("Execute", mock.Anything).Return([]byte("Port check failed"), exitErr)
			},
		},
		{
			name:          "generic execution error",
			skipFlag:      false,
			executeError:  errors.New("network error"),
			executeOutput: []byte(""),
			expectedError: "Could not execute a script to check if all necessary ports are open on the node:",
			setupMock: func(mni *mockNodeInterface, msc *mockScript) {
				mni.On("UploadScript", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(msc)
				msc.On("Execute", mock.Anything).Return([]byte(""), errors.New("network error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalSkipFlag := app.PreflightSkipAvailabilityPorts
			defer func() {
				app.PreflightSkipAvailabilityPorts = originalSkipFlag
			}()

			app.PreflightSkipAvailabilityPorts = tt.skipFlag

			mockNode := &mockNodeInterface{}
			mockScript := &mockScript{}
			tt.setupMock(mockNode, mockScript)

			checker := &Checker{
				nodeInterface: mockNode,
			}

			err := checker.CheckAvailabilityPorts(context.Background())

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockNode.AssertExpectations(t)
			mockScript.AssertExpectations(t)
		})
	}
}

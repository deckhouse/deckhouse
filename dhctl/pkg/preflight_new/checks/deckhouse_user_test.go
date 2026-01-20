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

func TestChecker_CheckDeckhouseUser(t *testing.T) {
	tests := []struct {
		name          string
		executeError  error
		executeOutput []byte
		expectedError string
		setupMock     func(*mocks.MockNodeInterface, *mocks.MockScript)
	}{
		{
			name:          "deckhouse user check successful",
			executeError:  nil,
			executeOutput: []byte("deckhouse user and group are not present\n"),
			setupMock: func(mni *mocks.MockNodeInterface, msc *mocks.MockScript) {
				mni.On("UploadScript", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(msc)
				msc.On("Execute", mock.Anything).Return([]byte("deckhouse user and group are not present\n"), nil)
			},
		},
		{
			name:          "deckhouse user check failed with exit error",
			executeError:  &exec.ExitError{Stderr: []byte("deckhouse user exists")},
			executeOutput: []byte("User check failed"),
			expectedError: "Deckhouse user existence check failed:",
			setupMock: func(mni *mocks.MockNodeInterface, msc *mocks.MockScript) {
				mni.On("UploadScript", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(msc)
				exitErr := &exec.ExitError{Stderr: []byte("deckhouse user exists")}
				msc.On("Execute", mock.Anything).Return([]byte("User check failed"), exitErr)
			},
		},
		{
			name:          "generic execution error",
			executeError:  errors.New("network error"),
			executeOutput: []byte(""),
			expectedError: "Could not execute a script to check deckhouse user and group aren't present on the node:",
			setupMock: func(mni *mocks.MockNodeInterface, msc *mocks.MockScript) {
				mni.On("UploadScript", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(msc)
				msc.On("Execute", mock.Anything).Return([]byte(""), errors.New("network error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNode := &mocks.MockNodeInterface{}
			mockScript := &mocks.MockScript{}
			tt.setupMock(mockNode, mockScript)

			check := DeckhouseUserCheck{Node: mockNode}
			err := check.Run(context.Background())

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

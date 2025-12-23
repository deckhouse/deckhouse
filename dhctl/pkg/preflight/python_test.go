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
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
)

func TestChecker_CheckPythonAndItsModules(t *testing.T) {
	tests := []struct {
		name          string
		skipFlag      bool
		setupMock     func(*mockNodeInterface)
		expectedError string
	}{
		{
			name:     "skip flag enabled",
			skipFlag: true,
			setupMock: func(mni *mockNodeInterface) {
				// No mock setup needed as function should return early
			},
		},
		{
			name:     "python3 available with all modules",
			skipFlag: false,
			setupMock: func(mni *mockNodeInterface) {
				// Mock python3 detection
				cmdPython3 := &mockCommand{}
				cmdPython3.On("Run", mock.Anything).Return(nil)
				mni.On("Command", "command", []string{"-v", "python3"}).Return(cmdPython3)

				// Mock module checks - all modules available
				modules := []string{
					"urllib.request", "urllib.error", "configparser", "http.server", "http.server",
				}
				for _, module := range modules {
					cmdModule := &mockCommand{}
					cmdModule.On("Run", mock.Anything).Return(nil)
					mni.On("Command", "python3", []string{"-c", "import " + module}).Return(cmdModule)
				}
			},
		},
		{
			name:     "python2 available with fallback modules",
			skipFlag: false,
			setupMock: func(mni *mockNodeInterface) {
				// Mock python3 not available
				cmdPython3 := &mockCommand{}
				cmdPython3.On("Run", mock.Anything).Return(&exec.ExitError{})
				mni.On("Command", "command", []string{"-v", "python3"}).Return(cmdPython3)

				// Mock python2 available
				cmdPython2 := &mockCommand{}
				cmdPython2.On("Run", mock.Anything).Return(nil)
				mni.On("Command", "command", []string{"-v", "python2"}).Return(cmdPython2)

				// Mock module checks - python3 modules fail, python2 modules succeed
				python3Modules := []string{"urllib.request", "urllib.error", "configparser", "http.server"}
				python2Modules := []string{"urllib2", "urllib2", "ConfigParser", "SimpleHTTPServer", "SocketServer"}

				for i, module := range python3Modules {
					cmdModule := &mockCommand{}
					cmdModule.On("Run", mock.Anything).Return(&exec.ExitError{})
					mni.On("Command", "python2", []string{"-c", "import " + module}).Return(cmdModule)

					// Add fallback module
					if i < len(python2Modules) {
						cmdFallback := &mockCommand{}
						cmdFallback.On("Run", mock.Anything).Return(nil)
						mni.On("Command", "python2", []string{"-c", "import " + python2Modules[i]}).Return(cmdFallback)
					}
				}

				// Handle the duplicate http.server case
				cmdModule := &mockCommand{}
				cmdModule.On("Run", mock.Anything).Return(&exec.ExitError{})
				mni.On("Command", "python2", []string{"-c", "import http.server"}).Return(cmdModule)

				cmdFallback := &mockCommand{}
				cmdFallback.On("Run", mock.Anything).Return(nil)
				mni.On("Command", "python2", []string{"-c", "import SocketServer"}).Return(cmdFallback)
			},
		},
		{
			name:     "no python available",
			skipFlag: false,
			setupMock: func(mni *mockNodeInterface) {
				// Mock all python binaries not available
				for _, binary := range []string{"python3", "python2", "python"} {
					cmd := &mockCommand{}
					cmd.On("Run", mock.Anything).Return(&exec.ExitError{})
					mni.On("Command", "command", []string{"-v", binary}).Return(cmd)
				}
			},
			expectedError: "Python was not found under any of expected names",
		},
		{
			name:     "python available but missing required modules",
			skipFlag: false,
			setupMock: func(mni *mockNodeInterface) {
				// Mock python3 available
				cmdPython3 := &mockCommand{}
				cmdPython3.On("Run", mock.Anything).Return(nil)
				mni.On("Command", "command", []string{"-v", "python3"}).Return(cmdPython3)

				// Mock first module set missing
				cmdModule1 := &mockCommand{}
				cmdModule1.On("Run", mock.Anything).Return(&exec.ExitError{})
				mni.On("Command", "python3", []string{"-c", "import urllib.request"}).Return(cmdModule1)

				cmdModule2 := &mockCommand{}
				cmdModule2.On("Run", mock.Anything).Return(&exec.ExitError{})
				mni.On("Command", "python3", []string{"-c", "import urllib2"}).Return(cmdModule2)
			},
			expectedError: "Please install at least one of the following python modules",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalSkipFlag := app.PreflightSkipPythonChecks
			defer func() {
				app.PreflightSkipPythonChecks = originalSkipFlag
			}()

			app.PreflightSkipPythonChecks = tt.skipFlag

			mockNode := &mockNodeInterface{}
			tt.setupMock(mockNode)

			checker := &Checker{
				nodeInterface: mockNode,
			}

			err := checker.CheckPythonAndItsModules(context.Background())

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockNode.AssertExpectations(t)
		})
	}
}

func TestDetectPythonBinary(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*mockNodeInterface)
		expectedBin   string
		expectedError string
	}{
		{
			name: "python3 available",
			setupMock: func(mni *mockNodeInterface) {
				cmd := &mockCommand{}
				cmd.On("Run", mock.Anything).Return(nil)
				mni.On("Command", "command", []string{"-v", "python3"}).Return(cmd)
			},
			expectedBin: "python3",
		},
		{
			name: "python2 available (python3 not available)",
			setupMock: func(mni *mockNodeInterface) {
				cmdPython3 := &mockCommand{}
				cmdPython3.On("Run", mock.Anything).Return(&exec.ExitError{})
				mni.On("Command", "command", []string{"-v", "python3"}).Return(cmdPython3)

				cmdPython2 := &mockCommand{}
				cmdPython2.On("Run", mock.Anything).Return(nil)
				mni.On("Command", "command", []string{"-v", "python2"}).Return(cmdPython2)
			},
			expectedBin: "python2",
		},
		{
			name: "python available (python3 and python2 not available)",
			setupMock: func(mni *mockNodeInterface) {
				cmdPython3 := &mockCommand{}
				cmdPython3.On("Run", mock.Anything).Return(&exec.ExitError{})
				mni.On("Command", "command", []string{"-v", "python3"}).Return(cmdPython3)

				cmdPython2 := &mockCommand{}
				cmdPython2.On("Run", mock.Anything).Return(&exec.ExitError{})
				mni.On("Command", "command", []string{"-v", "python2"}).Return(cmdPython2)

				cmdPython := &mockCommand{}
				cmdPython.On("Run", mock.Anything).Return(nil)
				mni.On("Command", "command", []string{"-v", "python"}).Return(cmdPython)
			},
			expectedBin: "python",
		},
		{
			name: "no python available",
			setupMock: func(mni *mockNodeInterface) {
				for _, binary := range []string{"python3", "python2", "python"} {
					cmd := &mockCommand{}
					cmd.On("Run", mock.Anything).Return(&exec.ExitError{})
					mni.On("Command", "command", []string{"-v", binary}).Return(cmd)
				}
			},
			expectedError: "Python was not found under any of expected names",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNode := &mockNodeInterface{}
			tt.setupMock(mockNode)

			binary, err := detectPythonBinary(context.Background(), mockNode)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Empty(t, binary)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedBin, binary)
			}

			mockNode.AssertExpectations(t)
		})
	}
}

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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
)

func TestChecker_CheckSSHCredential(t *testing.T) {
	tests := []struct {
		name          string
		skipFlag      bool
		nodeInterface node.Interface
		setupMock     func(*mockSSHClient, *mockCheck)
		expectedError string
	}{
		{
			name:     "skip flag enabled",
			skipFlag: true,
			setupMock: func(msc *mockSSHClient, msch *mockCheck) {
				// No mock setup needed as function should return early
			},
		},
		{
			name:          "local run - not SSH wrapper",
			skipFlag:      false,
			nodeInterface: &mockNodeInterface{},
			setupMock: func(msc *mockSSHClient, msch *mockCheck) {
				// No mock setup needed as function should return early for local run
			},
		},
		{
			name:     "SSH credentials check successful",
			skipFlag: false,
			setupMock: func(msc *mockSSHClient, msch *mockCheck) {
				msc.On("Check").Return(msch)
				msch.On("CheckAvailability", mock.Anything).Return(nil)
			},
		},
		{
			name:     "SSH credentials check failed",
			skipFlag: false,
			setupMock: func(msc *mockSSHClient, msch *mockCheck) {
				msc.On("Check").Return(msch)
				msch.On("CheckAvailability", mock.Anything).Return(errors.New("authentication failed"))
			},
			expectedError: "ssh authentication failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalSkipFlag := app.PreflightSkipSSHCredentialsCheck
			defer func() {
				app.PreflightSkipSSHCredentialsCheck = originalSkipFlag
			}()

			app.PreflightSkipSSHCredentialsCheck = tt.skipFlag

			mockSSHClient := &mockSSHClient{}
			mockSSHCheck := &mockCheck{}
			tt.setupMock(mockSSHClient, mockSSHCheck)

			var nodeInterface node.Interface
			if tt.nodeInterface != nil {
				nodeInterface = tt.nodeInterface
			} else {
				wrapper := &mockNodeInterfaceWrapper{client: mockSSHClient}
				nodeInterface = wrapper
			}

			checker := &Checker{
				nodeInterface: nodeInterface,
			}

			err := checker.CheckSSHCredential(context.Background())

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockSSHClient.AssertExpectations(t)
			mockSSHCheck.AssertExpectations(t)
		})
	}
}

func TestChecker_CheckSingleSSHHostForStatic(t *testing.T) {
	tests := []struct {
		name          string
		skipFlag      bool
		nodeInterface node.Interface
		setupMock     func(*mockSSHClient, *mockSession)
		expectedError string
	}{
		{
			name:     "skip flag enabled",
			skipFlag: true,
			setupMock: func(msc *mockSSHClient, mss *mockSession) {
				// No mock setup needed as function should return early
			},
		},
		{
			name:          "local run - not SSH wrapper",
			skipFlag:      false,
			nodeInterface: &mockNodeInterface{},
			setupMock: func(msc *mockSSHClient, mss *mockSession) {
				// No mock setup needed as function should return early for local run
			},
		},
		{
			name:     "single SSH host - valid",
			skipFlag: false,
			setupMock: func(msc *mockSSHClient, mss *mockSession) {
				sessionObj := session.NewSession(session.Input{
					AvailableHosts: []session.Host{{Host: "host1", Name: "host1"}},
				})
				msc.On("Session").Return(sessionObj)
			},
		},
		{
			name:     "multiple SSH hosts - invalid",
			skipFlag: false,
			setupMock: func(msc *mockSSHClient, mss *mockSession) {
				sessionObj := session.NewSession(session.Input{
					AvailableHosts: []session.Host{{Host: "host1", Name: "host1"}, {Host: "host2", Name: "host2"}},
				})
				msc.On("Session").Return(sessionObj)
			},
			expectedError: "during the bootstrap of the first static master node, only one --ssh-host parameter is allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalSkipFlag := app.PreflightSkipOneSSHHost
			defer func() {
				app.PreflightSkipOneSSHHost = originalSkipFlag
			}()

			app.PreflightSkipOneSSHHost = tt.skipFlag

			mockSSHClient := &mockSSHClient{}
			mockSSHSession := &mockSession{}
			tt.setupMock(mockSSHClient, mockSSHSession)

			var nodeInterface node.Interface
			if tt.nodeInterface != nil {
				nodeInterface = tt.nodeInterface
			} else {
				wrapper := &mockNodeInterfaceWrapper{client: mockSSHClient}
				nodeInterface = wrapper
			}

			checker := &Checker{
				nodeInterface: nodeInterface,
			}

			err := checker.CheckSingleSSHHostForStatic(context.Background())

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockSSHClient.AssertExpectations(t)
			mockSSHSession.AssertExpectations(t)
		})
	}
}

func TestGetTunnelPreflightCheckFailedError(t *testing.T) {
	originalErr := errors.New("connection failed")
	stdout := "some output"

	err := getTunnelPreflightCheckFailedError(originalErr, stdout)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Cannot establish working tunnel to control-plane host")
	assert.Contains(t, err.Error(), "connection failed")
	assert.Contains(t, err.Error(), "AllowTcpForwarding")
}

func TestHealthUrl(t *testing.T) {
	port := 8080
	expected := "http://127.0.0.1:8080/healthz"
	actual := healthURL(port)
	assert.Equal(t, expected, actual)
}

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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func TestNewChecker(t *testing.T) {
	nodeInterface := &mockNodeInterface{}
	installConfig := &config.DeckhouseInstaller{}
	metaConfig := &config.MetaConfig{}
	bootstrapState := &mockState{}

	checker := NewChecker(nodeInterface, installConfig, metaConfig, bootstrapState)

	assert.Equal(t, nodeInterface, checker.nodeInterface)
	assert.Equal(t, installConfig, checker.installConfig)
	assert.Equal(t, metaConfig, checker.metaConfig)
	assert.Equal(t, bootstrapState, checker.bootstrapState)
	assert.NotNil(t, checker.imageDescriptorProvider)
}

func TestChecker_Global(t *testing.T) {
	tests := []struct {
		name          string
		skipAll       bool
		wasRan        bool
		wasRanError   error
		setRanError   error
		expectedError string
		setupMock     func(*mockState)
	}{
		{
			name:    "checks already ran",
			wasRan:  true,
			skipAll: false,
			setupMock: func(ms *mockState) {
				ms.On("GlobalPreflightchecksWasRan").Return(true, nil)
			},
		},
		{
			name:        "error getting state",
			wasRan:      false,
			wasRanError: errors.New("state error"),
			skipAll:     false,
			setupMock: func(ms *mockState) {
				ms.On("GlobalPreflightchecksWasRan").Return(false, errors.New("state error"))
			},
			expectedError: "Cannot get state for global preflight checks from cache:",
		},
		{
			name:    "skip all checks enabled",
			wasRan:  false,
			skipAll: true,
			setupMock: func(ms *mockState) {
				ms.On("GlobalPreflightchecksWasRan").Return(false, nil)
				ms.On("SetGlobalPreflightchecksWasRan").Return(nil)
			},
		},
		{
			name:        "error setting state after checks",
			wasRan:      false,
			skipAll:     true,
			setRanError: errors.New("set state error"),
			setupMock: func(ms *mockState) {
				ms.On("GlobalPreflightchecksWasRan").Return(false, nil)
				ms.On("SetGlobalPreflightchecksWasRan").Return(errors.New("set state error"))
			},
			expectedError: "set state error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalSkipAll := app.PreflightSkipAll
			originalSkipPublicDomain := app.PreflightSkipPublicDomainTemplateCheck
			originalSkipRegistry := app.PreflightSkipRegistryCredentials
			originalSkipEdition := app.PreflightSkipDeckhouseEditionCheck
			originalSkipCIDR := app.PreflightSkipCIDRIntersection

			defer func() {
				app.PreflightSkipAll = originalSkipAll
				app.PreflightSkipPublicDomainTemplateCheck = originalSkipPublicDomain
				app.PreflightSkipRegistryCredentials = originalSkipRegistry
				app.PreflightSkipDeckhouseEditionCheck = originalSkipEdition
				app.PreflightSkipCIDRIntersection = originalSkipCIDR
			}()

			app.PreflightSkipAll = tt.skipAll
			app.PreflightSkipPublicDomainTemplateCheck = true
			app.PreflightSkipRegistryCredentials = true
			app.PreflightSkipDeckhouseEditionCheck = true
			app.PreflightSkipCIDRIntersection = true

			mockState := &mockState{}
			tt.setupMock(mockState)

			checker := &Checker{
				bootstrapState: mockState,
			}

			err := checker.Global(context.Background())

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockState.AssertExpectations(t)
		})
	}
}

func TestChecker_Static(t *testing.T) {
	tests := []struct {
		name          string
		skipAll       bool
		wasRan        bool
		wasRanError   error
		setRanError   error
		expectedError string
		setupMock     func(*mockState)
	}{
		{
			name:    "checks already ran",
			wasRan:  true,
			skipAll: false,
			setupMock: func(ms *mockState) {
				ms.On("StaticPreflightchecksWasRan").Return(true, nil)
			},
		},
		{
			name:        "error getting state",
			wasRan:      false,
			wasRanError: errors.New("state error"),
			skipAll:     false,
			setupMock: func(ms *mockState) {
				ms.On("StaticPreflightchecksWasRan").Return(false, errors.New("state error"))
			},
			expectedError: "Cannot get state for static preflight checks from cache:",
		},
		{
			name:    "skip all checks enabled",
			wasRan:  false,
			skipAll: true,
			setupMock: func(ms *mockState) {
				ms.On("StaticPreflightchecksWasRan").Return(false, nil)
				ms.On("SetStaticPreflightchecksWasRan").Return(nil)
			},
		},
		{
			name:        "error setting state after checks",
			wasRan:      false,
			skipAll:     true,
			setRanError: errors.New("set state error"),
			setupMock: func(ms *mockState) {
				ms.On("StaticPreflightchecksWasRan").Return(false, nil)
				ms.On("SetStaticPreflightchecksWasRan").Return(errors.New("set state error"))
			},
			expectedError: "set state error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalSkipAll := app.PreflightSkipAll
			originalSkipStaticIP := app.PreflightSkipStaticInstancesIPDuplication
			originalSkipOneSSH := app.PreflightSkipOneSSHHost
			originalSkipSSHCred := app.PreflightSkipSSHCredentialsCheck
			originalSkipSSHForward := app.PreflightSkipSSHForward
			originalSkipDeckhouseUser := app.PreflightSkipDeckhouseUserCheck
			originalSkipSystemReq := app.PreflightSkipSystemRequirementsCheck
			originalSkipPython := app.PreflightSkipPythonChecks
			originalSkipRegistry := app.PreflightSkipRegistryThroughProxy
			originalSkipPorts := app.PreflightSkipAvailabilityPorts
			originalSkipLocalhost := app.PreflightSkipResolvingLocalhost
			originalSkipSudo := app.PreflightSkipSudoIsAllowedForUserCheck
			originalSkipTime := app.PreflightSkipTimeDrift
			originalSkipCIDR := app.PreflightSkipCIDRIntersection

			defer func() {
				app.PreflightSkipAll = originalSkipAll
				app.PreflightSkipStaticInstancesIPDuplication = originalSkipStaticIP
				app.PreflightSkipOneSSHHost = originalSkipOneSSH
				app.PreflightSkipSSHCredentialsCheck = originalSkipSSHCred
				app.PreflightSkipSSHForward = originalSkipSSHForward
				app.PreflightSkipDeckhouseUserCheck = originalSkipDeckhouseUser
				app.PreflightSkipSystemRequirementsCheck = originalSkipSystemReq
				app.PreflightSkipPythonChecks = originalSkipPython
				app.PreflightSkipRegistryThroughProxy = originalSkipRegistry
				app.PreflightSkipAvailabilityPorts = originalSkipPorts
				app.PreflightSkipResolvingLocalhost = originalSkipLocalhost
				app.PreflightSkipSudoIsAllowedForUserCheck = originalSkipSudo
				app.PreflightSkipTimeDrift = originalSkipTime
				app.PreflightSkipCIDRIntersection = originalSkipCIDR
			}()

			app.PreflightSkipAll = tt.skipAll
			app.PreflightSkipStaticInstancesIPDuplication = true
			app.PreflightSkipOneSSHHost = true
			app.PreflightSkipSSHCredentialsCheck = true
			app.PreflightSkipSSHForward = true
			app.PreflightSkipDeckhouseUserCheck = true
			app.PreflightSkipSystemRequirementsCheck = true
			app.PreflightSkipPythonChecks = true
			app.PreflightSkipRegistryThroughProxy = true
			app.PreflightSkipAvailabilityPorts = true
			app.PreflightSkipResolvingLocalhost = true
			app.PreflightSkipSudoIsAllowedForUserCheck = true
			app.PreflightSkipTimeDrift = true
			app.PreflightSkipCIDRIntersection = true

			mockState := &mockState{}
			tt.setupMock(mockState)

			checker := &Checker{
				bootstrapState: mockState,
			}

			err := checker.Static(context.Background())

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockState.AssertExpectations(t)
		})
	}
}

func TestChecker_StaticSudo(t *testing.T) {
	tests := []struct {
		name          string
		skipAll       bool
		wasRanError   error
		expectedError string
		setupMock     func(*mockState)
	}{
		{
			name:    "successful static sudo check",
			skipAll: true,
			setupMock: func(ms *mockState) {
				ms.On("StaticPreflightchecksWasRan").Return(false, nil)
			},
		},
		{
			name:        "error getting state",
			wasRanError: errors.New("state error"),
			skipAll:     false,
			setupMock: func(ms *mockState) {
				ms.On("StaticPreflightchecksWasRan").Return(false, errors.New("state error"))
			},
			expectedError: "Cannot get state for static sudo preflight checks from cache:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalSkipAll := app.PreflightSkipAll
			originalSkipSSHCred := app.PreflightSkipSSHCredentialsCheck
			originalSkipSSHForward := app.PreflightSkipSSHForward
			originalSkipSudo := app.PreflightSkipSudoIsAllowedForUserCheck

			defer func() {
				app.PreflightSkipAll = originalSkipAll
				app.PreflightSkipSSHCredentialsCheck = originalSkipSSHCred
				app.PreflightSkipSSHForward = originalSkipSSHForward
				app.PreflightSkipSudoIsAllowedForUserCheck = originalSkipSudo
			}()

			app.PreflightSkipAll = tt.skipAll
			app.PreflightSkipSSHCredentialsCheck = true
			app.PreflightSkipSSHForward = true
			app.PreflightSkipSudoIsAllowedForUserCheck = true

			mockState := &mockState{}
			tt.setupMock(mockState)

			checker := &Checker{
				bootstrapState: mockState,
			}

			err := checker.StaticSudo(context.Background())

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockState.AssertExpectations(t)
		})
	}
}

func TestChecker_do_DuplicatedSkipFlag(t *testing.T) {
	checker := &Checker{}

	// This should panic due to duplicated skip flags
	assert.Panics(t, func() {
		checker.do(context.Background(), "Test", []checkStep{
			{
				fun:            func(ctx context.Context) error { return nil },
				successMessage: "test1",
				skipFlag:       "duplicate-flag",
			},
			{
				fun:            func(ctx context.Context) error { return nil },
				successMessage: "test2",
				skipFlag:       "duplicate-flag", // Same flag as above
			},
		})
	})
}

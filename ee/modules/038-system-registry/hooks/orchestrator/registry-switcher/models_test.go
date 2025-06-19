/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registryswitcher

import (
	"testing"

	"github.com/stretchr/testify/assert"

	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/deckhouse-registry"
)

func TestState_processResult(t *testing.T) {
	state := &State{
		Config: deckhouse_registry.Config{
			Address: "registry.d8-system.svc:5001",
			Scheme:  "https",
			CA:      "test-ca",
			Path:    "/system/deckhouse",
		},
	}

	tests := []struct {
		name          string
		params        Params
		inputs        Inputs
		expectedReady bool
		expectedMsg   string
	}{
		{
			name: "Secret not ready - returns false",
			params: Params{
				RegistrySecret: deckhouse_registry.Config{
					Address: "old-registry.deckhouse.io",
				},
			},
			inputs: Inputs{
				GlobalRegistryValues: GlobalRegistryValues{
					Address: "registry.d8-system.svc:5001",
					Scheme:  "https",
					CA:      "test-ca",
					Path:    "/system/deckhouse",
				},
				DeckhouseDeployment: DeckhouseDeploymentStatus{
					IsExist: true,
					IsReady: true,
				},
			},
			expectedReady: false,
			expectedMsg:   "Waiting secret update",
		},
		{
			name: "Global values don't match - not switched",
			params: Params{
				RegistrySecret: deckhouse_registry.Config{
					Address: "registry.d8-system.svc:5001",
					Scheme:  "https",
					CA:      "test-ca",
					Path:    "/system/deckhouse",
				},
			},
			inputs: Inputs{
				GlobalRegistryValues: GlobalRegistryValues{
					Address: "old-registry.deckhouse.io",
					Scheme:  "https",
					CA:      "test-ca",
					Path:    "/deckhouse/ce",
				},
				DeckhouseDeployment: DeckhouseDeploymentStatus{
					IsExist: true,
					IsReady: true,
				},
			},
			expectedReady: false,
			expectedMsg:   "Waiting global vars update",
		},
		{
			name: "Deployment not ready - not switched",
			params: Params{
				RegistrySecret: deckhouse_registry.Config{
					Address: "registry.d8-system.svc:5001",
					Scheme:  "https",
					CA:      "test-ca",
					Path:    "/system/deckhouse",
				},
			},
			inputs: Inputs{
				GlobalRegistryValues: GlobalRegistryValues{
					Address: "registry.d8-system.svc:5001",
					Scheme:  "https",
					CA:      "test-ca",
					Path:    "/system/deckhouse",
				},
				DeckhouseDeployment: DeckhouseDeploymentStatus{
					IsExist:  true,
					IsReady:  false,
					ReadyMsg: "Deployment is updating",
				},
			},
			expectedReady: false,
			expectedMsg:   "Deployment is updating",
		},
		{
			name: "Deployment doesn't exist - not switched",
			params: Params{
				RegistrySecret: deckhouse_registry.Config{
					Address: "registry.d8-system.svc:5001",
					Scheme:  "https",
					CA:      "test-ca",
					Path:    "/system/deckhouse",
				},
			},
			inputs: Inputs{
				GlobalRegistryValues: GlobalRegistryValues{
					Address: "registry.d8-system.svc:5001",
					Scheme:  "https",
					CA:      "test-ca",
					Path:    "/system/deckhouse",
				},
				DeckhouseDeployment: DeckhouseDeploymentStatus{
					IsExist: false,
					IsReady: false,
				},
			},
			expectedReady: false,
			expectedMsg:   "Deckhouse deployment is not exist",
		},
		{
			name: "All conditions met - switch complete",
			params: Params{
				RegistrySecret: deckhouse_registry.Config{
					Address: "registry.d8-system.svc:5001",
					Scheme:  "https",
					CA:      "test-ca",
					Path:    "/system/deckhouse",
				},
			},
			inputs: Inputs{
				GlobalRegistryValues: GlobalRegistryValues{
					Address: "registry.d8-system.svc:5001",
					Scheme:  "https",
					CA:      "test-ca",
					Path:    "/system/deckhouse",
				},
				DeckhouseDeployment: DeckhouseDeploymentStatus{
					IsExist: true,
					IsReady: true,
				},
			},
			expectedReady: true,
			expectedMsg:   "Switch is ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := state.processResult(tt.params, tt.inputs)
			assert.Equal(t, tt.expectedReady, result.Ready)
			assert.Equal(t, tt.expectedMsg, result.Message)
		})
	}
}

func TestState_Process(t *testing.T) {
	tests := []struct {
		name          string
		params        Params
		inputs        Inputs
		expectedReady bool
		wantErr       bool
	}{
		{
			name: "Secret not ready - returns false",
			params: Params{
				RegistrySecret: deckhouse_registry.Config{
					Address: "old-registry.deckhouse.io",
				},
				ManagedMode: &ManagedModeParams{
					CA:       "test-ca",
					Username: "user",
					Password: "pass",
				},
			},
			inputs: Inputs{
				GlobalRegistryValues: GlobalRegistryValues{
					Address: "registry.d8-system.svc:5001",
				},
				DeckhouseDeployment: DeckhouseDeploymentStatus{
					IsExist: true,
					IsReady: true,
				},
			},
			expectedReady: false,
			wantErr:       false,
		},
		{
			name: "Both secret and Deckhouse ready - returns true",
			params: Params{
				RegistrySecret: deckhouse_registry.Config{
					Address:      "registry.d8-system.svc:5001",
					Path:         "/system/deckhouse",
					Scheme:       "https",
					CA:           "test-ca",
					DockerConfig: []byte(`{"auths":{"registry.d8-system.svc:5001":{"username":"user","password":"pass","auth":"dXNlcjpwYXNz"}}}`),
				},
				ManagedMode: &ManagedModeParams{
					CA:       "test-ca",
					Username: "user",
					Password: "pass",
				},
			},
			inputs: Inputs{
				GlobalRegistryValues: GlobalRegistryValues{
					Address: "registry.d8-system.svc:5001",
					Scheme:  "https",
					CA:      "test-ca",
					Path:    "/system/deckhouse",
				},
				DeckhouseDeployment: DeckhouseDeploymentStatus{
					IsExist: true,
					IsReady: true,
				},
			},
			expectedReady: true,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &State{}
			result, err := state.Process(tt.params, tt.inputs)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedReady, result.Ready)
			}
		})
	}
}

func TestBuildManagedRegistrySecret(t *testing.T) {
	params := Params{
		ManagedMode: &ManagedModeParams{
			CA:       "test-ca",
			Username: "user",
			Password: "pass",
		},
	}

	secret, err := buildRegistrySecret(params)
	assert.NoError(t, err)

	// Print the generated values for debugging
	t.Logf("Generated Address: %s", secret.Address)
	t.Logf("Generated Path: %s", secret.Path)
	t.Logf("Generated Scheme: %s", secret.Scheme)
	t.Logf("Generated DockerConfig: %s", secret.DockerConfig)

	// Check expected values
	assert.Equal(t, "registry.d8-system.svc:5001", secret.Address)
	assert.Equal(t, "/system/deckhouse", secret.Path)
	assert.Equal(t, "https", secret.Scheme)
	assert.Equal(t, "test-ca", secret.CA)
}

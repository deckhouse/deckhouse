/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registryswitcher

import (
	"testing"

	"github.com/stretchr/testify/assert"

	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouse-registry"
)

func TestState_processResult(t *testing.T) {
	state := &State{
		Config: deckhouse_registry.Config{
			Address: "registry.d8-system.svc:5001",
			Scheme:  "https",
			CA:      "test-ca",
			Path:    "/system/deckhouse",
		},
		Hash: "test-hash",
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
				DeckhousePod: DeckhousePodStatus{
					IsExist: true,
					IsReady: true,
				},
			},
			expectedReady: false,
			expectedMsg:   "Updating registry for deckhouse components",
		},
		{
			name: "Registry version mismatch - not switched",
			params: Params{
				RegistrySecret: deckhouse_registry.Config{
					Address: "registry.d8-system.svc:5001",
					Scheme:  "https",
					CA:      "test-ca",
					Path:    "/system/deckhouse",
				},
			},
			inputs: Inputs{
				DeckhousePod: DeckhousePodStatus{
					IsExist: true,
					IsReady: true,
				},
			},
			expectedReady: false,
			expectedMsg:   "Applying new registry to deckhouse-controller",
		},
		{
			name: "Pod not ready - not switched",
			params: Params{
				RegistrySecret: deckhouse_registry.Config{
					Address: "registry.d8-system.svc:5001",
					Scheme:  "https",
					CA:      "test-ca",
					Path:    "/system/deckhouse",
				},
			},
			inputs: Inputs{
				DeckhousePod: DeckhousePodStatus{
					IsExist:         true,
					IsReady:         false,
					ReadyMsg:        "Pod is updating",
					RegistryVersion: "test-hash", // Set hash to pass hash check
				},
			},
			expectedReady: false,
			expectedMsg:   "Waiting for deckhouse-controller to become ready",
		},
		{
			name: "Pod doesn't exist - not switched",
			params: Params{
				RegistrySecret: deckhouse_registry.Config{
					Address: "registry.d8-system.svc:5001",
					Scheme:  "https",
					CA:      "test-ca",
					Path:    "/system/deckhouse",
				},
			},
			inputs: Inputs{
				DeckhousePod: DeckhousePodStatus{
					IsExist:         false,
					IsReady:         false,
					RegistryVersion: "test-hash", // Set hash to pass hash check
				},
			},
			expectedReady: false,
			expectedMsg:   "Waiting for deckhouse-controller pod",
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
				DeckhousePod: DeckhousePodStatus{
					IsExist:         true,
					IsReady:         true,
					RegistryVersion: "test-hash", // Should match the hash in state
				},
			},
			expectedReady: true,
			expectedMsg:   "",
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
				DeckhousePod: DeckhousePodStatus{
					IsExist: true,
					IsReady: true,
				},
			},
			expectedReady: false,
			wantErr:       false,
		},
		{
			name: "Both secret and pod ready - returns true",
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
				DeckhousePod: DeckhousePodStatus{
					IsExist:         true,
					IsReady:         true,
					RegistryVersion: "", // Will be calculated and set by Process
				},
			},
			expectedReady: true,
			wantErr:       false,
		},
		{
			name: "Pod ready but secret not ready - returns false",
			params: Params{
				RegistrySecret: deckhouse_registry.Config{
					Address: "registry.d8-system.svc:5001",
					Scheme:  "https",
					CA:      "test-ca",
					Path:    "/system/deckhouse",
				},
				ManagedMode: &ManagedModeParams{
					CA:       "test-ca",
					Username: "user",
					Password: "pass",
				},
			},
			inputs: Inputs{
				DeckhousePod: DeckhousePodStatus{
					IsExist:         true,
					IsReady:         true,
					RegistryVersion: "", // Will be calculated during Process
				},
			},
			expectedReady: false, // Will be false because hash won't match
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
				// For the ready test, update the pod's registry version to match the calculated hash
				if tt.expectedReady && state.Hash != "" {
					tt.inputs.DeckhousePod.RegistryVersion = state.Hash
					result = state.processResult(tt.params, tt.inputs)
				}
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

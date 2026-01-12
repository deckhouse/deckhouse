/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package registryswitcher

import (
	"testing"

	"github.com/stretchr/testify/assert"

	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouseregistry"
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
	// --- Setup for Managed Mode ---
	managedParams := &ManagedModeParams{
		CA:       "test-ca",
		Username: "user",
		Password: "pass",
	}
	expectedManagedSecret, err := buildManagedRegistrySecret(managedParams)
	assert.NoError(t, err)
	managedHash, err := expectedManagedSecret.Hash()
	assert.NoError(t, err)

	// --- Setup for Unmanaged Mode ---
	unmanagedParams := &UnmanagedModeParams{
		ImagesRepo: "my-registry.com/my-project",
		Scheme:     "HTTPS",
		CA:         "unmanaged-ca",
		Username:   "unmanaged-user",
		Password:   "unmanaged-pass",
	}
	expectedUnmanagedSecret, err := buildUnmanagedRegistrySecret(unmanagedParams)
	assert.NoError(t, err)
	unmanagedHash, err := expectedUnmanagedSecret.Hash()
	assert.NoError(t, err)

	tests := []struct {
		name          string
		params        Params
		inputs        Inputs
		expectedReady bool
		expectedMsg   string
		wantErr       bool
	}{
		// --- Managed Mode Tests ---
		{
			name: "Managed mode - Secret needs update",
			params: Params{
				RegistrySecret: deckhouse_registry.Config{Address: "old-registry"},
				ManagedMode:    managedParams,
			},
			inputs:        Inputs{},
			expectedReady: false,
			expectedMsg:   "Updating registry for deckhouse components",
		},
		{
			name: "Managed mode - Pod annotation needs update",
			params: Params{
				RegistrySecret: expectedManagedSecret,
				ManagedMode:    managedParams,
			},
			inputs: Inputs{
				DeckhousePod: DeckhousePodStatus{
					RegistryVersion: "wrong-hash",
				},
			},
			expectedReady: false,
			expectedMsg:   "Applying new registry to deckhouse-controller",
		},
		{
			name: "Managed mode - Pod not ready",
			params: Params{
				RegistrySecret: expectedManagedSecret,
				ManagedMode:    managedParams,
			},
			inputs: Inputs{
				DeckhousePod: DeckhousePodStatus{
					IsExist:         true,
					IsReady:         false,
					RegistryVersion: managedHash,
				},
			},
			expectedReady: false,
			expectedMsg:   "Waiting for deckhouse-controller to become ready",
		},
		{
			name: "Managed mode - Pod does not exist",
			params: Params{
				RegistrySecret: expectedManagedSecret,
				ManagedMode:    managedParams,
			},
			inputs: Inputs{
				DeckhousePod: DeckhousePodStatus{
					IsExist:         false,
					RegistryVersion: managedHash,
				},
			},
			expectedReady: false,
			expectedMsg:   "Waiting for deckhouse-controller pod",
		},
		{
			name: "Managed mode - Switch complete",
			params: Params{
				RegistrySecret: expectedManagedSecret,
				ManagedMode:    managedParams,
			},
			inputs: Inputs{
				DeckhousePod: DeckhousePodStatus{
					IsExist:         true,
					IsReady:         true,
					RegistryVersion: managedHash,
				},
			},
			expectedReady: true,
			expectedMsg:   "",
		},

		// --- Unmanaged Mode Tests ---
		{
			name: "Unmanaged mode - Secret needs update",
			params: Params{
				RegistrySecret: deckhouse_registry.Config{Address: "old-registry"},
				UnmanagedMode:  unmanagedParams,
			},
			inputs:        Inputs{},
			expectedReady: false,
			expectedMsg:   "Updating registry for deckhouse components",
		},
		{
			name: "Unmanaged mode - Pod annotation needs update",
			params: Params{
				RegistrySecret: expectedUnmanagedSecret,
				UnmanagedMode:  unmanagedParams,
			},
			inputs: Inputs{
				DeckhousePod: DeckhousePodStatus{
					RegistryVersion: "wrong-hash",
				},
			},
			expectedReady: false,
			expectedMsg:   "Applying new registry to deckhouse-controller",
		},
		{
			name: "Unmanaged mode - Switch complete",
			params: Params{
				RegistrySecret: expectedUnmanagedSecret,
				UnmanagedMode:  unmanagedParams,
			},
			inputs: Inputs{
				DeckhousePod: DeckhousePodStatus{
					IsExist:         true,
					IsReady:         true,
					RegistryVersion: unmanagedHash,
				},
			},
			expectedReady: true,
			expectedMsg:   "",
		},
		{
			name:    "No mode provided - returns error",
			params:  Params{},
			inputs:  Inputs{},
			wantErr: true,
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
				assert.Equal(t, tt.expectedMsg, result.Message)
			}
		})
	}
}

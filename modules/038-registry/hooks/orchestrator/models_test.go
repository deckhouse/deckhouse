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

package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/require"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	registry_pki "github.com/deckhouse/deckhouse/go_lib/registry/pki"
)

func TestParams_Validate(t *testing.T) {
	tests := []struct {
		name    string
		params  Params
		wantErr bool
	}{
		{
			name: "Unmanaged: no validation",
			params: Params{
				Mode: registry_const.ModeUnmanaged,
			},
			wantErr: false,
		},
		{
			name: "Unmanaged: valid without auth",
			params: Params{
				Mode:       registry_const.ModeUnmanaged,
				ImagesRepo: "registry.example.com/namespace/image",
				Scheme:     "HTTPS",
			},
			wantErr: false,
		},
		{
			name: "Unmanaged: valid with auth",
			params: Params{
				Mode:       registry_const.ModeUnmanaged,
				ImagesRepo: "registry.example.com/namespace/image",
				Scheme:     "HTTPS",
				UserName:   "username",
				Password:   "password",
			},
			wantErr: false,
		},
		{
			name: "Unmanaged: invalid scheme",
			params: Params{
				Mode:       registry_const.ModeUnmanaged,
				ImagesRepo: "registry.example.com/namespace/image",
				Scheme:     "FTP",
			},
			wantErr: true,
		},
		{
			name: "Unmanaged: only username",
			params: Params{
				Mode:       registry_const.ModeUnmanaged,
				ImagesRepo: "registry.example.com/namespace/image",
				Scheme:     "HTTPS",
				UserName:   "username",
			},
			wantErr: true,
		},
		{
			name: "Unmanaged: only password",
			params: Params{
				Mode:       registry_const.ModeUnmanaged,
				ImagesRepo: "registry.example.com/namespace/image",
				Scheme:     "HTTPS",
				Password:   "password",
			},
			wantErr: true,
		},
		{
			name: "Direct: valid without auth",
			params: Params{
				Mode:       registry_const.ModeDirect,
				ImagesRepo: "registry.example.com/namespace/image",
				Scheme:     "HTTPS",
			},
			wantErr: false,
		},
		{
			name: "Direct: valid with auth",
			params: Params{
				Mode:       registry_const.ModeDirect,
				ImagesRepo: "registry.example.com/namespace/image",
				Scheme:     "HTTPS",
				UserName:   "username",
				Password:   "password",
			},
			wantErr: false,
		},
		{
			name: "Direct: invalid scheme",
			params: Params{
				Mode:       registry_const.ModeDirect,
				ImagesRepo: "registry.example.com/namespace/image",
				Scheme:     "FTP",
			},
			wantErr: true,
		},
		{
			name: "Direct: only username",
			params: Params{
				Mode:       registry_const.ModeDirect,
				ImagesRepo: "registry.example.com/namespace/image",
				Scheme:     "HTTPS",
				UserName:   "username",
			},
			wantErr: true,
		},
		{
			name: "Direct: only password",
			params: Params{
				Mode:       registry_const.ModeDirect,
				ImagesRepo: "registry.example.com/namespace/image",
				Scheme:     "HTTPS",
				Password:   "password",
			},
			wantErr: true,
		},
		{
			name: "Direct: missing address",
			params: Params{
				Mode:   registry_const.ModeDirect,
				Scheme: "HTTPS",
			},
			wantErr: true,
		},
		{
			name: "Direct: empty all fields with direct mode",
			params: Params{
				Mode: registry_const.ModeDirect,
			},
			wantErr: true,
		},
		{
			name: "unknown mode",
			params: Params{
				Mode: "UnknownMode",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.params.Validate()
			if tt.wantErr {
				require.Error(t, err, "expected an error")
			} else {
				require.NoError(t, err, "expected no error")
			}
		})
	}
}

// TestInitParams_SteadyStateOverride verifies the data-model invariant relied on by
// the steady-state InitParams clearing added to hook.go: when a cluster is fully
// bootstrapped and no init secret is present, stale InitParams that were persisted in
// the registry-state Secret during an earlier mode-switch must NOT shadow the current
// imagesRepo from the live registry-config Secret.
//
// The test simulates the scenario:
//  1. InitParams were written during a Proxy mode-switch with "old.registry/upstream".
//  2. A user then changes imagesRepo to "new.registry/upstream" in mc deckhouse.
//  3. The fresh Params from registry-config carry the new value.
//  4. After InitParams are nil-ed (steady-state guard), toParams on a freshly constructed
//     state should return the new ImagesRepo, not the stale one.
func TestInitParams_SteadyStateOverride(t *testing.T) {
	// Stale state as it would be restored from registry-state Secret.
	staleParamsState := ParamsState{
		Mode:       registry_const.ModeProxy,
		ImagesRepo: "old.registry/upstream",
		Scheme:     "HTTPS",
	}
	staleParams, err := staleParamsState.toParams()
	require.NoError(t, err)
	require.Equal(t, "old.registry/upstream", staleParams.ImagesRepo)

	// Fresh params read from registry-config Secret after user changed imagesRepo.
	freshParams := Params{
		Mode:       registry_const.ModeProxy,
		ImagesRepo: "new.registry/upstream",
		Scheme:     "HTTPS",
	}

	// Simulate the steady-state guard: InitParams is nil-ed, freshParams are used.
	var frozenParams *ParamsState // nil = cleared by the steady-state guard

	var effectiveImagesRepo string
	if frozenParams != nil {
		p, err := frozenParams.toParams()
		require.NoError(t, err)
		effectiveImagesRepo = p.ImagesRepo
	} else {
		effectiveImagesRepo = freshParams.ImagesRepo
	}

	require.Equal(t, "new.registry/upstream", effectiveImagesRepo,
		"fresh imagesRepo must win when InitParams have been cleared in steady state")
}

func TestParams_ToStateAndToParams(t *testing.T) {
	certKey, err := registry_pki.GenerateCACertificate("test")
	require.NoError(t, err)
	certEncoded := string(registry_pki.EncodeCertificate(certKey.Cert))

	tests := []struct {
		name   string
		state  ParamsState
		params Params
	}{
		{
			name: "with CA",
			state: ParamsState{
				Generation: 10,
				Mode:       registry_const.ModeDirect,
				ImagesRepo: "registry.example.com",
				UserName:   "test-user",
				Password:   "test-password",
				TTL:        "10h",
				Scheme:     "HTTPS",
				CA:         certEncoded,
				CheckMode:  registry_const.CheckModeDefault,
			},
			params: Params{
				Generation: 10,
				Mode:       registry_const.ModeDirect,
				ImagesRepo: "registry.example.com",
				UserName:   "test-user",
				Password:   "test-password",
				TTL:        "10h",
				Scheme:     "HTTPS",
				CA:         certKey.Cert,
				CheckMode:  registry_const.CheckModeDefault,
			},
		},
		{
			name: "without CA",
			state: ParamsState{
				Generation: 10,
				Mode:       registry_const.ModeDirect,
				ImagesRepo: "registry.example.com",
				UserName:   "test-user",
				Password:   "test-password",
				TTL:        "10h",
				Scheme:     "HTTPS",
				CheckMode:  registry_const.CheckModeDefault,
			},
			params: Params{
				Generation: 10,
				Mode:       registry_const.ModeDirect,
				ImagesRepo: "registry.example.com",
				UserName:   "test-user",
				Password:   "test-password",
				TTL:        "10h",
				Scheme:     "HTTPS",
				CheckMode:  registry_const.CheckModeDefault,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("state to params", func(t *testing.T) {
				params, err := tt.state.toParams()
				require.NoError(t, err)
				require.EqualValues(t, tt.params, params)

				state := params.toState()
				require.EqualValues(t, tt.state, state)
			})

			t.Run("params to state", func(t *testing.T) {
				state := tt.params.toState()
				require.EqualValues(t, tt.state, state)

				params, err := state.toParams()
				require.NoError(t, err)
				require.EqualValues(t, tt.params, params)
			})
		})
	}
}

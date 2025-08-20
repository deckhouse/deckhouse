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
			name: "Direct: missing address",
			params: Params{
				Mode:   registry_const.ModeDirect,
				Scheme: "HTTPS",
			},
			wantErr: true,
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

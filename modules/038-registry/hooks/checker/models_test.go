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

package checker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistryParams_Validate(t *testing.T) {
	tests := []struct {
		name    string
		params  RegistryParams
		wantErr bool
	}{
		{
			name: "valid without auth",
			params: RegistryParams{
				Address: "registry.example.com/namespace/image",
				Scheme:  "HTTPS",
			},
			wantErr: false,
		},
		{
			name: "valid with auth",
			params: RegistryParams{
				Address:  "registry.example.com/namespace/image",
				Scheme:   "HTTPS",
				Username: "username",
				Password: "password",
			},
			wantErr: false,
		},
		{
			name: "missing address",
			params: RegistryParams{
				Scheme: "HTTPS",
			},
			wantErr: true,
		},
		{
			name: "invalid scheme",
			params: RegistryParams{
				Address: "registry.example.com/namespace/image",
				Scheme:  "FTP",
			},
			wantErr: true,
		},
		{
			name: "only username",
			params: RegistryParams{
				Address:  "registry.example.com/namespace/image",
				Scheme:   "HTTPS",
				Username: "username",
			},
			wantErr: true,
		},
		{
			name: "only password",
			params: RegistryParams{
				Address:  "registry.example.com/namespace/image",
				Scheme:   "HTTPS",
				Password: "password",
			},
			wantErr: true,
		},
		{
			name:    "empty all fields",
			params:  RegistryParams{},
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

// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package settings

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidateTmpPath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid path",
			path:        "/tmp/dhctl",
			expectError: false,
		},
		{
			name:        "valid relative path",
			path:        "tmp/dhctl",
			expectError: false,
		},
		{
			name:        "empty path becomes dot",
			path:        "",
			expectError: false,
		},
		{
			name:        "root path",
			path:        "/",
			expectError: true,
			errorMsg:    "tmpdir should not be /",
		},
		{
			name:        "path with dots gets cleaned",
			path:        "/tmp/../tmp/dhctl",
			expectError: false,
		},
		{
			name:        "path with trailing slash gets cleaned",
			path:        "/tmp/dhctl/",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTmpPath(tt.path)

			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestServerGeneralParams_Validate(t *testing.T) {
	tests := []struct {
		name        string
		params      ServerGeneralParams
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid params",
			params: ServerGeneralParams{
				Network: "tcp",
				Address: "localhost:8080",
				TmpDir:  "/tmp/dhctl",
			},
			expectError: false,
		},
		{
			name: "empty network",
			params: ServerGeneralParams{
				Network: "",
				Address: "localhost:8080",
				TmpDir:  "/tmp/dhctl",
			},
			expectError: true,
			errorMsg:    "network is required",
		},
		{
			name: "empty address",
			params: ServerGeneralParams{
				Network: "tcp",
				Address: "",
				TmpDir:  "/tmp/dhctl",
			},
			expectError: true,
			errorMsg:    "address is required",
		},
		{
			name: "invalid tmp dir",
			params: ServerGeneralParams{
				Network: "tcp",
				Address: "localhost:8080",
				TmpDir:  "/",
			},
			expectError: true,
			errorMsg:    "tmpdir should not be /",
		},
		{
			name: "empty tmp dir becomes dot",
			params: ServerGeneralParams{
				Network: "tcp",
				Address: "localhost:8080",
				TmpDir:  "",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()

			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestServerSingleshotParams_Validate(t *testing.T) {
	tests := []struct {
		name        string
		params      ServerSingleshotParams
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid params",
			params: ServerSingleshotParams{
				ServerGeneralParams: ServerGeneralParams{
					Network: "unix",
					Address: "/tmp/dhctl.sock",
					TmpDir:  "/tmp/dhctl",
				},
			},
			expectError: false,
		},
		{
			name: "invalid general params",
			params: ServerSingleshotParams{
				ServerGeneralParams: ServerGeneralParams{
					Network: "",
					Address: "/tmp/dhctl.sock",
					TmpDir:  "/tmp/dhctl",
				},
			},
			expectError: true,
			errorMsg:    "network is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()

			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestServerParams_Validate(t *testing.T) {
	tests := []struct {
		name        string
		params      ServerParams
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid params",
			params: ServerParams{
				ServerGeneralParams: ServerGeneralParams{
					Network: "tcp",
					Address: "localhost:8080",
					TmpDir:  "/tmp/dhctl",
				},
				ParallelTasksLimit:         10,
				RequestsCounterMaxDuration: 5 * time.Minute,
			},
			expectError: false,
		},
		{
			name: "invalid general params",
			params: ServerParams{
				ServerGeneralParams: ServerGeneralParams{
					Network: "tcp",
					Address: "",
					TmpDir:  "/tmp/dhctl",
				},
				ParallelTasksLimit:         10,
				RequestsCounterMaxDuration: 5 * time.Minute,
			},
			expectError: true,
			errorMsg:    "address is required",
		},
		{
			name: "zero parallel tasks limit is valid",
			params: ServerParams{
				ServerGeneralParams: ServerGeneralParams{
					Network: "tcp",
					Address: "localhost:8080",
					TmpDir:  "/tmp/dhctl",
				},
				ParallelTasksLimit:         0,
				RequestsCounterMaxDuration: 5 * time.Minute,
			},
			expectError: false,
		},
		{
			name: "zero duration is valid",
			params: ServerParams{
				ServerGeneralParams: ServerGeneralParams{
					Network: "tcp",
					Address: "localhost:8080",
					TmpDir:  "/tmp/dhctl",
				},
				ParallelTasksLimit:         10,
				RequestsCounterMaxDuration: 0,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()

			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

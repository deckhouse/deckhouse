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

package initsecret

import (
	"testing"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextBootstrapProxy_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   Config
		wantErr bool
	}{
		// Valid
		{
			name: "valid with all fields",
			input: Config{
				CA: CertKey{
					Cert: "cert",
					Key:  "key",
				},
				ROUser: User{
					Name:         "ro_name",
					Password:     "ro_password",
					PasswordHash: "ro_password_hash",
				},
				RWUser: User{
					Name:         "rw_name",
					Password:     "rw_password",
					PasswordHash: "rw_password_hash",
				},
			},
			wantErr: false,
		},
		// Invalid
		{
			name: "missing CA",
			input: Config{
				ROUser: User{
					Name:         "ro_name",
					Password:     "ro_password",
					PasswordHash: "ro_password_hash",
				},
				RWUser: User{
					Name:         "rw_name",
					Password:     "rw_password",
					PasswordHash: "rw_password_hash",
				},
			},
			wantErr: true,
		},
		{
			name: "missing RO user",
			input: Config{
				CA: CertKey{
					Cert: "cert",
					Key:  "key",
				},
				RWUser: User{
					Name:         "rw_name",
					Password:     "rw_password",
					PasswordHash: "rw_password_hash",
				},
			},
			wantErr: true,
		},
		{
			name: "missing RW user",
			input: Config{
				CA: CertKey{
					Cert: "cert",
					Key:  "key",
				},
				ROUser: User{
					Name:         "ro_name",
					Password:     "ro_password",
					PasswordHash: "ro_password_hash",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if err != nil {
				if e, ok := err.(validation.InternalError); ok {
					assert.Fail(t, "Internal validation error: %w", e.InternalError())
				}
			}

			if tt.wantErr {
				assert.Error(t, err, "Expected errors but got none")
			} else {
				assert.NoError(t, err, "Expected no errors but got some")
			}
		})
	}
}

func TestContextToMap(t *testing.T) {
	tests := []struct {
		name   string
		input  Config
		result map[string]any
	}{
		{
			name: "with all fields",
			input: Config{
				CA: CertKey{
					Cert: "cert",
					Key:  "key",
				},
				ROUser: User{
					Name:         "ro_name",
					Password:     "ro_password",
					PasswordHash: "ro_password_hash",
				},
				RWUser: User{
					Name:         "rw_name",
					Password:     "rw_password",
					PasswordHash: "rw_password_hash",
				},
			},
			result: map[string]any{
				"ca": map[string]any{
					"cert": "cert",
					"key":  "key",
				},
				"ro_user": map[string]any{
					"name":          "ro_name",
					"password":      "ro_password",
					"password_hash": "ro_password_hash",
				},
				"rw_user": map[string]any{
					"name":          "rw_name",
					"password":      "rw_password",
					"password_hash": "rw_password_hash",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.result, tt.input.ToMap())
		})
	}
}

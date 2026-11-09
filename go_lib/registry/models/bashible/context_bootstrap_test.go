/*
Copyright 2026 Flant JSC

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

package bashible

import (
	"testing"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/go_lib/registry/models/initsecret"
)

func TestContextBootstrap_Validate(t *testing.T) {
	initConfig := initsecret.Config{
		CA: initsecret.CertKey{
			Cert: "---cert---",
			Key:  "---key---",
		},
		ROUser: initsecret.User{
			Name:         "ro_name",
			Password:     "ro_password",
			PasswordHash: "ro_password_hash",
		},
		RWUser: initsecret.User{
			Name:         "rw_name",
			Password:     "rw_password",
			PasswordHash: "rw_password_hash",
		},
	}

	tests := []struct {
		name    string
		input   ContextBootstrap
		wantErr bool
	}{
		// Valid
		{
			name: "valid",
			input: ContextBootstrap{
				Init: initConfig,
			},
			wantErr: false,
		},
		{
			name: "valid from a bundle",
			input: ContextBootstrap{
				Init:       initConfig,
				FromBundle: true,
			},
			wantErr: false,
		},
		// Invalid
		{
			name:    "missing init",
			input:   ContextBootstrap{},
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

func TestContextBootstrap_ToMap(t *testing.T) {
	initConfig := initsecret.Config{
		CA: initsecret.CertKey{
			Cert: "---cert---",
			Key:  "---key---",
		},
		ROUser: initsecret.User{
			Name:         "ro_name",
			Password:     "ro_password",
			PasswordHash: "ro_password_hash",
		},
		RWUser: initsecret.User{
			Name:         "rw_name",
			Password:     "rw_password",
			PasswordHash: "rw_password_hash",
		},
	}

	initConfigMap := initConfig.ToMap()

	tests := []struct {
		name     string
		input    ContextBootstrap
		expected map[string]any
	}{
		{
			name: "from a bundle",
			input: ContextBootstrap{
				Init:       initConfig,
				FromBundle: true,
			},
			expected: map[string]any{
				"init":       initConfigMap,
				"fromBundle": true,
			},
		},
		{
			name: "not from a bundle",
			input: ContextBootstrap{
				Init: initConfig,
			},
			expected: map[string]any{
				"init": initConfigMap,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.ToMap()
			assert.Equal(t, tt.expected, result)
		})
	}
}

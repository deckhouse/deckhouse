/*
Copyright 2022 Flant JSC

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

package generate_password

import (
	"testing"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/stretchr/testify/require"
)

func TestRestoreGeneratedPassword(t *testing.T) {
	const (
		expectNoError = false
		expectError   = true
	)
	genPass := GeneratePassword()

	tests := []struct {
		name       string
		snapshot   []sdkpkg.Snapshot
		expectPass string
		expectErr  bool
	}{
		{
			"generated password",
			[]sdkpkg.Snapshot{
				go_hook.Wrapped{
					Wrapped: map[string][]byte{
						defaultBasicAuthPlainField: []byte("admin:{PLAIN}" + genPass),
					},
				},
			},
			genPass,
			expectNoError,
		},
		{
			"custom password",
			[]sdkpkg.Snapshot{
				go_hook.Wrapped{
					Wrapped: map[string][]byte{
						defaultBasicAuthPlainField: []byte("admin:{PLAIN}pass"),
					},
				},
			},
			"pass",
			expectNoError,
		},
		{
			"no PLAIN marker",
			[]sdkpkg.Snapshot{
				go_hook.Wrapped{
					Wrapped: map[string][]byte{
						defaultBasicAuthPlainField: []byte("admin:pass"),
					},
				},
			},
			"",
			expectError,
		},
		{
			"empty snapshot",
			[]sdkpkg.Snapshot{},
			"",
			expectError,
		},
		{
			"empty data",
			[]sdkpkg.Snapshot{
				go_hook.Wrapped{
					Wrapped: map[string][]byte{},
				},
			},
			"",
			expectError,
		},
		{
			"multiple fields",
			[]sdkpkg.Snapshot{
				go_hook.Wrapped{
					Wrapped: map[string][]byte{"one": []byte(""), "two": []byte("")},
				},
			},
			"",
			expectError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewBasicAuthPlainHook(HookSettings{ModuleName: "testMod", Namespace: "default", SecretName: "auth"})
			pass, err := h.restoreGeneratedPasswordFromSnapshot(tt.snapshot)
			if tt.expectErr == expectError {
				require.NotNil(t, err, "input '%s' should not success", tt.snapshot)
			} else {
				require.Nil(t, err, "should restore password successfully")
				require.Equal(t, tt.expectPass, pass, "should extract password from '%s'", tt.snapshot)
			}
		})
	}
}

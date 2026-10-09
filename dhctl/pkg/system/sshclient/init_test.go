// Copyright 2026 Flant JSC
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

package sshclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
)

func TestNewInitClientFromFlagsWithoutHostsReturnsNilInterface(t *testing.T) {
	originalHosts := app.SSHHosts
	originalPrivateKeys := app.SSHPrivateKeys
	originalLegacyMode := app.SSHLegacyMode
	originalModernMode := app.SSHModernMode
	t.Cleanup(func() {
		app.SSHHosts = originalHosts
		app.SSHPrivateKeys = originalPrivateKeys
		app.SSHLegacyMode = originalLegacyMode
		app.SSHModernMode = originalModernMode
	})

	tests := []struct {
		name        string
		legacyMode  bool
		modernMode  bool
		privateKeys []string
	}{
		{
			name: "default gossh",
		},
		{
			name:       "explicit legacy clissh",
			legacyMode: true,
		},
		{
			name:       "explicit modern gossh",
			modernMode: true,
		},
		{
			name:        "private keys select clissh",
			privateKeys: []string{"unused-private-key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app.SSHHosts = nil
			app.SSHPrivateKeys = tt.privateKeys
			app.SSHLegacyMode = tt.legacyMode
			app.SSHModernMode = tt.modernMode

			client, err := NewInitClientFromFlags(context.Background(), false)

			require.NoError(t, err)
			if client != nil {
				t.Fatalf("expected a nil SSH client interface, got %T", client)
			}
		})
	}
}

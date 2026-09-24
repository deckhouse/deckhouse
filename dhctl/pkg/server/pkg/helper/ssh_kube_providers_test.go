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

package helper

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCreateProvidersWithAPIServerNeedsNoConnectionConfig covers the case
// Deckhouse Commander hits for clusters it reaches only through its AMPG tunnel:
// an API server endpoint and no SSH connection config at all. The providers must
// come up anyway, and the SSH flags of the dhctl-server process must not be
// mistaken for the cluster's.
func TestCreateProvidersWithAPIServerNeedsNoConnectionConfig(t *testing.T) {
	sshProviderInitializer, kubeProvider, cleanup, err := CreateProviders(
		t.Context(),
		"",
		false,
		t.TempDir(),
		WithAPIServer(&APIServerConnection{
			URL:   "https://ampg-api-server-0d9f.example:6443",
			Token: "commander-agent-token",
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cleanup()) })

	require.NotNil(t, kubeProvider)

	// Non-nil but hostless: lib-connection dereferences the config while cloning
	// the connection, and there is no host to reach over SSH in this mode.
	require.NotNil(t, sshProviderInitializer)
	require.False(t, sshProviderInitializer.CheckHosts(t.Context()))
}

func TestAPIServerConnectionDefined(t *testing.T) {
	require.False(t, (*APIServerConnection)(nil).Defined())
	require.False(t, (&APIServerConnection{Token: "token"}).Defined())
	require.True(t, (&APIServerConnection{URL: "https://api.example:6443"}).Defined())
}

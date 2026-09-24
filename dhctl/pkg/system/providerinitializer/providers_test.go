// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package providerinitializer

import (
	"crypto/ed25519"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"k8s.io/client-go/rest"

	"github.com/deckhouse/lib-connection/pkg/provider"
	"github.com/deckhouse/lib-connection/pkg/settings"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

const connectionConfigWithoutHosts = `
apiVersion: dhctl.deckhouse.io/v1
kind: SSHConfig
sshUser: ubuntu
sshPort: 22
sudoPassword: test
`

func testProviderSettings() *settings.BaseProviders {
	return settings.NewBaseProviders(settings.ProviderParams{
		Logger: dhlog.Discard(),
		TmpDir: options.DefaultTmpDir(),
	})
}

// plantPrivateKey gives the process a $HOME holding a usable private key, which is what the argv
// arm of getProviderInitializer picks up when no SSH flags are passed. Without it the outcome of
// that arm depends on the keys the machine running the test happens to have.
func plantPrivateKey(t *testing.T) {
	t.Helper()

	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".ssh"), 0o700))

	_, key, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	block, err := ssh.MarshalPrivateKey(key, "")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(home, ".ssh", "id_rsa"), pem.EncodeToMemory(block), 0o600))

	t.Setenv("HOME", home)
}

func TestGetProviderInitializerConnectionConfigOnly(t *testing.T) {
	tests := []struct {
		name    string
		opts    []ProviderOptions
		wantNil bool
	}{
		{
			name:    "connection config only, and none supplied, means the operation runs without ssh",
			opts:    []ProviderOptions{WithConnectionConfig(""), WithConnectionConfigOnly()},
			wantNil: true,
		},
		{
			name: "connection config only still parses the config it is given",
			opts: []ProviderOptions{WithConnectionConfig(connectionConfigWithoutHosts), WithConnectionConfigOnly()},
		},
		{
			name: "without the option an empty config still falls back to the process arguments",
			opts: []ProviderOptions{WithConnectionConfig("")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plantPrivateKey(t)

			initializer, err := getProviderInitializer(t.Context(), testProviderSettings(), tt.opts...)
			require.NoError(t, err)

			if tt.wantNil {
				require.Nil(t, initializer)

				return
			}

			require.NotNil(t, initializer)
		})
	}
}

// TestWithKubeRestConfigNeedsNoSSH pins the contract Deckhouse Commander relies on:
// once an API server endpoint is supplied, the Kubernetes connection is served
// without any SSH provider at all — no session to a master, no kubectl proxy there.
// Passing a nil SSH initializer is the assertion: over SSH, lib-connection refuses.
func TestWithKubeRestConfigNeedsNoSSH(t *testing.T) {
	sett := testProviderSettings()

	cfg, err := resolveKubeConfig(sett, newProviderOptions(
		WithConnectionConfig(connectionConfigWithoutHosts),
		WithKubeRestConfig(&rest.Config{Host: "https://ampg-api-server.example:6443", BearerToken: "token"}),
	))
	require.NoError(t, err)
	require.True(t, cfg.IsRest())
	require.False(t, cfg.OverSSH())

	runner, err := provider.GetRunnerInterface(t.Context(), cfg, sett, nil)
	require.NoError(t, err)
	require.IsType(t, &provider.RunnerInterfaceNoAction{}, runner)
}

// TestWithoutKubeRestConfigGoesOverSSH is the other half of the contract: a request
// that carries no API server endpoint keeps the SSH path it has always had.
func TestWithoutKubeRestConfigGoesOverSSH(t *testing.T) {
	sett := testProviderSettings()

	cfg, err := resolveKubeConfig(sett, newProviderOptions(WithConnectionConfig(connectionConfigWithoutHosts)))
	require.NoError(t, err)
	require.False(t, cfg.IsRest())
	require.True(t, cfg.OverSSH())

	_, err = provider.GetRunnerInterface(t.Context(), cfg, sett, nil)
	require.Error(t, err)
}

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
	"crypto/ed25519"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/deckhouse/lib-connection/pkg/settings"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

const kubeconfigYAML = `
apiVersion: v1
kind: Config
current-context: test
clusters:
- name: test
  cluster:
    server: https://127.0.0.1:6443
contexts:
- name: test
  context:
    cluster: test
    user: test
users:
- name: test
  user:
    token: test
`

func newInitializer(t *testing.T, hosts []sshconfig.Host) *providerinitializer.SSHProviderInitializer {
	t.Helper()

	params := settings.ProviderParams{Logger: dhlog.Discard(), TmpDir: options.DefaultTmpDir()}

	// Config must be non-nil: lib-connection dereferences it while cloning the connection.
	return providerinitializer.NewSSHProviderInitializer(
		settings.NewBaseProviders(params),
		&sshconfig.ConnectionConfig{Config: &sshconfig.Config{}, Hosts: hosts},
	)
}

func TestSSHProviderOrNil(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		hosts          []sshconfig.Host
		nilInitializer bool
		kubeConfig     string
		wantProvider   bool
		wantErr        error
	}{
		{
			name:         "hosts from the connection config give a provider",
			hosts:        []sshconfig.Host{{Host: "10.0.0.1"}},
			wantProvider: true,
		},
		{
			name:         "hosts win over a kubeconfig",
			hosts:        []sshconfig.Host{{Host: "10.0.0.1"}},
			kubeConfig:   kubeconfigYAML,
			wantProvider: true,
		},
		{
			name:    "no hosts and no kubeconfig is a misconfiguration",
			wantErr: providerinitializer.ErrHostsFromCacheNotFound,
		},
		{
			name:       "no hosts with a kubeconfig runs without ssh",
			kubeConfig: kubeconfigYAML,
		},
		{
			name:           "a nil initializer is ssh-less whatever the kubeconfig",
			nilInitializer: true,
			kubeConfig:     kubeconfigYAML,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var initializer *providerinitializer.SSHProviderInitializer
			if !tt.nilInitializer {
				initializer = newInitializer(t, tt.hosts)
			}

			sshProvider, err := SSHProviderOrNil(t.Context(), initializer, tt.kubeConfig)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, sshProvider)

				return
			}

			require.NoError(t, err)

			if tt.wantProvider {
				assert.NotNil(t, sshProvider)

				return
			}

			assert.Nil(t, sshProvider)
		})
	}
}

// plantSSHKey gives the process a $HOME holding a usable private key, which is what the argv arm
// of the provider initializer picks up when the caller passes no SSH flags. The key is what makes
// the test below meaningful: with an unusable one that arm fails, and failing looks the same as
// not being taken at all.
func plantSSHKey(t *testing.T) {
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

func TestCreateProvidersKubeConfigWithoutConnectionConfig(t *testing.T) {
	plantSSHKey(t)

	sshProviderInitializer, kubeProvider, cleanup, err := CreateProviders(
		t.Context(), "", false, t.TempDir(),
		WithKubeConfig(kubeconfigYAML),
	)
	require.NoError(t, err)
	require.Nil(t, sshProviderInitializer)
	require.NotNil(t, kubeProvider)
	require.NoError(t, cleanup())
}

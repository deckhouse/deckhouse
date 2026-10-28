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

	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/settings"
	sshconfig "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/config"
	dhlog "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestCreateProvidersKubeConfigWithoutConnectionConfig(t *testing.T) {
	sshProviderInitializer, kubeProvider, cleanup, err := CreateProviders(
		t.Context(), "", false, t.TempDir(),
		WithKubeConfig(kubeconfigYAML),
	)
	require.NoError(t, err)
	require.Nil(t, sshProviderInitializer)
	require.NotNil(t, kubeProvider)
	require.NoError(t, cleanup())
}

func TestCreateProvidersAllowMissingConnectionConfig(t *testing.T) {
	sshProviderInitializer, kubeProvider, cleanup, err := CreateProviders(
		t.Context(), "", false, t.TempDir(),
		AllowMissingConnectionConfig(),
		WithKubeConfig(kubeconfigYAML),
	)
	require.NoError(t, err)
	require.NotNil(t, sshProviderInitializer)
	require.NotNil(t, sshProviderInitializer.GetSettings())
	require.NotNil(t, sshProviderInitializer.GetConfig())
	require.False(t, sshProviderInitializer.CheckHosts(t.Context()))
	require.NotNil(t, kubeProvider)
	require.NoError(t, cleanup())
}

// TestCreateProvidersAPIServerWithoutConnectionConfig covers the case Deckhouse
// Commander hits for clusters it reaches only through its AMPG tunnel: an API
// server endpoint and no SSH connection config. The kube provider must come up
// anyway.
func TestCreateProvidersAPIServerWithoutConnectionConfig(t *testing.T) {
	sshProviderInitializer, kubeProvider, cleanup, err := CreateProviders(
		t.Context(), "", false, t.TempDir(),
		WithAPIServer(&APIServerConnection{
			URL:   "https://ampg-api-server-0d9f.example:6443",
			Token: "commander-agent-token",
		}),
	)
	require.NoError(t, err)
	require.Nil(t, sshProviderInitializer)
	require.NotNil(t, kubeProvider)
	require.NoError(t, cleanup())
}

// A request with neither a connection config nor a direct Kubernetes route has nothing to connect
// with. The process the server runs in is not consulted: neither its arguments nor its environment
// nor the keys in its home directory.
func TestCreateProvidersWithoutAnyConnection(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DHCTL_CLI_SSH_HOSTS", "10.0.0.1")
	t.Setenv("KUBECONFIG", "")

	sshProviderInitializer, kubeProvider, cleanup, err := CreateProviders(t.Context(), "", false, t.TempDir())
	require.ErrorIs(t, err, providerinitializer.ErrSSHHostRequiredForKubernetesConnection)
	require.Nil(t, sshProviderInitializer)
	require.Nil(t, kubeProvider)
	require.NoError(t, cleanup())

	// Bootstrap is written against a present initializer: it takes the hosts from its state later
	// and builds the kube provider from the initializer then.
	sshProviderInitializer, kubeProvider, cleanup, err = CreateProviders(t.Context(), "", false, t.TempDir(), AllowMissingHostsFromCache(), AllowMissingConnectionConfig())
	require.NoError(t, err)
	require.NotNil(t, sshProviderInitializer)
	require.Empty(t, sshProviderInitializer.GetConfig().Hosts)
	require.Nil(t, kubeProvider)
	require.NoError(t, cleanup())
}

func TestAPIServerConnectionDefined(t *testing.T) {
	assert.False(t, (*APIServerConnection)(nil).Defined())
	assert.False(t, (&APIServerConnection{Token: "token"}).Defined())
	assert.True(t, (&APIServerConnection{URL: "https://api.example:6443"}).Defined())

	assert.Nil(t, (*APIServerConnection)(nil).RestConfig())
	assert.Nil(t, (&APIServerConnection{}).RestConfig())
}

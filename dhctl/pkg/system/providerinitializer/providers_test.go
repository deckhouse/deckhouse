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
	"strings"
	"testing"

	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/kube"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/settings"
	sshconfig "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/config"
	dhlog "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

const connectionConfigWithoutHosts = `
apiVersion: dhctl.deckhouse.io/v1
kind: SSHConfig
sshUser: ubuntu
sshPort: 22
sudoPassword: test
`

// writePrivateKey writes an unencrypted private key to path.
func writePrivateKey(t *testing.T, path string) {
	t.Helper()

	_, key, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	block, err := ssh.MarshalPrivateKey(key, "")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(block), 0o600))
}

// GetProviders takes the connections it is given and nothing else: without an SSH connection
// Kubernetes over SSH is refused at once, the process arguments and environment are not consulted.
func TestGetProvidersTakesOnlyWhatItIsGiven(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DHCTL_CLI_SSH_HOSTS", "10.0.0.1")
	t.Setenv("DHCTL_CLI_SSH_USER", "ubuntu")
	t.Setenv("KUBECONFIG", "")

	params := settings.ProviderParams{Logger: dhlog.Discard(), TmpDir: options.DefaultTmpDir()}

	sshProviderInitializer, kubeProvider, err := GetProviders(t.Context(), params, &kube.Config{}, GetProvidersOptions{})
	require.ErrorIs(t, err, ErrSSHHostRequiredForKubernetesConnection)
	require.Nil(t, sshProviderInitializer)
	require.Nil(t, kubeProvider)

	hostless, err := sshconfig.ParseConnectionConfig(strings.NewReader(connectionConfigWithoutHosts), settings.NewBaseProviders(params), sshconfig.ParseWithRequiredSSHHost(false))
	require.NoError(t, err)

	// The hosts may still come from the state cache later, unless the command needs them now.
	sshProviderInitializer, kubeProvider, err = GetProviders(t.Context(), params, &kube.Config{}, GetProvidersOptions{SSH: hostless, RequireKube: true})
	require.ErrorIs(t, err, ErrSSHHostRequiredForKubernetesConnection)
	require.NotNil(t, sshProviderInitializer)
	require.Nil(t, kubeProvider)
	require.Equal(t, "ubuntu", sshProviderInitializer.GetConfig().Config.User)
	require.Empty(t, sshProviderInitializer.GetConfig().Hosts)
}

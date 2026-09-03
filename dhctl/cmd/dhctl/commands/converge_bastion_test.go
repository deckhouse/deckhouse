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

package commands

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/deckhouse/lib-connection/pkg/settings"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

// The converge cache is keyed by the kubeconfig's PATH, not its content
// (pkg/state/cache/init.go, GetCacheIdentityFromKubeconfig). Pointing the option
// at the temporary copy would give every run a new identity and lose the state
// an interrupted converge resumes from.
func TestTheKubeconfigOptionSurvivesTheBastionChannel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "admin.kubeconfig")
	require.NoError(t, os.WriteFile(path, []byte("apiVersion: v1\nkind: Config\n"), 0o600))

	opts := options.New()
	opts.Kube.Config = path

	// No SSH provider: the guards return the provider untouched, which is the
	// only branch reachable without a bastion to dial.
	_, stop, err := kubeProviderThroughBastion(t.Context(), opts, nil, nil)
	require.NoError(t, err)
	defer stop()

	require.Equal(t, path, opts.Kube.Config, "the cache identity is derived from this path")
}

const twoContextKubeconfig = `
apiVersion: v1
kind: Config
current-context: other
clusters:
- name: other
  cluster:
    server: https://other.example:6443
- name: wanted
  cluster:
    server: https://master-0.example:6443
contexts:
- name: other
  context:
    cluster: other
    user: other
- name: wanted
  context:
    cluster: wanted
    user: wanted
users:
- name: other
  user:
    client-certificate: /nonexistent/other-user.crt
    client-key: /nonexistent/other-user.key
- name: wanted
  user:
    client-certificate: /nonexistent/wanted-user.crt
    client-key: /nonexistent/wanted-user.key
`

// --kube-config-context names the apiserver the channel forwards to, and the
// client has to take its cluster and its user from that same context. Left to
// current-context, it dials the forwarded apiserver with another cluster's CA
// and another user's certificate.
func TestTheChannelUsesTheCredentialsOfTheNamedContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "admin.kubeconfig")
	require.NoError(t, os.WriteFile(path, []byte(twoContextKubeconfig), 0o600))

	opts := options.New()
	opts.Kube.Config = path
	opts.Kube.ConfigContext = "wanted"
	opts.Global.TmpDir = dir

	// The configuration is built as the channel opens, and building it reads the
	// credentials. Both users name a certificate that does not exist, so the one
	// the failure names is the one the client would have been built from.
	_, _, err := kubeProviderThroughBastion(
		t.Context(), opts, bastionOnlyInitializer(t), nil,
	)
	require.ErrorContains(t, err, "wanted-user.crt",
		"the client must be built from the credentials of the context the operator named")
	require.NotContains(t, err.Error(), "other-user.crt")
}

// bastionOnlyInitializer is an operator who named a bastion and no SSH host:
// the kubeconfig is the only way into the cluster, which is the run the channel
// exists for.
func bastionOnlyInitializer(t *testing.T) *providerinitializer.SSHProviderInitializer {
	t.Helper()

	host, port := splitHostPort(t, startFakeBastion(t))

	return providerinitializer.NewSSHProviderInitializer(
		settings.NewBaseProviders(settings.ProviderParams{}),
		&sshconfig.ConnectionConfig{Config: (&sshconfig.Config{
			BastionHost:     host,
			BastionPort:     &port,
			BastionUser:     "ubuntu",
			BastionPassword: "password",
		}).FillDefaults()},
	)
}

// startFakeBastion answers one SSH connection and refuses every channel on it:
// this test fails before any traffic reaches the forward.
func startFakeBastion(t *testing.T) string {
	t.Helper()

	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	hostKey, err := ssh.NewSignerFromKey(private)
	require.NoError(t, err)

	serverConfig := &ssh.ServerConfig{NoClientAuth: true}
	serverConfig.AddHostKey(hostKey)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveFakeBastion(conn, serverConfig)
		}
	}()

	return listener.Addr().String()
}

func serveFakeBastion(conn net.Conn, serverConfig *ssh.ServerConfig) {
	sshConn, newChannels, requests, err := ssh.NewServerConn(conn, serverConfig)
	if err != nil {
		return
	}
	defer sshConn.Close()
	go ssh.DiscardRequests(requests)

	for newChannel := range newChannels {
		newChannel.Reject(ssh.ConnectionFailed, "nothing is reachable from this bastion")
	}
}

func splitHostPort(t *testing.T, address string) (string, int) {
	t.Helper()

	host, port, err := net.SplitHostPort(address)
	require.NoError(t, err)
	number, err := strconv.Atoi(port)
	require.NoError(t, err)

	return host, number
}

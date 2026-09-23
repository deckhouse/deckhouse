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

package immutable

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
)

const serverKubeconfig = `
apiVersion: v1
kind: Config
current-context: ctx
clusters:
- name: cluster
  cluster:
    server: %s
contexts:
- name: ctx
  context:
    cluster: cluster
    user: user
users:
- name: user
  user: {}
`

func kubeconfigFor(server string) []byte {
	return fmt.Appendf(nil, serverKubeconfig, server)
}

// The forward has to land on the address the kubeconfig itself names: that is
// the address whose certificate the client then verifies against.
func TestKubeconfigServerIsTheAddressTheForwardMustReach(t *testing.T) {
	tests := []struct {
		name     string
		server   string
		wantHost string
		wantPort int
	}{
		{"an address", "https://10.12.5.24:6443", "10.12.5.24", 6443},
		{"a name", "https://master-0.example:6443", "master-0.example", 6443},
		{"IPv6", "https://[2001:db8::1]:6443", "2001:db8::1", 6443},
		{"no port is 443", "https://master-0.example", "master-0.example", 443},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := kubeconfigServer(kubeconfigFor(tt.server), "")
			require.NoError(t, err)
			require.Equal(t, tt.wantHost, host)
			require.Equal(t, tt.wantPort, port)
		})
	}
}

func TestKubeconfigServerRefusesWhatItCannotForwardTo(t *testing.T) {
	_, _, err := kubeconfigServer(kubeconfigFor("https://"), "")
	require.ErrorContains(t, err, "names no host")

	_, _, err = kubeconfigServer(kubeconfigFor("https://master-0.example:6443"), "missing")
	require.ErrorContains(t, err, `no context "missing"`)
}

// Without a bastion the channel is the address itself, so this drives the whole
// of OpenKubeconfigChannel but the tunnel: the configuration it builds must keep
// the name TLS is verified against, or a converge through a bastion cannot
// handshake. Built in memory: the credentials never touch the disk.
func TestTheChannelConfigurationKeepsTheNameItWasIssuedFor(t *testing.T) {
	const serverName = "master-0.example"

	dir := t.TempDir()
	original := filepath.Join(dir, "admin.kubeconfig")
	require.NoError(t, os.WriteFile(original, kubeconfigFor("https://"+serverName+":6443"), 0o600))

	restConfig, stop, err := OpenKubeconfigChannel(t.Context(), nil, nil, original, "")
	require.NoError(t, err)
	defer stop()

	require.Equal(t, serverName, restConfig.TLSClientConfig.ServerName,
		"the client must be verified against the name the kubeconfig named")
	require.Equal(t, "https://"+serverName+":6443", restConfig.Host,
		"with no bastion the server is the address itself")
	require.Equal(t, global.ImpersonateUser, restConfig.Impersonate.UserName)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "nothing but the operator's own kubeconfig may be on disk: %v", entries)
}

const proxiedKubeconfig = `
apiVersion: v1
kind: Config
current-context: ctx
clusters:
- name: cluster
  cluster:
    server: https://master-0.example:6443
    proxy-url: http://127.0.0.1:1
    certificate-authority: ca.crt
contexts:
- name: ctx
  context:
    cluster: cluster
    user: user
users:
- name: user
  user:
    client-certificate: client.crt
    client-key: client.key
`

// The local end of the forward is reached directly. A proxy the operator set for
// the real server address is applied to every request client-go makes through
// it — http.ProxyURL exempts no address, loopback included — so the retargeted
// copy must not carry it.
func TestTheChannelConfigurationDropsTheProxyOfTheAddressItLeft(t *testing.T) {
	_, restConfig := openWithoutBastion(t, proxiedKubeconfig)

	require.Nil(t, restConfig.Proxy,
		"a proxy for the real server address would swallow the loopback connection")
}

// A configuration built from bytes has no directory of its own, and client-go
// resolves the file references of a kubeconfig against the directory it was
// loaded from: a relative certificate-authority would stop resolving on the way.
func TestTheChannelConfigurationKeepsPointingAtTheFilesItReferences(t *testing.T) {
	dir, restConfig := openWithoutBastion(t, proxiedKubeconfig)

	require.Equal(t, filepath.Join(dir, "ca.crt"), restConfig.TLSClientConfig.CAFile,
		"a relative path resolves against the original's directory, where the file lives")
	require.Equal(t, filepath.Join(dir, "client.crt"), restConfig.TLSClientConfig.CertFile,
		"the user's files travel the same way the cluster's do")
}

// openWithoutBastion runs the channel over the given kubeconfig with no bastion,
// and returns the directory the original lives in together with the
// configuration the Kubernetes client is built from.
func openWithoutBastion(t *testing.T, content string) (string, *rest.Config) {
	t.Helper()

	dir := t.TempDir()
	original := filepath.Join(dir, "admin.kubeconfig")
	require.NoError(t, os.WriteFile(original, []byte(content), 0o600))
	// Building the configuration checks that every file it names can be read.
	for _, name := range []string{"ca.crt", "client.crt", "client.key"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600))
	}

	restConfig, stop, err := OpenKubeconfigChannel(t.Context(), nil, nil, original, "")
	require.NoError(t, err)
	defer stop()

	return dir, restConfig
}

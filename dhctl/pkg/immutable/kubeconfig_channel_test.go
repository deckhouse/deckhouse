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
	"k8s.io/client-go/tools/clientcmd"
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
// of OpenKubeconfigChannel but the tunnel: the copy it writes must keep the name
// TLS is verified against, or a converge through a bastion cannot handshake.
func TestTheWrittenKubeconfigKeepsTheNameItWasIssuedFor(t *testing.T) {
	const serverName = "master-0.example"

	dir := t.TempDir()
	original := filepath.Join(dir, "admin.kubeconfig")
	require.NoError(t, os.WriteFile(original, kubeconfigFor("https://"+serverName+":6443"), 0o600))

	path, stop, err := OpenKubeconfigChannel(t.Context(), nil, nil, original, "", dir)
	require.NoError(t, err)
	defer stop()

	written, err := os.ReadFile(path)
	require.NoError(t, err)
	parsed, err := clientcmd.Load(written)
	require.NoError(t, err)

	require.Equal(t, serverName, parsed.Clusters["cluster"].TLSServerName,
		"the copy must be verified against the name the kubeconfig named")
	require.Equal(t, "https://"+serverName+":6443", parsed.Clusters["cluster"].Server,
		"with no bastion the server is the address itself")

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "it holds cluster-admin credentials")

	stop()
	_, err = os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist, "the copy must not outlive the channel")
}

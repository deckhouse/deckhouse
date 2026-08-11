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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeKubeconfig lays a kubeconfig down in its own directory, so a relative
// certificate-authority path resolves the way clientcmd resolves it.
func writeKubeconfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// The identity has to come from the CA as well as the address: the bastion-forward
// line makes every immutable cluster reachable at https://127.0.0.1:6445, and the
// cache this keys holds the infrastructure state destroy tears down.
func TestKubeconfigClusterIdentityReadsTheCAFromDisk(t *testing.T) {
	const template = `apiVersion: v1
kind: Config
clusters:
- name: kubernetes
  cluster:
    server: https://127.0.0.1:6445
    certificate-authority: ca.crt
contexts:
- name: kubernetes-admin@kubernetes
  context:
    cluster: kubernetes
    user: kubernetes-admin
current-context: kubernetes-admin@kubernetes
users:
- name: kubernetes-admin
  user: {}
`

	first := writeKubeconfig(t, template)
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(first), "ca.crt"), []byte("first cluster CA"), 0o600))

	second := writeKubeconfig(t, template)
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(second), "ca.crt"), []byte("second cluster CA"), 0o600))

	firstIdentity, err := kubeconfigClusterIdentity(first, "")
	require.NoError(t, err)
	secondIdentity, err := kubeconfigClusterIdentity(second, "")
	require.NoError(t, err)

	require.NotEqual(t, firstIdentity, secondIdentity,
		"two clusters reached at the same address must not share a cache directory")

	// Pinned to the byte: the identity names a cache directory, so a changed
	// spelling hands an existing destroy an empty cache and no state to finish from.
	require.Equal(t, "kubeconfig-741dd75a3d36042546c194b64f943ecb25034960483cb14a160efd00b1812e3a", firstIdentity)
}

// insecure-skip-tls-verify carries no CA at all, so the identity would collapse
// to the address alone. Refused rather than accepted: the failure this guards
// is destroying somebody else's infrastructure, and it is silent.
func TestKubeconfigClusterIdentityRefusesAKubeconfigWithoutCA(t *testing.T) {
	path := writeKubeconfig(t, `apiVersion: v1
kind: Config
clusters:
- name: kubernetes
  cluster:
    server: https://127.0.0.1:6445
    insecure-skip-tls-verify: true
contexts:
- name: kubernetes-admin@kubernetes
  context:
    cluster: kubernetes
    user: kubernetes-admin
current-context: kubernetes-admin@kubernetes
users:
- name: kubernetes-admin
  user: {}
`)

	_, err := kubeconfigClusterIdentity(path, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "carries no cluster CA")
}

// A bare "in-cluster" would be shared by every in-cluster run on this machine.
// With no API server in the environment there is nothing to name the cluster by,
// so the run stops rather than reaching for a constant.
func TestInClusterCacheIdentity(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.223.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "443")
	first, err := inClusterCacheIdentity()
	require.NoError(t, err)
	require.Equal(t, "in-cluster-10.223.0.1:443", first)

	t.Setenv("KUBERNETES_SERVICE_HOST", "10.224.0.1")
	second, err := inClusterCacheIdentity()
	require.NoError(t, err)
	require.NotEqual(t, first, second)

	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	_, err = inClusterCacheIdentity()
	require.Error(t, err)
	require.Contains(t, err.Error(), "KUBERNETES_SERVICE_HOST is empty")
}

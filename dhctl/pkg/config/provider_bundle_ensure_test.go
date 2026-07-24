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

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// EnsureExternalProviderBundle must not touch the cluster for providers that
// need no downloaded bundle: a static cluster (no provider) and an in-tree
// provider whose schemas ship in candi. A nil kube client would panic in
// GetRegistryData, so a nil error proves the early exit before any cluster read.
func TestEnsureExternalProviderBundleSkipsClusterRead(t *testing.T) {
	staticCluster := `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "1.32"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
`
	err := EnsureExternalProviderBundle(t.Context(), nil, staticCluster, &options.GlobalOptions{DownloadDir: t.TempDir()})
	require.NoError(t, err)

	candiDir := t.TempDir()
	schemaPath := filepath.Join(candiDir, "cloud-providers", "yandex", "openapi", "cluster_configuration.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(schemaPath), 0o755))
	require.NoError(t, os.WriteFile(schemaPath, []byte("type: object\n"), 0o644))

	yandexCluster := `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
kubernetesVersion: "1.32"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
cloud:
  provider: Yandex
  prefix: test
`
	err = EnsureExternalProviderBundle(t.Context(), nil, yandexCluster, &options.GlobalOptions{DownloadDir: t.TempDir(), CandiDir: candiDir})
	require.NoError(t, err)
}

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

package template

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/directoryconfig"
)

func TestRenderBashibleTemplateUsesOnlyKubeAPIEndpoints(t *testing.T) {
	tplContent, err := os.ReadFile("/deckhouse/candi/bashible/bashible.sh.tpl")
	require.NoError(t, err)

	data := map[string]interface{}{
		"runType":     "Normal",
		"clusterUUID": "848c3b2c-eda6-11ec-9289-dff550c719eb",
		"nodeGroup": map[string]interface{}{
			"name":     "master",
			"nodeType": "CloudPermanent",
		},
		"clusterMasterEndpoints": []map[string]interface{}{
			{
				"address":     "10.0.0.1",
				"kubeApiPort": 6443,
			},
			{
				"address":                "10.0.0.2",
				"rppServerPort":          4219,
				"rppBootstrapServerPort": 4300,
			},
		},
		"clusterMasterKubeAPIEndpoints": []string{
			"10.0.0.1:6443",
		},
		"images": map[string]interface{}{
			"registrypackages": map[string]interface{}{
				"rppGet": "sha256:test",
			},
		},
		"registry": map[string]interface{}{
			"registryModuleEnable": "false",
		},
	}

	rendered, err := RenderTemplate("bashible.sh.tpl", tplContent, data)
	require.NoError(t, err)

	content := rendered.Content.String()
	require.Contains(t, content, `for server in 10.0.0.1:6443; do`)
	require.Contains(t, content, `export PACKAGES_PROXY_BOOTSTRAP_CLUSTER_UUID="848c3b2c-eda6-11ec-9289-dff550c719eb"`)
	require.Contains(t, content, `export PACKAGES_PROXY_KUBE_APISERVER_ENDPOINTS="10.0.0.1:6443"`)
	require.NotContains(t, content, `bb-minget-install`)
	require.Contains(t, content, `bb-rpp-get-install`)
	require.NotContains(t, content, `10.0.0.2:<no value>`)
}

func TestRenderBashibleTemplateUsesClusterMasterRPPAddressesForBootstrap(t *testing.T) {
	dc := &directoryconfig.DirectoryConfig{
		DownloadDir:      "/tmp",
		DownloadCacheDir: "/tmp/cache",
	}

	metaConfig, err := config.ParseConfigFromData(context.TODO(), clusterConfig+initConfig, config.DummyPreparatorProvider(), dc)
	require.NoError(t, err)
	mingetPath := filepath.Join(t.TempDir(), "minget")
	require.NoError(t, os.WriteFile(mingetPath, []byte("test-minget"), 0o600))
	t.Setenv("DHCTL_MINGET_PATH", mingetPath)

	data, err := metaConfig.ConfigForBashibleBundleTemplate("10.0.0.2")
	require.NoError(t, err)

	tplContent, err := os.ReadFile("/deckhouse/candi/bashible/bashible.sh.tpl")
	require.NoError(t, err)

	rendered, err := RenderTemplate("bashible.sh.tpl", tplContent, data)
	require.NoError(t, err)

	content := rendered.Content.String()
	require.Contains(t, content, `unset PACKAGES_PROXY_BOOTSTRAP_CLUSTER_UUID`)
	require.Contains(t, content, `export PACKAGES_PROXY_ADDRESSES="127.0.0.1:5444"`)
	require.Contains(t, content, `export PACKAGES_PROXY_TOKEN="passthrough"`)
	require.Contains(t, content, `bb-minget-install`)
}

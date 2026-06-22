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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

const testRPPBootstrapServerPort = 4282

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
				"rppBootstrapServerPort": testRPPBootstrapServerPort,
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

// cleanSeedData builds a clean registry context for 022 render tests.
// 022 gates on registryModuleEnable AND bootstrap.seed (not mode).
func cleanSeedData(enable bool, seed bool) map[string]interface{} {
	reg := map[string]interface{}{
		"registryModuleEnable": enable,
		"mode":                 "Managed",
		"bootstrap": map[string]interface{}{
			"init": map[string]interface{}{
				"ca": map[string]interface{}{
					"cert": "FAKECACERT",
					"key":  "FAKECAKEY",
				},
				"ro_user": map[string]interface{}{
					"name":          "ro",
					"password":      "rop",
					"password_hash": "$2y$12$h",
				},
				"rw_user": map[string]interface{}{
					"name":          "rw",
					"password":      "rwp",
					"password_hash": "$2y$12$h",
				},
			},
			"seed": seed,
		},
	}
	images := map[string]interface{}{
		"registry": map[string]interface{}{
			"dockerAuth":         "sha256:auth",
			"dockerDistribution": "sha256:dist",
			"syncer":             "sha256:sync",
		},
		"registrypackages": map[string]interface{}{
			"cfssl165": "sha256:cfssl",
		},
	}
	return map[string]interface{}{"registry": reg, "images": images}
}

func TestRender022GatedOnBootstrapSeed(t *testing.T) {
	tplContent, err := os.ReadFile("/deckhouse/candi/bashible/common-steps/cluster-bootstrap/022_install_registry_seed.sh.tpl")
	require.NoError(t, err)

	t.Run("air-gap: registryModuleEnable=true + seed=true renders seed body", func(t *testing.T) {
		rendered, err := RenderTemplate("022_install_registry_seed.sh.tpl", tplContent, cleanSeedData(true, true))
		require.NoError(t, err)
		content := rendered.Content.String()
		require.Contains(t, content, "bb-package-install")
		require.Contains(t, content, "bootstrap-seed")
		require.Contains(t, content, "127.0.0.1:5010")
		require.Contains(t, content, "127.0.0.1:5061")
		require.Contains(t, content, "registry-bootstrap")
	})

	t.Run("connected: registryModuleEnable=true + seed=false skips seed body", func(t *testing.T) {
		rendered, err := RenderTemplate("022_install_registry_seed.sh.tpl", tplContent, cleanSeedData(true, false))
		require.NoError(t, err)
		content := rendered.Content.String()
		require.NotContains(t, content, "bb-package-install")
		require.NotContains(t, content, "127.0.0.1:5010")
		require.NotContains(t, content, "registry-bootstrap")
	})

	t.Run("module disabled: registryModuleEnable=false + seed=false skips seed body", func(t *testing.T) {
		rendered, err := RenderTemplate("022_install_registry_seed.sh.tpl", tplContent, cleanSeedData(false, false))
		require.NoError(t, err)
		content := rendered.Content.String()
		require.NotContains(t, content, "bb-package-install")
		require.NotContains(t, content, "127.0.0.1:5010")
		require.NotContains(t, content, "registry-bootstrap")
	})
}

// TestRenderRegistrySeedStepGating is retained to verify the clean-context gate
// (registryModuleEnable AND bootstrap.seed) using the same helper as TestRender022GatedOnBootstrapSeed.
func TestRenderRegistrySeedStepGating(t *testing.T) {
	tplContent, err := os.ReadFile("/deckhouse/candi/bashible/common-steps/cluster-bootstrap/022_install_registry_seed.sh.tpl")
	require.NoError(t, err)

	t.Run("renders non-empty for registryModuleEnable=true + seed=true", func(t *testing.T) {
		rendered, err := RenderTemplate("022_install_registry_seed.sh.tpl", tplContent, cleanSeedData(true, true))
		require.NoError(t, err)
		content := rendered.Content.String()
		require.NotEmpty(t, content)
		require.Contains(t, content, "bb-package-install")
		require.Contains(t, content, "127.0.0.1:5010")
		require.Contains(t, content, "127.0.0.1:5061")
		require.Contains(t, content, "registry-bootstrap")
	})

	t.Run("does not render functional content for registryModuleEnable=true + seed=false", func(t *testing.T) {
		rendered, err := RenderTemplate("022_install_registry_seed.sh.tpl", tplContent, cleanSeedData(true, false))
		require.NoError(t, err)
		content := rendered.Content.String()
		require.NotContains(t, content, "bb-package-install")
		require.NotContains(t, content, "127.0.0.1:5010")
		require.NotContains(t, content, "registry-bootstrap")
	})

	t.Run("does not render functional content for registryModuleEnable=false + seed=false", func(t *testing.T) {
		rendered, err := RenderTemplate("022_install_registry_seed.sh.tpl", tplContent, cleanSeedData(false, false))
		require.NoError(t, err)
		content := rendered.Content.String()
		require.NotContains(t, content, "bb-package-install")
		require.NotContains(t, content, "127.0.0.1:5010")
		require.NotContains(t, content, "registry-bootstrap")
	})
}

func TestRender099CleanupDoesNotRemoveSeedPackagesForNewModelLocal(t *testing.T) {
	tplContent, err := os.ReadFile("/deckhouse/candi/bashible/common-steps/cluster-bootstrap/099_cleanup_after_cluster_bootstrap.sh.tpl")
	require.NoError(t, err)

	data := map[string]interface{}{
		"registry": map[string]interface{}{
			"registryModuleEnable": true,
			"mode":                 "Local",
		},
	}

	// 099 must NOT remove the registry packages on a new-model install — the on-node
	// seed (022) needs registry-syncer alive until dhctl FillCacheFromSeed runs at
	// finalize, and TeardownSeed cleans up afterward.
	rendered, err := RenderTemplate("099_cleanup_after_cluster_bootstrap.sh.tpl", tplContent, data)
	require.NoError(t, err)
	require.NotContains(t, rendered.Content.String(), "bb-package-remove")
	require.NotContains(t, rendered.Content.String(), "REGISTRY_MODULE_IGNITER_DIR")
}

func TestRenderBashibleTemplateUsesClusterMasterRPPAddressesForBootstrap(t *testing.T) {
	metaConfig, err := config.ParseConfigFromData(context.TODO(), clusterConfig+initConfig, config.DummyPreparatorProvider(), &options.New().Global)
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

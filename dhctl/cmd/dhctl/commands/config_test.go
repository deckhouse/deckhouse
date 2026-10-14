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
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/tests"
)

// TestLoadConfigForRender_RefusesTheConfigOfACommandThatCannotRenderIt keeps the refusal the loader
// gave up. LoadConfigFromFile no longer demands a ClusterConfiguration, because a full bootstrap
// must be able to install into a cluster whose control plane dhctl did not create; the render
// commands still cannot do anything with such a config, and they fail silently - every template
// they fill reads an empty map and renders a bundle with no cluster domain and no Kubernetes
// version rather than returning an error.
func TestLoadConfigForRender_RefusesTheConfigOfACommandThatCannotRenderIt(t *testing.T) {
	candiDir := tests.RequireCandiDir(t)
	deckhouseDir := filepath.Dir(candiDir)

	versionDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(versionDir, "version"), []byte("dev"), 0o600))

	opts := &options.Options{Global: options.GlobalOptions{
		ConfigPaths:       []string{"../../../pkg/config/testdata/eks_without_cluster_configuration.yaml"},
		CandiDir:          candiDir,
		ModulesDir:        filepath.Join(deckhouseDir, "modules"),
		GlobalHooksModule: filepath.Join(deckhouseDir, "global-hooks"),
		VersionMap:        filepath.Join(candiDir, "version_map.yml"),
		DeckhouseDir:      versionDir,
	}}

	_, err := loadConfigForRender(t.Context(), "dhctl config render bashible-bundle", opts)

	require.ErrorContains(t, err, "dhctl config render bashible-bundle", "the message must name the command that cannot proceed")
	require.ErrorContains(t, err, "ClusterConfiguration")
}

const staticClusterConfig = `
---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "1.32"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: test
  devBranch: test
  registryDockerCfg: eyJhdXRocyI6IHsgInRlc3QiOiB7fX19
`

// The bundle this command writes is the one bootstrap runs on the first master, and that
// master's node agent starts from the seed dhctl builds: without it the render stops.
func TestRenderBashibleBundleSeedsTheFirstMastersAgent(t *testing.T) {
	candiDir := tests.RequireCandiDir(t)
	metaConfig, err := config.ParseConfigFromData(t.Context(), staticClusterConfig, config.DummyValidatorProvider(), &options.New().Global)
	require.NoError(t, err)
	require.NoError(t, metaConfig.LoadVersionMap(filepath.Join(candiDir, "version_map.yml")))
	mingetPath := filepath.Join(t.TempDir(), "minget")
	require.NoError(t, os.WriteFile(mingetPath, []byte("test-minget"), 0o600))
	t.Setenv("DHCTL_MINGET_PATH", mingetPath)
	metaConfig.Images = templateImages(t, candiDir)
	bundleDir := t.TempDir()
	opts := &options.Options{
		Global: options.GlobalOptions{CandiDir: candiDir},
		Render: options.RenderOptions{BashibleBundleDir: bundleDir},
	}

	require.NoError(t, renderBashibleBundle(t.Context(), metaConfig, opts))

	steps, err := filepath.Glob(filepath.Join(bundleDir, "*", "var", "lib", "bashible", "bundle_steps", "003_install_nodelet*"))
	require.NoError(t, err)
	require.Len(t, steps, 1)
	step, err := os.ReadFile(steps[0])
	require.NoError(t, err)
	require.Contains(t, string(step), "@base64d", "the agent's document must come from the seed")
}

// templateImages gives a digest to every image the templates under candi's bashible and
// nodelet directories name, so the whole bundle renders without a built release.
func templateImages(t *testing.T, candiDir string) map[string]map[string]any {
	t.Helper()
	references := []*regexp.Regexp{
		regexp.MustCompile(`\.images\.(\w+)\.(\w+)`),
		regexp.MustCompile(`index \.images\.(\w+) "(\w+)"`),
	}
	images := map[string]map[string]any{}
	for _, dir := range []string{"bashible", "nodelet"} {
		err := filepath.WalkDir(filepath.Join(candiDir, dir), func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			text, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, reference := range references {
				for _, match := range reference.FindAllStringSubmatch(string(text), -1) {
					if images[match[1]] == nil {
						images[match[1]] = map[string]any{}
					}
					images[match[1]][match[2]] = "sha256:" + match[2]
				}
			}
			return nil
		})
		require.NoError(t, err)
	}
	return images
}

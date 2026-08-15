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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
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

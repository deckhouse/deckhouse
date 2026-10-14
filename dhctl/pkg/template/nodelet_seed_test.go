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
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func firstMasterConfig(t *testing.T, packages map[string]any) *config.MetaConfig {
	t.Helper()
	metaConfig, err := config.ParseConfigFromData(t.Context(), clusterConfig+initConfig, config.DummyValidatorProvider(), &options.New().Global)
	require.NoError(t, err)
	mingetPath := filepath.Join(t.TempDir(), "minget")
	require.NoError(t, os.WriteFile(mingetPath, []byte("test-minget"), 0o600))
	t.Setenv("DHCTL_MINGET_PATH", mingetPath)
	metaConfig.Images = map[string]map[string]any{"registrypackages": packages}
	return metaConfig
}

// platformPackages gives a digest to every registry package candi's platform document
// names: the same scan as imageKeys in dhctl/pkg/nodeletseed/seed_test.go.
func platformPackages(t *testing.T) map[string]any {
	t.Helper()
	text, err := os.ReadFile(filepath.Join(candiDir(t), "nodelet", "nodeconfig-mutable.yaml.tpl"))
	require.NoError(t, err)
	packages := map[string]any{}
	for _, match := range regexp.MustCompile(`\.images\.registrypackages\.(\w+)`).FindAllStringSubmatch(string(text), -1) {
		packages[match[1]] = "sha256:" + match[1]
	}
	return packages
}

// The first master is seeded from candi's platform document: a package that document
// names and the release did not build stops the render.
func TestFirstMasterDataIsSeededFromThePlatformDocument(t *testing.T) {
	metaConfig := firstMasterConfig(t, map[string]any{"nodelet": "sha256:nodelet"})

	_, err := FirstMasterBashibleData(t.Context(), metaConfig, "10.0.0.2", candiDir(t))

	require.ErrorContains(t, err, "build the node agent seed")
	require.ErrorContains(t, err, "map has no entry for key")
}

// Every edition ships the agent: a release without it stops the render.
func TestFirstMasterDataRefusesAReleaseWithoutTheAgent(t *testing.T) {
	metaConfig := firstMasterConfig(t, map[string]any{"jq": "sha256:jq"})

	_, err := FirstMasterBashibleData(t.Context(), metaConfig, "10.0.0.2", candiDir(t))

	require.ErrorContains(t, err, "build the node agent seed")
	require.ErrorContains(t, err, `map has no entry for key "nodelet"`)
}

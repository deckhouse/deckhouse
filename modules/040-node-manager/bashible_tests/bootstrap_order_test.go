/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package bashible_tests

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

const (
	bootstrapPrerequisites = "candi/bashible/bootstrap/01-bootstrap-prerequisites.sh.tpl"
	networkConfigured      = "echo provider-network-configured"
)

// A node may reach the masters of other zones only after the provider's network script has run, so
// it goes before the packages. Not earlier: the AWS script installs a package with rpp-get itself.
func TestTheProviderNetworkIsSetUpBeforePackagesAreInstalled(t *testing.T) {
	repository, err := filepath.Abs("../../..")
	require.NoError(t, err)

	phase, err := os.ReadFile(filepath.Join(repository, bootstrapPrerequisites))
	require.NoError(t, err)

	lib, err := os.ReadFile(filepath.Join(repository, "candi/bashible/lib.sh.tpl"))
	require.NoError(t, err)

	candi := t.TempDir()
	provider := filepath.Join(candi, "candi/cloud-providers/testcloud/bashible")
	require.NoError(t, os.MkdirAll(provider, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(candi, "candi/bashible"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(candi, "candi/bashible/lib.sh.tpl"), lib, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(provider, "bootstrap-networks.sh.tpl"),
		[]byte("#!/bin/bash\n"+networkConfigured+"\n"), 0o600))
	t.Chdir(candi)

	rendered, err := template.RenderTemplate(filepath.Base(bootstrapPrerequisites), phase, map[string]any{
		"runType":                       "Normal",
		"provider":                      "testcloud",
		"nodeGroup":                     map[string]any{"name": "worker", "nodeType": "CloudEphemeral", "cloudProviderType": "testcloud"},
		"clusterMasterKubeAPIEndpoints": []any{"10.0.0.1:6443"},
		"images": map[string]any{"registrypackages": map[string]any{
			"jq": "sha256:jq", "jq171": "sha256:jq",
			"d8Curl": "sha256:curl", "d8Curl891": "sha256:curl",
			"tailLog": "sha256:taillog",
		}},
	})
	require.NoError(t, err)

	body := rendered.Content.String()
	network := strings.Index(body, networkConfigured)
	require.GreaterOrEqual(t, network, 0, "the provider's network script is not in the phase")

	rppGet := regexp.MustCompile(`(?m)^bb-rpp-get-install( \d+)?$`).FindStringIndex(body)
	require.NotNil(t, rppGet, "bb-rpp-get-install is not called in the phase")
	require.Less(t, rppGet[0], network, "the provider's network script runs before rpp-get is on the node")

	packages := regexp.MustCompile(`(?m)^/opt/deckhouse/bin/rpp-get install `).FindStringIndex(body)
	require.NotNil(t, packages, "rpp-get install is not called in the phase")
	require.Less(t, network, packages[0], "packages are installed before the provider's network script")
}

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
)

func TestPrepareNodeGroupConfigurationSteps(t *testing.T) {
	templateController := NewTemplateController("")
	t.Cleanup(templateController.Close)

	resourcesYAML := `
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: d8-early-node-bootstrap-internal.sh
spec:
  weight: 15
  content: |
    echo {{ .nodeGroup.name }} {{ .clusterBootstrap.clusterDomain }}
  nodeGroups:
    - "*"
  bundles:
    - "*"
`

	templateData := map[string]interface{}{
		"nodeGroup": map[string]interface{}{
			"name": "master",
		},
		"clusterBootstrap": map[string]interface{}{
			"clusterDomain": "cluster.local",
		},
	}

	err := prepareNodeGroupConfigurationSteps(context.Background(), templateController, resourcesYAML, templateData)
	require.NoError(t, err)

	stepsPath := filepath.Join(templateController.TmpDir, stepsDir)
	content, err := os.ReadFile(filepath.Join(stepsPath, "015_d8-early-node-bootstrap-internal.sh"))
	require.NoError(t, err)
	require.Contains(t, string(content), "echo master cluster.local")
	require.NotContains(t, string(content), "bb-is-bundle")
}

func TestPrepareNodeGroupConfigurationSteps_NoNGC(t *testing.T) {
	templateController := NewTemplateController("")
	t.Cleanup(templateController.Close)

	err := prepareNodeGroupConfigurationSteps(context.Background(), templateController, "", map[string]interface{}{})
	require.NoError(t, err)

	stepsPath := filepath.Join(templateController.TmpDir, stepsDir)
	_, err = os.Stat(stepsPath)
	require.True(t, os.IsNotExist(err))
}

func TestPrepareNodeGroupConfigurationSteps_DefaultWeight(t *testing.T) {
	templateController := NewTemplateController("")
	t.Cleanup(templateController.Close)

	resourcesYAML := `
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: d8-early-node-bootstrap-internal.sh
spec:
  content: |
    echo hi
  nodeGroups:
    - master
  bundles:
    - "*"
`

	err := prepareNodeGroupConfigurationSteps(context.Background(), templateController, resourcesYAML, map[string]interface{}{})
	require.NoError(t, err)

	stepsPath := filepath.Join(templateController.TmpDir, stepsDir)
	_, err = os.Stat(filepath.Join(stepsPath, "100_d8-early-node-bootstrap-internal.sh"))
	require.NoError(t, err)
}

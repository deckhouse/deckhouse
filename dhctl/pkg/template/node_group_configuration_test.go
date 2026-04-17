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
    - ubuntu-lts
    - centos
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: user-master-bundles
spec:
  weight: 15
  content: |
    echo {{ .clusterBootstrap.clusterDomain }}
  nodeGroups:
    - master
  bundles:
    - ubuntu-lts
    - centos
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: worker-only
spec:
  weight: 5
  content: |
    echo worker
  nodeGroups:
    - worker
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
	masterStep, err := os.ReadFile(filepath.Join(stepsPath, "015_d8-early-node-bootstrap-internal.sh"))
	require.NoError(t, err)
	require.Contains(t, string(masterStep), "Auto-generated NGC header start")
	require.Contains(t, string(masterStep), "ubuntu-lts|centos")
	require.Contains(t, string(masterStep), "echo master cluster.local")

	_, err = os.Stat(filepath.Join(stepsPath, "015_user-master-bundles"))
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))

	_, err = os.Stat(filepath.Join(stepsPath, "005_worker-only"))
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
}

func TestPrepareNodeGroupConfigurationSteps_SkipsExistingStepConflict(t *testing.T) {
	templateController := NewTemplateController("")
	t.Cleanup(templateController.Close)

	stepsPath := filepath.Join(templateController.TmpDir, stepsDir)
	err := os.MkdirAll(stepsPath, 0o755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(stepsPath, "100_d8-early-node-bootstrap-internal.sh"), []byte("existing step"), 0o600)
	require.NoError(t, err)

	resourcesYAML := `
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: d8-early-node-bootstrap-internal.sh
spec:
  content: |
    echo conflict
  nodeGroups:
    - master
  bundles:
    - "*"
`

	err = prepareNodeGroupConfigurationSteps(context.Background(), templateController, resourcesYAML, map[string]interface{}{})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(stepsPath, "100_d8-early-node-bootstrap-internal.sh"))
	require.NoError(t, err)
	require.Equal(t, "existing step", string(content))
}

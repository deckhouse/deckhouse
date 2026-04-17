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
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseNodeGroupConfigurations(t *testing.T) {
	resourcesYAML := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key: value
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: master-settings
spec:
  content: |
    echo master
  nodeGroups:
    - master
  bundles:
    - ubuntu-lts
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudEphemeral
`

	nodeGroupConfigurations, err := ParseNodeGroupConfigurations(context.Background(), resourcesYAML)
	require.NoError(t, err)

	require.Len(t, nodeGroupConfigurations, 1)
	require.Equal(t, "master-settings", nodeGroupConfigurations[0].Name)
	require.Nil(t, nodeGroupConfigurations[0].Spec.Weight)
	require.Equal(t, []string{"master"}, nodeGroupConfigurations[0].Spec.NodeGroups)
	require.Equal(t, []string{"ubuntu-lts"}, nodeGroupConfigurations[0].Spec.Bundles)
}

func TestParseNodeGroupConfigurations_ValidatesRequiredFields(t *testing.T) {
	resourcesYAML := `
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: invalid
spec:
  content: |
    echo invalid
  nodeGroups:
    - master
`

	_, err := ParseNodeGroupConfigurations(context.Background(), resourcesYAML)
	require.ErrorContains(t, err, "spec.bundles")
}

func TestParseNodeGroupConfigurations_PreservesExplicitZeroWeight(t *testing.T) {
	resourcesYAML := `
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: zero-weight
spec:
  weight: 0
  content: |
    echo zero
  nodeGroups:
    - master
  bundles:
    - "*"
`

	nodeGroupConfigurations, err := ParseNodeGroupConfigurations(context.Background(), resourcesYAML)
	require.NoError(t, err)
	require.Len(t, nodeGroupConfigurations, 1)
	require.NotNil(t, nodeGroupConfigurations[0].Spec.Weight)
	require.Equal(t, 0, *nodeGroupConfigurations[0].Spec.Weight)
}

func TestParseNodeGroupConfigurations_IgnoresTemplatedNonNodeGroupConfigurationDocuments(t *testing.T) {
	resourcesYAML := `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: system
spec:
  cloudInstances:
    additionalSubnets:
      - '{{ index .cloudDiscovery.zoneToSubnetIdMap "ru-central1-a" }}'
`

	nodeGroupConfigurations, err := ParseNodeGroupConfigurations(context.Background(), resourcesYAML)
	require.NoError(t, err)
	require.Empty(t, nodeGroupConfigurations)
}

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

func TestParseInternalBootstrapNodeGroupConfiguration(t *testing.T) {
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
  name: some-other-ngc
spec:
  content: |
    echo other
  nodeGroups:
    - master
  bundles:
    - ubuntu-lts
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: d8-early-node-bootstrap-internal.sh
spec:
  weight: 15
  content: |
    echo master
  nodeGroups:
    - master
  bundles:
    - "*"
`

	ngc, err := ParseInternalBootstrapNodeGroupConfiguration(context.Background(), resourcesYAML)
	require.NoError(t, err)
	require.NotNil(t, ngc)
	require.Equal(t, InternalBootstrapNodeGroupConfigurationName, ngc.Name)
	require.NotNil(t, ngc.Spec.Weight)
	require.Equal(t, 15, *ngc.Spec.Weight)
}

func TestParseInternalBootstrapNodeGroupConfiguration_NotPresent(t *testing.T) {
	resourcesYAML := `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudEphemeral
`

	ngc, err := ParseInternalBootstrapNodeGroupConfiguration(context.Background(), resourcesYAML)
	require.NoError(t, err)
	require.Nil(t, ngc)
}

func TestParseInternalBootstrapNodeGroupConfiguration_RejectsEmptyContent(t *testing.T) {
	resourcesYAML := `
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: d8-early-node-bootstrap-internal.sh
spec:
  content: ""
  nodeGroups:
    - master
  bundles:
    - "*"
`

	_, err := ParseInternalBootstrapNodeGroupConfiguration(context.Background(), resourcesYAML)
	require.ErrorContains(t, err, "spec.content is required")
}

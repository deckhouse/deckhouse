// Copyright 2024 Flant JSC
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
	"testing"

	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/stretchr/testify/require"
)

func TestValidateClusterSettingsChanges(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		phase       phases.OperationPhase
		oldConfig   string
		newConfig   string
		schema      *SchemaStore
		errContains string
	}{
		"ok, no changes": {
			phase: phases.FinalizationPhase,
			oldConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
masterNodeGroup:
  replicas: 1
  instanceClass:
    imageID: foo`,
			newConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
masterNodeGroup:
  replicas: 1
  instanceClass:
    imageID: foo`,
			schema: testSchemaStore(t),
		},
		"ok, no unsafe changes": {
			phase: phases.FinalizationPhase,
			oldConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
masterNodeGroup:
  replicas: 1`,
			newConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
masterNodeGroup:
  replicas: 2`,
			schema: testSchemaStore(t),
		},
		"ok, unsafe change in BaseInfra phase": {
			phase: phases.BaseInfraPhase,
			oldConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
masterNodeGroup:
  replicas: 1`,
			newConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
masterNodeGroup:
  replicas: 1`,
			schema: testSchemaStore(t),
		},
		"ok, schema not found": {
			phase: phases.FinalizationPhase,
			oldConfig: `
apiVersion: deckhouse.io/v1alpha1
kind: SomeKind
metadata:
  name: foo
spec:
  enabled: true`,
			newConfig: `
apiVersion: deckhouse.io/v1alpha1
kind: SomeKind
metadata:
  name: bar
spec:
  enabled: true`,
			schema: testSchemaStore(t),
		},
		"failed: invalid config": {
			phase: phases.FinalizationPhase,
			oldConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration`,
			newConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration`,
			schema:      testSchemaStore(t),
			errContains: `.clusterType is required`,
		},
		"unsafe field changed": {
			phase: phases.FinalizationPhase,
			oldConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
masterNodeGroup:
  replicas: 1`,
			newConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
masterNodeGroup:
  replicas: 1`,
			schema:      testSchemaStore(t),
			errContains: `"clusterType": unsafe field has been changed`,
		},
		"unsafe rule, ok: updateReplicas": {
			phase: phases.FinalizationPhase,
			oldConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
masterNodeGroup:
  replicas: 1`,
			newConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
masterNodeGroup:
  replicas: 3`,
			schema: testSchemaStore(t),
		},
		"unsafe rule, failed: updateReplicas": {
			phase: phases.FinalizationPhase,
			oldConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
masterNodeGroup:
  replicas: 3`,
			newConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
masterNodeGroup:
  replicas: 1`,
			schema:      testSchemaStore(t),
			errContains: `the new masterNodeGroup.replicas value (1) cannot be less that than 2 (3)`,
		},
		"unsafe rule, ok: deleteZones": {
			phase: phases.FinalizationPhase,
			oldConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
zones: [ru-central1, ru-central2]
masterNodeGroup:
  replicas: 3`,
			newConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
zones: [ru-central1]
masterNodeGroup:
  replicas: 3`,
			schema: testSchemaStore(t),
		},
		"unsafe rule, failed: deleteZones": {
			phase: phases.FinalizationPhase,
			oldConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
zones: [ru-central1, ru-central2]
masterNodeGroup:
  replicas: 1`,
			newConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
zones: [ru-central1]
masterNodeGroup:
  replicas: 1`,
			schema:      testSchemaStore(t),
			errContains: "can't delete zone if masterNodeGroup.replicas < 3",
		},
		"unsafe rule, ok: updateMasterImage": {
			phase: phases.FinalizationPhase,
			oldConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
masterNodeGroup:
  replicas: 3
  instanceClass:
    imageID: foo`,
			newConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
masterNodeGroup:
  replicas: 3
  instanceClass:
    imageID: bar`,
			schema: testSchemaStore(t),
		},
		"unsafe rule, failed: updateMasterImage": {
			phase: phases.FinalizationPhase,
			oldConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
masterNodeGroup:
  replicas: 1
  instanceClass:
    imageID: foo`,
			newConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
masterNodeGroup:
  replicas: 1
  instanceClass:
    imageID: bar`,
			schema:      testSchemaStore(t),
			errContains: "can't update masterNodeGroup.imageID if masterNodeGroup.replicas == 1",
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := ValidateClusterSettingsChanges(tt.phase, tt.oldConfig, tt.newConfig, tt.schema, validateOpts...)
			if tt.errContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.errContains)
			}
		})
	}
}

func testSchemaStore(t *testing.T) *SchemaStore {
	schemaStore := newSchemaStore([]string{"/tmp"})

	schema := []byte(`
kind: ClusterConfiguration
apiVersions:
- apiVersion: deckhouse.io/v1
  openAPISpec:
    type: object
    additionalProperties: false
    required: [clusterType]
    x-unsafe-rules: [deleteZones]
    properties:
      apiVersion:
        type: string
        enum: [deckhouse.io/v1, deckhouse.io/v1alpha1]
      kind:
        type: string
        enum: [ClusterConfiguration]
      clusterType:
        type: string
        x-unsafe: true
        enum: [Cloud, Static]
      zones:
        type: array
        items:
          type: string
        minItems: 1
        uniqueItems: true
      masterNodeGroup:
        type: object
        additionalProperties: false
        x-unsafe-rules: [updateMasterImage]
        properties:
          replicas:
            type: integer
            x-unsafe-rules: [updateReplicas]
          instanceClass:
            type: object
            properties:
              imageID:
                type: string
`)

	require.NoError(t, schemaStore.upload(schema))
	return schemaStore
}

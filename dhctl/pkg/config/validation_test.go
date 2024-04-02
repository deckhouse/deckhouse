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

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
)

func TestValidateClusterSettingsFormat(t *testing.T) {
	once.Do(func() {
		store = newSchemaStore([]string{"./../../../candi/openapi"})
	})

	t.Run("ok", func(t *testing.T) {
		t.Run("cluster configuration", func(t *testing.T) {
			err := ValidateClusterSettingsFormat(clusterConfigFormat, validateOpts)
			require.NoError(t, err)
		})
		t.Run("resource", func(t *testing.T) {
			err := ValidateClusterSettingsFormat(resourceFormat, validateOpts)
			require.NoError(t, err)
		})
		t.Run("cluster configuration with resource", func(t *testing.T) {
			err := ValidateClusterSettingsFormat(clusterConfigWithResourcesFormat, validateOpts)
			require.NoError(t, err)
		})
	})

	t.Run("not ok", func(t *testing.T) {
		t.Run("unexpected field", func(t *testing.T) {
			err := ValidateClusterSettingsFormat(unknownFieldFormat, validateOpts)
			require.Error(t, err)
		})
	})
}

func TestValidateClusterSettingsChanges(t *testing.T) {
	err := loadTestSchemaStore()
	require.NoError(t, err)

	t.Run("ok", func(t *testing.T) {
		t.Run("cluster configuration", func(t *testing.T) {
			err = ValidateClusterSettingsChanges(phases.FinalizationPhase, oldSettings, newSettings, validateOpts)
			require.NoError(t, err)
		})

		t.Run("base infra phase", func(t *testing.T) {
			err = ValidateClusterSettingsChanges(phases.BaseInfraPhase, oldSettings, unsafeNewSettings, validateOpts)
			require.NoError(t, err)
		})

		t.Run("non-config resources without changes", func(t *testing.T) {
			err = ValidateClusterSettingsChanges(phases.FinalizationPhase, oldResourceSettings, oldResourceSettings, validateOpts)
			require.NoError(t, err)
		})

		t.Run("non-config resources with changes", func(t *testing.T) {
			err = ValidateClusterSettingsChanges(phases.FinalizationPhase, oldResourceSettings, newResourceSettings, validateOpts)
			require.NoError(t, err)
		})
	})

	t.Run("not ok", func(t *testing.T) {
		t.Run("unsafe field changed", func(t *testing.T) {
			err = ValidateClusterSettingsChanges(phases.FinalizationPhase, oldSettings, unsafeNewSettings, validateOpts)
			require.ErrorIs(t, err, ErrUnsafeFieldChanged)
		})

		t.Run("without expected config", func(t *testing.T) {
			err = ValidateClusterSettingsChanges(phases.FinalizationPhase, oldSettings, oldResourceSettings, validateOpts)
			require.ErrorIs(t, err, ErrConfigAmountChanged)
		})

		t.Run("invalid document format", func(t *testing.T) {
			err = ValidateClusterSettingsChanges(phases.FinalizationPhase, oldSettings, invalidSchemaSettings, validateOpts)
			require.Error(t, err)
		})
	})
}

func TestValidateRulesClusterSettingsChanges(t *testing.T) {
	err := loadTestRulesSchemaStore()
	require.NoError(t, err)

	t.Run("ok", func(t *testing.T) {
		t.Run("x-unsafe-rules validation", func(t *testing.T) {
			err = ValidateClusterSettingsChanges(
				phases.FinalizationPhase,
				validateRuleSettingsOK,
				validateRuleSettingsOK1,
				validateOpts,
			)
			require.NoError(t, err)
		})
	})

	t.Run("not ok", func(t *testing.T) {
		t.Run("x-unsafe-rules updateReplicas validation failed, 0 replicas", func(t *testing.T) {
			err = ValidateClusterSettingsChanges(
				phases.FinalizationPhase,
				validateRuleSettingsOK1,
				validateRuleSettingsUpdateReplicasInvalid1,
				validateOpts,
			)
			require.ErrorIs(t, err, ErrValidationRuleFailed)
		})

		t.Run("x-unsafe-rules updateReplicas validation failed, less than 2 replicas", func(t *testing.T) {
			err = ValidateClusterSettingsChanges(
				phases.FinalizationPhase,
				validateRuleSettingsOK1,
				validateRuleSettingsUpdateReplicasInvalid2,
				validateOpts,
			)
			require.ErrorIs(t, err, ErrValidationRuleFailed)
		})

		t.Run("x-unsafe-rules deleteZones validation failed", func(t *testing.T) {
			err = ValidateClusterSettingsChanges(
				phases.FinalizationPhase,
				validateRuleSettingsOK,
				validateRuleSettingsDeleteZonesInvalid,
				validateOpts,
			)
			require.ErrorIs(t, err, ErrValidationRuleFailed)
		})
	})
}

var validateOpts = ValidateOptions{CommanderMode: true}

var (
	clusterConfigFormat = `---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: Yandex
  prefix: "cmdr-test-03051973"
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "cluster.local"`
	resourceFormat = `---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse-admin
spec:
  enabled: true`
	clusterConfigWithResourcesFormat = clusterConfigFormat + "\n" + resourceFormat
	unknownFieldFormat               = `---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: Yandex
  prefix: "cmdr-test-03051973"
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "cluster.local"
unexpected: "fail"`
)

var (
	oldSettings = `---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
clusterDomain: old-domain
cloud:
  prefix: safe`
	newSettings = `---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
clusterDomain: new-domain
cloud:
  prefix: safe`
	unsafeNewSettings = `---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
cloud:
  prefix: unsafe`
	invalidSchemaSettings = `---
apiVersion: deckhouse.io/v1
cloud:
prefix: bar`
	oldResourceSettings = `---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: foo
spec:
  enabled: true`
	newResourceSettings = `---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: bar
spec:
  enabled: true`
	validateRuleSettingsOK = `---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
zones: [ru-central1, ru-central2]
masterNodeGroup:
  replicas: 1`
	validateRuleSettingsOK1 = `---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
zones: [ru-central1, ru-central2, ru-central3]
masterNodeGroup:
  replicas: 3`
	validateRuleSettingsUpdateReplicasInvalid1 = `---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
zones: [ru-central1, ru-central2]
masterNodeGroup:
  replicas: 0`
	validateRuleSettingsUpdateReplicasInvalid2 = `---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
zones: [ru-central1, ru-central2]
masterNodeGroup:
  replicas: 1`
	validateRuleSettingsDeleteZonesInvalid = `---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
zones: [ru-central2]
masterNodeGroup:
  replicas: 1`
)

func loadTestSchemaStore() error {
	once.Do(func() {
		store = newSchemaStore([]string{"/tmp"})
	})

	schema := []byte(`
kind: ClusterConfiguration
apiVersions:
- apiVersion: deckhouse.io/v1
  openAPISpec:
    type: object
    additionalProperties: false
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
      clusterDomain:
        type: string
      cloud:
        type: object
        x-unsafe: true
        additionalProperties: false
        properties:
          prefix:
            type: string
            pattern: '^[a-z0-9]([-a-z0-9]*[a-z0-9])?$'
`)

	return store.upload(schema)
}

func loadTestRulesSchemaStore() error {
	once.Do(func() {
		store = newSchemaStore([]string{"/tmp"})
	})

	schema := []byte(`
kind: ClusterConfiguration
apiVersions:
- apiVersion: deckhouse.io/v1
  openAPISpec:
    type: object
    additionalProperties: false
    x-unsafe-rules: [deleteZones]
    properties:
      apiVersion:
        type: string
        enum: [deckhouse.io/v1, deckhouse.io/v1alpha1]
      kind:
        type: string
        enum: [ClusterConfiguration]
      zones:
        type: array
        items:
          type: string
        minItems: 1
        uniqueItems: true
      masterNodeGroup:
        type: object
        additionalProperties: false
        properties:
          replicas:
            type: integer
            x-unsafe-rules: [updateReplicas]
`)

	return store.upload(schema)
}

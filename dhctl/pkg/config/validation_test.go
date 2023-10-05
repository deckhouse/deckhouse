// Copyright 2021 Flant JSC
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
			err := ValidateClusterSettingsFormat(clusterConfigFormat)
			require.NoError(t, err)
		})
		t.Run("resource", func(t *testing.T) {
			err := ValidateClusterSettingsFormat(resourceFormat)
			require.NoError(t, err)
		})
		t.Run("cluster configuration with resource", func(t *testing.T) {
			err := ValidateClusterSettingsFormat(clusterConfigWithResourcesFormat)
			require.NoError(t, err)
		})
		t.Run("unexpected field", func(t *testing.T) {
			err := ValidateClusterSettingsFormat(unknownFieldFormat)
			require.Error(t, err)
		})
	})
}

func TestValidateClusterSettingsChanges(t *testing.T) {
	err := loadTestSchemaStore()
	require.NoError(t, err)

	t.Run("ok", func(t *testing.T) {
		err = ValidateClusterSettingsChanges(phases.FinalizationPhase, oldSettings, newSettings)
		require.NoError(t, err)
	})

	t.Run("ok: base infra phase", func(t *testing.T) {
		err = ValidateClusterSettingsChanges(phases.BaseInfraPhase, oldSettings, unsafeNewSettings)
		require.NoError(t, err)
	})

	t.Run("unsafe field changed", func(t *testing.T) {
		err = ValidateClusterSettingsChanges(phases.FinalizationPhase, oldSettings, unsafeNewSettings)
		require.ErrorIs(t, err, ErrUnsafeFieldChanged)
	})

	t.Run("without expected config", func(t *testing.T) {
		err = ValidateClusterSettingsChanges(phases.FinalizationPhase, oldSettings, oldResourceSettings)
		require.ErrorIs(t, err, ErrConfigAmountChanged)
	})

	t.Run("invalid document format", func(t *testing.T) {
		err = ValidateClusterSettingsChanges(phases.FinalizationPhase, oldSettings, invalidSchemaSettings)
		require.Error(t, err)
	})

	t.Run("non-config resources without changes", func(t *testing.T) {
		err = ValidateClusterSettingsChanges(phases.FinalizationPhase, oldResourceSettings, oldResourceSettings)
		require.NoError(t, err)
	})

	t.Run("non-config resources with changes", func(t *testing.T) {
		err = ValidateClusterSettingsChanges(phases.FinalizationPhase, oldResourceSettings, newResourceSettings)
		require.NoError(t, err)
	})

	t.Run("custom rule validation failed", func(t *testing.T) {
		err = ValidateClusterSettingsChanges(phases.FinalizationPhase,
			customValidationSettingsOld,
			customValidationSettingsNew,
			NewRuleValidationOption("testEmbeddedValidation.testValidation", NotLessRule),
		)
		require.ErrorIs(t, err, ErrValidationRuleFailed)
	})
}

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
	customValidationSettingsOld = `---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
clusterDomain: old-domain
testEmbeddedValidation:
  testValidation: 222`
	customValidationSettingsNew = `---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
clusterDomain: new-domain
testEmbeddedValidation:
  testValidation: 111`
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
      testEmbeddedValidation:
        type: object
        additionalProperties: false
        properties:
          testValidation:
            type: number
`)

	return store.upload(schema)
}

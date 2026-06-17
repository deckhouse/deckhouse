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

package infrastructureprovider

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

func TestInstanceClassValidator(t *testing.T) {
	metaConfig, err := config.ParseConfigFromData(
		context.TODO(),
		`
---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "Automatic"
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
clusterDomain: cluster.local
---
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: system
spec:
  cores: 4
  memory: 8192
  imageID: image-id
---
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: system-v1
spec:
  cores: 2
  memory: 4096
  imageID: image-id
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-release-data
`,
		MetaConfigPreparatorProvider(NewPreparatorProviderParamsWithoutLogger()),
		nil,
	)
	require.NoError(t, err)
	metaConfig.ProviderName = "yandex"

	validator := NewInstanceClassValidator(metaConfig)

	require.Equal(t, "yandex", validator.ProviderName())

	instanceClasses, err := validator.InstanceClasses()
	require.NoError(t, err)
	require.Len(t, instanceClasses, 2)

	names := make([]string, 0, len(instanceClasses))
	apiVersions := make([]string, 0, len(instanceClasses))
	for _, resource := range instanceClasses {
		require.Equal(t, "YandexInstanceClass", resource.GetKind())
		names = append(names, resource.GetName())
		apiVersions = append(apiVersions, resource.GetAPIVersion())
	}

	slices.Sort(names)
	slices.Sort(apiVersions)

	require.Equal(t, []string{"system", "system-v1"}, names)
	require.Equal(t, []string{"deckhouse.io/v1", "deckhouse.io/v1"}, apiVersions)
}

func TestInstanceClassValidatorInstanceClassesFiltersResources(t *testing.T) {
	validator := NewInstanceClassValidator(&config.MetaConfig{
		ProviderName: "yandex",
		ResourcesYAML: input.CombineYAMLs(
			`
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: system
`,
			`
apiVersion: deckhouse.io/v1
kind: AWSInstanceClass
metadata:
  name: workers
`,
			`
apiVersion: deckhouse.io/v1alpha1
kind: AWSInstanceClass
metadata:
  name: workers-v1
`,
			`
apiVersion: deckhouse.io/v1
kind: InstanceClass
metadata:
  name: cloud-instance-class
`,
			`
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-release-data
`,
		),
	})

	instanceClasses, err := validator.InstanceClasses()
	require.NoError(t, err)
	require.Len(t, instanceClasses, 3)

	names := make([]string, 0, len(instanceClasses))
	kinds := make([]string, 0, len(instanceClasses))
	apiVersions := make([]string, 0, len(instanceClasses))
	for _, resource := range instanceClasses {
		names = append(names, resource.GetName())
		kinds = append(kinds, resource.GetKind())
		apiVersions = append(apiVersions, resource.GetAPIVersion())
	}

	slices.Sort(names)
	slices.Sort(kinds)
	slices.Sort(apiVersions)

	require.Equal(t, []string{"system", "workers", "workers-v1"}, names)
	require.Equal(t, []string{"AWSInstanceClass", "AWSInstanceClass", "YandexInstanceClass"}, kinds)
	require.Equal(t, []string{"deckhouse.io/v1", "deckhouse.io/v1", "deckhouse.io/v1alpha1"}, apiVersions)
}

func TestInstanceClassValidatorInstanceClassesEmptyResources(t *testing.T) {
	validator := NewInstanceClassValidator(&config.MetaConfig{
		ProviderName: "yandex",
	})

	instanceClasses, err := validator.InstanceClasses()
	require.NoError(t, err)
	require.Nil(t, instanceClasses)
}

func TestInstanceClassValidatorInstanceClassesInvalidYAML(t *testing.T) {
	validator := NewInstanceClassValidator(&config.MetaConfig{
		ResourcesYAML: `
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: system
  labels: [
`,
	})

	instanceClasses, err := validator.InstanceClasses()
	require.Error(t, err)
	require.Nil(t, instanceClasses)
	require.ErrorContains(t, err, "parse resources document 0 index")
}

func TestInstanceClassValidatorWithNilMetaConfig(t *testing.T) {
	validator := NewInstanceClassValidator(nil)

	require.Equal(t, "", validator.ProviderName())

	instanceClasses, err := validator.InstanceClasses()
	require.Error(t, err)
	require.Nil(t, instanceClasses)
	require.ErrorContains(t, err, "metaConfig must not be nil")
}

func TestInstanceClassValidatorValidateProviderInstanceClasses(t *testing.T) {
	validator := NewInstanceClassValidator(&config.MetaConfig{
		ProviderName: "yandex",
		ResourcesYAML: input.CombineYAMLs(
			`
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: system
`,
			`
apiVersion: deckhouse.io/v1
kind: AWSInstanceClass
metadata:
  name: workers
`,
		),
	})

	err := validator.ValidateProviderInstanceClasses()
	require.Error(t, err)
	require.ErrorContains(t, err, `instance class "AWSInstanceClass" does not match provider "yandex"`)
}

func TestInstanceClassValidatorValidateProviderInstanceClassesMismatch(t *testing.T) {
	validator := NewInstanceClassValidator(&config.MetaConfig{
		ProviderName: "yandex",
		ResourcesYAML: input.CombineYAMLs(
			`
apiVersion: deckhouse.io/v1
kind: AWSInstanceClass
metadata:
  name: workers
`,
		),
	})

	err := validator.ValidateProviderInstanceClasses()
	require.Error(t, err)
	require.ErrorContains(t, err, `instance class "AWSInstanceClass" does not match provider "yandex"`)
}

func TestInstanceClassValidatorValidateProviderInstanceClassesMatch(t *testing.T) {
	validator := NewInstanceClassValidator(&config.MetaConfig{
		ProviderName: "aws",
		ResourcesYAML: input.CombineYAMLs(
			`
apiVersion: deckhouse.io/v1
kind: AWSInstanceClass
metadata:
  name: workers
`,
		),
	})

	err := validator.ValidateProviderInstanceClasses()
	require.NoError(t, err)
}

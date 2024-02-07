/*
Copyright 2022 Flant JSC

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

package capacity

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
)

func TestCapacityExtractor(t *testing.T) {
	t.Run("VsphereSpec", func(t *testing.T) {
		t.Parallel()
		var instanceClass map[string]interface{}
		err := yaml.Unmarshal([]byte(vsphereSpec), &instanceClass)
		require.NoError(t, err)
		catalog := NewInstanceTypesCatalog(make([]v1alpha1.InstanceType, 0))
		capac, err := CalculateNodeTemplateCapacity(instanceClass["kind"].(string), instanceClass["spec"], catalog)
		require.NoError(t, err)
		assert.Equal(t, "10Gi", capac.Memory.String())
		assert.Equal(t, "4", capac.CPU.String())
	})

	t.Run("YandexSpec", func(t *testing.T) {
		t.Parallel()
		var instanceClass map[string]interface{}
		err := yaml.Unmarshal([]byte(yandexSpec), &instanceClass)
		require.NoError(t, err)
		catalog := NewInstanceTypesCatalog(make([]v1alpha1.InstanceType, 0))
		capac, err := CalculateNodeTemplateCapacity(instanceClass["kind"].(string), instanceClass["spec"], catalog)
		require.NoError(t, err)
		assert.Equal(t, "16Gi", capac.Memory.String())
		assert.Equal(t, "4", capac.CPU.String())
	})

	t.Run("AWSSpec", func(t *testing.T) {
		t.Parallel()
		var instanceClass map[string]interface{}
		err := yaml.Unmarshal([]byte(awsSpec), &instanceClass)
		require.NoError(t, err)
		catalog := NewInstanceTypesCatalog([]v1alpha1.InstanceType{
			{
				Name:   "t3a.xlarge",
				CPU:    resource.MustParse("4"),
				Memory: resource.MustParse(strconv.FormatInt(16384, 10) + "Mi"),
			},
			{
				Name:   "t4g.2xlarge",
				CPU:    resource.MustParse("8"),
				Memory: resource.MustParse(strconv.FormatInt(32768, 10) + "Mi"),
			},
		})
		capac, err := CalculateNodeTemplateCapacity(instanceClass["kind"].(string), instanceClass["spec"], catalog)
		require.NoError(t, err)
		assert.Equal(t, "16Gi", capac.Memory.String())
		assert.Equal(t, "4", capac.CPU.String())
	})

	t.Run("AWSSpecWithCapacity", func(t *testing.T) {
		t.Parallel()
		var instanceClass map[string]interface{}
		err := yaml.Unmarshal([]byte(awsSpecWithCapacity), &instanceClass)
		require.NoError(t, err)
		catalog := NewInstanceTypesCatalog(make([]v1alpha1.InstanceType, 0))
		capac, err := CalculateNodeTemplateCapacity(instanceClass["kind"].(string), instanceClass["spec"], catalog)
		require.NoError(t, err)
		assert.Equal(t, "32Gi", capac.Memory.String())
		assert.Equal(t, "2", capac.CPU.String())
	})

	t.Run("GCPSpec", func(t *testing.T) {
		t.Parallel()
		var instanceClass map[string]interface{}
		err := yaml.Unmarshal([]byte(gcpSpec), &instanceClass)
		require.NoError(t, err)
		catalog := NewInstanceTypesCatalog([]v1alpha1.InstanceType{
			{
				Name:   "t2a-standard-2",
				CPU:    resource.MustParse("2"),
				Memory: resource.MustParse(strconv.FormatInt(8192, 10) + "Mi"),
			},
			{
				Name:   "n2d-standard-4",
				CPU:    resource.MustParse("4"),
				Memory: resource.MustParse(strconv.FormatInt(16384, 10) + "Mi"),
			},
		})
		capac, err := CalculateNodeTemplateCapacity(instanceClass["kind"].(string), instanceClass["spec"], catalog)
		require.NoError(t, err)
		assert.Equal(t, "16Gi", capac.Memory.String())
		assert.Equal(t, "4", capac.CPU.String())
	})

	t.Run("GCPSpecWithCapacity", func(t *testing.T) {
		t.Parallel()
		var instanceClass map[string]interface{}
		err := yaml.Unmarshal([]byte(gcpSpecWithCapacity), &instanceClass)
		require.NoError(t, err)
		catalog := NewInstanceTypesCatalog(make([]v1alpha1.InstanceType, 0))
		capac, err := CalculateNodeTemplateCapacity(instanceClass["kind"].(string), instanceClass["spec"], catalog)
		require.NoError(t, err)
		assert.Equal(t, "64Gi", capac.Memory.String())
		assert.Equal(t, "8", capac.CPU.String())
	})

	t.Run("AzureSpec", func(t *testing.T) {
		t.Parallel()
		var instanceClass map[string]interface{}
		err := yaml.Unmarshal([]byte(azureSpec), &instanceClass)
		require.NoError(t, err)
		catalog := NewInstanceTypesCatalog([]v1alpha1.InstanceType{
			{
				Name:   "Standard_F4",
				CPU:    resource.MustParse("4"),
				Memory: resource.MustParse(strconv.FormatInt(8192, 10) + "Mi"),
			},
			{
				Name:   "Standard_F8s",
				CPU:    resource.MustParse("8"),
				Memory: resource.MustParse(strconv.FormatInt(16384, 10) + "Mi"),
			},
		})
		capac, err := CalculateNodeTemplateCapacity(instanceClass["kind"].(string), instanceClass["spec"], catalog)
		require.NoError(t, err)
		assert.Equal(t, "8Gi", capac.Memory.String())
		assert.Equal(t, "4", capac.CPU.String())
	})

	t.Run("AzureSpecWithCapacity", func(t *testing.T) {
		t.Parallel()
		var instanceClass map[string]interface{}
		err := yaml.Unmarshal([]byte(azureSpecWithCapacity), &instanceClass)
		require.NoError(t, err)
		catalog := NewInstanceTypesCatalog(make([]v1alpha1.InstanceType, 0))
		capac, err := CalculateNodeTemplateCapacity(instanceClass["kind"].(string), instanceClass["spec"], catalog)
		require.NoError(t, err)
		assert.Equal(t, "4Gi", capac.Memory.String())
		assert.Equal(t, "8", capac.CPU.String())
	})

	t.Run("OpenstackSpec", func(t *testing.T) {
		t.Parallel()
		var instanceClass map[string]interface{}
		err := yaml.Unmarshal([]byte(openstackSpec), &instanceClass)
		require.NoError(t, err)
		catalog := NewInstanceTypesCatalog([]v1alpha1.InstanceType{
			{
				Name:   "m2.xlarge",
				CPU:    resource.MustParse("8"),
				Memory: resource.MustParse(strconv.FormatInt(16384, 10) + "Mi"),
			},
			{
				Name:   "m1.large",
				CPU:    resource.MustParse("4"),
				Memory: resource.MustParse(strconv.FormatInt(8192, 10) + "Mi"),
			},
		})
		capac, err := CalculateNodeTemplateCapacity(instanceClass["kind"].(string), instanceClass["spec"], catalog)
		require.NoError(t, err)
		assert.Equal(t, "8Gi", capac.Memory.String())
		assert.Equal(t, "4", capac.CPU.String())
	})

	t.Run("OpenstackSpecWithCapacity", func(t *testing.T) {
		t.Parallel()
		var instanceClass map[string]interface{}
		err := yaml.Unmarshal([]byte(openstackSpecWithCapacity), &instanceClass)
		require.NoError(t, err)
		catalog := NewInstanceTypesCatalog(make([]v1alpha1.InstanceType, 0))
		capac, err := CalculateNodeTemplateCapacity(instanceClass["kind"].(string), instanceClass["spec"], catalog)
		require.NoError(t, err)
		assert.Equal(t, "16Gi", capac.Memory.String())
		assert.Equal(t, "8", capac.CPU.String())
	})
}

const (
	vsphereSpec = `
apiVersion: deckhouse.io/v1
kind: VsphereInstanceClass
metadata:
  name: system
spec:
  datastore: 3par_4_Lun101
  mainNetwork: DEVOPS_45
  memory: 10240
  numCPUs: 4
  rootDiskSize: 30
  template: Templates/ubuntu-jammy-22.04
`

	yandexSpec = `
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: system
spec:
  cores: 4
  diskSizeGB: 50
  diskType: network-hdd
  imageID: fd83bj827tp2slnpp7f0
  memory: 16384
  networkType: Standard
  platformID: standard-v2
`

	awsSpec = `
apiVersion: deckhouse.io/v1
kind: AWSInstanceClass
metadata:
  name: system
spec:
  additionalTags:
    cluster: prod
    team: flant
  instanceType: t3a.xlarge
`

	awsSpecWithCapacity = `
apiVersion: deckhouse.io/v1
kind: AWSInstanceClass
metadata:
  name: system
spec:
  additionalTags:
    cluster: prod
    team: flant
  instanceType: custom
  capacity:
    cpu: 2
    memory: "32Gi"
`

	gcpSpec = `
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  name: system
spec:
  diskSizeGb: 30
  machineType: n2d-standard-4
`

	gcpSpecWithCapacity = `
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  name: system
spec:
  diskSizeGb: 30
  machineType: n2d-standard-4
  capacity:
    cpu: "8000m"
    memory: "64Gi"
`

	azureSpec = `
apiVersion: deckhouse.io/v1
kind: AzureInstanceClass
metadata:
  name: example
spec:
  machineSize: Standard_F4
`

	azureSpecWithCapacity = `
apiVersion: deckhouse.io/v1
kind: AzureInstanceClass
metadata:
  name: example
spec:
  machineSize: Standard_F4
  capacity:
    cpu: "8"
    memory: "4096Mi"
`

	openstackSpec = `
apiVersion: deckhouse.io/v1
kind: OpenStackInstanceClass
metadata:
  name: system
spec:
  flavorName: m1.large
  imageName: ubuntu-18-04-cloud-amd64
  mainNetwork: ndev
`

	openstackSpecWithCapacity = `
apiVersion: deckhouse.io/v1
kind: OpenStackInstanceClass
metadata:
  name: system
spec:
  capacity:
    cpu: 8
    memory: "16Gi"
  flavorName: m1.large
  imageName: ubuntu-18-04-cloud-amd64
  mainNetwork: ndev
`
)

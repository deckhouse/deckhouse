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

	t.Run("DVPSpec", func(t *testing.T) {
		t.Parallel()
		var instanceClass map[string]interface{}
		err := yaml.Unmarshal([]byte(dvpSpec), &instanceClass)
		require.NoError(t, err)
		catalog := NewInstanceTypesCatalog(make([]v1alpha1.InstanceType, 0))
		capac, err := CalculateNodeTemplateCapacity(instanceClass["kind"].(string), instanceClass["spec"], catalog)
		require.NoError(t, err)
		assert.Equal(t, "8Gi", capac.Memory.String())
		assert.Equal(t, "4", capac.CPU.String())
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

	dvpSpec = `
apiVersion: deckhouse.io/v1alpha1
kind: DVPInstanceClass
metadata:
  creationTimestamp: "2025-12-30T06:47:54Z"
  generation: 1
  name: worker
  resourceVersion: "9580100"
  uid: 878e51e6-a6e6-4f00-a408-d78a17ff0763
spec:
  rootDisk:
    image:
      kind: ClusterVirtualImage
      name: ubuntu-24-04-lts
    size: 30Gi
    storageClass: replicated
  virtualMachine:
    bootloader: EFI
    cpu:
      coreFraction: 20%
      cores: 4
    liveMigrationPolicy: PreferForced
    memory:
      size: 8Gi
    runPolicy: AlwaysOnUnlessStoppedManually
    virtualMachineClassName: amd-epyc-gen-3
`
)

// The boot disk feeds the CAPI ephemeral-disk annotation. A size the InstanceClass leaves unset
// is the one the provider creates the disk with: its default, or for a catalog type the catalog's
// disk. The capacity itself, rootDisk included, must not change.
func TestBootDisk(t *testing.T) {
	catalog := []v1alpha1.InstanceType{
		// The openstack discoverer publishes the flavor's disk.
		{Name: "m1.large", CPU: resource.MustParse("4"), Memory: resource.MustParse("8Gi"), RootDisk: resource.MustParse("20Gi")},
		{Name: "s7n.xlarge.2", CPU: resource.MustParse("4"), Memory: resource.MustParse("8Gi"), RootDisk: resource.MustParse("0")},
		{Name: "vcd-4-8", CPU: resource.MustParse("4"), Memory: resource.MustParse("8Gi"), RootDisk: resource.MustParse("0Gi")},
		{Name: "t3a.xlarge", CPU: resource.MustParse("4"), Memory: resource.MustParse("16Gi"), RootDisk: resource.MustParse("0")},
	}

	cases := []struct {
		name string
		kind string
		spec string
		want string
	}{
		{"yandex set", "YandexInstanceClass", `{cores: 4, memory: 8192, diskSizeGB: 100}`, "100Gi"},
		{"yandex template default", "YandexInstanceClass", `{cores: 4, memory: 8192}`, "50Gi"},
		{"openstack root volume", "OpenStackInstanceClass", `{flavorName: m1.large, rootDiskSize: 30}`, "30Gi"},
		{"openstack flavor disk", "OpenStackInstanceClass", `{flavorName: m1.large}`, "20Gi"},
		{"openstack explicit capacity without a disk", "OpenStackInstanceClass",
			`{flavorName: custom, capacity: {cpu: 4, memory: 16Gi}}`, "0"},
		{"vcd set", "VCDInstanceClass", `{sizingPolicy: vcd-4-8, rootDiskSizeGb: 40}`, "40Gi"},
		{"vcd unset, defaulted by the CRD in a cluster", "VCDInstanceClass", `{sizingPolicy: vcd-4-8}`, "0"},
		{"zvirt set", "ZvirtInstanceClass", `{cores: 4, memory: 8192, rootDiskSizeGb: 50}`, "50Gi"},
		{"zvirt machine template default", "ZvirtInstanceClass", `{cores: 4, memory: 8192}`, "20Gi"},
		{"dynamix set", "DynamixInstanceClass", `{cores: 4, memory: 8192, rootDiskSizeGb: 50}`, "50Gi"},
		{"dynamix machine template default", "DynamixInstanceClass", `{cores: 4, memory: 8192}`, "30Gi"},
		{"huaweicloud set", "HuaweiCloudInstanceClass", `{flavorName: s7n.xlarge.2, rootDiskSize: 50}`, "50Gi"},
		{"huaweicloud machine template default", "HuaweiCloudInstanceClass", `{flavorName: s7n.xlarge.2}`, "30Gi"},
		{"dvp", "DVPInstanceClass", `{rootDisk: {size: 30Gi}, virtualMachine: {cpu: {cores: 4}, memory: {size: 8Gi}}}`, "30Gi"},
		// Any quantity form the DVPInstanceClass pattern takes comes out in the nodeCapacity one.
		{"dvp decimal size is normalized", "DVPInstanceClass", `{rootDisk: {size: "32212254720"}, virtualMachine: {cpu: {cores: 4}, memory: {size: 8Gi}}}`, "30Gi"},
		{"dvp without a disk", "DVPInstanceClass", `{virtualMachine: {cpu: {cores: 4}, memory: {size: 8Gi}}}`, "0"},
		{"dvp unparsable disk", "DVPInstanceClass", `{rootDisk: {size: thirty}, virtualMachine: {cpu: {cores: 4}, memory: {size: 8Gi}}}`, "0"},
		// MCM only, no CAPI: nothing reads a boot disk there.
		{"aws", "AWSInstanceClass", `{instanceType: t3a.xlarge}`, "0"},
		{"unknown kind", "AcmeInstanceClass", `{}`, "0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var spec map[string]interface{}
			require.NoError(t, yaml.Unmarshal([]byte(tc.spec), &spec))
			got := CalculateBootDisk(tc.kind, spec, NewInstanceTypesCatalog(catalog))
			assert.Equal(t, tc.want, got.String())
		})
	}

	t.Run("the capacity is unchanged", func(t *testing.T) {
		t.Parallel()
		var spec map[string]interface{}
		require.NoError(t, yaml.Unmarshal([]byte(`{flavorName: m1.large, rootDiskSize: 30}`), &spec))
		capac, err := CalculateNodeTemplateCapacity("OpenStackInstanceClass", spec, NewInstanceTypesCatalog(catalog))
		require.NoError(t, err)
		assert.Equal(t, "20Gi", capac.RootDisk.String(), "nodeCapacity.rootDisk keeps the catalog's value")
	})
}

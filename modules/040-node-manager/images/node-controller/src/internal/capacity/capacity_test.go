/*
Copyright 2026 Flant JSC

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
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"
)

func TestCapacityExtractor(t *testing.T) {
	t.Run("VsphereSpec", func(t *testing.T) {
		t.Parallel()
		var instanceClass map[string]interface{}
		err := yaml.Unmarshal([]byte(vsphereSpec), &instanceClass)
		require.NoError(t, err)
		catalog := NewInstanceTypesCatalog(make([]InstanceType, 0))
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
		catalog := NewInstanceTypesCatalog(make([]InstanceType, 0))
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
		catalog := NewInstanceTypesCatalog([]InstanceType{
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
		catalog := NewInstanceTypesCatalog(make([]InstanceType, 0))
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
		catalog := NewInstanceTypesCatalog([]InstanceType{
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
		catalog := NewInstanceTypesCatalog(make([]InstanceType, 0))
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
		catalog := NewInstanceTypesCatalog([]InstanceType{
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
		catalog := NewInstanceTypesCatalog(make([]InstanceType, 0))
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
		catalog := NewInstanceTypesCatalog([]InstanceType{
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
		catalog := NewInstanceTypesCatalog(make([]InstanceType, 0))
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
		catalog := NewInstanceTypesCatalog(make([]InstanceType, 0))
		capac, err := CalculateNodeTemplateCapacity(instanceClass["kind"].(string), instanceClass["spec"], catalog)
		require.NoError(t, err)
		assert.Equal(t, "8Gi", capac.Memory.String())
		assert.Equal(t, "4", capac.CPU.String())
	})

	// An out-of-tree provider node-controller has no case for still works if its
	// InstanceClass publishes an explicit spec.capacity.
	t.Run("OutOfTreeProviderWithExplicitCapacity", func(t *testing.T) {
		t.Parallel()
		var instanceClass map[string]interface{}
		err := yaml.Unmarshal([]byte(outOfTreeSpecWithCapacity), &instanceClass)
		require.NoError(t, err)
		catalog := NewInstanceTypesCatalog(make([]InstanceType, 0))
		capac, err := CalculateNodeTemplateCapacity(instanceClass["kind"].(string), instanceClass["spec"], catalog)
		require.NoError(t, err)
		assert.Equal(t, "16Gi", capac.Memory.String())
		assert.Equal(t, "8", capac.CPU.String())
	})

	// Without a case and without spec.capacity there is nothing to derive capacity from.
	t.Run("UnknownProviderWithoutCapacity", func(t *testing.T) {
		t.Parallel()
		var instanceClass map[string]interface{}
		err := yaml.Unmarshal([]byte(outOfTreeSpecNoCapacity), &instanceClass)
		require.NoError(t, err)
		catalog := NewInstanceTypesCatalog(make([]InstanceType, 0))
		_, err = CalculateNodeTemplateCapacity(instanceClass["kind"].(string), instanceClass["spec"], catalog)
		assert.Error(t, err)
	})
}

// The boot disk feeds the CAPI ephemeral-disk annotation. A size the InstanceClass leaves unset
// is the one the provider creates the disk with: its default, or for a catalog type the catalog's
// disk. RootDisk is published in the NodeGroup element and must keep the value it always had.
func TestBootDisk(t *testing.T) {
	catalog := []InstanceType{
		// The openstack discoverer publishes the flavor's disk.
		{Name: "m1.large", CPU: resource.MustParse("4"), Memory: resource.MustParse("8Gi"), RootDisk: resource.MustParse("20Gi")},
		{Name: "s7n.xlarge.2", CPU: resource.MustParse("4"), Memory: resource.MustParse("8Gi"), RootDisk: resource.MustParse("0")},
		{Name: "vcd-4-8", CPU: resource.MustParse("4"), Memory: resource.MustParse("8Gi"), RootDisk: resource.MustParse("0Gi")},
		{Name: "t3a.xlarge", CPU: resource.MustParse("4"), Memory: resource.MustParse("16Gi"), RootDisk: resource.MustParse("0")},
	}

	cases := []struct {
		name         string
		kind         string
		spec         string
		wantBootDisk string
		wantRootDisk string
	}{
		{"vsphere set", "VsphereInstanceClass", `{numCPUs: 4, memory: 8192, rootDiskSize: 30}`, "30Gi", "0"},
		{"vsphere template default", "VsphereInstanceClass", `{numCPUs: 4, memory: 8192}`, "20Gi", "0"},
		{"yandex set", "YandexInstanceClass", `{cores: 4, memory: 8192, diskSizeGB: 100}`, "100Gi", "0"},
		{"yandex template default", "YandexInstanceClass", `{cores: 4, memory: 8192}`, "50Gi", "0"},
		{"openstack root volume", "OpenStackInstanceClass", `{flavorName: m1.large, rootDiskSize: 30}`, "30Gi", "20Gi"},
		{"openstack flavor disk", "OpenStackInstanceClass", `{flavorName: m1.large}`, "20Gi", "20Gi"},
		{"openstack explicit capacity", "OpenStackInstanceClass",
			`{flavorName: custom, rootDiskSize: 40, capacity: {cpu: 4, memory: 16Gi}}`, "40Gi", "0"},
		{"openstack explicit capacity without a disk", "OpenStackInstanceClass",
			`{flavorName: custom, capacity: {cpu: 4, memory: 16Gi}}`, "0", "0"},
		{"vcd set", "VCDInstanceClass", `{sizingPolicy: vcd-4-8, rootDiskSizeGb: 40}`, "40Gi", "0"},
		{"vcd unset, defaulted by the CRD in a cluster", "VCDInstanceClass", `{sizingPolicy: vcd-4-8}`, "0", "0"},
		{"zvirt set", "ZvirtInstanceClass", `{cores: 4, memory: 8192, rootDiskSizeGb: 50}`, "50Gi", "0"},
		{"zvirt machine template default", "ZvirtInstanceClass", `{cores: 4, memory: 8192}`, "20Gi", "0"},
		{"dynamix set", "DynamixInstanceClass", `{cores: 4, memory: 8192, rootDiskSizeGb: 50}`, "50Gi", "0"},
		{"dynamix machine template default", "DynamixInstanceClass", `{cores: 4, memory: 8192}`, "30Gi", "0"},
		{"huaweicloud set", "HuaweiCloudInstanceClass", `{flavorName: s7n.xlarge.2, rootDiskSize: 50}`, "50Gi", "0"},
		{"huaweicloud machine template default", "HuaweiCloudInstanceClass", `{flavorName: s7n.xlarge.2}`, "30Gi", "0"},
		{"dvp", "DVPInstanceClass",
			`{rootDisk: {size: 30Gi}, virtualMachine: {cpu: {cores: 4}, memory: {size: 8Gi}}}`, "30Gi", "0"},
		{"dvp without a disk", "DVPInstanceClass",
			`{virtualMachine: {cpu: {cores: 4}, memory: {size: 8Gi}}}`, "0", "0"},
		{"dvp unparsable disk keeps cpu and memory", "DVPInstanceClass",
			`{rootDisk: {size: thirty}, virtualMachine: {cpu: {cores: 4}, memory: {size: 8Gi}}}`, "0", "0"},
		// MCM only: the boot disk is not resolved, nothing reads it there.
		{"aws", "AWSInstanceClass", `{instanceType: t3a.xlarge}`, "0", "0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var spec map[string]interface{}
			require.NoError(t, yaml.Unmarshal([]byte(tc.spec), &spec))
			capac, err := CalculateNodeTemplateCapacity(tc.kind, spec, NewInstanceTypesCatalog(catalog))
			require.NoError(t, err)
			assert.Equal(t, tc.wantBootDisk, capac.BootDisk.String())
			assert.Equal(t, tc.wantRootDisk, capac.RootDisk.String(), "the published rootDisk must not change")
			assert.Equal(t, "4", capac.CPU.String(), "the disk must not disturb the CPU")
		})
	}

	t.Run("the catalog entry is not modified", func(t *testing.T) {
		t.Parallel()
		c := NewInstanceTypesCatalog(catalog)
		var spec map[string]interface{}
		require.NoError(t, yaml.Unmarshal([]byte(`{flavorName: m1.large, rootDiskSize: 30}`), &spec))
		_, err := CalculateNodeTemplateCapacity("OpenStackInstanceClass", spec, c)
		require.NoError(t, err)
		entry := c.Types["m1.large"]
		assert.Equal(t, "20Gi", entry.RootDisk.String())
		assert.True(t, entry.BootDisk.IsZero())
	})

	// The NodeGroup element is the JSON form of InstanceType; the boot disk must not reach it.
	t.Run("the boot disk is not serialized", func(t *testing.T) {
		t.Parallel()
		data, err := json.Marshal(InstanceType{
			CPU: resource.MustParse("4"), Memory: resource.MustParse("8Gi"),
			RootDisk: resource.MustParse("0"), BootDisk: resource.MustParse("30Gi"),
		})
		require.NoError(t, err)
		assert.JSONEq(t, `{"name":"","cpu":"4","memory":"8Gi","rootDisk":"0"}`, string(data))
	})
}

const (
	outOfTreeSpecWithCapacity = `
apiVersion: deckhouse.io/v1
kind: AcmeCloudInstanceClass
metadata:
  name: system
spec:
  someProviderField: whatever
  capacity:
    cpu: 8
    memory: 16Gi
`

	outOfTreeSpecNoCapacity = `
apiVersion: deckhouse.io/v1
kind: AcmeCloudInstanceClass
metadata:
  name: system
spec:
  someProviderField: whatever
`
)

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

/*
Copyright 2021 Flant JSC

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

package hooks

import (
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-azure :: hooks :: azure_cluster_configuration ::", func() {
	var providerClusterConfiguration = `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "AzureClusterConfiguration",
  "layout": "Standard",
  "sshPublicKey": "ssh-rsa AAA",
  "vNetCIDR": "10.50.0.0/16",
  "subnetCIDR": "10.50.0.0/24",
  "masterNodeGroup": {
    "replicas": 1,
    "zones": [
      "1",
      "2",
      "3"
    ],
    "instanceClass": {
      "machineSize": "test",
      "urn": "test",
      "diskSizeGb": 50,
      "diskType": "test"
    }
  },
  "provider": {
    "subscriptionId": "test",
    "clientId": "test",
    "clientSecret": "test",
    "tenantId": "test",
    "location": "test"
  }
}
`

	var providerClusterConfigurationBad = `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "AzureClusterConfiguration",
  "layout": "Standard"
}
`

	var providerDiscoveryData = `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "AzureCloudDiscoveryData",
  "resourceGroupName": "test",
  "vnetName": "test",
  "subnetName": "test",
  "zones": ["1", "2", "3"],
  "instances": {
    "urn": "test:test:test",
    "diskType": "test",
    "additionalTags": {}
  }
}
`

	var providerDiscoveryDataBad = `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "AzureCloudDiscoveryData"
}
`

	var secretD8ProviderClusterConfiguration = fmt.Sprintf(`
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(providerClusterConfiguration)), base64.StdEncoding.EncodeToString([]byte(providerDiscoveryData)))

	var secretD8ProviderClusterConfigurationBadA = fmt.Sprintf(`
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(providerClusterConfigurationBad)), base64.StdEncoding.EncodeToString([]byte(providerDiscoveryData)))

	var secretD8ProviderClusterConfigurationBadB = fmt.Sprintf(`
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(providerClusterConfiguration)), base64.StdEncoding.EncodeToString([]byte(providerDiscoveryDataBad)))

	f := HookExecutionConfigInit(`{"cloudProviderAzure":{"internal":{}}}`, `{}`)

	Context("Provider data and discovery data is successfully discovered", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(secretD8ProviderClusterConfiguration))
			f.RunHook()
		})

		It("All values should be gathered from discovered data and provider cluster configuration", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("cloudProviderAzure.internal.providerDiscoveryData.vnetName").String()).To(Equal("test"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerDiscoveryData.subnetName").String()).To(Equal("test"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerDiscoveryData.resourceGroupName").String()).To(Equal("test"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerDiscoveryData.instances.urn").String()).To(Equal("test:test:test"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerDiscoveryData.instances.diskType").String()).To(Equal("test"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerDiscoveryData.instances.additionalTags").String()).To(Equal("{}"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.masterNodeGroup.instanceClass.diskSizeGb").Int()).To(BeEquivalentTo(50))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.masterNodeGroup.instanceClass.etcdDiskSizeGb").Int()).To(BeEquivalentTo(20))

			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.sshPublicKey").String()).To(Equal("ssh-rsa AAA"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.provider.subscriptionId").String()).To(Equal("test"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.provider.clientId").String()).To(Equal("test"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.provider.clientSecret").String()).To(Equal("test"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.provider.tenantId").String()).To(Equal("test"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.provider.location").String()).To(Equal("test"))
		})
	})

	Context("Provider data is successfully discovered", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(secretD8ProviderClusterConfigurationBadA))
			f.RunHook()
		})

		It("All values should be gathered from discovered data and provider cluster configuration", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))

			Expect(f.GoHookError.Error()).Should(ContainSubstring(`provider in body is required`))
			Expect(f.GoHookError.Error()).Should(ContainSubstring(`vNetCIDR in body is required`))
			Expect(f.GoHookError.Error()).Should(ContainSubstring(`subnetCIDR in body is required`))
			Expect(f.GoHookError.Error()).Should(ContainSubstring(`masterNodeGroup in body is required`))
			Expect(f.GoHookError.Error()).Should(ContainSubstring(`sshPublicKey in body is required`))
		})
	})

	Context("Provider data is successfully discovered", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(secretD8ProviderClusterConfigurationBadB))
			f.RunHook()
		})

		It("All values should be gathered from discovered data and provider cluster configuration", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))

			Expect(f.GoHookError.Error()).Should(ContainSubstring(`resourceGroupName in body is required`))
			Expect(f.GoHookError.Error()).Should(ContainSubstring(`vnetName in body is required`))
			Expect(f.GoHookError.Error()).Should(ContainSubstring(`subnetName in body is required`))
			Expect(f.GoHookError.Error()).Should(ContainSubstring(`zones in body is required`))
		})
	})
})

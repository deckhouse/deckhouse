package hooks

import (
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-azure :: hooks :: azure_cluster_configuration ::", func() {
	var providerClusterConfiguration = `
{
  "apiVersion": "deckhouse.io/v1alpha1",
  "kind": "AzureClusterConfiguration",
  "layout": "Standard",
  "sshPublicKey": "ssh-rsa AAA",
  "vNetCIDR": "10.50.0.0/16",
	"subnetCIDR": "10.50.0.0/24",
	"standard": {
		"natGatewayPublicIpCount": 1
	},
	"masterNodeGroup": {
	  "replicas": 1,
	  "zones": ["1","2","3"],
	  "instanceClass": {
	    "machineSize": "Standard_F2",
	    "urn": "Canonical:UbuntuServer:18.04-LTS:18.04.202010140",
	    "diskSizeGb": 50,
			"diskType": "StandardSSD_LRS",
	    "additionalTags": {
	      "node": "master"
	    }
	  }
	},
  "nodeGroups": [
    {
      "name": "static",
      "replicas": 1,
      "zones": ["1","2","3"],
      "instanceClass": {
		    "machineSize": "Standard_F2",
		    "urn": "Canonical:UbuntuServer:18.04-LTS:18.04.202010140",
		    "diskSizeGb": 50,
				"diskType": "StandardSSD_LRS",
		    "additionalTags": {
		      "node": "static"
		    }
      }
    }
  ],
  "provider": {
    "subscriptionId": "aaa",
		"clientId": "bbb",
		"clientSecret": "ccc",
		"tenantId": "ddd",
		"location": "eee"
  },
	"peeredVNets": [
	  {
			"resourceGroupName": "kube-bastion",
			"vnetName": "kube-bastion-vnet"
		}
	]
}
`

	var providerClusterConfigurationBad = `
{
  "apiVersion": "deckhouse.io/v1alpha1",
  "kind": "AzureClusterConfiguration",
  "layout": "Standard"
}
`

	var providerDiscoveryData = `
{
  "apiVersion": "deckhouse.io/v1alpha1",
  "kind": "AzureCloudDiscoveryData",
  "resourceGroupName": "example",
  "vnetName": "example",
	"subnetName": "example",
  "zones": ["1", "2", "3"],
  "instances": {
    "urn": "Canonical:UbuntuServer:18.04-LTS:18.04.202010140",
    "diskType": "example",
    "additionalTags": {}
  }
}
`

	var providerDiscoveryDataBad = `
{
  "apiVersion": "deckhouse.io/v1alpha1",
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

		It("All values should be gathered from discovered data", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.layout").String()).To(Equal("Standard"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.sshPublicKey").String()).To(Equal("ssh-rsa AAA"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.vNetCIDR").String()).To(Equal("10.50.0.0/16"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.subnetCIDR").String()).To(Equal("10.50.0.0/24"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.standard.natGatewayPublicIpCount").String()).To(Equal("1"))

			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.masterNodeGroup.replicas").String()).To(Equal("1"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.masterNodeGroup.zones").String()).To(MatchJSON(`["1","2","3"]`))

			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.masterNodeGroup.instanceClass.machineSize").String()).To(Equal("Standard_F2"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.masterNodeGroup.instanceClass.urn").String()).To(Equal("Canonical:UbuntuServer:18.04-LTS:18.04.202010140"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.masterNodeGroup.instanceClass.diskSizeGb").String()).To(Equal("50"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.masterNodeGroup.instanceClass.diskType").String()).To(Equal("StandardSSD_LRS"))

			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.masterNodeGroup.instanceClass.additionalTags.node").String()).To(Equal("master"))

			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.nodeGroups.0.name").String()).To(Equal("static"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.nodeGroups.0.replicas").String()).To(Equal("1"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.nodeGroups.0.zones").String()).To(MatchJSON(`["1","2","3"]`))

			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.nodeGroups.0.instanceClass.machineSize").String()).To(Equal("Standard_F2"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.nodeGroups.0.instanceClass.urn").String()).To(Equal("Canonical:UbuntuServer:18.04-LTS:18.04.202010140"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.nodeGroups.0.instanceClass.diskSizeGb").String()).To(Equal("50"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.nodeGroups.0.instanceClass.diskType").String()).To(Equal("StandardSSD_LRS"))

			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.nodeGroups.0.instanceClass.additionalTags.node").String()).To(Equal("static"))

			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.provider.subscriptionId").String()).To(Equal("aaa"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.provider.clientId").String()).To(Equal("bbb"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.provider.clientSecret").String()).To(Equal("ccc"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.provider.tenantId").String()).To(Equal("ddd"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.provider.location").String()).To(Equal("eee"))

			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.peeredVNets.0.resourceGroupName").String()).To(Equal("kube-bastion"))
			Expect(f.ValuesGet("cloudProviderAzure.internal.providerClusterConfiguration.peeredVNets.0.vnetName").String()).To(Equal("kube-bastion-vnet"))
		})
	})

	Context("Provider data is successfully discovered", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(secretD8ProviderClusterConfigurationBadA))
			f.RunHook()
		})

		It("All values should be gathered from discovered data and provider cluster configuration", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))

			Expect(f.Session.Err).Should(gbytes.Say(`.provider in body is required`))
			Expect(f.Session.Err).Should(gbytes.Say(`.vNetCIDR in body is required`))
			Expect(f.Session.Err).Should(gbytes.Say(`.subnetCIDR in body is required`))
			Expect(f.Session.Err).Should(gbytes.Say(`.masterNodeGroup in body is required`))
			Expect(f.Session.Err).Should(gbytes.Say(`.sshPublicKey in body is required`))
		})
	})

	Context("Provider data is successfully discovered", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(secretD8ProviderClusterConfigurationBadB))
			f.RunHook()
		})

		It("All values should be gathered from discovered data and provider cluster configuration", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))

			Expect(f.Session.Err).Should(gbytes.Say(`.resourceGroupName in body is required`))
			Expect(f.Session.Err).Should(gbytes.Say(`.vnetName in body is required`))
			Expect(f.Session.Err).Should(gbytes.Say(`.subnetName in body is required`))
			Expect(f.Session.Err).Should(gbytes.Say(`.zones in body is required`))
		})
	})
})

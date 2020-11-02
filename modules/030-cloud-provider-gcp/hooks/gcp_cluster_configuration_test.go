package hooks

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Modules :: cloud-provider-gcp :: hooks :: gcp_cluster_configuration ::", func() {
	var providerClusterConfiguration = `
{
  "apiVersion": "deckhouse.io/v1alpha1",
  "kind": "GCPClusterConfiguration",
  "layout": "Standard",
  "sshKey": "ssh-rsa AAA",
  "subnetworkCIDR": "10.36.0.0/24",
  "peeredVPCs": ["default"],
  "labels": {
    "kube": "test"
  },
	"masterNodeGroup": {
	  "replicas": 1,
	  "zones": ["a"],
	  "instanceClass": {
	    "machineType": "n1-standard-4",
	    "image": "ubuntu",
	    "diskSizeGb": 20,
	    "additionalNetworkTags": ["example"],
	    "additionalLabels": {
	      "node": "master"
	    }
	  }
	},
  "nodeGroups": [
    {
      "name": "static",
      "replicas": 1,
      "zones": ["a"],
      "instanceClass": {
        "machineType": "n1-standard-4",
        "image": "ubuntu",
        "diskSizeGb": 20,
        "additionalNetworkTags": ["example"],
        "additionalLabels": {
          "node": "static"
        }
      }
    }
  ],
  "provider": {
    "region": "europe-west4",
    "serviceAccountJSON": "{\"type\": \"test\", \"project_id\": \"test\", \"private_key_id\": \"test\", \"private_key\": \"test\", \"client_email\": \"test@test\", \"client_id\": \"test\", \"auth_uri\": \"test\", \"token_uri\": \"test\", \"auth_provider_x509_cert_url\": \"test\", \"client_x509_cert_url\": \"test\"}"
  }
}
`

	var providerClusterConfigurationBad = `
{
  "apiVersion": "deckhouse.io/v1alpha1",
  "kind": "GCPClusterConfiguration",
  "layout": "Standard"
}
`

	var providerDiscoveryData = `
{
  "apiVersion": "deckhouse.io/v1alpha1",
  "kind": "GCPCloudDiscoveryData",
  "networkName": "example",
  "subnetworkName": "example",
  "zones": ["a", "b", "c"],
  "disableExternalIP": true,
  "instances": {
    "image": "ubuntu",
    "diskSizeGb": 50,
    "diskType": "pd-standard",
    "networkTags": [""],
    "labels": {}
  }
}
`

	var providerDiscoveryDataBad = `
{
  "apiVersion": "deckhouse.io/v1alpha1",
  "kind": "GCPCloudDiscoveryData"
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

	f := HookExecutionConfigInit(`{"cloudProviderGcp":{"internal":{}}}`, `{}`)

	Context("Provider data and discovery data is successfully discovered", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(secretD8ProviderClusterConfiguration))
			f.RunHook()
		})

		It("All values should be gathered from discovered data", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.provider.region").String()).To(Equal("europe-west4"))

			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.layout").String()).To(Equal("Standard"))
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.sshKey").String()).To(Equal("ssh-rsa AAA"))
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.subnetworkCIDR").String()).To(Equal("10.36.0.0/24"))
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.peeredVPCs.0").String()).To(Equal("default"))
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.labels.kube").String()).To(Equal("test"))
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.masterNodeGroup.replicas").String()).To(Equal("1"))
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.masterNodeGroup.zones.0").String()).To(Equal("a"))
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.masterNodeGroup.instanceClass.machineType").String()).To(Equal("n1-standard-4"))
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.masterNodeGroup.instanceClass.image").String()).To(Equal("ubuntu"))
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.masterNodeGroup.instanceClass.diskSizeGb").String()).To(Equal("20"))
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.masterNodeGroup.instanceClass.additionalNetworkTags.0").String()).To(Equal("example"))
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.masterNodeGroup.instanceClass.additionalLabels.node").String()).To(Equal("master"))

			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.nodeGroups.0.name").String()).To(Equal("static"))

			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.nodeGroups.0.replicas").String()).To(Equal("1"))
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.nodeGroups.0.zones.0").String()).To(Equal("a"))

			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.nodeGroups.0.instanceClass.machineType").String()).To(Equal("n1-standard-4"))
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.nodeGroups.0.instanceClass.image").String()).To(Equal("ubuntu"))
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.nodeGroups.0.instanceClass.diskSizeGb").String()).To(Equal("20"))
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.nodeGroups.0.instanceClass.additionalNetworkTags.0").String()).To(Equal("example"))
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.nodeGroups.0.instanceClass.additionalLabels.node").String()).To(Equal("static"))

			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.").String()).To(Equal(""))
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.").String()).To(Equal(""))
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.").String()).To(Equal(""))

			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.provider.serviceAccountJSON").String()).To(MatchJSON(`{"type": "test", "project_id": "test", "private_key_id": "test", "private_key": "test", "client_email": "test@test", "client_id": "test", "auth_uri": "test", "token_uri": "test", "auth_provider_x509_cert_url": "test", "client_x509_cert_url": "test"}`))
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerDiscoveryData").String()).To(MatchJSON(`{"apiVersion":"deckhouse.io/v1alpha1","kind":"GCPCloudDiscoveryData","networkName":"example","subnetworkName":"example","zones":["a","b","c"],"disableExternalIP":true,"instances":{"image":"ubuntu","diskSizeGb":50,"diskType":"pd-standard","networkTags":[""],"labels":{}}}`))
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
			Expect(f.Session.Err).Should(gbytes.Say(`.masterNodeGroup in body is required`))
			Expect(f.Session.Err).Should(gbytes.Say(`.sshKey in body is required`))
		})
	})

	Context("Provider data is successfully discovered", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(secretD8ProviderClusterConfigurationBadB))
			f.RunHook()
		})

		It("All values should be gathered from discovered data and provider cluster configuration", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))

			Expect(f.Session.Err).Should(gbytes.Say(`.networkName in body is required`))
			Expect(f.Session.Err).Should(gbytes.Say(`.subnetworkName in body is required`))
			Expect(f.Session.Err).Should(gbytes.Say(`.zones in body is required`))
		})
	})
})

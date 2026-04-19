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

var _ = Describe("Modules :: cloud-provider-gcp :: hooks :: gcp_cluster_configuration ::", func() {

	var providerClusterConfiguration = `
{
  "apiVersion": "deckhouse.io/v1",
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
  "apiVersion": "deckhouse.io/v1",
  "kind": "GCPClusterConfiguration",
  "layout": "Standard"
}
`

	var providerDiscoveryData = `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "GCPCloudDiscoveryData",
  "networkName": "example",
  "subnetworkName": "example",
  "zones": ["a", "b", "c"],
  "disableExternalIP": true,
  "instances": {
    "image": "ubuntu",
    "diskSizeGb": 50,
    "diskType": "pd-standard",
    "networkTags": ["tag"],
    "labels": {}
  }
}
`

	var providerDiscoveryDataBad = `
{
  "apiVersion": "deckhouse.io/v1",
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
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.sshKey").String()).To(Equal("ssh-rsa AAA"))
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.masterNodeGroup.instanceClass.diskSizeGb").Int()).To(BeEquivalentTo(20))
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.masterNodeGroup.instanceClass.etcdDiskSizeGb").Int()).To(BeEquivalentTo(20))

			desiredServiceAccountJSON := `
{
  "type": "test",
  "project_id": "test",
  "private_key_id": "test",
  "private_key": "test",
  "client_email": "test@test",
  "client_id": "test",
  "auth_uri": "test",
  "token_uri": "test",
  "auth_provider_x509_cert_url": "test",
  "client_x509_cert_url": "test"
}
`
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerClusterConfiguration.provider.serviceAccountJSON").String()).To(MatchJSON(desiredServiceAccountJSON))

			desiredJSON := `
{
  "apiVersion": "deckhouse.io/v1",
  "disableExternalIP": true,
  "instances": {
    "diskSizeGb": 50,
    "diskType": "pd-standard",
    "image": "ubuntu",
    "labels": {},
    "networkTags": [
      "tag"
    ]
  },
  "kind": "GCPCloudDiscoveryData",
  "networkName": "example",
  "subnetworkName": "example",
  "zones": [
    "a",
    "b",
    "c"
  ]
}
`
			Expect(f.ValuesGet("cloudProviderGcp.internal.providerDiscoveryData").String()).To(MatchJSON(desiredJSON))
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
			Expect(f.GoHookError.Error()).Should(ContainSubstring(`masterNodeGroup in body is required`))
			Expect(f.GoHookError.Error()).Should(ContainSubstring(`sshKey in body is required`))
		})
	})

	Context("Provider data is successfully discovered", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(secretD8ProviderClusterConfigurationBadB))
			f.RunHook()
		})

		It("All values should be gathered from discovered data and provider cluster configuration", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
			Expect(f.GoHookError.Error()).Should(ContainSubstring(`networkName in body is required`))
			Expect(f.GoHookError.Error()).Should(ContainSubstring(`subnetworkName in body is required`))
			Expect(f.GoHookError.Error()).Should(ContainSubstring(`zones in body is required`))
		})
	})

})

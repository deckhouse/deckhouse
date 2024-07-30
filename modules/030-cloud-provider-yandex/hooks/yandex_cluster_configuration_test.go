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

var _ = Describe("Modules :: cloud-provider-yandex :: hooks :: yandex_cluster_configuration ::", func() {
	const (
		initValuesString = `
global:
  discovery: {}
cloudProviderYandex:
  internal: {}
`
	)

	var (
		// correct cdd
		stateBCloudDiscoveryData = `
{
  "apiVersion": "deckhouse.io/v1",
  "defaultLbTargetGroupNetworkId": "test",
  "internalNetworkIDs": [
    "test"
  ],
  "kind": "YandexCloudDiscoveryData",
  "region": "test",
  "routeTableID": "test",
  "shouldAssignPublicIPAddress": false,
  "zoneToSubnetIdMap": {
    "ru-central1-a": "test",
    "ru-central1-b": "test",
    "ru-central1-c": "test"
  },
  "zones": [
    "ru-central1-a",
    "ru-central1-b",
    "ru-central1-c"
  ]
}
`

		// wrong cdd
		stateCCloudDiscoveryData = `
{
  "apiVersion": "deckhouse.io/v1",
  "internalNetworkIDs": [
    "testtest"
  ],
  "kind": "YandexCloudDiscoveryData"
}
`

		// correct cc
		stateBClusterConfiguration = `
apiVersion: deckhouse.io/v1
existingNetworkID: enpma5uvcfbkuac1i1jb
kind: YandexClusterConfiguration
layout: WithNATInstance
masterNodeGroup:
  instanceClass:
    cores: 2
    etcdDiskSizeGb: 10
    imageID: test
    memory: 4096
    platform: standard-v2
  replicas: 1
provider:
  cloudID: test
  folderID: test
  serviceAccountJSON: |-
    {
      "id": "test"
    }
withNATInstance:
  internalSubnetID: test
  natInstanceExternalAddress: 84.201.160.148
  exporterAPIKey: ""
  natInstanceResources:
    cores: 2
    memory: 2048
nodeNetworkCIDR: 84.201.160.148/31
sshPublicKey: ssh-rsa AAAAAbbbb
`

		// wrong cc
		stateDClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: WithNATInstance
`

		stateB = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(stateBClusterConfiguration)), base64.StdEncoding.EncodeToString([]byte(stateBCloudDiscoveryData)))

		stateC = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(stateBClusterConfiguration)), base64.StdEncoding.EncodeToString([]byte(stateCCloudDiscoveryData)))

		stateD = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(stateDClusterConfiguration)), base64.StdEncoding.EncodeToString([]byte(stateBCloudDiscoveryData)))
	)

	a := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Cluster has empty state", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.KubeStateSet(``))
			a.RunHook()
		})

		It("Hook should fail with errors", func() {
			Expect(a).To(Not(ExecuteSuccessfully()))

			Expect(a.GoHookError.Error()).Should(ContainSubstring(`kube-system/d8-provider-cluster-configuration`))
		})
	})

	b := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Provider data is successfully discovered", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.KubeStateSet(stateB))
			b.RunHook()
		})

		It("All values should be gathered from discovered data", func() {
			Expect(b).To(ExecuteSuccessfully())

			Expect(b.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.defaultLbTargetGroupNetworkId").String()).To(Equal("test"))
			Expect(b.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.internalNetworkIDs").AsStringSlice()).To(Equal([]string{"test"}))
			Expect(b.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.region").String()).To(Equal("test"))
			Expect(b.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.routeTableID").String()).To(Equal("test"))
			Expect(b.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.shouldAssignPublicIPAddress").Bool()).To(BeFalse())
			Expect(b.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.zoneToSubnetIdMap").String()).To(MatchYAML(`
ru-central1-a: test
ru-central1-b: test
ru-central1-c: test
`))
			Expect(b.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.zones").AsStringSlice()).To(Equal([]string{"ru-central1-a", "ru-central1-b", "ru-central1-c"}))

			Expect(b.ValuesGet("cloudProviderYandex.internal.providerClusterConfiguration").String()).To(MatchYAML(stateBClusterConfiguration))
		})
	})

	c := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Discovery data is wrong", func() {
		BeforeEach(func() {
			c.BindingContexts.Set(c.KubeStateSet(stateC))
			c.RunHook()
		})

		It("All values should be gathered from discovered data", func() {
			Expect(c).To(Not(ExecuteSuccessfully()))

			Expect(c.GoHookError.Error()).Should(ContainSubstring(`validate cloud-provider-discovery-data.json: Loading schema file: Document validation failed`))
			Expect(c.GoHookError.Error()).Should(ContainSubstring(`.region is required`))
			Expect(c.GoHookError.Error()).Should(ContainSubstring(`.routeTableID is required`))
			Expect(c.GoHookError.Error()).Should(ContainSubstring(`.defaultLbTargetGroupNetworkId is required`))
			Expect(c.GoHookError.Error()).Should(ContainSubstring(`.zones is required`))
			Expect(c.GoHookError.Error()).Should(ContainSubstring(`.zoneToSubnetIdMap is required`))
			Expect(c.GoHookError.Error()).Should(ContainSubstring(`.shouldAssignPublicIPAddress is required`))
		})
	})

	d := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Discovery data is wrong", func() {
		BeforeEach(func() {
			d.BindingContexts.Set(d.KubeStateSet(stateD))
			d.RunHook()
		})

		It("All values should be gathered from discovered data", func() {
			Expect(d).To(Not(ExecuteSuccessfully()))

			Expect(d.GoHookError.Error()).To(ContainSubstring(`validate cloud-provider-cluster-configuration.yaml: Config document validation failed: Document validation failed`))
			Expect(d.GoHookError.Error()).Should(ContainSubstring(`must validate one and only one schema (oneOf). Found none valid`))
			Expect(d.GoHookError.Error()).Should(ContainSubstring(`layout should be one of [Standard WithoutNAT]`))
			// Expect(d.GoHookError.Error()).Should(ContainSubstring(`.masterNodeGroup is required`))
			Expect(d.GoHookError.Error()).Should(ContainSubstring(`.nodeNetworkCIDR is required`))
			Expect(d.GoHookError.Error()).Should(ContainSubstring(`.sshPublicKey is required`))
			Expect(d.GoHookError.Error()).Should(ContainSubstring(`.provider is required`))
		})
	})
})

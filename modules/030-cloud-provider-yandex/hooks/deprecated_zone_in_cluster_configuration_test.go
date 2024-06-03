/*
Copyright 2024 Flant JSC

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

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-yandex :: hooks :: deprecated_zone_in_cluster_configuration ::", func() {
	var (
		initValuesString = `
global:
  discovery: {}
cloudProviderYandex:
  internal: {}
`

		clusterConfigurationWithDeprecatedZone = `
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
zones:
- ru-central1-a
- ru-central1-b
- ru-central1-c
`
		clusterConfigurationWithZones = `
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
zones:
- ru-central1-a
- ru-central1-b
- ru-central1-d
`
		clusterConfigurationWithoutZones = `
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
		clusterConfigurationWithMasterDeprecatedZone = `
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
  zones:
    - ru-central1-a
    - ru-central1-b
    - ru-central1-c
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
		clusterConfigurationWithNGDeprecatedZone = `
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
nodeGroups:
  - name: worker
    replicas: 1
    zones:
      - ru-central1-a
      - ru-central1-b
      - ru-central1-c
    instanceClass:
      cores: 4
      memory: 8192
      imageID: fd8nb7ecsbvj76dfaa8b
      coreFraction: 50
      externalIPAddresses:
        - 198.51.100.5
        - Auto
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

		stateWithDeprecatedZone = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(clusterConfigurationWithDeprecatedZone)), "e30=")
		stateWithMasterDeprecatedZone = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(clusterConfigurationWithMasterDeprecatedZone)), "e30=")
		stateWithNGDeprecatedZone = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(clusterConfigurationWithNGDeprecatedZone)), "e30=")

		stateWithZones = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(clusterConfigurationWithZones)), "e30=")

		stateWithoutZones = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(clusterConfigurationWithoutZones)), "e30=")
	)

	a := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Cluster has empty state", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.KubeStateSet(``))
			a.RunHook()
		})

		It("Hook should fail with errors", func() {
			Expect(a).To(Not(ExecuteSuccessfully()))

			Expect(a.GoHookError.Error()).Should(ContainSubstring(`Can't find Secret d8-provider-cluster-configuration in Namespace kube-system`))
		})
	})

	b := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Provider cluster configuration contains deprecated zone", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.KubeStateSet(stateWithDeprecatedZone))
			b.RunHook()
		})

		It("Should set deprecatedZoneInUse to true", func() {
			Expect(b).To(ExecuteSuccessfully())
			requirements.GetValue(yandexDeprecatedZoneKey)
			hasDeprecatedZone, exists := requirements.GetValue(yandexDeprecatedZoneKey)
			Expect(exists).To(BeTrue())
			Expect(hasDeprecatedZone).To(BeTrue())
		})
	})

	c := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Provider cluster configuration contains zones without deprecated one", func() {
		BeforeEach(func() {
			c.BindingContexts.Set(c.KubeStateSet(stateWithZones))
			c.RunHook()
		})

		It("Should set deprecatedZoneInUse to false", func() {
			Expect(c).To(ExecuteSuccessfully())

			hasDeprecatedZone, exists := requirements.GetValue(yandexDeprecatedZoneKey)
			Expect(exists).To(BeTrue())
			Expect(hasDeprecatedZone).To(BeFalse())
		})
	})

	d := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Provider cluster configuration doesn't contain zones", func() {
		BeforeEach(func() {
			d.BindingContexts.Set(d.KubeStateSet(stateWithoutZones))
			d.RunHook()
		})

		It("Should set deprecatedZoneInUse to false", func() {
			Expect(d).To(ExecuteSuccessfully())

			hasDeprecatedZone, exists := requirements.GetValue(yandexDeprecatedZoneKey)
			Expect(exists).To(BeTrue())
			Expect(hasDeprecatedZone).To(BeFalse())
		})
	})

	e := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Provider cluster configuration contains master deprecated zone", func() {
		BeforeEach(func() {
			e.BindingContexts.Set(e.KubeStateSet(stateWithMasterDeprecatedZone))
			e.RunHook()
		})

		It("Should set deprecatedZoneInUse to true", func() {
			Expect(e).To(ExecuteSuccessfully())
			requirements.GetValue(yandexDeprecatedZoneKey)
			hasDeprecatedZone, exists := requirements.GetValue(yandexDeprecatedZoneKey)
			Expect(exists).To(BeTrue())
			Expect(hasDeprecatedZone).To(BeTrue())
		})
	})
	f := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Provider cluster configuration contains ng deprecated zone", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateWithNGDeprecatedZone))
			f.RunHook()
		})

		It("Should set deprecatedZoneInUse to true", func() {
			Expect(f).To(ExecuteSuccessfully())
			requirements.GetValue(yandexDeprecatedZoneKey)
			hasDeprecatedZone, exists := requirements.GetValue(yandexDeprecatedZoneKey)
			Expect(exists).To(BeTrue())
			Expect(hasDeprecatedZone).To(BeTrue())
		})
	})
})

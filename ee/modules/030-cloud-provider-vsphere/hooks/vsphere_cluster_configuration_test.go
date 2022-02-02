/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/base64"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-vsphere :: hooks :: vsphere_cluster_configuration ::", func() {
	const (
		initValuesStringA = `
global:
  discovery: {}
cloudProviderVsphere:
  internal: {}
`
		initValuesStringB = `
global:
  discovery: {}
cloudProviderVsphere:
  internal: {}
  host: override
  username: override
  password: override
  insecure: true
  regionTagCategory: override
  zoneTagCategory: override
  internalNetworkNames: [override1, override2]
  externalNetworkNames: [override1, override2]
  disableTimesync: false
  vmFolderPath: override
  sshKeys: [override1, override2]
  region: override
  zones: [override1, override2]
`
	)

	var (
		stateACloudDiscoveryData = `
{
  "apiVersion":"deckhouse.io/v1",
  "kind":"VsphereCloudDiscoveryData",
  "vmFolderPath":"test",
  "resourcePoolPath": "test"
}
`
		stateAClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: VsphereClusterConfiguration
layout: Standard
provider:
  server: test
  username: test
  password: test
  insecure: true
vmFolderPath: test
regionTagCategory: test
zoneTagCategory: test
region: test
internalNetworkCIDR: test
sshPublicKey: test
internalNetworkNames: [test1, test2]
externalNetworkNames: [test1, test2]
zones: [test1, test2]
masterNodeGroup:
  replicas: 1
  zones:
  - test
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: dev/golden_image
    datastore: dev/lun_1
    mainNetwork: k8s-msk/test_187
nodeGroups:
- name: khm
  replicas: 1
  zones:
  - test
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: dev/golden_image
    datastore: dev/lun_1
    mainNetwork: k8s-msk/test_187
`
		notEmptyProviderClusterConfigurationState = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(stateAClusterConfiguration)), base64.StdEncoding.EncodeToString([]byte(stateACloudDiscoveryData)))

		emptyProviderClusterConfigurationState = `
apiVersion: v1
kind: Secret
metadata:
 name: d8-provider-cluster-configuration
 namespace: kube-system
data: {}
`
	)

	// todo(31337Ghost) eliminate the following dirty hack after `ee` subdirectory will be merged to the root
	// Used to make dhctl config function able to validate `VsphereClusterConfiguration`.
	_ = os.Setenv("DHCTL_CLI_ADDITIONAL_SCHEMAS_PATHS", "/deckhouse/ee/candi")

	a := HookExecutionConfigInit(initValuesStringA, `{}`)

	Context("Cluster has minimal cloudProviderVsphere configuration", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.KubeStateSet(notEmptyProviderClusterConfigurationState))
			a.RunHook()
		})

		It("Should fill values", func() {
			Expect(a).To(ExecuteSuccessfully())
			Expect(a.ValuesGet("cloudProviderVsphere.internal.providerClusterConfiguration").String()).To(MatchYAML(stateAClusterConfiguration + "disableTimesync: true"))
			Expect(a.ValuesGet("cloudProviderVsphere.internal.providerDiscoveryData").String()).To(MatchJSON(stateACloudDiscoveryData))
		})
	})

	b := HookExecutionConfigInit(initValuesStringB, `{}`)
	Context("BeforeHelm", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.GenerateBeforeHelmContext())
			b.RunHook()
		})

		It("Should fill values from cloudProviderVsphere", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("cloudProviderVsphere.internal.providerClusterConfiguration").String()).To(MatchYAML(`
provider:
  server: override
  username: override
  password: override
  insecure: true
regionTagCategory: override
zoneTagCategory: override
internalNetworkNames: [override1, override2]
externalNetworkNames: [override1, override2]
disableTimesync: false
vmFolderPath: override
sshPublicKey: override1
region: override
zones: [override1, override2]
`))
			Expect(b.ValuesGet("cloudProviderVsphere.internal.providerDiscoveryData").String()).To(MatchJSON("{}"))
		})
	})

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.KubeStateSet(emptyProviderClusterConfigurationState))
			b.RunHook()
		})
		It("Should fill values from config", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("cloudProviderVsphere.internal.providerClusterConfiguration").String()).To(MatchYAML(`
provider:
  server: override
  username: override
  password: override
  insecure: true
regionTagCategory: override
zoneTagCategory: override
internalNetworkNames: [override1, override2]
externalNetworkNames: [override1, override2]
disableTimesync: false
vmFolderPath: override
sshPublicKey: override1
region: override
zones: [override1, override2]
`))
			Expect(b.ValuesGet("cloudProviderVsphere.internal.providerDiscoveryData").String()).To(MatchJSON("{}"))
		})

		Context("Cluster has cloudProviderVsphere and discovery data", func() {
			BeforeEach(func() {
				b.BindingContexts.Set(b.KubeStateSet(notEmptyProviderClusterConfigurationState))
				b.RunHook()
			})

			It("Should override values cloudProviderVsphere configuration", func() {
				Expect(b).To(ExecuteSuccessfully())
				Expect(b.ValuesGet("cloudProviderVsphere.internal.providerClusterConfiguration").String()).To(MatchYAML(`
apiVersion: deckhouse.io/v1
kind: VsphereClusterConfiguration
provider:
  server: override
  username: override
  password: override
  insecure: true
regionTagCategory: override
zoneTagCategory: override
internalNetworkCIDR: test
internalNetworkNames: [override1, override2]
externalNetworkNames: [override1, override2]
disableTimesync: false
layout: Standard
vmFolderPath: override
sshPublicKey: override1
region: override
zones: [override1, override2]
masterNodeGroup:
  instanceClass:
    datastore: dev/lun_1
    mainNetwork: k8s-msk/test_187
    memory: 8192
    numCPUs: 4
    template: dev/golden_image
  replicas: 1
  zones:
    - test
nodeGroups:
  - instanceClass:
      datastore: dev/lun_1
      mainNetwork: k8s-msk/test_187
      memory: 8192
      numCPUs: 4
      template: dev/golden_image
    name: khm
    replicas: 1
    zones:
      - test

`))
				Expect(b.ValuesGet("cloudProviderVsphere.internal.providerDiscoveryData").String()).To(MatchJSON(stateACloudDiscoveryData))
			})
		})
	})
})

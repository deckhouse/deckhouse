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
		emptyValues = `
global:
  discovery: {}
cloudProviderVsphere:
  internal: {}
`
		filledValues = `
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
		filledValuesWithoutSomeFields = `
global:
  discovery: {}
cloudProviderVsphere:
  internal: {}
  host: override
  username: override
  password: override
  insecure: true
  internalNetworkNames: [override1, override2]
  externalNetworkNames: [override1, override2]
  disableTimesync: false
  vmFolderPath: override
  sshKeys: [override1, override2]
  region: override
`
		filledValuesWithNSXT = filledValues + `
  nsxt:
    defaultIpPoolName: pool1
    defaultTcpAppProfileName: default-tcp-lb-app-profile
    defaultUdpAppProfileName: default-udp-lb-app-profile
    size: SMALL
    tier1GatewayPath: /host/tier1
    user: nsxt
    password: pass
    host: host
`
	)

	var (
		stateACloudDiscoveryData = `
{
  "apiVersion":"deckhouse.io/v1",
  "kind":"VsphereCloudDiscoveryData",
  "vmFolderPath":"test",
  "resourcePoolPath": "test",
  "zones": ["test"]
}
`
		stateAClusterConfiguration1 = `
apiVersion: deckhouse.io/v1
kind: VsphereClusterConfiguration
disableTimesync: true
layout: Standard
provider:
  server: test
  username: test
  password: test
  insecure: true
vmFolderPath: test
vmFolderExists: false
regionTagCategory: test
zoneTagCategory: test
region: test
internalNetworkCIDR: 192.168.199.0/24
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
		stateAClusterConfiguration2 = `
disableTimesync: false
externalNetworkNames:
- override1
- override2
internalNetworkNames:
- override1
- override2
provider:
  insecure: true
  password: override
  server: override
  username: override
region: override
regionTagCategory: override
sshPublicKey: override1
vmFolderPath: override
vmFolderExists: false
zoneTagCategory: override
zones:
- override1
- override2
`
		stateAClusterConfiguration3 = `
apiVersion: deckhouse.io/v1
disableTimesync: false
externalNetworkNames:
- override1
- override2
internalNetworkCIDR: 192.168.199.0/24
internalNetworkNames:
- override1
- override2
kind: VsphereClusterConfiguration
layout: Standard
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
provider:
  insecure: true
  password: override
  server: override
  username: override
region: override
regionTagCategory: override
sshPublicKey: override1
vmFolderPath: override
vmFolderExists: false
zoneTagCategory: override
zones:
- override1
- override2
`
		stateAClusterConfiguration4 = `
disableTimesync: false
externalNetworkNames:
- override1
- override2
internalNetworkNames:
- override1
- override2
provider:
  insecure: true
  password: override
  server: override
  username: override
region: override
regionTagCategory: k8s-region
sshPublicKey: override1
vmFolderPath: override
vmFolderExists: false
zoneTagCategory: k8s-zone
zones:
- test
`
		nsxt = `
nsxt:
  defaultIpPoolName: pool1
  defaultTcpAppProfileName: default-tcp-lb-app-profile
  defaultUdpAppProfileName: default-udp-lb-app-profile
  size: SMALL
  tier1GatewayPath: /host/tier1
  user: nsxt
  password: pass
  host: host
`

		nsxt2 = `
nsxt:
  defaultIpPoolName: pool2
  defaultTcpAppProfileName: default-tcp-lb-app-profile
  defaultUdpAppProfileName: default-udp-lb-app-profile
  size: LARGE
  tier1GatewayPath: /host2/tier1
  user: nsxt1
  password: pass1
  host: host1
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
`, base64.StdEncoding.EncodeToString([]byte(stateAClusterConfiguration1)), base64.StdEncoding.EncodeToString([]byte(stateACloudDiscoveryData)))

		notEmptyProviderClusterConfigurationStateNSXT = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(stateAClusterConfiguration1+nsxt)), base64.StdEncoding.EncodeToString([]byte(stateACloudDiscoveryData)))

		notEmptyProviderClusterConfigurationStateNSXT2 = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(stateAClusterConfiguration1+nsxt2)), base64.StdEncoding.EncodeToString([]byte(stateACloudDiscoveryData)))

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

	a := HookExecutionConfigInit(emptyValues, `{}`)
	Context("Cluster without module configuration, with secret (without nsx-t)", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.KubeStateSet(notEmptyProviderClusterConfigurationState))
			a.RunHook()
		})

		It("Should fill values from secret", func() {
			Expect(a).To(ExecuteSuccessfully())
			Expect(a.ValuesGet("cloudProviderVsphere.internal.providerClusterConfiguration").String()).To(MatchYAML(stateAClusterConfiguration1))
			Expect(a.ValuesGet("cloudProviderVsphere.internal.providerDiscoveryData").String()).To(MatchJSON(stateACloudDiscoveryData))
		})
	})
	Context("Cluster without module configuration, with secret (with nsx-t)", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.KubeStateSet(notEmptyProviderClusterConfigurationStateNSXT))
			a.RunHook()
		})

		It("Should fill values from secret", func() {
			Expect(a).To(ExecuteSuccessfully())
			Expect(a.ValuesGet("cloudProviderVsphere.internal.providerClusterConfiguration").String()).To(MatchYAML(stateAClusterConfiguration1 + nsxt))
			Expect(a.ValuesGet("cloudProviderVsphere.internal.providerDiscoveryData").String()).To(MatchJSON(stateACloudDiscoveryData))
		})
	})

	b := HookExecutionConfigInit(filledValues, `{}`)
	Context("Cluster with module configuration, with empty secret", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.KubeStateSet(emptyProviderClusterConfigurationState))
			b.RunHook()
		})

		It("Should fill values from module configuration", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("cloudProviderVsphere.internal.providerClusterConfiguration").String()).To(MatchYAML(stateAClusterConfiguration2))
			Expect(b.ValuesGet("cloudProviderVsphere.internal.providerDiscoveryData").String()).To(MatchJSON("{}"))
		})
	})
	Context("Cluster with module configuration, with secret (without nsx-t)", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.KubeStateSet(notEmptyProviderClusterConfigurationState))
			b.RunHook()
		})

		It("Should merge values from secret and module configuration", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("cloudProviderVsphere.internal.providerClusterConfiguration").String()).To(MatchYAML(stateAClusterConfiguration3))
			Expect(b.ValuesGet("cloudProviderVsphere.internal.providerDiscoveryData").String()).To(MatchJSON(stateACloudDiscoveryData))
		})
	})
	Context("Cluster with module configuration, with secret (with nsx-t), ", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.KubeStateSet(notEmptyProviderClusterConfigurationStateNSXT))
			b.RunHook()
		})

		It("Should merge values from secret and module configuration", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("cloudProviderVsphere.internal.providerClusterConfiguration").String()).To(MatchYAML(stateAClusterConfiguration3 + nsxt))
			Expect(b.ValuesGet("cloudProviderVsphere.internal.providerDiscoveryData").String()).To(MatchJSON(stateACloudDiscoveryData))
		})
	})

	c := HookExecutionConfigInit(filledValuesWithNSXT, `{}`)
	Context("Cluster with module configuration(with nsx-t), with empty secret", func() {
		BeforeEach(func() {
			c.BindingContexts.Set(c.KubeStateSet(emptyProviderClusterConfigurationState))
			c.RunHook()
		})

		It("Should fill values from module configuration", func() {
			Expect(c).To(ExecuteSuccessfully())
			Expect(c.ValuesGet("cloudProviderVsphere.internal.providerClusterConfiguration").String()).To(MatchYAML(stateAClusterConfiguration2 + nsxt))
			Expect(c.ValuesGet("cloudProviderVsphere.internal.providerDiscoveryData").String()).To(MatchJSON("{}"))
		})
	})
	Context("Cluster with module configuration(with nsx-t), with secret (without nsx-t)", func() {
		BeforeEach(func() {
			c.BindingContexts.Set(c.KubeStateSet(notEmptyProviderClusterConfigurationState))
			c.RunHook()
		})

		It("Should merge values from secret and module configuration", func() {
			Expect(c).To(ExecuteSuccessfully())
			Expect(c.ValuesGet("cloudProviderVsphere.internal.providerClusterConfiguration").String()).To(MatchYAML(stateAClusterConfiguration3 + nsxt))
			Expect(c.ValuesGet("cloudProviderVsphere.internal.providerDiscoveryData").String()).To(MatchJSON(stateACloudDiscoveryData))
		})
	})
	Context("Cluster with module configuration(with nsx-t), with secret (with nsx-t), ", func() {
		BeforeEach(func() {
			c.BindingContexts.Set(b.KubeStateSet(notEmptyProviderClusterConfigurationStateNSXT2))
			c.RunHook()
		})

		It("Should merge values from secret and module configuration", func() {
			Expect(c).To(ExecuteSuccessfully())
			Expect(c.ValuesGet("cloudProviderVsphere.internal.providerClusterConfiguration").String()).To(MatchYAML(stateAClusterConfiguration3 + nsxt))
			Expect(c.ValuesGet("cloudProviderVsphere.internal.providerDiscoveryData").String()).To(MatchJSON(stateACloudDiscoveryData))
		})
	})

	d := HookExecutionConfigInit(filledValuesWithoutSomeFields, `{}`)
	Context("Cluster with module configuration without zones, with empty secret", func() {
		BeforeEach(func() {
			d.BindingContexts.Set(d.KubeStateSet(emptyProviderClusterConfigurationState))
			d.RunHook()
		})

		It("Should fail", func() {
			Expect(d).To(Not(ExecuteSuccessfully()))
		})
	})
	Context("Cluster with module configuration with zones, but without zoneTagCategory and regionTagCategory, with empty secret", func() {
		BeforeEach(func() {
			d.BindingContexts.Set(d.KubeStateSet(emptyProviderClusterConfigurationState))
			d.ValuesSet("cloudProviderVsphere.zones", []string{"test"})
			d.RunHook()
		})

		It("Should fill values from module configuration", func() {
			Expect(d).To(ExecuteSuccessfully())
			Expect(d.ValuesGet("cloudProviderVsphere.internal.providerClusterConfiguration").String()).To(MatchYAML(stateAClusterConfiguration4))
			Expect(d.ValuesGet("cloudProviderVsphere.internal.providerDiscoveryData").String()).To(MatchJSON("{}"))
		})
	})
})

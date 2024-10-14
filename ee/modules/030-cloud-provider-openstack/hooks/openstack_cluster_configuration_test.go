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

var _ = Describe("Modules :: cloud-provider-openstack :: hooks :: openstack_cluster_configuration ::", func() {
	const (
		initValuesStringA = `
global:
  discovery: {}
cloudProviderOpenstack:
  internal:
    instances: {}
    zones: []
  instances: {}
  additionalExternalNetworkNames:
  - additional-ext-net
`
		initValuesStringB = `
global:
  discovery: {}
cloudProviderOpenstack:
  internal:
    instances: {}
  connection:
    authURL: https://test.tests.com:5000/v3/
    domainName: default
    tenantName: default
    username: jamie
    password: nein
    region: HetznerFinland
  externalNetworkNames: [public1, public2]
  additionalExternalNetworkNames:
  - additional-ext-net
  - public2
  internalNetworkNames: [int1, int2]
  podNetworkMode: DirectRouting
  instances:
    sshKeyPairName: my-ssh-keypair
    securityGroups:
    - security_group_1
    - security_group_2
  loadBalancer:
    subnetID: overrideSubnetID
  tags:
    aaa: bbb
    ccc: ddd
`
		initValuesStringC = `
global:
  discovery: {}
cloudProviderOpenstack:
  internal:
    instances: {}
  connection:
    authURL: https://test.tests.com:5000/v3/
    domainName: default
    tenantName: default
    username: jamie
    password: nein
    region: HetznerFinland
  externalNetworkNames: [public1, public2]
  internalNetworkNames: [int1, int2]
  podNetworkMode: DirectRouting
  instances:
    sshKeyPairName: my-ssh-keypair
    securityGroups:
    - security_group_1
    - security_group_2
  loadBalancer: {}
`
	)

	var (
		stateACloudDiscoveryData = `
{
  "apiVersion":"deckhouse.io/v1",
  "kind":"OpenStackCloudDiscoveryData",
  "externalNetworkNames": [
    "external"
  ],
  "layout": "Standard",
  "instances": {
    "imageName": "ubuntu",
    "mainNetwork": "kube",
    "sshKeyPairName": "my-key",
    "securityGroups": [
      "default",
      "ssh-and-ping",
      "security_group_1"
    ]
  },
  "internalNetworkNames": [
    "internal"
  ],
  "podNetworkMode": "DirectRoutingWithPortSecurityEnabled",
  "zones": ["zone1", "zone2"],
  "loadBalancer": {
    "subnetID": "subnetID",
    "floatingNetworkID": "floatingNetworkID"
  }
}
`

		stateACloudDiscoveryDataWithoutLoadbalancers = `
{
  "apiVersion":"deckhouse.io/v1",
  "kind":"OpenStackCloudDiscoveryData",
  "externalNetworkNames": [
	"external"
  ],
  "layout": "StandardWithNoRouter",
  "instances": {
	"imageName": "ubuntu",
	"mainNetwork": "kube",
	"sshKeyPairName": "my-key",
    "additionalNetworks": [
	  "extra1",
      "extra2"
	],
	"securityGroups": [
	  "default",
	  "ssh-and-ping",
	  "security_group_1"
	]
  },
  "internalNetworkNames": [
    "int1",
    "int2"
  ],
  "podNetworkMode": "DirectRoutingWithPortSecurityEnabled",
  "zones": ["zone1", "zone2"]
}
`
		stateAClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
layout: Standard
standard:
  internalNetworkCIDR: 192.168.199.0/24
  internalNetworkDNSServers: ["8.8.8.8"]
  internalNetworkSecurity: true
  externalNetworkName: public
provider:
  authURL: https://cloud.flant.com/v3/
  domainName: Default
  tenantName: tenant-name
  username: user-name
  password: pa$$word
  region: HetznerFinland
sshPublicKey: "aaaa"
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large
    imageName: ubuntu-18-04-cloud-amd64
  volumeTypeMap:
    nova: ceph-ssd
tags:
  project: default
  env: production
`
		stateAClusterConfigurationWithNoRouter = `
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
layout: StandardWithNoRouter
standardWithNoRouter:
  internalNetworkCIDR: 192.168.199.0/24
  internalNetworkSecurity: true
  externalNetworkName: public
provider:
  authURL: https://cloud.flant.com/v3/
  domainName: Default
  tenantName: tenant-name
  username: user-name
  password: pa$$word
  region: HetznerFinland
sshPublicKey: "aaaa"
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large
    imageName: ubuntu-18-04-cloud-amd64
  volumeTypeMap:
    nova: ceph-ssd
tags:
  project: default
  env: production
`

		stateAClusterConfigurationSimpleWithInternalNetworkWithoutDHCP = `
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
layout: SimpleWithInternalNetwork
simpleWithInternalNetwork:
  internalSubnetName: pivot-standard
  externalNetworkDHCP: false
provider:
  authURL: https://cloud.flant.com/v3/
  domainName: Default
  tenantName: tenant-name
  username: user-name
  password: pa$$word
  region: HetznerFinland
sshPublicKey: "aaaa"
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large
    imageName: ubuntu-18-04-cloud-amd64
  volumeTypeMap:
    nova: ceph-ssd
tags:
  project: default
  env: production
`

		stateAWithoutLoadbalancers = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(stateAClusterConfigurationWithNoRouter)), base64.StdEncoding.EncodeToString([]byte(stateACloudDiscoveryDataWithoutLoadbalancers)))

		stateA = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(stateAClusterConfiguration)), base64.StdEncoding.EncodeToString([]byte(stateACloudDiscoveryData)))

		stateASimpleWithInternalNetworkWithoutDHCP = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(stateAClusterConfigurationSimpleWithInternalNetworkWithoutDHCP)), base64.StdEncoding.EncodeToString([]byte(stateACloudDiscoveryData)))

		stateB = `
apiVersion: v1
kind: Secret
metadata:
 name: d8-provider-cluster-configuration
 namespace: kube-system
data: {}
`
	)

	// TODO: eliminate the following dirty hack after `ee` subdirectory will be merged to the root
	// Used to make dhctl config function able to validate `VsphereClusterConfiguration`.
	_ = os.Setenv("DHCTL_CLI_ADDITIONAL_SCHEMAS_PATHS", "/deckhouse/ee/candi")
	f := HookExecutionConfigInit(initValuesStringA, `{}`)

	Context("Cluster has minimal cloudProviderOpenstack configuration and not empty discovery data", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateA))
			f.RunHook()
		})

		It("Should fill values from discovery data", func() {
			Expect(f).To(ExecuteSuccessfully())
			connection := "cloudProviderOpenstack.internal.connection."
			Expect(f.ValuesGet(connection + "authURL").String()).To(Equal("https://cloud.flant.com/v3/"))
			Expect(f.ValuesGet(connection + "domainName").String()).To(Equal("Default"))
			Expect(f.ValuesGet(connection + "tenantName").String()).To(Equal("tenant-name"))
			Expect(f.ValuesGet(connection + "username").String()).To(Equal("user-name"))
			Expect(f.ValuesGet(connection + "password").String()).To(Equal("pa$$word"))
			Expect(f.ValuesGet(connection + "region").String()).To(Equal("HetznerFinland"))
			internal := "cloudProviderOpenstack.internal."
			Expect(f.ValuesGet(internal + "internalNetworkNames").String()).To(MatchYAML(`
[internal]
`))
			Expect(f.ValuesGet(internal + "externalNetworkNames").String()).To(MatchYAML(`
[additional-ext-net, external]
`))
			Expect(f.ValuesGet(internal + "externalNetworkDHCP").Bool()).To(BeTrue())
			Expect(f.ValuesGet(internal + "zones").String()).To(MatchYAML(`
["zone1", "zone2"]
`))
			Expect(f.ValuesGet(internal + "podNetworkMode").String()).To(Equal("DirectRoutingWithPortSecurityEnabled"))
			Expect(f.ValuesGet(internal + "instances").String()).To(MatchYAML(`
"imageName": "ubuntu"
"mainNetwork": "kube"
"sshKeyPairName": "my-key"
"securityGroups": [
  "default",
  "ssh-and-ping",
  "security_group_1"
]
`))
			Expect(f.ValuesGet(internal + "loadBalancer").String()).To(MatchYAML(`
subnetID: "subnetID"
floatingNetworkID: "floatingNetworkID"
`))
			Expect(f.ValuesGet(internal + "tags").String()).To(MatchYAML(`
project: default
env: production
`))
		})
	})

	b := HookExecutionConfigInit(initValuesStringB, `{}`)
	Context("BeforeHelm", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.GenerateBeforeHelmContext())
			b.RunHook()
		})

		It("Should fill values from cloudProviderOpenstack", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("cloudProviderOpenstack.internal").String()).To(MatchYAML(`
connection:
  authURL: https://test.tests.com:5000/v3/
  domainName: default
  tenantName: default
  username: jamie
  password: nein
  region: HetznerFinland
externalNetworkNames: [additional-ext-net, public1, public2]
externalNetworkDHCP: true
internalNetworkNames: [int1, int2]
podNetworkMode: DirectRouting
instances:
  sshKeyPairName: my-ssh-keypair
  securityGroups:
  - security_group_1
  - security_group_2
zones: []
loadBalancer:
  subnetID: overrideSubnetID
tags:
  aaa: bbb
  ccc: ddd
`))
		})
	})

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.KubeStateSet(""))
			b.RunHook()
		})
		It("Should fill values from cloudProviderOpenstack", func() {
			Expect(b).To(ExecuteSuccessfully())
			connection := "cloudProviderOpenstack.internal.connection."
			Expect(b.ValuesGet(connection + "authURL").String()).To(Equal("https://test.tests.com:5000/v3/"))
			Expect(b.ValuesGet(connection + "domainName").String()).To(Equal("default"))
			Expect(b.ValuesGet(connection + "tenantName").String()).To(Equal("default"))
			Expect(b.ValuesGet(connection + "username").String()).To(Equal("jamie"))
			Expect(b.ValuesGet(connection + "password").String()).To(Equal("nein"))
			Expect(b.ValuesGet(connection + "region").String()).To(Equal("HetznerFinland"))
			internal := "cloudProviderOpenstack.internal."
			Expect(b.ValuesGet(internal + "internalNetworkNames").String()).To(MatchYAML(`
[int1, int2]
`))
			Expect(b.ValuesGet(internal + "externalNetworkNames").String()).To(MatchYAML(`
[additional-ext-net, public1, public2]
`))
			Expect(b.ValuesGet(internal + "zones").String()).To(MatchYAML("[]"))
			Expect(b.ValuesGet(internal + "podNetworkMode").String()).To(Equal("DirectRouting"))
			Expect(b.ValuesGet(internal + "instances.securityGroups").String()).To(MatchYAML(`
[security_group_1, security_group_2]
`))
			Expect(b.ValuesGet(internal + "loadBalancer").String()).To(MatchYAML(`
subnetID: overrideSubnetID
`))
			Expect(b.ValuesGet(internal + "tags").String()).To(MatchYAML(`
aaa: bbb
ccc: ddd
`))
		})

		Context("Cluster has cloudProviderOpenstack and discovery data", func() {
			BeforeEach(func() {
				b.BindingContexts.Set(b.KubeStateSet(stateA))
				b.RunHook()
			})

			It("Should override values from discovery data with cloudProviderOpenstack configuration", func() {
				Expect(b).To(ExecuteSuccessfully())
				connection := "cloudProviderOpenstack.internal.connection."
				Expect(b.ValuesGet(connection + "authURL").String()).To(Equal("https://test.tests.com:5000/v3/"))
				Expect(b.ValuesGet(connection + "domainName").String()).To(Equal("default"))
				Expect(b.ValuesGet(connection + "tenantName").String()).To(Equal("default"))
				Expect(b.ValuesGet(connection + "username").String()).To(Equal("jamie"))
				Expect(b.ValuesGet(connection + "password").String()).To(Equal("nein"))
				Expect(b.ValuesGet(connection + "region").String()).To(Equal("HetznerFinland"))
				internal := "cloudProviderOpenstack.internal."
				Expect(b.ValuesGet(internal + "internalNetworkNames").String()).To(MatchYAML(`
[int1, int2]
`))
				Expect(b.ValuesGet(internal + "externalNetworkNames").String()).To(MatchYAML(`
[additional-ext-net, public1, public2]
`))
				Expect(b.ValuesGet(internal + "zones").String()).To(MatchYAML(`
["zone1", "zone2"]
`))
				Expect(b.ValuesGet(internal + "podNetworkMode").String()).To(Equal("DirectRouting"))
				Expect(b.ValuesGet(internal + "instances").String()).To(MatchYAML(`
securityGroups:
- security_group_1
- security_group_2
sshKeyPairName: my-ssh-keypair
`))
				Expect(b.ValuesGet(internal + "loadBalancer").String()).To(MatchYAML(`
subnetID: overrideSubnetID
`))
				Expect(b.ValuesGet(internal + "tags").String()).To(MatchYAML(`
aaa: bbb
ccc: ddd
`))
			})
		})
	})

	Context("Cluster has cloudProviderOpenstack and empty discovery data", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.KubeStateSet(stateB))
			b.RunHook()
		})

		It("Should fill values from cloudProviderOpenstack", func() {
			Expect(b).To(ExecuteSuccessfully())
			connection := "cloudProviderOpenstack.internal.connection."
			Expect(b.ValuesGet(connection + "authURL").String()).To(Equal("https://test.tests.com:5000/v3/"))
			Expect(b.ValuesGet(connection + "domainName").String()).To(Equal("default"))
			Expect(b.ValuesGet(connection + "tenantName").String()).To(Equal("default"))
			Expect(b.ValuesGet(connection + "username").String()).To(Equal("jamie"))
			Expect(b.ValuesGet(connection + "password").String()).To(Equal("nein"))
			Expect(b.ValuesGet(connection + "region").String()).To(Equal("HetznerFinland"))
			internal := "cloudProviderOpenstack.internal."
			Expect(b.ValuesGet(internal + "internalNetworkNames").String()).To(MatchYAML(`
[int1, int2]
`))
			Expect(b.ValuesGet(internal + "externalNetworkNames").String()).To(MatchYAML(`
[additional-ext-net, public1, public2]
`))
			Expect(b.ValuesGet(internal + "podNetworkMode").String()).To(Equal("DirectRouting"))
			Expect(b.ValuesGet(internal + "instances.securityGroups").String()).To(MatchYAML(`
[security_group_1, security_group_2]
`))
			Expect(b.ValuesGet(internal + "loadBalancer").String()).To(MatchYAML(`
subnetID: overrideSubnetID
`))
			Expect(b.ValuesGet(internal + "tags").String()).To(MatchYAML(`
aaa: bbb
ccc: ddd
`))
		})
	})

	c := HookExecutionConfigInit(initValuesStringC, `{}`)
	Context("Cluster has cloudProviderOpenstack and discovery data without loadbalancers", func() {
		BeforeEach(func() {
			c.BindingContexts.Set(c.KubeStateSet(stateAWithoutLoadbalancers))
			c.RunHook()
		})

		It("Should override values from discovery data with cloudProviderOpenstack configuration", func() {
			Expect(c).To(ExecuteSuccessfully())
			connection := "cloudProviderOpenstack.internal.connection."
			Expect(c.ValuesGet(connection + "authURL").String()).To(Equal("https://test.tests.com:5000/v3/"))
			Expect(c.ValuesGet(connection + "domainName").String()).To(Equal("default"))
			Expect(c.ValuesGet(connection + "tenantName").String()).To(Equal("default"))
			Expect(c.ValuesGet(connection + "username").String()).To(Equal("jamie"))
			Expect(c.ValuesGet(connection + "password").String()).To(Equal("nein"))
			Expect(c.ValuesGet(connection + "region").String()).To(Equal("HetznerFinland"))
			internal := "cloudProviderOpenstack.internal."
			Expect(c.ValuesGet(internal + "internalNetworkNames").String()).To(MatchYAML(`
[int1, int2]
`))
			Expect(c.ValuesGet(internal + "externalNetworkNames").String()).To(MatchYAML(`
[public1, public2]
`))
			Expect(c.ValuesGet(internal + "zones").String()).To(MatchYAML(`
["zone1", "zone2"]
`))
			Expect(c.ValuesGet(internal + "podNetworkMode").String()).To(Equal("DirectRouting"))
			Expect(c.ValuesGet(internal + "instances").String()).To(MatchYAML(`
securityGroups:
- security_group_1
- security_group_2
sshKeyPairName: my-ssh-keypair
`))
			Expect(c.ValuesGet(internal + "loadBalancer").String()).To(MatchYAML(`{}`))
			Expect(c.ValuesGet(internal + "tags").String()).To(MatchYAML(`
project: default
env: production
`))
		})
	})

	d := HookExecutionConfigInit(initValuesStringA, `{}`)

	Context("Cluster has SimpleWithInternalNetworkLayout without DHCP and not empty discovery data", func() {
		BeforeEach(func() {
			d.BindingContexts.Set(f.KubeStateSet(stateASimpleWithInternalNetworkWithoutDHCP))
			d.RunHook()
		})

		It("Should fill values from discovery data", func() {
			Expect(d).To(ExecuteSuccessfully())
			connection := "cloudProviderOpenstack.internal.connection."
			Expect(d.ValuesGet(connection + "authURL").String()).To(Equal("https://cloud.flant.com/v3/"))
			Expect(d.ValuesGet(connection + "domainName").String()).To(Equal("Default"))
			Expect(d.ValuesGet(connection + "tenantName").String()).To(Equal("tenant-name"))
			Expect(d.ValuesGet(connection + "username").String()).To(Equal("user-name"))
			Expect(d.ValuesGet(connection + "password").String()).To(Equal("pa$$word"))
			Expect(d.ValuesGet(connection + "region").String()).To(Equal("HetznerFinland"))
			internal := "cloudProviderOpenstack.internal."
			Expect(d.ValuesGet(internal + "internalNetworkNames").String()).To(MatchYAML(`
[internal]
`))
			Expect(d.ValuesGet(internal + "externalNetworkNames").String()).To(MatchYAML(`
[additional-ext-net, external]
`))
			Expect(d.ValuesGet(internal + "externalNetworkDHCP").Bool()).To(BeFalse())
			Expect(d.ValuesGet(internal + "zones").String()).To(MatchYAML(`
["zone1", "zone2"]
`))
			Expect(d.ValuesGet(internal + "podNetworkMode").String()).To(Equal("DirectRoutingWithPortSecurityEnabled"))
			Expect(d.ValuesGet(internal + "instances").String()).To(MatchYAML(`
"imageName": "ubuntu"
"mainNetwork": "kube"
"sshKeyPairName": "my-key"
"securityGroups": [
  "default",
  "ssh-and-ping",
  "security_group_1"
]
`))
			Expect(d.ValuesGet(internal + "loadBalancer").String()).To(MatchYAML(`
subnetID: "subnetID"
floatingNetworkID: "floatingNetworkID"
`))
			Expect(d.ValuesGet(internal + "tags").String()).To(MatchYAML(`
project: default
env: production
`))
		})
	})
})

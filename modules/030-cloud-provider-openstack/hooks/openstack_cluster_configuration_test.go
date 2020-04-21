package hooks

import (
	"encoding/base64"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery/cluster_dns_address ::", func() {
	const (
		initValuesStringA = `
global:
  discovery": {}
cloudProviderOpenstack:
  internal:
    instances: {}
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
  internalNetworkNames: [int1, int2]
  podNetworkMode: DirectRouting
  instances:
    sshKeyPairName: my-ssh-keypair
    securityGroups:
    - security_group_1
    - security_group_2
  internalSubnet: "10.0.201.0/16"
`
	)

	var (
		stateACloudDiscoveryData = `
{
  "externalNetworkNames": [
    "external"
  ],
  "instances": {
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
  "zones": ["zone1", "zone2"]
}
`
		stateAClusterConfiguration = `
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackClusterConfiguration
spec:
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
`
		stateA = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: d8-system
data:
  "openstack-cluster-configuration.yaml": %s
  "openstack-cloud-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(stateAClusterConfiguration)), base64.StdEncoding.EncodeToString([]byte(stateACloudDiscoveryData)))

		stateB = `
apiVersion: v1
kind: Secret
metadata:
 name: d8-cluster-configuration
 namespace: d8-system
data: {}
`
	)

	f := HookExecutionConfigInit(initValuesStringA, `{}`)

	Context("Cluster without cloudProviderOpenstack config", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateA))
			f.RunHook()
		})

		It("Should correctly fill the Values store from it", func() {
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
[external]
`))
			Expect(f.ValuesGet(internal + "zones").String()).To(MatchYAML(`
["zone1", "zone2"]
`))
			Expect(f.ValuesGet(internal + "podNetworkMode").String()).To(Equal("DirectRoutingWithPortSecurityEnabled"))
			Expect(f.ValuesGet(internal + "instances.securityGroups").String()).To(MatchYAML(`
[default, security_group_1, ssh-and-ping]
`))
		})
	})

	b := HookExecutionConfigInit(initValuesStringB, `{}`)
	Context("BeforeHelm", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(BeforeHelmContext)
			b.RunHook()
		})

		It("Should correctly fill the Values store from cloudProviderOpenstack", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("cloudProviderOpenstack.internal").String()).To(MatchYAML(`
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
zones: []
`))
		})
	})

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.KubeStateSet(""))
			b.RunHook()
		})
		It("Should correctly fill the Values store from cloudProviderOpenstack", func() {
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
[public1, public2]
`))
			Expect(b.ValuesGet(internal + "zones").String()).To(MatchYAML("[]"))
			Expect(b.ValuesGet(internal + "podNetworkMode").String()).To(Equal("DirectRouting"))
			Expect(b.ValuesGet(internal + "instances.securityGroups").String()).To(MatchYAML(`
[security_group_1, security_group_2]
`))
		})

		Context("Cluster has cloudProviderOpenstack", func() {
			BeforeEach(func() {
				b.BindingContexts.Set(b.KubeStateSet(stateA))
				b.RunHook()
			})

			It("Should correctly fill the Values store from it", func() {
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
[int1, int2, internal]
`))
				Expect(b.ValuesGet(internal + "externalNetworkNames").String()).To(MatchYAML(`
[external, public1, public2]
`))
				Expect(b.ValuesGet(internal + "zones").String()).To(MatchYAML(`
["zone1", "zone2"]
`))
				Expect(b.ValuesGet(internal + "podNetworkMode").String()).To(Equal("DirectRouting"))
				Expect(b.ValuesGet(internal + "instances.securityGroups").String()).To(MatchYAML(`
[default, security_group_1, security_group_2, ssh-and-ping]
`))
			})
		})
	})

	Context("Cluster has cloudProviderOpenstack with empty secret", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.KubeStateSet(stateB))
			b.RunHook()
		})

		It("Should correctly fill the Values store from it", func() {
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
[public1, public2]
`))
			Expect(b.ValuesGet(internal + "podNetworkMode").String()).To(Equal("DirectRouting"))
			Expect(b.ValuesGet(internal + "instances.securityGroups").String()).To(MatchYAML(`
[security_group_1, security_group_2]
`))
		})
	})
})

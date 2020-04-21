package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-openstack :: hooks :: migrate_configuration ::", func() {
	const (
		config = `
cloudProviderOpenstack:
  authURL: https://test.tests.com:5000/v3/
  domainName: default
  tenantName: default
  username: jamie
  password: nein
  region: HetznerFinland
  networkName: public
  addPodSubnetToPortWhitelist: true
  internalNetworkName: kube
  sshKeyPairName: my-ssh-keypair
  securityGroups:
  - default
  - allow-ssh-and-icmp
  internalSubnet: "10.0.201.0/16"
  instances: {}
`
	)
	f := HookExecutionConfigInit(`{"cloudProviderOpenstack":{"internal":{}}}`, config)

	Context("Startup", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(OnStartupContext)
			f.RunHook()
		})

		It("Hook must not fail and config must be migrated", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ConfigValuesGet("cloudProviderOpenstack")).To(MatchYAML(`
connection:
  authURL: https://test.tests.com:5000/v3/
  domainName: default
  tenantName: default
  username: jamie
  password: nein
  region: HetznerFinland
externalNetworkNames: [public]
internalNetworkNames: [kube]
podNetworkMode: DirectRoutingWithPortSecurityEnabled
instances:
  sshKeyPairName: my-ssh-keypair
  securityGroups:
  - default
  - allow-ssh-and-icmp
internalSubnet: "10.0.201.0/16"
`))
		})
	})
})

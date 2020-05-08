package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-openstack :: hooks :: migrate_openstack_instance_class ::", func() {
	const (
		config = `
cloudProviderOpenstack:
  connection:
    authURL: https://test.tests.com:5000/v3/
    domainName: default
    tenantName: default
    username: jamie
    password: nein
    region: HetznerFinland
  externalNetworkNames: [public]
  internalNetworkNames: [int1, int2]
  podNetworkMode: DirectRoutingWithPortSecurityEnabled
  instances:
    sshKeyPairName: my-ssh-keypair
    securityGroups:
    - default
    - allow-ssh-and-icmp
  internalSubnet: "10.0.201.0/16"
`
		newIc = `
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackInstanceClass
metadata:
  name: worker_new
spec:
  bashible:
    bundle: ubuntu-18.04-1.0
  options:
    kubernetesVersion: 1.16.6
  flavorName: m1.medium
  imageName: ubuntu-18-04-cloud-amd64
  mainNetwork: public
  additionalNetworks:
  - int_net_1
  - int_net_2
`
		oldIc = `
---
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackInstanceClass
metadata:
  name: worker_new
spec:
  bashible:
    bundle: ubuntu-18.04-1.0
  options:
    kubernetesVersion: 1.16.6
  flavorName: m1.medium
  imageName: ubuntu-18-04-cloud-amd64
  mainNetwork: external
  additionalNetworks:
  - int_net_1
  - int_net_2
---
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackInstanceClass
metadata:
  name: worker
spec:
  bashible:
    bundle: ubuntu-18.04-1.0
  options:
    kubernetesVersion: 1.16.6
  flavorName: m1.medium
  imageName: ubuntu-18-04-cloud-amd64
---
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackInstanceClass
metadata:
  name: worker2
spec:
  bashible:
    bundle: ubuntu-18.04-1.0
  options:
    kubernetesVersion: 1.16.6
  flavorName: m1.medium
  imageName: ubuntu-18-04-cloud-amd64
`
	)
	f := HookExecutionConfigInit(`{"cloudProviderOpenstack":{"internal":{}}}`, config)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "OpenStackInstanceClass", false)

	Context("Cluster with new openstack instance class format", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(newIc))
			f.RunHook()

			It("Hook must not fail and nothing to migrate", func() {
				Expect(f).To(ExecuteSuccessfully())
			})
		})
	})

	Context("Cluster with old openstack instance class format", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(oldIc))
			f.RunHook()
		})

		It("Hook must not fail and migrate runs successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			for _, icName := range []string{"worker", "worker2"} {
				ic := f.KubernetesGlobalResource("OpenStackInstanceClass", icName)
				Expect(ic.Exists()).To(BeTrue())
				Expect(ic.Field("spec.mainNetwork").String()).To(Equal("public"))
				Expect(ic.Field("spec.additionalNetworks").String()).To(MatchYAML(`[int1, int2]`))
			}
			workerNew := f.KubernetesGlobalResource("OpenStackInstanceClass", "worker_new")
			Expect(workerNew.Exists()).To(BeTrue())
			Expect(workerNew.Field("spec.mainNetwork").String()).To(Equal("external"))
			Expect(workerNew.Field("spec.additionalNetworks").String()).To(MatchYAML(`[int_net_1, int_net_2]`))
		})
	})
})

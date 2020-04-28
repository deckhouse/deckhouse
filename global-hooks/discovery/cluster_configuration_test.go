package hooks

import (
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery/cluster_dns_address ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	var (
		stateAClusterConfiguration = `
apiVersion: deckhouse.io/v1alpha1
kind: ClusterConfiguration
clusterType: Static
cloud:
  provider: OpenStack
  prefix: kube
podSubnetCIDR: 10.111.0.0/16
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.15"
`
		stateA = `
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cluster-configuration.yaml": ` + base64.StdEncoding.EncodeToString([]byte(stateAClusterConfiguration))

		stateBClusterConfiguration = `
apiVersion: deckhouse.io/v1alpha1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: AWS
  prefix: lube
podSubnetCIDR: 10.122.0.0/16
podSubnetNodeCIDRPrefix: "26"
serviceSubnetCIDR: 10.213.0.0/16
kubernetesVersion: "1.18"
`
		stateB = `
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cluster-configuration.yaml": ` + base64.StdEncoding.EncodeToString([]byte(stateBClusterConfiguration))
	)

	Context("Cluster has a d8-cluster-configuration Secret", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateA))
			f.RunHook()
		})

		It("Should correctly fill the Values store from it", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.clusterConfiguration.clusterType").String()).To(Equal("Static"))
			Expect(f.ValuesGet("global.clusterConfiguration.cloud.provider").String()).To(Equal("OpenStack"))
			Expect(f.ValuesGet("global.clusterConfiguration.cloud.prefix").String()).To(Equal("kube"))
			Expect(f.ValuesGet("global.clusterConfiguration.podSubnetCIDR").String()).To(Equal("10.111.0.0/16"))
			Expect(f.ValuesGet("global.clusterConfiguration.podSubnetNodeCIDRPrefix").String()).To(Equal("24"))
			Expect(f.ValuesGet("global.clusterConfiguration.serviceSubnetCIDR").String()).To(Equal("10.222.0.0/16"))
			Expect(f.ValuesGet("global.clusterConfiguration.kubernetesVersion").String()).To(Equal("1.15"))

			Expect(f.ValuesGet("global.discovery.podSubnet").String()).To(Equal("10.111.0.0/16"))
			Expect(f.ValuesGet("global.discovery.serviceSubnet").String()).To(Equal("10.222.0.0/16"))
		})

		Context("d8-cluster-configuration Secret has changed", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateB))
				f.RunHook()
			})

			It("Should correctly fill the Values store from it", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.clusterConfiguration.clusterType").String()).To(Equal("Cloud"))
				Expect(f.ValuesGet("global.clusterConfiguration.cloud.provider").String()).To(Equal("AWS"))
				Expect(f.ValuesGet("global.clusterConfiguration.cloud.prefix").String()).To(Equal("lube"))
				Expect(f.ValuesGet("global.clusterConfiguration.podSubnetCIDR").String()).To(Equal("10.122.0.0/16"))
				Expect(f.ValuesGet("global.clusterConfiguration.podSubnetNodeCIDRPrefix").String()).To(Equal("26"))
				Expect(f.ValuesGet("global.clusterConfiguration.serviceSubnetCIDR").String()).To(Equal("10.213.0.0/16"))
				Expect(f.ValuesGet("global.clusterConfiguration.kubernetesVersion").String()).To(Equal("1.18"))

				Expect(f.ValuesGet("global.discovery.podSubnet").String()).To(Equal("10.122.0.0/16"))
				Expect(f.ValuesGet("global.discovery.serviceSubnet").String()).To(Equal("10.213.0.0/16"))
			})
		})

		Context("d8-cluster-configuration Secret got deleted", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(""))
				f.RunHook()
			})

			It("Should not fail, but should not create any Values", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(f.ValuesGet("global.clusterConfiguration").Exists()).To(Not(BeTrue()))
			})
		})
	})

	Context("Cluster doesn't have a d8-cluster-configuration Secret", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Should not fail, but should not create any Values", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("global.clusterConfiguration").Exists()).To(Not(BeTrue()))
		})
	})
})

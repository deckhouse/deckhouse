/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template_tests

import (
	"encoding/base64"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const globalValues = `
  enabledModules: ["vertical-pod-autoscaler-crd", "cloud-provider-vcd"]
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: sandbox
      provider: VCD
    clusterDomain: cluster.local
    clusterType: Cloud
    defaultCRI: Containerd
    kind: ClusterConfiguration
    kubernetesVersion: "1.25"
    podSubnetCIDR: 10.111.0.0/16
    podSubnetNodeCIDRPrefix: "24"
    serviceSubnetCIDR: 10.222.0.0/16
  modules:
    placement: {}
  discovery:
    d8SpecificNodeCountByRole:
      worker: 1
      master: 3
    podSubnet: 10.0.1.0/16
    kubernetesVersion: 1.25.1
    clusterUUID: cluster
`

const moduleValuesA = `
    internal:
      capcdControllerManagerWebhookCert:
        ca: ca
        crt: crt
        key: key
      providerDiscoveryData:
        kind: VCDCloudProviderDiscoveryData
        apiVersion: deckhouse.io/v1
        zones:
        - default
      discoveryData:
        kind: VCDCloudProviderDiscoveryData
        apiVersion: deckhouse.io/v1
        version:
          vcdVersion: "10.4.2"
          apiVersion: "37.2"
      providerClusterConfiguration:
        apiVersion: deckhouse.io/v1
        kind: VCDClusterConfiguration
        provider:
          username: myuname
          password: myPaSsWd
          insecure: true
          server: "http://server/api/"
        layout: Standard
        sshPublicKey: rsa-aaaa
        organization: org
        virtualDataCenter: dc
        virtualApplicationName: v1rtual-app
        mainNetwork: internal
        masterNodeGroup:
          replicas: 1
          instanceClass:
            template: Templates/ubuntu-focal-20.04
            sizingPolicy: 4cpu8ram
            rootDiskSizeGb: 20
            etcdDiskSizeGb: 20
            storageProfile: nvme
`

var _ = Describe("Module :: cloud-provider-vcd :: helm template ::", func() {
	f := SetupHelmConfig(``)
	BeforeSuite(func() {
		err := os.Remove("/deckhouse/ee/modules/030-cloud-provider-vcd/candi")
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Symlink("/deckhouse/ee/candi/cloud-providers/vcd", "/deckhouse/ee/modules/030-cloud-provider-vcd/candi")
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterSuite(func() {
		err := os.Remove("/deckhouse/ee/modules/030-cloud-provider-vcd/candi")
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Symlink("/deckhouse/candi/cloud-providers/vcd", "/deckhouse/ee/modules/030-cloud-provider-vcd/candi")
		Expect(err).ShouldNot(HaveOccurred())
	})

	Context("VCD", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderVcd", moduleValuesA)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			regSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")
			Expect(regSecret.Exists()).To(BeTrue())
			Expect(regSecret.Field("data.capiClusterName").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("v1rtual-app"))))
		})
	})
})

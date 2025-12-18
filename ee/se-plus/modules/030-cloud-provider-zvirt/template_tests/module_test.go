/*
Copyright 2025 Flant JSC
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

// fake *-crd modules are required for backward compatibility with lib_helm library
// TODO: remove fake crd modules
const globalValues = `
  clusterIsBootstrapped: true
  enabledModules: ["vertical-pod-autoscaler", "vertical-pod-autoscaler-crd", "cloud-provider-zvirt"]
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: sandbox
      provider: Zvirt
    clusterDomain: cluster.local
    clusterType: Cloud
    defaultCRI: Containerd
    kind: ClusterConfiguration
    kubernetesVersion: "1.30"
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
    kubernetesVersion: 1.30.0
    clusterUUID: cluster
`

const moduleValuesA = `
internal:
  providerClusterConfiguration:
    apiVersion: deckhouse.io/v1
    clusterID: 6f0ce074-3a26-11f0-ab77-00163e2d8193
    kind: ZvirtClusterConfiguration
    layout: Standard
    masterNodeGroup:
      instanceClass:
        etcdDiskSizeGb: 10
        memory: 8192
        numCPUs: 4
        rootDiskSizeGb: 50
        storageDomainID: fdc40068-1975-46a3-a1db-7b3731316d87
        template: awesome-template
        vnicProfileID: ad0bfe09-f7a3-4f88-b6af-b71680a82ca4
      replicas: 1
    provider:
      caBundle: ""
      insecure: true
      password: imsostrong
      server: https://zvirt.example.com/api
      username: user
    sshPublicKey: ssh-rsa deadbeef
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: ZvirtCloudProviderDiscoveryData
    storageDomains: []
    zones:
      - default`

const tolerationsAnyNodeWithUninitialized = `
- key: node-role.kubernetes.io/master
- key: node-role.kubernetes.io/control-plane
- key: node.deckhouse.io/etcd-arbiter
- key: dedicated.deckhouse.io
  operator: "Exists"
- key: dedicated
  operator: "Exists"
- key: DeletionCandidateOfClusterAutoscaler
- key: ToBeDeletedByClusterAutoscaler
- key: drbd.linbit.com/lost-quorum
- key: drbd.linbit.com/force-io-error
- key: drbd.linbit.com/ignore-fail-over
- effect: NoSchedule
  key: node.deckhouse.io/bashible-uninitialized
  operator: Exists
- effect: NoSchedule
  key: node.deckhouse.io/uninitialized
  operator: Exists
- key: ToBeDeletedTaint
  operator: Exists
- effect: NoSchedule
  key: node.deckhouse.io/csi-not-bootstrapped
  operator: Exists
- key: node.kubernetes.io/not-ready
- key: node.kubernetes.io/out-of-disk
- key: node.kubernetes.io/memory-pressure
- key: node.kubernetes.io/disk-pressure
- key: node.kubernetes.io/pid-pressure
- key: node.kubernetes.io/unreachable
- key: node.kubernetes.io/network-unavailable`

var _ = Describe("Module :: cloud-provider-zvirt :: helm template ::", func() {
	f := SetupHelmConfig(``)
	BeforeSuite(func() {
		err := os.Remove("/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/candi")
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Symlink("/deckhouse/ee/se-plus/candi/cloud-providers/zvirt", "/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/candi")
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterSuite(func() {
		err := os.Remove("/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/candi")
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Symlink("/deckhouse/ee/se-plus/candi/cloud-providers/zvirt", "/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/candi")
		Expect(err).ShouldNot(HaveOccurred())
	})

	Context("zVirt Suite A", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderZvirt", moduleValuesA)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			regSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")
			Expect(regSecret.Exists()).To(BeTrue())
			Expect(regSecret.Field("data.capiClusterName").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("zvirt"))))

			ccmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-zvirt", "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.spec.containers.0.args").String()).To(MatchYAML(`
- --leader-elect=true
- --cloud-provider=zvirt
- --allow-untagged-cloud=true
- --configure-cloud-routes=false
- --controllers=cloud-node,cloud-node-lifecycle
- --bind-address=127.0.0.1
- --secure-port=10471
- --v=4`))

			csiControllerDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-zvirt", "csi-controller")
			Expect(csiControllerDeployment.Exists()).To(BeTrue())
			Expect(csiControllerDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))

			csiNodeDaemonSet := f.KubernetesResource("DaemonSet", "d8-cloud-provider-zvirt", "csi-node")
			Expect(csiNodeDaemonSet.Exists()).To(BeTrue())
			Expect(csiNodeDaemonSet.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))

			cddDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-zvirt", "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())
			Expect(cddDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(cddDeployment.Field("spec.template.spec.tolerations").String()).To(MatchYAML(tolerationsAnyNodeWithUninitialized))

		})
	})
})

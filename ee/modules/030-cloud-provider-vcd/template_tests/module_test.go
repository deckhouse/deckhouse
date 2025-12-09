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

// fake *-crd modules are required for backward compatibility with lib_helm library
// TODO: remove fake crd modules
const globalValues = `
  clusterIsBootstrapped: true
  enabledModules: ["vertical-pod-autoscaler", "vertical-pod-autoscaler-crd", "cloud-provider-vcd"]
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: sandbox
      provider: VCD
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
        vcdInstallationVersion: "10.4.2"
        vcdAPIVersion: "37.2"
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
            affinityRule:
              polarity: AntiAffinity
            template: Templates/ubuntu-focal-20.04
            sizingPolicy: 4cpu8ram
            rootDiskSizeGb: 20
            etcdDiskSizeGb: 20
            storageProfile: nvme
        nodeGroups:
        - name: front
          replicas: 3
          instanceClass:
            rootDiskSizeGb: 20
            sizingPolicy: 16cpu32ram
            template: Templates/ubuntu-focal-20.04
            storageProfile: nvme
            affinityRule:
              polarity: AntiAffinity
              required: false
      affinityRules:
      - nodeGroupName: master
        polarity: AntiAffinity
      - nodeGroupName: front
        polarity: AntiAffinity
        required: false
      - nodeGroupName: ephemeral-node
        polarity: Affinity
        required: true
`

const moduleValuesB = `
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
        vcdInstallationVersion: "10.4.2"
        vcdAPIVersion: "37.2"
        loadBalancer:
          enabled: false
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

const moduleValuesC = `
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
        vcdInstallationVersion: "10.4.2"
        vcdAPIVersion: "37.2"
        loadBalancer:
          enabled: true
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

const tolerationsAnyNodeWithUninitialized = `
- key: node-role.kubernetes.io/master
- key: node-role.kubernetes.io/control-plane
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

	Context("VCD Suite A", func() {
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

			masterAffinityRule := f.KubernetesGlobalResource("VCDAffinityRule", "sandbox-master")
			Expect(masterAffinityRule.Exists()).To(BeTrue())
			Expect(masterAffinityRule.Parse().String()).To(MatchYAML(`
apiVersion: deckhouse.io/v1alpha1
kind: VCDAffinityRule
metadata:
  name: sandbox-master
  labels:
    heritage: deckhouse
    module: cloud-provider-vcd
spec:
  nodeLabelSelector:
    matchLabels:
      node.deckhouse.io/group: master
  polarity: "AntiAffinity"
  required: false
`))

			frontAffinityRule := f.KubernetesGlobalResource("VCDAffinityRule", "sandbox-front")
			Expect(frontAffinityRule.Exists()).To(BeTrue())
			Expect(frontAffinityRule.Parse().String()).To(MatchYAML(`
apiVersion: deckhouse.io/v1alpha1
kind: VCDAffinityRule
metadata:
  name: sandbox-front
  labels:
    heritage: deckhouse
    module: cloud-provider-vcd
spec:
  nodeLabelSelector:
    matchLabels:
      node.deckhouse.io/group: front
  polarity: "AntiAffinity"
  required: false
`))

			ephemeralAffinityRule := f.KubernetesGlobalResource("VCDAffinityRule", "sandbox-ephemeral-node")
			Expect(ephemeralAffinityRule.Exists()).To(BeTrue())
			Expect(ephemeralAffinityRule.Parse().String()).To(MatchYAML(`
apiVersion: deckhouse.io/v1alpha1
kind: VCDAffinityRule
metadata:
  name: sandbox-ephemeral-node
  labels:
    heritage: deckhouse
    module: cloud-provider-vcd
spec:
  nodeLabelSelector:
    matchLabels:
      node.deckhouse.io/group: ephemeral-node
  polarity: "Affinity"
  required: true
`))

			ccmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.spec.containers.0.args").String()).To(MatchYAML(`
- --leader-elect=true
- --cloud-config=/etc/cloud/cloud-config
- --cloud-provider=vmware-cloud-director
- --allow-untagged-cloud=true
- --configure-cloud-routes=false
- --controllers=cloud-node,cloud-node-lifecycle
- --bind-address=127.0.0.1
- --secure-port=10471
- --v=4`))

			csiControllerDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "csi-controller")
			Expect(csiControllerDeployment.Exists()).To(BeTrue())
			Expect(csiControllerDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))

			csiNodeDaemonSet := f.KubernetesResource("DaemonSet", "d8-cloud-provider-vcd", "csi-node")
			Expect(csiNodeDaemonSet.Exists()).To(BeTrue())
			Expect(csiNodeDaemonSet.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))

			cddDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())
			Expect(cddDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(cddDeployment.Field("spec.template.spec.tolerations").String()).To(MatchYAML(tolerationsAnyNodeWithUninitialized))

			icmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "infra-controller-manager")
			Expect(icmDeployment.Exists()).To(BeTrue())
			Expect(icmDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(icmDeployment.Field("spec.template.spec.tolerations").String()).To(MatchYAML(tolerationsAnyNodeWithUninitialized))
		})
	})

	Context("VCD Suite B", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderVcd", moduleValuesB)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.spec.containers.0.args").String()).To(MatchYAML(`
- --leader-elect=true
- --cloud-config=/etc/cloud/cloud-config
- --cloud-provider=vmware-cloud-director
- --allow-untagged-cloud=true
- --configure-cloud-routes=false
- --controllers=cloud-node,cloud-node-lifecycle
- --bind-address=127.0.0.1
- --secure-port=10471
- --v=4
`))
		})
	})

	Context("VCD Suite C", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderVcd", moduleValuesC)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.spec.containers.0.args").String()).To(MatchYAML(`
- --leader-elect=true
- --cloud-config=/etc/cloud/cloud-config
- --cloud-provider=vmware-cloud-director
- --allow-untagged-cloud=true
- --configure-cloud-routes=false
- --controllers=cloud-node,cloud-node-lifecycle,service
- --bind-address=127.0.0.1
- --secure-port=10471
- --v=4
`))
		})
	})
})

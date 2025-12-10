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
  enabledModules: ["vertical-pod-autoscaler", "vertical-pod-autoscaler-crd", "cloud-provider-huaweicloud"]
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: sandbox
      provider: Huaweicloud
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
  cniSecretData: "base64-encoded-string-or-placeholder"
  providerClusterConfiguration:
    apiVersion: deckhouse.io/v1
    kind: HuaweiCloudClusterConfiguration
    layout: Standard
    sshPublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCu..."
    zones:
      - eu-3a
    provider:
      cloud: huaweicloud.example.com
      region: eu-3
      accessKey: "YOUR_ACCESS_KEY"
      secretKey: "YOUR_SECRET_KEY"
      domainName: "example.com"
      insecure: false
    standard:
      internalNetworkCIDR: 192.168.200.0/24
      internalNetworkDNSServers:
        - 8.8.8.8
        - 8.8.4.4
      internalNetworkSecurity: true
      enableEIP: true
    masterNodeGroup:
      replicas: 3
      instanceClass:
        flavorName: s3.xlarge.2
        imageName: "debian-11-genericcloud-amd64-20220911-1135"
        rootDiskSize: 50
        etcdDiskSizeGb: 10
      volumeTypeMap:
        eu-3a: fast-eu-3a
        eu-3b: fast-eu-3b
      serverGroup:
        policy: AntiAffinity
    nodeGroups:
      - name: front
        replicas: 2
        instanceClass:
          flavorName: m1.large
          imageName: "debian-11-genericcloud-amd64-20220911-1135"
          rootDiskSize: 50
          mainNetwork: "aaaff8f9-26af-43e3-9c49-c4d083e59c61"
          additionalNetworks:
            - "11111111-1111-1111-1111-111111111111"
        zones:
          - eu-1a
          - eu-1b
        volumeTypeMap:
          eu-1a: fast-eu-1a
          eu-1b: fast-eu-1b
        nodeTemplate:
          labels:
            role: frontend
            environment: production
          annotations:
            note: "frontend nodes"
          taints:
            - effect: NoSchedule
              key: front-node
              value: "true"
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: HuaweiCloudDiscoveryData
    layout: Standard
    zones:
      - eu-3a
    instances:
      vpcIPv4SubnetId: "00000000-0000-0000-0000-000000000000"
    volumeTypes:
      - id: "11111111-1111-1111-1111-111111111111"
        name: "ssd"
        isPublic: true
  storageClasses:
    - name: cinder-ssd
      type: ssd
      allowVolumeExpansion: true
    - name: cinder-hdd
      type: hdd
      allowVolumeExpansion: false`

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

var _ = Describe("Module :: cloud-provider-huaweicloud :: helm template ::", func() {
	f := SetupHelmConfig(``)
	BeforeSuite(func() {
		err := os.Remove("/deckhouse/ee/modules/030-cloud-provider-huaweicloud/candi")
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Symlink("/deckhouse/ee/candi/cloud-providers/huaweicloud", "/deckhouse/ee/modules/030-cloud-provider-huaweicloud/candi")
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterSuite(func() {
		err := os.Remove("/deckhouse/ee/modules/030-cloud-provider-huaweicloud/candi")
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Symlink("/deckhouse/candi/cloud-providers/huaweicloud", "/deckhouse/ee/modules/030-cloud-provider-huaweicloud/candi")
		Expect(err).ShouldNot(HaveOccurred())
	})

	Context("HuaweiCloud Suite A", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderHuaweicloud", moduleValuesA)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			regSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")
			Expect(regSecret.Exists()).To(BeTrue())
			Expect(regSecret.Field("data.capiClusterName").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("huaweicloud"))))

			ccmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-huaweicloud", "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.spec.containers.0.args").String()).To(MatchYAML(`
- --leader-elect=true
- --cluster-name=sandbox
- --cluster-cidr=10.0.1.0/16
- --allocate-node-cidrs=true
- --configure-cloud-routes=true
- --cloud-config=/etc/cloud/cloud-config
- --cloud-provider=huaweicloud
- --bind-address=127.0.0.1
- --secure-port=10471
- --tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,TLS_RSA_WITH_AES_128_CBC_SHA,TLS_RSA_WITH_AES_256_CBC_SHA,TLS_RSA_WITH_AES_128_GCM_SHA256,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA
- --v=4`))

			csiControllerDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-huaweicloud", "csi-controller")
			Expect(csiControllerDeployment.Exists()).To(BeTrue())
			Expect(csiControllerDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))

			csiNodeDaemonSet := f.KubernetesResource("DaemonSet", "d8-cloud-provider-huaweicloud", "csi-node")
			Expect(csiNodeDaemonSet.Exists()).To(BeTrue())
			Expect(csiNodeDaemonSet.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))

			cddDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-huaweicloud", "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())
			Expect(cddDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(cddDeployment.Field("spec.template.spec.tolerations").String()).To(MatchYAML(tolerationsAnyNodeWithUninitialized))
		})
	})
})

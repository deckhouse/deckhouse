/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template_tests

import (
	"encoding/base64"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const providerID = "dynamix"
const nameLabelKey = "cloud-provider\\.deckhouse\\.io/name"
const registrationLabelKey = "cloud-provider\\.deckhouse\\.io/registration"
const ephemeralNodesTemplatesLabelKey = "cloud-provider\\.deckhouse\\.io/ephemeral-nodes-templates"
const bashibleLabelKey = "cloud-provider\\.deckhouse\\.io/bashible"

// fake *-crd modules are required for backward compatibility with lib_helm library
// TODO: remove fake crd modules
const globalValues = `
  clusterIsBootstrapped: true
  enabledModules: ["vertical-pod-autoscaler", "vertical-pod-autoscaler-crd", "cloud-provider-dynamix"]
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: sandbox
      provider: Dynamix
    clusterDomain: cluster.local
    clusterType: Cloud
    defaultCRI: Containerd
    kind: ClusterConfiguration
    kubernetesVersion: "1.31"
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
    kubernetesVersion: 1.31.0
    clusterUUID: cluster
`

const moduleValuesA = `
internal:
  cniSecretData: "REVDT0RJUlVZIE9CUkFUTk8gQllTVFJP"
  providerClusterConfiguration:
    apiVersion: deckhouse.io/v1
    kind: DynamixClusterConfiguration
    layout: StandardWithInternalNetwork
    sshPublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCu..."
    location: dynamix
    account: acc_user
    nodeNetworkCIDR: "10.241.32.0/24"
    nameservers:
      - "10.0.0.10"
    provider:
      controllerUrl: "https://controller.example.com"
      oAuth2Url: "https://sso.example.com"
      appId: "example-app-id"
      appSecret: "example-app-secret"
      insecure: true
    masterNodeGroup:
      replicas: 1
      instanceClass:
        numCPUs: 6
        memory: 16384
        rootDiskSizeGb: 50
        etcdDiskSizeGb: 15
        imageName: "dynamix-image-1.0"
        storageEndpoint: "SharedTatlin_G1_SEP"
        pool: "pool_a"
        externalNetwork: "extnet_vlan_1700"
    nodeGroups:
      - name: worker
        replicas: 2
        instanceClass:
          numCPUs: 4
          memory: 8192
          rootDiskSizeGb: 50
          imageName: "dynamix-image-1.0"
          externalNetwork: "extnet_vlan_1700"
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: DynamixCloudProviderDiscoveryData
    zones:
      - zone-1
    storageEndpoints:
      - name: Default
        pools:
          - pool_a
          - pool_b
        isEnabled: true
        isDefault: true
  storageClasses:
    - name: dynamix-ssd
      storageEndpoint: SharedTatlin_G1_SEP
      pool: pool_a
      allowVolumeExpansion: true
    - name: dynamix-hdd
      storageEndpoint: SharedTatlin_G1_SEP
      pool: pool_b
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

var _ = Describe("Module :: cloud-provider-dynamix :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("dynamix Suite A", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDynamix", moduleValuesA)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())
			Expect(providerRegistrationSecret.Field("data.capiClusterName").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte(providerID))))
			Expect(providerRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", registrationLabelKey)).String()).To(Equal(""))
			Expect(providerRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))

			providerSpecificRegistrationSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-node-manager-cloud-provider-%s", providerID))
			Expect(providerSpecificRegistrationSecret.Exists()).To(BeTrue())
			Expect(providerSpecificRegistrationSecret.Field("data.capiClusterName").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte(providerID))))
			Expect(providerSpecificRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", registrationLabelKey)).String()).To(Equal(""))
			Expect(providerSpecificRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))

			providerSpecificCAPISecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-cloud-provider-%s-capi", providerID))
			Expect(providerSpecificCAPISecret.Exists()).To(BeTrue())
			Expect(providerSpecificCAPISecret.Field(fmt.Sprintf("metadata.labels.%s", ephemeralNodesTemplatesLabelKey)).String()).To(Equal("capi"))
			Expect(providerSpecificCAPISecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))
			providerSpecificCAPISecretData := providerSpecificCAPISecret.Field("data").Map()
			Expect(providerSpecificCAPISecretData).To(Not(BeEmpty()))
			Expect(len(providerSpecificCAPISecretData) >= 1).To(BeTrue())
			Expect(len(providerSpecificCAPISecretData["cluster.yaml"].String()) > 0).To(BeTrue())

			providerSpecificBashibleStepsSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-cloud-provider-%s-bashible-steps", providerID))
			Expect(providerSpecificBashibleStepsSecret.Exists()).To(BeFalse())

			providerSpecificBashibleBootstrapSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-cloud-provider-%s-bashible-bootstrap", providerID))
			Expect(providerSpecificBashibleBootstrapSecret.Exists()).To(BeFalse())

			ccmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-dynamix", "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.spec.containers.0.args").String()).To(MatchYAML(`
- --leader-elect=true
- --cloud-provider=dynamix
- --allow-untagged-cloud=true
- --configure-cloud-routes=false
- --controllers=cloud-node,cloud-node-lifecycle,service-lb-controller
- --bind-address=127.0.0.1
- --secure-port=10471
- --v=4`))

			csiControllerDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-dynamix", "csi-controller")
			Expect(csiControllerDeployment.Exists()).To(BeTrue())
			Expect(csiControllerDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))

			csiNodeDaemonSet := f.KubernetesResource("DaemonSet", "d8-cloud-provider-dynamix", "csi-node")
			Expect(csiNodeDaemonSet.Exists()).To(BeTrue())
			Expect(csiNodeDaemonSet.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))

			cddDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-dynamix", "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())
			Expect(cddDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(cddDeployment.Field("spec.template.spec.tolerations").String()).To(MatchYAML(tolerationsAnyNodeWithUninitialized))
		})
	})
})

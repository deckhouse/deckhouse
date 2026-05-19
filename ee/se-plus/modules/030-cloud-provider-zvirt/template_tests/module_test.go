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

const providerID = "zvirt"
const nameLabelKey = "cloud-provider\\.deckhouse\\.io/name"
const registrationLabelKey = "cloud-provider\\.deckhouse\\.io/registration"
const ephemeralNodesTemplatesLabelKey = "cloud-provider\\.deckhouse\\.io/ephemeral-nodes-templates"
const bashibleLabelKey = "cloud-provider\\.deckhouse\\.io/bashible"

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

	Context("zVirt Suite A", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderZvirt", moduleValuesA)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

  		ccmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-zvirt", "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.spec.containers.0.args").String()).To(MatchYAML(`
- --leader-elect=true
- --bind-address=127.0.0.1
- --secure-port=10471
- --cloud-provider=zvirt
- --allow-untagged-cloud=true
- --configure-cloud-routes=false
- --controllers=cloud-node,cloud-node-lifecycle
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

			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())
			Expect(providerRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", registrationLabelKey)).String()).To(Equal(""))
			Expect(providerRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))
			providerRegistrationSecretData := providerRegistrationSecret.Field("data").Map()
			Expect(providerRegistrationSecretData).To(Not(BeEmpty()))
			Expect(providerRegistrationSecretData["capiClusterName"].String()).To(Equal(base64.StdEncoding.EncodeToString([]byte(providerID))))
			Expect(providerRegistrationSecretData["sshPublicKey"].String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("ssh-rsa deadbeef"))))

			providerSpecificRegistrationSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-node-manager-cloud-provider-%s", providerID))
			Expect(providerSpecificRegistrationSecret.Exists()).To(BeTrue())
			Expect(providerSpecificRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", registrationLabelKey)).String()).To(Equal(""))
			Expect(providerSpecificRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))
			providerSpecificRegistrationSecretData := providerSpecificRegistrationSecret.Field("data").Map()
			Expect(providerSpecificRegistrationSecretData).To(Not(BeEmpty()))
			Expect(providerSpecificRegistrationSecretData["capiClusterName"].String()).To(Equal(base64.StdEncoding.EncodeToString([]byte(providerID))))
			Expect(providerSpecificRegistrationSecretData["sshPublicKey"].String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("ssh-rsa deadbeef"))))

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
		})
	})
})

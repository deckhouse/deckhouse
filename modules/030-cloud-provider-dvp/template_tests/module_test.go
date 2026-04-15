/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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

const providerID = "dvp"
const nameLabelKey = "cloud-provider\\.deckhouse\\.io/name"
const registrationLabelKey = "cloud-provider\\.deckhouse\\.io/registration"
const ephemeralNodesTemplatesLabelKey = "cloud-provider\\.deckhouse\\.io/ephemeral-nodes-templates"
const bashibleLabelKey = "cloud-provider\\.deckhouse\\.io/bashible"

const globalValues = `
  clusterIsBootstrapped: true
  enabledModules: ["vertical-pod-autoscaler", "vertical-pod-autoscaler-crd", "cloud-provider-dvp"]
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: sandbox
      provider: DVP
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
    kubernetesVersion: 1.31.1
    clusterUUID: cluster
`

const moduleValuesA = `
internal:
  providerClusterConfiguration:
    apiVersion: deckhouse.io/v1
    kind: DVPClusterConfiguration
    layout: Standard
    masterNodeGroup:
      instanceClass:
        etcdDisk:
          size: 15Gi
          storageClass: ceph-pool-r2-csi-rbd-immediate
        rootDisk:
          image:
            kind: ClusterVirtualImage
            name: ubuntu-2204
          size: 50Gi
          storageClass: ceph-pool-r2-csi-rbd-immediate
        virtualMachine:
          bootloader: EFI
          cpu:
            coreFraction: 100%
            cores: 4
          ipAddresses:
            - Auto
          memory:
            size: 8Gi
      replicas: 3
    provider:
      kubeconfigDataBase64: YXBpVmV=
      namespace: cloud-provider01
    sshPublicKey: ssh-rsa AAAAB3N
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: DVPCloudDiscoveryData
    zones:
      - default
  storageClasses:
    - dvpStorageClass: 1test
      name: 1test
    - dvpStorageClass: ceph-pool-r2-csi-cephfs
      name: ceph-pool-r2-csi-cephfs
    - dvpStorageClass: ceph-pool-r2-csi-rbd
      name: ceph-pool-r2-csi-rbd
    - dvpStorageClass: ceph-pool-r2-csi-rbd-immediate
      name: ceph-pool-r2-csi-rbd-immediate
    - dvpStorageClass: ceph-pool-r2-csi-rbd-immediate-feat
      name: ceph-pool-r2-csi-rbd-immediate-feat
    - dvpStorageClass: linstor-thin-r1
      name: linstor-thin-r1
    - dvpStorageClass: linstor-thin-r2
      name: linstor-thin-r2
    - dvpStorageClass: sds-local-storage
      name: sds-local-storage
    - dvpStorageClass: xxx
      name: xxx
`

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

const moduleNamespace = "d8-cloud-provider-dvp"

var _ = Describe("Module :: cloud-provider-dvp :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("DVP", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesA)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			csiController := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			Expect(csiController.Exists()).To(BeTrue())
			Expect(csiController.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))

			csiNode := f.KubernetesResource("DaemonSet", moduleNamespace, "csi-node")
			Expect(csiNode.Exists()).To(BeTrue())
			Expect(csiNode.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))

			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")
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
			Expect(providerRegistrationSecretData["sshPublicKey"].String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("ssh-rsa AAAAB3N"))))

			providerSpecificRegistrationSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-node-manager-cloud-provider-%s", providerID))
			Expect(providerSpecificRegistrationSecret.Exists()).To(BeTrue())
			Expect(providerSpecificRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", registrationLabelKey)).String()).To(Equal(""))
			Expect(providerSpecificRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))
			providerSpecificRegistrationSecretData := providerSpecificRegistrationSecret.Field("data").Map()
			Expect(providerSpecificRegistrationSecretData).To(Not(BeEmpty()))
			Expect(providerSpecificRegistrationSecretData["capiClusterName"].String()).To(Equal(base64.StdEncoding.EncodeToString([]byte(providerID))))
			Expect(providerSpecificRegistrationSecretData["sshPublicKey"].String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("ssh-rsa AAAAB3N"))))

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

		It("must render SecurityPolicyException for capdvp-controller-manager", func() {
			capdvpDeployment := f.KubernetesResource("Deployment", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpDeployment.Exists()).To(BeTrue())

			securityPolicyException := f.KubernetesResource("SecurityPolicyException", moduleNamespace, "capdvp-controller-manager")
			Expect(securityPolicyException.Exists()).To(BeTrue())
			Expect(securityPolicyException.Field("spec.network.hostNetwork.allowedValue").Bool()).To(BeTrue())

			Expect(capdvpDeployment.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").String()).
				To(Equal("capdvp-controller-manager"))

			Expect(capdvpDeployment.Field("spec.template.metadata.labels").Map()).
				To(HaveKey("security\\.deckhouse\\.io/security-policy-exception"))

			Expect(securityPolicyException.Field("metadata.namespace").String()).
				To(Equal("d8-cloud-provider-dvp"))
		})
	})
})

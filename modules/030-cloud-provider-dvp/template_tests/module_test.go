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
    kubernetesVersion: "1.32"
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
    kubernetesVersion: 1.32.1
    clusterUUID: cluster
`

const moduleValuesA = `
nodes:
  enabled: true
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa AAAAB3N
provider:
  parameters:
    namespace: cloud-provider01
storage:
  enabled: true
  parameters: {}
internal:
  credentialSecrets:
    d8-credentials:
      authScheme: Kubeconfig
      secret: YXBpVmV=
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

const moduleValuesStorageDisabled = `
nodes:
  enabled: true
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa AAAAB3N
provider:
  parameters:
    namespace: cloud-provider01
storage:
  enabled: false
  parameters: {}
internal:
  credentialSecrets:
    d8-credentials:
      authScheme: Kubeconfig
      secret: YXBpVmV=
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

const moduleValuesNodesDisabled = `
nodes:
  enabled: false
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa AAAAB3N
provider:
  parameters:
    namespace: cloud-provider01
storage:
  enabled: true
  parameters: {}
internal:
  credentialSecrets:
    d8-credentials:
      authScheme: Kubeconfig
      secret: YXBpVmV=
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: DVPCloudDiscoveryData
    zones:
      - default
  storageClasses:
    - dvpStorageClass: 1test
      name: 1test
    - dvpStorageClass: ceph-pool-r2-csi-rbd
      name: ceph-pool-r2-csi-rbd
`

const moduleValuesBothDisabled = `
nodes:
  enabled: false
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa AAAAB3N
provider:
  parameters:
    namespace: cloud-provider01
storage:
  enabled: false
  parameters: {}
internal:
  credentialSecrets:
    d8-credentials:
      authScheme: Kubeconfig
      secret: YXBpVmV=
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: DVPCloudDiscoveryData
    zones:
      - default
  storageClasses: []
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

			ccmDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.spec.hostNetwork").Bool()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("Default"))

			ccmVPA := f.KubernetesResource("VerticalPodAutoscaler", moduleNamespace, "cloud-controller-manager")
			Expect(ccmVPA.Exists()).To(BeTrue())

			ccmPDB := f.KubernetesResource("PodDisruptionBudget", moduleNamespace, "cloud-controller-manager")
			Expect(ccmPDB.Exists()).To(BeTrue())

			capdvpDeployment := f.KubernetesResource("Deployment", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpDeployment.Exists()).To(BeTrue())
			Expect(capdvpDeployment.Field("spec.template.spec.hostNetwork").Bool()).To(BeTrue())
			Expect(capdvpDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))

			capdvpVPA := f.KubernetesResource("VerticalPodAutoscaler", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpVPA.Exists()).To(BeTrue())

			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())
			Expect(cddDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(cddDeployment.Field("spec.template.spec.tolerations").String()).To(MatchYAML(tolerationsAnyNodeWithUninitialized))

			cddVPA := f.KubernetesResource("VerticalPodAutoscaler", moduleNamespace, "cloud-data-discoverer")
			Expect(cddVPA.Exists()).To(BeTrue())

			userAuthzUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-dvp:user")
			Expect(userAuthzUser.Exists()).To(BeTrue())
			Expect(userAuthzUser.Field("rules").String()).To(MatchYAML(`
- apiGroups:
  - deckhouse.io
  resources:
  - dvpinstanceclasses
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - infrastructure.cluster.x-k8s.io
  resources:
  - deckhouseclusters
  - deckhousemachines
  - deckhousemachinetemplates
  verbs:
  - get
  - list
  - watch`))

			userAuthzClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-dvp:cluster-admin")
			Expect(userAuthzClusterAdmin.Exists()).To(BeTrue())
			Expect(userAuthzClusterAdmin.Field("rules").String()).To(MatchYAML(`
- apiGroups:
  - deckhouse.io
  resources:
  - dvpinstanceclasses
  verbs:
  - create
  - delete
  - deletecollection
  - patch
  - update
- apiGroups:
  - infrastructure.cluster.x-k8s.io
  resources:
  - deckhouseclusters
  - deckhousemachines
  - deckhousemachinetemplates
  verbs:
  - patch
  - update`))

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

	})

	Context("DVP with storage disabled", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesStorageDisabled)
			f.HelmRender()
		})

		It("CSI components must not render; CCM, capdvp, and cloud-data-discoverer must render", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// CSI controller Deployment must be absent.
			csiController := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			Expect(csiController.Exists()).To(BeFalse())

			// CSI node DaemonSet must be absent.
			csiNode := f.KubernetesResource("DaemonSet", moduleNamespace, "csi-node")
			Expect(csiNode.Exists()).To(BeFalse())

			// CSIDriver CR must be absent.
			csiDriver := f.KubernetesGlobalResource("CSIDriver", "csi.dvp.deckhouse.io")
			Expect(csiDriver.Exists()).To(BeFalse())

			// CSI ServiceAccount (RBAC) must be absent.
			csiSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "csi")
			Expect(csiSA.Exists()).To(BeFalse())

			// StorageClass must be absent.
			storageClass := f.KubernetesGlobalResource("StorageClass", "1test")
			Expect(storageClass.Exists()).To(BeFalse())

			// CCM must still be present.
			ccmDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())

			// CCM RBAC must still be present.
			ccmSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "cloud-controller-manager")
			Expect(ccmSA.Exists()).To(BeTrue())

			// capdvp must still be present.
			capdvpDeployment := f.KubernetesResource("Deployment", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpDeployment.Exists()).To(BeTrue())

			// capdvp RBAC must still be present.
			capdvpSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpSA.Exists()).To(BeTrue())

			// cloud-data-discoverer must still be present.
			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())
		})
	})

	Context("DVP with nodes disabled", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesNodesDisabled)
			f.HelmRender()
		})

		It("CCM and capdvp must not render; CSI and cloud-data-discoverer must render", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// CCM Deployment must be absent.
			ccmDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeFalse())

			// CCM ServiceAccount (RBAC) must be absent.
			ccmSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "cloud-controller-manager")
			Expect(ccmSA.Exists()).To(BeFalse())

			// capdvp Deployment must be absent.
			capdvpDeployment := f.KubernetesResource("Deployment", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpDeployment.Exists()).To(BeFalse())

			// capdvp ServiceAccount (RBAC) must be absent.
			capdvpSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpSA.Exists()).To(BeFalse())

			// CSI controller must still be present.
			csiController := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			Expect(csiController.Exists()).To(BeTrue())

			// CSIDriver CR must still be present.
			csiDriver := f.KubernetesGlobalResource("CSIDriver", "csi.dvp.deckhouse.io")
			Expect(csiDriver.Exists()).To(BeTrue())

			// CSI RBAC must still be present.
			csiSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "csi")
			Expect(csiSA.Exists()).To(BeTrue())

			// cloud-data-discoverer must still be present.
			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())
		})
	})

	Context("DVP with both storage and nodes disabled", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesBothDisabled)
			f.HelmRender()
		})

		It("Only cloud-data-discoverer and common artifacts must render", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// CSI must be absent.
			csiController := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			Expect(csiController.Exists()).To(BeFalse())

			csiNode := f.KubernetesResource("DaemonSet", moduleNamespace, "csi-node")
			Expect(csiNode.Exists()).To(BeFalse())

			csiDriver := f.KubernetesGlobalResource("CSIDriver", "csi.dvp.deckhouse.io")
			Expect(csiDriver.Exists()).To(BeFalse())

			csiSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "csi")
			Expect(csiSA.Exists()).To(BeFalse())

			// CCM must be absent.
			ccmDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeFalse())

			ccmSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "cloud-controller-manager")
			Expect(ccmSA.Exists()).To(BeFalse())

			// capdvp must be absent.
			capdvpDeployment := f.KubernetesResource("Deployment", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpDeployment.Exists()).To(BeFalse())

			capdvpSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpSA.Exists()).To(BeFalse())

			// cloud-data-discoverer must be present.
			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())

			// Namespace must be present.
			namespace := f.KubernetesGlobalResource("Namespace", moduleNamespace)
			Expect(namespace.Exists()).To(BeTrue())

			// Registration secret must be present.
			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())

			// User-authz ClusterRole must be present.
			userAuthzUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-dvp:user")
			Expect(userAuthzUser.Exists()).To(BeTrue())
		})
	})

	Context("DVP with storage disabled", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesStorageDisabled)
			f.HelmRender()
		})

		It("CSI components must not render; CCM, capdvp, and cloud-data-discoverer must render", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// CSI controller Deployment must be absent.
			csiController := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			Expect(csiController.Exists()).To(BeFalse())

			// CSI node DaemonSet must be absent.
			csiNode := f.KubernetesResource("DaemonSet", moduleNamespace, "csi-node")
			Expect(csiNode.Exists()).To(BeFalse())

			// CSIDriver CR must be absent.
			csiDriver := f.KubernetesGlobalResource("CSIDriver", "csi.dvp.deckhouse.io")
			Expect(csiDriver.Exists()).To(BeFalse())

			// CSI ServiceAccount (RBAC) must be absent.
			csiSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "csi")
			Expect(csiSA.Exists()).To(BeFalse())

			// StorageClass must be absent.
			storageClass := f.KubernetesGlobalResource("StorageClass", "1test")
			Expect(storageClass.Exists()).To(BeFalse())

			// CCM must still be present.
			ccmDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())

			// CCM RBAC must still be present.
			ccmSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "cloud-controller-manager")
			Expect(ccmSA.Exists()).To(BeTrue())

			// capdvp must still be present.
			capdvpDeployment := f.KubernetesResource("Deployment", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpDeployment.Exists()).To(BeTrue())

			// capdvp RBAC must still be present.
			capdvpSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpSA.Exists()).To(BeTrue())

			// cloud-data-discoverer must still be present.
			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())
		})
	})

	Context("DVP with nodes disabled", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesNodesDisabled)
			f.HelmRender()
		})

		It("CCM and capdvp must not render; CSI and cloud-data-discoverer must render", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// CCM Deployment must be absent.
			ccmDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeFalse())

			// CCM ServiceAccount (RBAC) must be absent.
			ccmSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "cloud-controller-manager")
			Expect(ccmSA.Exists()).To(BeFalse())

			// capdvp Deployment must be absent.
			capdvpDeployment := f.KubernetesResource("Deployment", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpDeployment.Exists()).To(BeFalse())

			// capdvp ServiceAccount (RBAC) must be absent.
			capdvpSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpSA.Exists()).To(BeFalse())

			// CSI controller must still be present.
			csiController := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			Expect(csiController.Exists()).To(BeTrue())

			// CSIDriver CR must still be present.
			csiDriver := f.KubernetesGlobalResource("CSIDriver", "csi.dvp.deckhouse.io")
			Expect(csiDriver.Exists()).To(BeTrue())

			// CSI RBAC must still be present.
			csiSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "csi")
			Expect(csiSA.Exists()).To(BeTrue())

			// cloud-data-discoverer must still be present.
			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())
		})
	})

	Context("DVP with both storage and nodes disabled", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesBothDisabled)
			f.HelmRender()
		})

		It("Only cloud-data-discoverer and common artifacts must render", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// CSI must be absent.
			csiController := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			Expect(csiController.Exists()).To(BeFalse())

			csiNode := f.KubernetesResource("DaemonSet", moduleNamespace, "csi-node")
			Expect(csiNode.Exists()).To(BeFalse())

			csiDriver := f.KubernetesGlobalResource("CSIDriver", "csi.dvp.deckhouse.io")
			Expect(csiDriver.Exists()).To(BeFalse())

			csiSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "csi")
			Expect(csiSA.Exists()).To(BeFalse())

			// CCM must be absent.
			ccmDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeFalse())

			ccmSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "cloud-controller-manager")
			Expect(ccmSA.Exists()).To(BeFalse())

			// capdvp must be absent.
			capdvpDeployment := f.KubernetesResource("Deployment", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpDeployment.Exists()).To(BeFalse())

			capdvpSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpSA.Exists()).To(BeFalse())

			// cloud-data-discoverer must be present.
			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())

			// Namespace must be present.
			namespace := f.KubernetesGlobalResource("Namespace", moduleNamespace)
			Expect(namespace.Exists()).To(BeTrue())

			// Registration secret must be present.
			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())

			// User-authz ClusterRole must be present.
			userAuthzUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-dvp:user")
			Expect(userAuthzUser.Exists()).To(BeTrue())
		})
	})
})

/*

User-stories:
1. There are module settings. They must be exported via Secret d8-node-manager-cloud-provider.
2. There are applications which must be deployed — cloud-controller-manager, pd-csi-driver, simple-bridge.

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

const globalValues = `
  clusterConfiguration:
    cloud:
      prefix: myprefix
    clusterType: "Cloud"
  enabledModules: ["vertical-pod-autoscaler-crd"]
  modules:
    placement: {}
  modulesImages:
    registry: registry.flant.com
    registryDockercfg: cfg
    tags:
      cloudProviderAzure:
        csiProvisioner: imagehash
        csiAttacher: imagehash
        csiResizer: imagehash
        csiSnapshotter: imagehash
        csiNodeDriverRegistrar: imagehash
        simpleBridge: imagehash
        cloudControllerManager: imagehash
        pdCsiPlugin: imagehash
        livenessprobe: imagehash
        azurediskCsi: imagehash
  discovery:
    d8SpecificNodeCountByRole:
      worker: 1
      master: 3
    nodeCountByType:
      cloud: 1
    podSubnet: 10.0.1.0/16
    kubernetesVersion: 1.15.4
`

const moduleValues = `
  internal:
    providerClusterConfiguration:
      sshPublicKey: zzz
      provider:
        clientId: zzz
        clientSecret: zzz
        subscriptionId: zzz
        tenantId: zzz
        location: zzz
    providerDiscoveryData:
      resourceGroupName: zzz
      vnetName: zzz
      subnetName: zzz
      instances:
        urn: zzz
        diskType: zzz
        additionalTags:
          tag: zzz
    storageClasses:
    - name: aaa
      type: AAA
    - name: bbb
      type: BBB
    - name: ccc
      type: CCC
`

var _ = Describe("Module :: cloud-provider-azure :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Azure", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("cloudProviderAzure", moduleValues)
			fmt.Println(f.ValuesGet(""))
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-provider-azure")
			registrySecret := f.KubernetesResource("Secret", "d8-cloud-provider-azure", "deckhouse-registry")

			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")

			ccmVPA := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-azure", "cloud-controller-manager")
			ccmDeploy := f.KubernetesResource("Deployment", "d8-cloud-provider-azure", "cloud-controller-manager")
			ccmSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-azure", "cloud-controller-manager")
			ccmCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-azure:cloud-controller-manager")
			ccmCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-azure:cloud-controller-manager")
			ccmSecret := f.KubernetesResource("Secret", "d8-cloud-provider-azure", "cloud-controller-manager")

			pdCSICSIDriver := f.KubernetesGlobalResource("CSIDriver", "disk.csi.azure.com")
			pdCSISS := f.KubernetesResource("Deployment", "d8-cloud-provider-azure", "csi-azuredisk-controller")
			pdCSIDS := f.KubernetesResource("DaemonSet", "d8-cloud-provider-azure", "csi-azuredisk-node")

			pdCSINodeSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-azure", "csi-node")
			pdCSIRegistrarCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-azure:csi:node:secret")
			pdCSIRegistrarCRD := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-azure:csi:node:secret")

			pdCSIControllerSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-azure", "csi-controller")
			pdCSIProvisionerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-azure:csi:controller:provisioner")
			pdCSIProvisionerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-azure:csi:controller:provisioner")
			pdCSIAttacherCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-azure:csi:controller:attacher")
			pdCSIAttacherCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-azure:csi:controller:attacher")
			pdCSIResizerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-azure:csi:controller:resizer")
			pdCSIResizerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-azure:csi:controller:resizer")
			pdCSISnapshotterCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-azure:csi:controller:snapshotter")
			pdCSISnapshotterCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-azure:csi:controller:snapshotter")
			pdCSISecretCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-azure:csi:controller:secret")
			pdCSISecretCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-azure:csi:controller:secret")

			pdCSIaaaSC := f.KubernetesGlobalResource("StorageClass", "aaa")
			pdCSIbbbSC := f.KubernetesGlobalResource("StorageClass", "bbb")
			pdCSIcccSC := f.KubernetesGlobalResource("StorageClass", "ccc")

			userAuthzUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-azure:user")
			userAuthzClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-azure:cluster-admin")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())

			// user story #1
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())
			expectedProviderRegistrationJSON := `{
        "additionalTags": {
          "tag": "zzz"
        },
        "clientId": "zzz",
        "clientSecret": "zzz",
        "diskType": "zzz",
        "location": "zzz",
        "resourceGroupName": "zzz",
        "sshPublicKey": "zzz",
        "subnetName": "zzz",
        "subscriptionId": "zzz",
        "tenantId": "zzz",
        "urn": "zzz",
        "vnetName": "zzz"
      }`
			providerRegistrationData, err := base64.StdEncoding.DecodeString(providerRegistrationSecret.Field("data.azure").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(providerRegistrationData)).To(MatchJSON(expectedProviderRegistrationJSON))

			// user story #2
			Expect(ccmVPA.Exists()).To(BeTrue())
			Expect(ccmDeploy.Exists()).To(BeTrue())
			Expect(ccmSA.Exists()).To(BeTrue())
			Expect(ccmCR.Exists()).To(BeTrue())
			Expect(ccmCRB.Exists()).To(BeTrue())
			Expect(ccmSecret.Exists()).To(BeTrue())

			Expect(pdCSICSIDriver.Exists()).To(BeTrue())
			Expect(pdCSISS.Exists()).To(BeTrue())
			Expect(pdCSIDS.Exists()).To(BeTrue())

			Expect(pdCSINodeSA.Exists()).To(BeTrue())
			Expect(pdCSIRegistrarCR.Exists()).To(BeTrue())
			Expect(pdCSIRegistrarCRD.Exists()).To(BeTrue())

			Expect(pdCSIControllerSA.Exists()).To(BeTrue())
			Expect(pdCSIProvisionerCR.Exists()).To(BeTrue())
			Expect(pdCSIProvisionerCRB.Exists()).To(BeTrue())
			Expect(pdCSIAttacherCR.Exists()).To(BeTrue())
			Expect(pdCSIAttacherCRB.Exists()).To(BeTrue())
			Expect(pdCSIResizerCR.Exists()).To(BeTrue())
			Expect(pdCSIResizerCRB.Exists()).To(BeTrue())
			Expect(pdCSISnapshotterCR.Exists()).To(BeTrue())
			Expect(pdCSISnapshotterCRB.Exists()).To(BeTrue())
			Expect(pdCSISecretCR.Exists()).To(BeTrue())
			Expect(pdCSISecretCRB.Exists()).To(BeTrue())

			Expect(pdCSIaaaSC.Exists()).To(BeTrue())
			Expect(pdCSIbbbSC.Exists()).To(BeTrue())
			Expect(pdCSIcccSC.Exists()).To(BeTrue())

			Expect(userAuthzUser.Exists()).To(BeTrue())
			Expect(userAuthzClusterAdmin.Exists()).To(BeTrue())
		})
	})
})

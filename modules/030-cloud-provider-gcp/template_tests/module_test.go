/*

User-stories:
1. There are module settings. They must be exported via Secret d8-node-manager-cloud-provider.
2. There are applications which must be deployed — cloud-controller-manager, pd-csi-driver.

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
      cloudProviderGcp:
        csiProvisioner: imagehash
        csiAttacher: imagehash
        csiResizer: imagehash
        csiSnapshotter: imagehash
        csiNodeDriverRegistrar: imagehash
        cloudControllerManager: imagehash
        pdCsiPlugin: imagehash
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
      sshKey: mysshkey
      subnetworkCIDR: 10.0.0.0/24
      provider:
        region: myregion
        serviceAccountJSON: mysvcacckey
    providerDiscoveryData:
      disableExternalIP: true
      instances:
        diskSizeGb: 50
        diskType: disk-type
        image: image
        networkTags: ["tag1", "tag2"]
        labels:
          test: test
      networkName: mynetname
      subnetworkName: mysubnetname
      zones: ["zonea", "zoneb"]
`

var _ = Describe("Module :: cloud-provider-gcp :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("GCP", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("cloudProviderGcp", moduleValues)
			fmt.Println(f.ValuesGet(""))
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-provider-gcp")
			registrySecret := f.KubernetesResource("Secret", "d8-cloud-provider-gcp", "deckhouse-registry")

			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")

			ccmVPA := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-gcp", "cloud-controller-manager")
			ccmDeploy := f.KubernetesResource("Deployment", "d8-cloud-provider-gcp", "cloud-controller-manager")
			ccmSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-gcp", "cloud-controller-manager")
			ccmCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-gcp:cloud-controller-manager")
			ccmCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-gcp:cloud-controller-manager")
			ccmSecret := f.KubernetesResource("Secret", "d8-cloud-provider-gcp", "cloud-controller-manager")

			pdCSICSIDriver := f.KubernetesGlobalResource("CSIDriver", "pd.csi.storage.gke.io")
			pdCSISS := f.KubernetesResource("StatefulSet", "d8-cloud-provider-gcp", "pd-csi-controller")
			pdCSIDS := f.KubernetesResource("DaemonSet", "d8-cloud-provider-gcp", "pd-csi-node")
			pdCSINodeSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-gcp", "pd-csi-node")
			pdCSIRegistrarCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-gcp:pd-csi:node")
			pdCSIRegistrarCRD := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-gcp:pd-csi:node")
			pdCSIControllerSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-gcp", "pd-csi-controller")
			pdCSIProvisionerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-gcp:pd-csi:controller:external-provisioner")
			pdCSIProvisionerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-gcp:pd-csi:controller:external-provisioner")
			pdCSIAttacherCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-gcp:pd-csi:controller:external-attacher")
			pdCSIAttacherCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-gcp:pd-csi:controller:external-attacher")
			pdCSIResizerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-gcp:pd-csi:controller:external-resizer")
			pdCSIResizerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-gcp:pd-csi:controller:external-resizer")
			pdCSISnapshotterCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-gcp:pd-csi:controller:external-snapshotter")
			pdCSISnapshotterCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-gcp:pd-csi:controller:external-snapshotter")
			pdCSICredentialsSecret := f.KubernetesResource("Secret", "d8-cloud-provider-gcp", "cloud-credentials")
			pdCSIStandardNotReplicatedSC := f.KubernetesGlobalResource("StorageClass", "pd-standard-not-replicated")
			pdCSIStandardReplicatedSC := f.KubernetesGlobalResource("StorageClass", "pd-standard-replicated")
			pdCSISSDNotReplicatedSC := f.KubernetesGlobalResource("StorageClass", "pd-ssd-not-replicated")
			pdCSISSDReplicatedSC := f.KubernetesGlobalResource("StorageClass", "pd-ssd-replicated")

			userAuthzUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-gcp:user")
			userAuthzClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-gcp:cluster-admin")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())

			// user story #1
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())
			expectedProviderRegistrationJSON := `{
          "disableExternalIP": true,
          "diskSizeGb": 50,
          "diskType": "disk-type",
          "image": "image",
          "labels": {
            "test": "test"
          },
          "networkName": "mynetname",
          "networkTags": [
            "tag1",
            "tag2"
          ],
          "region": "myregion",
          "serviceAccountJSON": "mysvcacckey",
          "sshKey": "mysshkey",
          "subnetworkName": "mysubnetname"
        }`
			providerRegistrationData, err := base64.StdEncoding.DecodeString(providerRegistrationSecret.Field("data.gcp").String())
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
			Expect(pdCSICredentialsSecret.Exists()).To(BeTrue())
			Expect(pdCSIStandardNotReplicatedSC.Exists()).To(BeTrue())
			Expect(pdCSIStandardReplicatedSC.Exists()).To(BeTrue())
			Expect(pdCSISSDNotReplicatedSC.Exists()).To(BeTrue())
			Expect(pdCSISSDReplicatedSC.Exists()).To(BeTrue())

			Expect(userAuthzUser.Exists()).To(BeTrue())
			Expect(userAuthzClusterAdmin.Exists()).To(BeTrue())
		})
	})
})

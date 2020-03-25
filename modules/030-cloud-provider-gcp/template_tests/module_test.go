/*

User-stories:
1. There are module settings. They must be exported via Secret d8-cloud-instance-manager-cloud-provider.
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
  enabledModules: ["vertical-pod-autoscaler-crd"]
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
        simpleBridge: imagehash
        cloudControllerManager: imagehash
        pdCsiPlugin: imagehash
  discovery:
    clusterMasterCount: "3"
    d8SpecificNodeCountByRole:
      worker: 1
    podSubnet: 10.0.1.0/16
    clusterVersion: 1.15.4
`

const moduleValues = `
  serviceAccountKey: mysvcacckey
  networkName: mynetname
  zones: ["zonea", "zoneb"]
  region: myregion
  subnetworkName: mysubnetname
  sshKey: mysshkey
  sshUser: mysshuser
  extraInstanceTags: ["tag1","tag2"]
  disableExternalIP: true
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
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-provider-gcp")
			registrySecret := f.KubernetesResource("Secret", "d8-cloud-provider-gcp", "deckhouse-registry")

			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-cloud-instance-manager-cloud-provider")

			ccmVPA := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-gcp", "cloud-controller-manager")
			ccmDeploy := f.KubernetesResource("Deployment", "d8-cloud-provider-gcp", "cloud-controller-manager")
			ccmSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-gcp", "cloud-controller-manager")
			ccmCR := f.KubernetesGlobalResource("ClusterRole", "d8-cloud-provider-gcp:cloud-controller-manager")
			ccmCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8-cloud-provider-gcp:cloud-controller-manager")
			ccmSecret := f.KubernetesResource("Secret", "d8-cloud-provider-gcp", "cloud-controller-manager")

			pdCSICSIDriver := f.KubernetesGlobalResource("CSIDriver", "pd.csi.storage.gke.io")
			pdCSISS := f.KubernetesResource("StatefulSet", "d8-cloud-provider-gcp", "pd-csi-controller")
			pdCSIDS := f.KubernetesResource("DaemonSet", "d8-cloud-provider-gcp", "pd-csi-node")
			pdCSINodeSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-gcp", "pd-csi-node")
			pdCSIRegistrarCR := f.KubernetesGlobalResource("ClusterRole", "d8-cloud-provider-gcp:csi-driver-registrar")
			pdCSIRegistrarCRD := f.KubernetesGlobalResource("ClusterRoleBinding", "d8-cloud-provider-gcp:csi-driver-registrar")
			pdCSIControllerSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-gcp", "pd-csi-controller")
			pdCSIProvisionerCR := f.KubernetesGlobalResource("ClusterRole", "d8-cloud-provider-gcp:csi-external-provisioner")
			pdCSIProvisionerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8-cloud-provider-gcp:csi-external-provisioner")
			pdCSIAttacherCR := f.KubernetesGlobalResource("ClusterRole", "d8-cloud-provider-gcp:csi-external-attacher")
			pdCSIAttacherCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8-cloud-provider-gcp:csi-external-attacher")
			pdCSIResizerCR := f.KubernetesGlobalResource("ClusterRole", "d8-cloud-provider-gcp:csi-external-resizer")
			pdCSIResizerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8-cloud-provider-gcp:csi-external-resizer")
			pdCSISnapshotterCR := f.KubernetesGlobalResource("ClusterRole", "d8-cloud-provider-gcp:csi-external-snapshotter")
			pdCSISnapshotterCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8-cloud-provider-gcp:csi-external-snapshotter")
			pdCSICredentialsSecret := f.KubernetesResource("Secret", "d8-cloud-provider-gcp", "csi-cloud-credentials")
			pdCSIStandardNotReplicatedSC := f.KubernetesGlobalResource("StorageClass", "pd-standard-not-replicated")
			pdCSIStandardReplicatedSC := f.KubernetesGlobalResource("StorageClass", "pd-standard-replicated")
			pdCSISSDNotReplicatedSC := f.KubernetesGlobalResource("StorageClass", "pd-ssd-not-replicated")
			pdCSISSDReplicatedSC := f.KubernetesGlobalResource("StorageClass", "pd-ssd-replicated")

			userAuthzUser := f.KubernetesGlobalResource("ClusterRole", "d8-cloud-provider-gcp:user-authz:user")
			userAuthzClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8-cloud-provider-gcp:user-authz:cluster-admin")

			simpleBridgeDS := f.KubernetesResource("DaemonSet", "d8-cloud-provider-gcp", "simple-bridge")
			simpleBridgeCR := f.KubernetesGlobalResource("ClusterRole", "d8-cloud-provider-gcp:simple-bridge")
			simpleBridgeCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8-cloud-provider-gcp:simple-bridge")
			simpleBridgeSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-gcp", "simple-bridge")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())

			// user story #1
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())
			expectedProviderRegistrationJSON := `{
          "disableExternalIP": true,
          "extraInstanceTags": [
            "tag1",
            "tag2"
          ],
          "networkName": "mynetname",
          "region": "myregion",
          "serviceAccountKey": "mysvcacckey",
          "sshKey": "mysshkey",
          "sshUser": "mysshuser",
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

			Expect(simpleBridgeDS.Exists()).To(BeTrue())
			Expect(simpleBridgeCR.Exists()).To(BeTrue())
			Expect(simpleBridgeCRB.Exists()).To(BeTrue())
			Expect(simpleBridgeSA.Exists()).To(BeTrue())
		})
	})
})

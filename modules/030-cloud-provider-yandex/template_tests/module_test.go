/*

User-stories:
1. There are module settings. They must be exported via Secret d8-node-manager-cloud-provider.
2. There are applications which must be deployed — cloud-controller-manager, yandex-csi, flannel.

*/

package template_tests

import (
	"encoding/base64"
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
      cloudProviderYandex:
        csiProvisioner: imagehash
        csiAttacher: imagehash
        csiResizer: imagehash
        csiSnapshotter: imagehash
        csiNodeDriverRegistrar: imagehash
        csiLivenessProbe: imagehash
        cloudControllerManager: imagehash
        flanneld: imagehash
        yandexCsiPlugin: imagehash
  discovery:
    clusterMasterCount: 3
    d8SpecificNodeCountByRole:
      worker: 1
    podSubnet: 10.0.1.0/16
    clusterVersion: 1.15.4
    clusterUUID: 3b5058e1-e93a-4dfa-be32-395ef4b3da45
`

const moduleValues = `
    zones: ["zonea", "zoneb"]
    zoneToSubnetIdMap:
      zonea: aaa
      zoneb: bbb
    defaultLbTargetGroupNetworkId: deftarggroupnetid
    internalNetworkIDs: ["id1", "id2"]
    externalNetworkIDs: ["id1", "id2"]
    sshKey: mysshkey
    sshUser: mysshuser
    serviceAccountJSON: '{"my": "json"}'
    nameservers: ["1.1.1.1", "2.2.2.2"]
    region: myreg
    folderID: myfoldid
`

var _ = Describe("Module :: cloud-provider-yandex :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Yandex", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("cloudProviderYandex", moduleValues)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-provider-yandex")
			registrySecret := f.KubernetesResource("Secret", "d8-cloud-provider-yandex", "deckhouse-registry")

			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")

			flannelCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-yandex:flannel")
			flannelCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-yandex:flannel")
			flannelSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-yandex", "flannel")
			flannelDS := f.KubernetesResource("DaemonSet", "d8-cloud-provider-yandex", "flannel")
			flannelCM := f.KubernetesResource("ConfigMap", "d8-cloud-provider-yandex", "flannel")

			csiDriver := f.KubernetesGlobalResource("CSIDriver", "yandex.csi.flant.com")
			csiControllerSS := f.KubernetesResource("StatefulSet", "d8-cloud-provider-yandex", "csi-controller")
			csiNodeDS := f.KubernetesResource("DaemonSet", "d8-cloud-provider-yandex", "csi-node")
			csiNodeSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-yandex", "yandex-csi-node")
			csiRegistrarCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-yandex:yandex-csi:node")
			csiRegistrarCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-yandex:yandex-csi:node")
			csiControllerSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-yandex", "yandex-csi-controller")
			csiProvisionerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-yandex:yandex-csi:controller:external-provisioner")
			csiProvisionerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-yandex:yandex-csi:controller:external-provisioner")
			csiExternalAttacherCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-yandex:yandex-csi:controller:external-attacher")
			csiExternalAttacherCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-yandex:yandex-csi:controller:external-attacher")
			csiExternalResizerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-yandex:yandex-csi:controller:external-resizer")
			csiExternalResizerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-yandex:yandex-csi:controller:external-resizer")
			csiExternalSnapshotterCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-yandex:yandex-csi:controller:external-snapshotter")
			csiExternalSnapshotterCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-yandex:yandex-csi:controller:external-snapshotter")
			csiCredentials := f.KubernetesResource("Secret", "d8-cloud-provider-yandex", "csi-credentials")
			csiHDDSC := f.KubernetesGlobalResource("StorageClass", "network-hdd")
			csiSSDSC := f.KubernetesGlobalResource("StorageClass", "network-ssd")

			ccmSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-yandex", "cloud-controller-manager")
			ccmCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-yandex:cloud-controller-manager")
			ccmCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-yandex:cloud-controller-manager")
			ccmVPA := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-yandex", "cloud-controller-manager")
			ccmDeploy := f.KubernetesResource("Deployment", "d8-cloud-provider-yandex", "cloud-controller-manager")
			ccmSecret := f.KubernetesResource("Secret", "d8-cloud-provider-yandex", "cloud-controller-manager")

			userAuthzUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-yandex:user")
			userAuthzClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-yandex:cluster-admin")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(userAuthzUser.Exists()).To(BeTrue())
			Expect(userAuthzClusterAdmin.Exists()).To(BeTrue())

			// user story #1
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())
			expectedProviderRegistrationJSON := `{
          "folderID": "myfoldid",
          "nameservers": [
            "1.1.1.1",
            "2.2.2.2"
          ],
          "region": "myreg",
          "serviceAccountJSON": "{\"my\": \"json\"}",
          "sshKey": "mysshkey",
          "sshUser": "mysshuser",
          "zoneToSubnetIdMap": {
            "zonea": "aaa",
            "zoneb": "bbb"
          }
        }`
			providerRegistrationData, err := base64.StdEncoding.DecodeString(providerRegistrationSecret.Field("data.yandex").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(providerRegistrationData)).To(MatchJSON(expectedProviderRegistrationJSON))

			// user story #2
			Expect(csiDriver.Exists()).To(BeTrue())
			Expect(csiControllerSS.Exists()).To(BeTrue())
			Expect(csiNodeDS.Exists()).To(BeTrue())
			Expect(csiNodeSA.Exists()).To(BeTrue())
			Expect(csiRegistrarCR.Exists()).To(BeTrue())
			Expect(csiRegistrarCRB.Exists()).To(BeTrue())
			Expect(csiControllerSA.Exists()).To(BeTrue())
			Expect(csiProvisionerCR.Exists()).To(BeTrue())
			Expect(csiProvisionerCRB.Exists()).To(BeTrue())
			Expect(csiExternalAttacherCR.Exists()).To(BeTrue())
			Expect(csiExternalAttacherCRB.Exists()).To(BeTrue())
			Expect(csiExternalResizerCR.Exists()).To(BeTrue())
			Expect(csiExternalResizerCRB.Exists()).To(BeTrue())
			Expect(csiExternalSnapshotterCR.Exists()).To(BeTrue())
			Expect(csiExternalSnapshotterCRB.Exists()).To(BeTrue())
			Expect(csiCredentials.Exists()).To(BeTrue())
			Expect(csiHDDSC.Exists()).To(BeTrue())
			Expect(csiSSDSC.Exists()).To(BeTrue())

			Expect(flannelCR.Exists()).To(BeTrue())
			Expect(flannelCRB.Exists()).To(BeTrue())
			Expect(flannelSA.Exists()).To(BeTrue())
			Expect(flannelDS.Exists()).To(BeTrue())
			Expect(flannelCM.Exists()).To(BeTrue())

			Expect(ccmSA.Exists()).To(BeTrue())
			Expect(ccmCR.Exists()).To(BeTrue())
			Expect(ccmCRB.Exists()).To(BeTrue())
			Expect(ccmVPA.Exists()).To(BeTrue())
			Expect(ccmDeploy.Exists()).To(BeTrue())
			Expect(ccmSecret.Exists()).To(BeTrue())
		})
	})
})

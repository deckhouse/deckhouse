/*

User-stories:
1. There are module settings. They must be exported via Secret d8-node-manager-cloud-provider.
2. There are applications which must be deployed — cloud-controller-manager, ebs-csi-driver, simple-bridge.

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
      cloudProviderAws:
        csiProvisioner: imagehash
        csiAttacher: imagehash
        csiResizer: imagehash
        csiSnapshotter: imagehash
        csiNodeDriverRegistrar: imagehash
        csiLivenessProbe: imagehash
        ebsCsiPlugin: imagehash
        simpleBridge: imagehash
        cloudControllerManager: imagehash
  discovery:
    d8SpecificNodeCountByRole:
      worker: 1
      master:
        __ConstantChoices__: "3"
    nodeCountByType:
      cloud: 1
    podSubnet: 10.0.1.0/16
    kubernetesVersion: 1.15.4
`

const moduleValues = `
  internal:
    zoneToSubnetIdMap:
      zonea: aaa
      zoneb: bbb
  providerAccessKeyId: myprovacckeyid
  providerSecretAccessKey: myprovsecretaccesskey
  zones: ["zonea", "zoneb"]
  region: myregion
  instances:
    iamProfileName: myiamprofile
    securityGroupIDs: ["id1", "id2"]
    extraTags: ["tag1", "tag2"]
  loadBalancerSecurityGroupID: mylbsecgroupid
  keyName: mykeyname
`

var _ = Describe("Module :: cloud-provider-aws :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("AWS", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("cloudProviderAws", moduleValues)
			fmt.Println(f.ValuesGet(""))
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-provider-aws")
			registrySecret := f.KubernetesResource("Secret", "d8-cloud-provider-aws", "deckhouse-registry")

			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")

			ccmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-aws", "cloud-controller-manager")
			ccmServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-aws", "cloud-controller-manager")
			ccmClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-aws:cloud-controller-manager")
			ccmClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-aws:cloud-controller-manager")
			ccmSecret := f.KubernetesResource("Secret", "d8-cloud-provider-aws", "cloud-controller-manager")

			ebsControllerPluginStatefulSet := f.KubernetesResource("StatefulSet", "d8-cloud-provider-aws", "ebs-csi-controller")
			ebsCSIDriver := f.KubernetesGlobalResource("CSIDriver", "ebs.csi.aws.com")
			ebsCredentialsSecret := f.KubernetesResource("Secret", "d8-cloud-provider-aws", "credentials")
			ebsNodePluginDaemonSet := f.KubernetesResource("DaemonSet", "d8-cloud-provider-aws", "ebs-csi-node")
			ebsNodePluginServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-aws", "ebs-csi-node")
			ebsNodeSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-aws", "ebs-csi-node")
			ebsControllerSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-aws", "ebs-csi-controller")
			ebsRegistrarCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-aws:ebs-csi:node")
			ebsRegistrarCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-aws:ebs-csi:node")
			ebsProvisionerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-aws:ebs-csi:controller:external-provisioner")
			ebsProvisionerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-aws:ebs-csi:controller:external-provisioner")
			ebsAttacherCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-aws:ebs-csi:controller:external-attacher")
			ebsAttacherCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-aws:ebs-csi:controller:external-attacher")
			ebsResizerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-aws:ebs-csi:controller:external-resizer")
			ebsResizerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-aws:ebs-csi:controller:external-resizer")
			ebsSnapshotterCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-aws:ebs-csi:controller:external-snapshotter")
			ebsSnapshotterCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-aws:ebs-csi:controller:external-snapshotter")
			ebsStorageClass := f.KubernetesGlobalResource("StorageClass", "gp2")

			simpleBridgeDS := f.KubernetesResource("DaemonSet", "d8-cloud-provider-aws", "simple-bridge")
			simpleBridgeSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-aws", "simple-bridge")
			simpleBridgeCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-aws:simple-bridge")
			simpleBridgeCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-aws:simple-bridge")

			userAuthzUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-aws:user")
			userAuthzClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-aws:cluster-admin")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(userAuthzUser.Exists()).To(BeTrue())
			Expect(userAuthzClusterAdmin.Exists()).To(BeTrue())

			// user story #1
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())
			expectedAWSJSON := `{
  "instances":{
    "extraTags":["tag1","tag2"],
    "iamProfileName":"myiamprofile",
    "securityGroupIDs":["id1","id2"]},
    "internal":{
      "zoneToSubnetIdMap":{"zonea":"aaa","zoneb":"bbb"}
     },
    "keyName":"mykeyname",
    "loadBalancerSecurityGroupID":"mylbsecgroupid",
    "providerAccessKeyId":"myprovacckeyid",
    "providerSecretAccessKey":"myprovsecretaccesskey",
    "region":"myregion"
}`
			dataAWS, err := base64.StdEncoding.DecodeString(providerRegistrationSecret.Field("data.aws").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(dataAWS)).To(MatchJSON(expectedAWSJSON))

			// user story #2
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmServiceAccount.Exists()).To(BeTrue())
			Expect(ccmClusterRole.Exists()).To(BeTrue())
			Expect(ccmClusterRoleBinding.Exists()).To(BeTrue())
			Expect(ccmSecret.Exists()).To(BeTrue())
			Expect(ebsControllerPluginStatefulSet.Exists()).To(BeTrue())
			Expect(ebsCSIDriver.Exists()).To(BeTrue())
			Expect(ebsCredentialsSecret.Exists()).To(BeTrue())
			Expect(ebsNodePluginDaemonSet.Exists()).To(BeTrue())
			Expect(ebsNodePluginServiceAccount.Exists()).To(BeTrue())
			Expect(ebsNodeSA.Exists()).To(BeTrue())
			Expect(ebsControllerSA.Exists()).To(BeTrue())
			Expect(ebsRegistrarCR.Exists()).To(BeTrue())
			Expect(ebsRegistrarCRB.Exists()).To(BeTrue())
			Expect(ebsProvisionerCR.Exists()).To(BeTrue())
			Expect(ebsProvisionerCRB.Exists()).To(BeTrue())
			Expect(ebsAttacherCR.Exists()).To(BeTrue())
			Expect(ebsAttacherCRB.Exists()).To(BeTrue())
			Expect(ebsResizerCR.Exists()).To(BeTrue())
			Expect(ebsResizerCRB.Exists()).To(BeTrue())
			Expect(ebsSnapshotterCR.Exists()).To(BeTrue())
			Expect(ebsSnapshotterCRB.Exists()).To(BeTrue())
			Expect(ebsStorageClass.Exists()).To(BeTrue())
			Expect(simpleBridgeDS.Exists()).To(BeTrue())
			Expect(simpleBridgeSA.Exists()).To(BeTrue())
			Expect(simpleBridgeCR.Exists()).To(BeTrue())
			Expect(simpleBridgeCRB.Exists()).To(BeTrue())
		})
	})
})

/*

User-stories:
1. There are module settings. They must be exported via Secret d8-cloud-instance-manager-cloud-provider.
2. There are applications which must be deployed — cloud-controller-manager, cinder-csi-driver, flannel.

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
      cloudProviderOpenstack:
        cinderCsiPlugin: imagehash
        csiProvisioner: imagehash
        csiAttacher: imagehash
        csiSnapshotter: imagehash
        csiNodeDriverRegistrar: imagehash
        cloudControllerManager: imagehash
        flanneld: imagehash
  discovery:
    clusterMasterCount: 3
    d8SpecificNodeCountByRole:
      worker: 1
    podSubnet: 10.0.1.0/16
    clusterVersion: 1.15.4
`

const moduleValues = `
  internal:
    internalSubnetRegex: myregex
  networkName: mynetname
  zones: ["zonea", "zoneb"]
  authURL: http://my.cloud.lalla/123/
  username: myuser
  password: myPaSs
  domainName: mydomain
  tenantName: mytenantname
  tenantID: mytenantid
  caCert: mycacert
  region: myreg
  internalNetworkName: myintnetname
  addPodSubnetToPortWhitelist: true
  sshKeyPairName: mysshkeypairname
  securityGroups: ["aaa","bbb"]
`

var _ = Describe("Module :: cloud-provider-openstack :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Openstack", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("cloudProviderOpenstack", moduleValues)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-provider-openstack")
			registrySecret := f.KubernetesResource("Secret", "d8-cloud-provider-openstack", "deckhouse-registry")

			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-cloud-instance-manager-cloud-provider")

			cinderCSIDriver := f.KubernetesGlobalResource("CSIDriver", "cinder.csi.openstack.org")
			cinderNodePluginSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-openstack", "cinder-csi.node")
			cinderNodePluginDS := f.KubernetesResource("DaemonSet", "d8-cloud-provider-openstack", "csi-cinder-node-plugin")
			cinterControllerPluginSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-openstack", "cinder-csi.controller")
			cinderCongrollerPluginSS := f.KubernetesResource("StatefulSet", "d8-cloud-provider-openstack", "csi-cinder-controller-plugin")
			cinderNodePluginCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-openstack:cinder-csi:node")
			cinderNodePluginCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-openstack:cinder-csi:node")
			cinderAttacherCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-openstack:cinder-csi:controller:attacher")
			cinderAttacherCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-openstack:cinder-csi:controller:attacher")
			cinderProvisionerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-openstack:cinder-csi:controller:provisioner")
			cinderProvisionerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-openstack:cinder-csi:controller:provisioner")
			cinderSnapshotterCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-openstack:cinder-csi:controller:snapshotter")
			cinderSnapshotterCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-openstack:cinder-csi:controller:snapshotter")
			cinderResizerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-openstack:cinder-csi:controller:external-resizer")
			cinderResizerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-openstack:cinder-csi:controller:external-resizer")
			cinderResizerR := f.KubernetesResource("Role", "d8-cloud-provider-openstack", "cinder-csi:controller")
			cinderResizerRB := f.KubernetesResource("RoleBinding", "d8-cloud-provider-openstack", "cinder-csi:controller")

			flannelCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-openstack:flannel")
			flannelCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-openstack:flannel")
			flannelSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-openstack", "flannel")
			flannelDS := f.KubernetesResource("DaemonSet", "d8-cloud-provider-openstack", "flannel")
			flannelCM := f.KubernetesResource("ConfigMap", "d8-cloud-provider-openstack", "flannel")

			ccmSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-openstack", "cloud-controller-manager")
			ccmCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-openstack:cloud-controller-manager")
			ccmCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-openstack:cloud-controller-manager")
			ccmVPA := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-openstack", "cloud-controller-manager")
			ccmDeploy := f.KubernetesResource("Deployment", "d8-cloud-provider-openstack", "cloud-controller-manager")
			ccmSecret := f.KubernetesResource("Secret", "d8-cloud-provider-openstack", "cloud-controller-manager")

			userAuthzUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-openstack:user")
			userAuthzClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-openstack:cluster-admin")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())

			// user story #1
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())
			expectedProviderRegistrationJSON := `{
          "addPodSubnetToPortWhitelist": true,
          "authURL": "http://my.cloud.lalla/123/",
          "caCert": "mycacert",
          "domainName": "mydomain",
          "internalNetworkName": "myintnetname",
          "networkName": "mynetname",
          "password": "myPaSs",
          "region": "myreg",
          "securityGroups": [
            "aaa",
            "bbb"
          ],
          "sshKeyPairName": "mysshkeypairname",
          "tenantID": "mytenantid",
          "tenantName": "mytenantname",
          "username": "myuser"
        }`
			providerRegistrationData, err := base64.StdEncoding.DecodeString(providerRegistrationSecret.Field("data.openstack").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(providerRegistrationData)).To(MatchJSON(expectedProviderRegistrationJSON))

			// user story #2
			Expect(cinderCSIDriver.Exists()).To(BeTrue())
			Expect(cinderNodePluginSA.Exists()).To(BeTrue())
			Expect(cinderNodePluginDS.Exists()).To(BeTrue())
			Expect(cinterControllerPluginSA.Exists()).To(BeTrue())
			Expect(cinderCongrollerPluginSS.Exists()).To(BeTrue())
			Expect(cinderNodePluginCR.Exists()).To(BeTrue())
			Expect(cinderNodePluginCRB.Exists()).To(BeTrue())
			Expect(cinderAttacherCR.Exists()).To(BeTrue())
			Expect(cinderAttacherCRB.Exists()).To(BeTrue())
			Expect(cinderProvisionerCR.Exists()).To(BeTrue())
			Expect(cinderProvisionerCRB.Exists()).To(BeTrue())
			Expect(cinderSnapshotterCR.Exists()).To(BeTrue())
			Expect(cinderSnapshotterCRB.Exists()).To(BeTrue())
			Expect(cinderResizerCR.Exists()).To(BeTrue())
			Expect(cinderResizerCRB.Exists()).To(BeTrue())
			Expect(cinderResizerR.Exists()).To(BeTrue())
			Expect(cinderResizerRB.Exists()).To(BeTrue())

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

			Expect(userAuthzUser.Exists()).To(BeTrue())
			Expect(userAuthzClusterAdmin.Exists()).To(BeTrue())
		})
	})
})

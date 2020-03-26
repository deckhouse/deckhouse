/*

User-stories:
1. There are module settings. They must be exported via Secret d8-cloud-instance-manager-cloud-provider.
2. There are applications which must be deployed — cloud-controller-manager, csi, flannel.
3. There is list of datastores in values.yaml. StorageClass must be created for every datastore. Datastore mentioned in value `defaultDatastore` must be annotated as default.

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
      cloudProviderVsphere:
        attacher: imagehash
        externalResizer: imagehash
        flanneld: imagehash
        livenessprobe: imagehash
        nodeRegistrar: imagehash
        provisioner: imagehash
        vsphereCsi: imagehash
        cloudControllerManager: imagehash
  discovery:
    clusterMasterCount: 3
    d8SpecificNodeCountByRole:
      worker: 1
    podSubnet: 10.0.1.0/16
    clusterVersion: 1.15.4
`

const moduleValues = `
    internal:
      datastores:
      - name: mydsname1
        path: /my/ds/path/mydsname1
        zones: ["zonea", "zoneb"]
      - name: mydsname2
        path: /my/ds/path/mydsname2
        zones: ["zonea", "zoneb"]
    defaultDatastore: mydsname2
    host: myhost
    username: myuname
    password: myPaSsWd
    insecure: true
    regionTagCategory: myregtagcat
    zoneTagCategory: myzonetagcat
    region: myreg
    sshKeys: ["mysshkey1", "mysshkey2"]
    vmFolderPath: dev/test
    zones: ["aaa", "bbb"]
`

var _ = Describe("Module :: cloud-provider-vsphere :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Vsphere", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("cloudProviderVsphere", moduleValues)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-provider-vsphere")
			registrySecret := f.KubernetesResource("Secret", "d8-cloud-provider-vsphere", "deckhouse-registry")

			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-cloud-instance-manager-cloud-provider")

			flannelCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-vsphere:flannel")
			flannelCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-vsphere:flannel")
			flannelSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-vsphere", "flannel")
			flannelDS := f.KubernetesResource("DaemonSet", "d8-cloud-provider-vsphere", "flannel")
			flannelCM := f.KubernetesResource("ConfigMap", "d8-cloud-provider-vsphere", "flannel")

			csiDriver := f.KubernetesGlobalResource("CSIDriver", "vsphere.csi.vmware.com")
			csiSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-vsphere", "vsphere-csi.controller")
			csiCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-vsphere:vsphere-csi:controller")
			csiCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-vsphere:vsphere-csi:controller")
			csiNodeVPA := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-vsphere", "vsphere-csi-node")
			csiNodeDS := f.KubernetesResource("DaemonSet", "d8-cloud-provider-vsphere", "vsphere-csi-node")
			csiDriverControllerVPA := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-vsphere", "vsphere-csi-driver-controller")
			csiDriverControllerSS := f.KubernetesResource("StatefulSet", "d8-cloud-provider-vsphere", "vsphere-csi-driver-controller")

			ccmSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-vsphere", "cloud-controller-manager")
			ccmCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-vsphere:cloud-controller-manager")
			ccmCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-vsphere:cloud-controller-manager")
			ccmVPA := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-vsphere", "cloud-controller-manager")
			ccmDeploy := f.KubernetesResource("Deployment", "d8-cloud-provider-vsphere", "cloud-controller-manager")
			ccmSecret := f.KubernetesResource("Secret", "d8-cloud-provider-vsphere", "cloud-controller-manager")

			userAuthzUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-vsphere:user")
			userAuthzClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-vsphere:cluster-admin")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(userAuthzUser.Exists()).To(BeTrue())
			Expect(userAuthzClusterAdmin.Exists()).To(BeTrue())

			// user story #1
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())
			expectedProviderRegistrationJSON := `{
          "host": "myhost",
          "insecure": true,
          "password": "myPaSsWd",
          "region": "myreg",
          "regionTagCategory": "myregtagcat",
          "sshKeys": [
            "mysshkey1",
            "mysshkey2"
          ],
          "username": "myuname",
          "vmFolderPath": "dev/test",
          "zoneTagCategory": "myzonetagcat"
        }`
			providerRegistrationData, err := base64.StdEncoding.DecodeString(providerRegistrationSecret.Field("data.vsphere").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(providerRegistrationData)).To(MatchJSON(expectedProviderRegistrationJSON))

			// user story #2
			Expect(csiDriver.Exists()).To(BeTrue())
			Expect(csiSA.Exists()).To(BeTrue())
			Expect(csiCR.Exists()).To(BeTrue())
			Expect(csiCRB.Exists()).To(BeTrue())
			Expect(csiNodeVPA.Exists()).To(BeTrue())
			Expect(csiNodeDS.Exists()).To(BeTrue())
			Expect(csiDriverControllerVPA.Exists()).To(BeTrue())
			Expect(csiDriverControllerSS.Exists()).To(BeTrue())
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

			// user story #3
			Expect(f.KubernetesGlobalResource("StorageClass", "mydsname1").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "mydsname1").Field(`metadata.annotations.storageclass\.kubernetes\.io/is-default-class`).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("StorageClass", "mydsname2").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "mydsname2").Field(`metadata.annotations.storageclass\.kubernetes\.io/is-default-class`).String()).To(Equal("true"))
		})
	})
})

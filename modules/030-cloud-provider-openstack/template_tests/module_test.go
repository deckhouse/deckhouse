/*

User-stories:
1. There are module settings. They must be exported via Secret d8-node-manager-cloud-provider.
2. There are applications which must be deployed — cloud-controller-manager, cinder-csi-driver.

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
  modules:
    placement: {}
  modulesImages:
    registry: registry.flant.com
    registryDockercfg: cfg
    tags:
      cloudProviderOpenstack:
        cinderCsiPlugin: imagehash
        csiProvisioner: imagehash
        csiAttacher: imagehash
        csiSnapshotter: imagehash
        csiResizer: imagehash
        csiNodeDriverRegistrar: imagehash
        cloudControllerManager116: imagehash
        cloudControllerManager119: imagehash
  discovery:
    d8SpecificNodeCountByRole:
      master: 3
      worker: 1
    nodeCountByType:
      cloud: 1
    podSubnet: 10.0.1.0/16
    kubernetesVersion: 1.16.4
    defaultStorageClass: fastssd
`

const moduleValues = `
  internal:
    storageClasses:
      - name: fastssd
        type: Fast HDD
      - name: slowhdd
        type: Slow HDD
    connection:
      authURL: http://my.cloud.lalla/123/
      username: myuser
      password: myPaSs
      domainName: mydomain
      tenantName: mytenantname
      caCert: mycacert
      region: myreg
    internalNetworkNames:
      - myintnetname
      - myintnetname2
    externalNetworkNames:
      - myextnetname
      - myextnetname2
    podNetworkMode: "VXLAN"
    instances:
      imageName: ubuntu
      mainNetwork: kube
      securityGroups: ["aaa","bbb"]
      sshKeyPairName: mysshkeypairname
    zones: ["zonea", "zoneb"]
    loadBalancer:
      subnetID: my-subnet-id
      floatingNetworkID: my-floating-network-id
`

const badModuleValues = `
  internal:
    connection:
      authURL: http://my.cloud.lalla/123/
      username: myuser
      password: myPaSs
      domainName: mydomain
      tenantName: mytenantname
      tenantID: mytenantid
      caCert: mycacert
      region: myreg
    internalNetworkNames:
      - myintnetname
      - myintnetname2
    externalNetworkNames:
      - myextnetname
      - myextnetname2
    podNetworkMode: "VXLAN"
    instances:
      sshKeyPairName: mysshkeypairname
      securityGroups: ["aaa","bbb"]
    zones: ["zonea", "zoneb"]
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
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-provider-openstack")
			registrySecret := f.KubernetesResource("Secret", "d8-cloud-provider-openstack", "deckhouse-registry")

			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")

			cinderCSIDriver := f.KubernetesGlobalResource("CSIDriver", "cinder.csi.openstack.org")
			cinderNodePluginSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-openstack", "cinder-csi-node")
			cinderNodePluginDS := f.KubernetesResource("DaemonSet", "d8-cloud-provider-openstack", "csi-cinder-node-plugin")
			cinterControllerPluginSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-openstack", "cinder-csi-controller")
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

			ccmSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-openstack", "cloud-controller-manager")
			ccmCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-openstack:cloud-controller-manager")
			ccmCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-openstack:cloud-controller-manager")
			ccmVPA := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-openstack", "cloud-controller-manager")
			ccmDeploy := f.KubernetesResource("Deployment", "d8-cloud-provider-openstack", "cloud-controller-manager")
			ccmSecret := f.KubernetesResource("Secret", "d8-cloud-provider-openstack", "cloud-controller-manager")

			userAuthzUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-openstack:user")
			userAuthzClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-openstack:cluster-admin")

			scFast := f.KubernetesGlobalResource("StorageClass", "fastssd")
			scSlow := f.KubernetesGlobalResource("StorageClass", "slowhdd")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())

			// user story #1
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())
			expectedProviderRegistrationJSON := `{
          "connection": {
            "authURL": "http://my.cloud.lalla/123/",
            "caCert": "mycacert",
            "domainName": "mydomain",
            "region": "myreg",
            "password": "myPaSs",
            "tenantName": "mytenantname",
            "username": "myuser"
          },
          "internalNetworkNames": ["myintnetname", "myintnetname2"],
          "externalNetworkNames": ["myextnetname", "myextnetname2"],
          "instances": {
            "imageName": "ubuntu",
            "mainNetwork": "kube",
            "securityGroups": [
              "aaa",
              "bbb"
            ],
            "sshKeyPairName": "mysshkeypairname"
          },
          "podNetworkMode": "VXLAN"
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

			Expect(ccmSA.Exists()).To(BeTrue())
			Expect(ccmCR.Exists()).To(BeTrue())
			Expect(ccmCRB.Exists()).To(BeTrue())
			Expect(ccmVPA.Exists()).To(BeTrue())
			Expect(ccmDeploy.Exists()).To(BeTrue())
			Expect(ccmSecret.Exists()).To(BeTrue())
			ccmExpectedConfig := `
[Global]
auth-url = "http://my.cloud.lalla/123/"
domain-name = "mydomain"
tenant-name = "mytenantname"
username = "myuser"
password = "myPaSs"
region = "myreg"
ca-file = /etc/cloud-contoller-manager-config/ca.crt
[Networking]
public-network-name = "myextnetname"
public-network-name = "myextnetname2"
internal-network-name = "myintnetname"
internal-network-name = "myintnetname2"
ipv6-support-disabled = true
[LoadBalancer]
create-monitor = "true"
monitor-delay = "2s"
monitor-timeout = "1s"
subnet-id = "my-subnet-id"
floating-network-id = "my-floating-network-id"`
			ccmConfig, err := base64.StdEncoding.DecodeString(ccmSecret.Field("data.cloud-config").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(ccmConfig)).To(Equal(ccmExpectedConfig))

			Expect(userAuthzUser.Exists()).To(BeTrue())
			Expect(userAuthzClusterAdmin.Exists()).To(BeTrue())

			Expect(scFast.Exists()).To(BeTrue())
			Expect(scFast.Field("metadata.annotations").String()).To(MatchYAML(`
storageclass.kubernetes.io/is-default-class: "true"
`))
			Expect(scSlow.Exists()).To(BeTrue())
		})
	})

	Context("Openstack with default StorageClass specified", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("cloudProviderOpenstack", moduleValues)
			f.ValuesSetFromYaml("cloudProviderOpenstack.internal.defaultStorageClass", `slowhdd`)
			f.HelmRender()
		})

		It("Everything must render properly with proper default StorageClass", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			scFast := f.KubernetesGlobalResource("StorageClass", "fastssd")
			scSlow := f.KubernetesGlobalResource("StorageClass", "slowhdd")

			Expect(scFast.Exists()).To(BeTrue())
			Expect(scSlow.Exists()).To(BeTrue())

			Expect(scFast.Field("metadata.annotations").Exists()).To(BeFalse())
			Expect(scSlow.Field("metadata.annotations").String()).To(MatchYAML(`
storageclass.kubernetes.io/is-default-class: "true"
`))
		})
	})

	Context("Openstack bad config", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("cloudProviderOpenstack", badModuleValues)
			f.HelmRender()
		})

		It("Test should fail", func() {
			Expect(f.RenderError).Should(HaveOccurred())
			Expect(f.RenderError.Error()).ShouldNot(BeEmpty())
		})
	})

	Context("Unsupported Kubernetes version", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("cloudProviderOpenstack", moduleValues)
			f.ValuesSet("global.discovery.kubernetesVersion", "1.17.8")
			f.HelmRender()
		})

		It("CCM should not be present on unsupported Kubernetes versions", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Deployment", "d8-cloud-provider-openstack", "cloud-controller-manager").Exists()).To(BeFalse())
		})
	})
})

/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template_tests

import (
	"os"
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
  enabledModules: ["vertical-pod-autoscaler-crd", "cloud-provider-vcd"]
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: sandbox
      provider: VCD
    clusterDomain: cluster.local
    clusterType: Cloud
    defaultCRI: Containerd
    kind: ClusterConfiguration
    kubernetesVersion: "1.25"
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
    kubernetesVersion: 1.25.1
    clusterUUID: cluster
`

const moduleValuesA = `
    internal:
      providerClusterConfiguration:
        apiVersion: deckhouse.io/v1alpha1
        kind: VCDClusterConfiguration
        provider:
          username: myuname
          password: myPaSsWd
          insecure: true
          server: "http://server/api"
        layout: Standard
        sshPublicKey: rsa-aaaa
        organization: org
        virtualDataCenter: dc
        virtualApplicationName: app
        masterNodeGroup:
          replicas: 1
          instanceClass:
            template: Templates/ubuntu-focal-20.04
            mainNetwork: internal
            sizingPolicy: 4cpu8ram
            rootDiskSizeGb: 20
            etcdDiskSizeGb: 20
            storageProfile: nvme
`

var _ = Describe("Module :: cloud-provider-vcd :: helm template ::", func() {
	f := SetupHelmConfig(``)
	BeforeSuite(func() {
		err := os.Remove("/deckhouse/ee/modules/030-cloud-provider-vcd/candi")
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Symlink("/deckhouse/ee/candi/cloud-providers/vcd", "/deckhouse/ee/modules/030-cloud-provider-vcd/candi")
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterSuite(func() {
		err := os.Remove("/deckhouse/ee/modules/030-cloud-provider-vcd/candi")
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Symlink("/deckhouse/candi/cloud-providers/vcd", "/deckhouse/ee/modules/030-cloud-provider-vcd/candi")
		Expect(err).ShouldNot(HaveOccurred())
	})

	Context("VCD", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderVcd", moduleValuesA)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			/*
							namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-provider-vcd")
							registrySecret := f.KubernetesResource("Secret", "d8-cloud-provider-vcd", "deckhouse-registry")

							providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")

							csiCongrollerPluginSS := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "csi-controller")
							csiDriver := f.KubernetesGlobalResource("CSIDriver", "named-disk.csi.cloud-director.vmware.com")
							csiNodePluginDS := f.KubernetesResource("DaemonSet", "d8-cloud-provider-vcd", "csi-node")
							csiSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-vcd", "csi")
							csiProvisionerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-vcd:csi:controller:external-provisioner")
							csiProvisionerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-vcd:csi:controller:external-provisioner")
							csiAttacherCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-vcd:csi:controller:external-attacher")
							csiAttacherCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-vcd:csi:controller:external-attacher")
							csiResizerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-vcd:csi:controller:external-resizer")
							csiResizerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-vcd:csi:controller:external-resizer")

							ccmSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-vcd", "cloud-controller-manager")
							ccmCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-vcd:cloud-controller-manager")
							ccmCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-vcd:cloud-controller-manager")
							ccmVPA := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-vcd", "cloud-controller-manager")
							ccmDeploy := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "cloud-controller-manager")
							ccmSecret := f.KubernetesResource("Secret", "d8-cloud-provider-vcd", "cloud-controller-manager")

							userAuthzUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-vcd:user")
							userAuthzClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-vcd:cluster-admin")

							Expect(namespace.Exists()).To(BeTrue())
							Expect(registrySecret.Exists()).To(BeTrue())
							Expect(userAuthzUser.Exists()).To(BeTrue())
							Expect(userAuthzClusterAdmin.Exists()).To(BeTrue())

							// user story #1
							Expect(providerRegistrationSecret.Exists()).To(BeTrue())
							expectedProviderRegistrationJSON := `{
				          "server": "myhost",
				          "insecure": true,
				          "password": "myPaSsWd",
				          "region": "myreg",
				          "regionTagCategory": "myregtagcat",
				          "instanceClassDefaults": {
				            "datastore": "dev/lun_1",
				            "template": "dev/golden_image",
				            "disableTimesync": true
				          },
				          "sshKey": "mysshkey1",
				          "username": "myuname",
				          "vmFolderPath": "dev/test",
				          "zoneTagCategory": "myzonetagcat"
				        }`
							providerRegistrationData, err := base64.StdEncoding.DecodeString(providerRegistrationSecret.Field("data.vsphere").String())
							Expect(err).ShouldNot(HaveOccurred())
							Expect(string(providerRegistrationData)).To(MatchJSON(expectedProviderRegistrationJSON))

							// user story #2
							Expect(csiDriver.Exists()).To(BeTrue())
							Expect(csiNodePluginDS.Exists()).To(BeTrue())
							Expect(csiSA.Exists()).To(BeTrue())
							Expect(csiCongrollerPluginSS.Exists()).To(BeTrue())
							Expect(csiAttacherCR.Exists()).To(BeTrue())
							Expect(csiAttacherCRB.Exists()).To(BeTrue())
							Expect(csiProvisionerCR.Exists()).To(BeTrue())
							Expect(csiProvisionerCRB.Exists()).To(BeTrue())
							Expect(csiResizerCR.Exists()).To(BeTrue())
							Expect(csiResizerCRB.Exists()).To(BeTrue())
							Expect(csiResizerCR.Exists()).To(BeTrue())
							Expect(csiResizerCRB.Exists()).To(BeTrue())

							Expect(ccmSA.Exists()).To(BeTrue())
							Expect(ccmCR.Exists()).To(BeTrue())
							Expect(ccmCRB.Exists()).To(BeTrue())
							Expect(ccmVPA.Exists()).To(BeTrue())
							Expect(ccmDeploy.Exists()).To(BeTrue())
							Expect(ccmSecret.Exists()).To(BeTrue())

							// user story #3
							scMydsname1 := f.KubernetesGlobalResource("StorageClass", "mydsname1")
							scMydsname2 := f.KubernetesGlobalResource("StorageClass", "mydsname2")

							Expect(scMydsname1.Exists()).To(BeTrue())
							Expect(scMydsname2.Exists()).To(BeTrue())

							Expect(scMydsname1.Field("metadata.annotations").String()).To(MatchYAML(`
				storageclass.deckhouse.io/volume-expansion-mode: offline
				storageclass.kubernetes.io/is-default-class: "true"
				`))
							Expect(scMydsname2.Field(`metadata.annotations.storageclass\.kubernetes\.io/is-default-class`).Exists()).To(BeFalse())
			*/
		})
	})
})

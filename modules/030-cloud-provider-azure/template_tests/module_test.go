/*
Copyright 2021 Flant JSC

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

/*

User-stories:
1. There are module settings. They must be exported via Secret d8-node-manager-cloud-provider.
2. There are applications which must be deployed — cloud-controller-manager, azure-csi-driver, simple-bridge.

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
  clusterIsBootstrapped: true
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: myprefix
      provider: Azure
    clusterDomain: cluster.local
    clusterType: "Cloud"
    defaultCRI: Containerd
    kind: ClusterConfiguration
    kubernetesVersion: "1.31"
    podSubnetCIDR: 10.111.0.0/16
    podSubnetNodeCIDRPrefix: "24"
    serviceSubnetCIDR: 10.222.0.0/16
  enabledModules: ["vertical-pod-autoscaler"]
  modules:
    placement: {}
  discovery:
    d8SpecificNodeCountByRole:
      worker: 1
      master: 3
    podSubnet: 10.0.1.0/16
    kubernetesVersion: 1.31.0
`

const moduleValues = `
  internal:
    providerClusterConfiguration:
      apiVersion: deckhouse.io/v1
      kind: AzureClusterConfiguration
      vNetCIDR: 10.0.0.0/16
      subnetCIDR: 10.0.0.0/24
      masterNodeGroup:
        replicas: 1
        instanceClass:
          machineSize: zzz
          urn: zzz
      layout: Standard
      sshPublicKey: zzz
      provider:
        clientId: zzz
        clientSecret: zzz
        subscriptionId: zzz
        tenantId: zzz
        location: zzz
    providerDiscoveryData:
      apiVersion: deckhouse.io/v1
      kind: AzureCloudDiscoveryData
      resourceGroupName: zzz
      vnetName: zzz
      subnetName: zzz
      zones: ["1"]
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

const moduleNamespace = "d8-cloud-provider-azure"

var _ = Describe("Module :: cloud-provider-azure :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Azure", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderAzure", moduleValues)
			fmt.Println(f.ValuesGet(""))
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", moduleNamespace)
			registrySecret := f.KubernetesResource("Secret", moduleNamespace, "deckhouse-registry")

			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")

			ccmVPA := f.KubernetesResource("VerticalPodAutoscaler", moduleNamespace, "cloud-controller-manager")
			ccmDeploy := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			ccmSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "cloud-controller-manager")
			ccmCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-azure:cloud-controller-manager")
			ccmCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-azure:cloud-controller-manager")
			ccmSecret := f.KubernetesResource("Secret", moduleNamespace, "cloud-controller-manager")

			azureControllerPluginSS := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			azureCSIDriver := f.KubernetesGlobalResource("CSIDriver", "disk.csi.azure.com")
			azureNodePluginDS := f.KubernetesResource("DaemonSet", moduleNamespace, "csi-node")
			azureControllerPluginSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "csi")
			azureProvisionerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-azure:csi:controller:external-provisioner")
			azureProvisionerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-azure:csi:controller:external-provisioner")
			azureAttacherCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-azure:csi:controller:external-attacher")
			azureAttacherCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-azure:csi:controller:external-attacher")
			azureResizerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-azure:csi:controller:external-resizer")
			azureResizerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-azure:csi:controller:external-resizer")

			azureCSIaaaSC := f.KubernetesGlobalResource("StorageClass", "aaa")
			azureCSIbbbSC := f.KubernetesGlobalResource("StorageClass", "bbb")
			azureCSIcccSC := f.KubernetesGlobalResource("StorageClass", "ccc")

			userAuthzUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-azure:user")
			userAuthzClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-azure:cluster-admin")

			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")

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

			Expect(azureCSIDriver.Exists()).To(BeTrue())
			Expect(azureNodePluginDS.Exists()).To(BeTrue())
			Expect(azureNodePluginDS.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(azureControllerPluginSA.Exists()).To(BeTrue())
			Expect(azureControllerPluginSS.Exists()).To(BeTrue())
			Expect(azureControllerPluginSS.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(azureAttacherCR.Exists()).To(BeTrue())
			Expect(azureAttacherCRB.Exists()).To(BeTrue())
			Expect(azureProvisionerCR.Exists()).To(BeTrue())
			Expect(azureProvisionerCRB.Exists()).To(BeTrue())
			Expect(azureResizerCR.Exists()).To(BeTrue())
			Expect(azureResizerCRB.Exists()).To(BeTrue())
			Expect(azureResizerCR.Exists()).To(BeTrue())
			Expect(azureResizerCRB.Exists()).To(BeTrue())

			Expect(azureCSIaaaSC.Exists()).To(BeTrue())
			Expect(azureCSIbbbSC.Exists()).To(BeTrue())
			Expect(azureCSIcccSC.Exists()).To(BeTrue())

			Expect(userAuthzUser.Exists()).To(BeTrue())
			Expect(userAuthzClusterAdmin.Exists()).To(BeTrue())

			Expect(cddDeployment.Exists()).To(BeTrue())
			Expect(cddDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(cddDeployment.Field("spec.template.spec.tolerations").String()).To(MatchYAML(tolerationsAnyNodeWithUninitialized))
		})

		Context("Unsupported Kubernetes version", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", globalValues)
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("cloudProviderAzure", moduleValues)
				f.ValuesSet("global.discovery.kubernetesVersion", "1.17.8")
				f.HelmRender()
			})

			It("CSI controller should not be present on unsupported Kubernetes versions", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				Expect(f.KubernetesResource("Deployment", moduleNamespace, "csi-controller").Exists()).To(BeFalse())
			})
		})
	})

	Context("Cloud data discoverer", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderAzure", moduleValues)
			f.HelmRender()
		})

		It("Should render cloud data discoverer deployment with two containers", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			d := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")
			Expect(d.Exists()).To(BeTrue())

			Expect(d.Field("spec.template.spec.containers.0.name").String()).To(Equal("cloud-data-discoverer"))
			Expect(d.Field("spec.template.spec.containers.1.name").String()).To(Equal("kube-rbac-proxy"))
		})

		It("Should render secret field", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			s := f.KubernetesResource("Secret", moduleNamespace, "cloud-data-discoverer")
			Expect(s.Exists()).To(BeTrue())
		})
	})

	Context("vertical-pod-autoscaler module enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler"]`)
			f.ValuesSetFromYaml("cloudProviderAzure", moduleValues)
			f.HelmRender()
		})

		It("Should render VPA resource", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			d := f.KubernetesResource("VerticalPodAutoscaler", moduleNamespace, "cloud-data-discoverer")
			Expect(d.Exists()).To(BeTrue())
		})
	})

	Context("vertical-pod-autoscaler module disabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("global.enabledModules", `[]`)
			f.ValuesSetFromYaml("cloudProviderAzure", moduleValues)
			f.HelmRender()
		})

		It("Should render VPA resource", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			d := f.KubernetesResource("VerticalPodAutoscaler", moduleNamespace, "cloud-data-discoverer")
			Expect(d.Exists()).To(BeFalse())
		})
	})
})

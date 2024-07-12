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
2. There are applications which must be deployed — cloud-controller-manager, ebs-csi-driver.

*/

package template_tests

import (
	"encoding/base64"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const globalValues = `
  enabledModules: ["vertical-pod-autoscaler-crd"]
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: sandbox
      provider: AWS
    clusterDomain: cluster.local
    clusterType: Cloud
    defaultCRI: Containerd
    kind: ClusterConfiguration
    kubernetesVersion: "1.27"
    podSubnetCIDR: 10.111.0.0/16
    podSubnetNodeCIDRPrefix: "24"
    serviceSubnetCIDR: 10.222.0.0/16
  modules:
    placement: {}
  discovery:
    d8SpecificNodeCountByRole:
      worker: 1
      master:
        __ConstantChoices__: "3"
    podSubnet: 10.0.1.0/16
    kubernetesVersion: 1.27.0
`

const moduleValues = `
  internal:
    storageClasses:
      - name: gp2
        type: gp2
      - name: st1
        type: st1
      - name: iops-foo
        type: io1
        iopsPerGB: "5"
      - name: gp3-foo
        type: gp3
        iops: "3000"
        throughput: "125"
    zoneToSubnetIdMap:
      zonea: aaa
      zoneb: bbb
    providerAccessKeyId: myprovacckeyid
    providerSecretAccessKey: myprovsecretaccesskey
    zones: ["zonea", "zoneb"]
    region: myregion
    instances:
      ami: ami-aaabbbccc
      associatePublicIPAddress: true
      iamProfileName: myiamprofile
      additionalSecurityGroups: ["id1", "id2"]
    loadBalancerSecurityGroup: mylbsecgroupid
    keyName: mykeyname
    tags:
      aaa: aaa
`

var _ = Describe("Module :: cloud-provider-aws :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("AWS", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderAws", moduleValues)
			fmt.Println(f.ValuesGet(""))
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-provider-aws")
			registrySecret := f.KubernetesResource("Secret", "d8-cloud-provider-aws", "deckhouse-registry")

			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")

			ccmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-aws", "cloud-controller-manager")
			ccmServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-aws", "cloud-controller-manager")
			ccmClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-aws:cloud-controller-manager")
			ccmClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-aws:cloud-controller-manager")
			ccmSecret := f.KubernetesResource("Secret", "d8-cloud-provider-aws", "cloud-controller-manager")

			ebsControllerPluginDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-aws", "csi-controller")
			ebsCSIDriver := f.KubernetesGlobalResource("CSIDriver", "ebs.csi.aws.com")
			ebsNodePluginDaemonSet := f.KubernetesResource("DaemonSet", "d8-cloud-provider-aws", "csi-node")
			ebsControllerSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-aws", "csi")
			ebsProvisionerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-aws:csi:controller:external-provisioner")
			ebsProvisionerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-aws:csi:controller:external-provisioner")
			ebsAttacherCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-aws:csi:controller:external-attacher")
			ebsAttacherCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-aws:csi:controller:external-attacher")
			ebsResizerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-aws:csi:controller:external-resizer")
			ebsResizerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-aws:csi:controller:external-resizer")
			ebsStorageClass := f.KubernetesGlobalResource("StorageClass", "gp2")

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
    "ami": "ami-aaabbbccc",
    "associatePublicIPAddress": true,
    "iamProfileName":"myiamprofile",
    "additionalSecurityGroups":["id1","id2"]},
    "internal":{
      "zoneToSubnetIdMap":{"zonea":"aaa","zoneb":"bbb"}
     },
    "keyName":"mykeyname",
    "loadBalancerSecurityGroup":"mylbsecgroupid",
    "providerAccessKeyId":"myprovacckeyid",
    "providerSecretAccessKey":"myprovsecretaccesskey",
    "region":"myregion",
    "tags":{
      "aaa": "aaa"
    }
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
			Expect(ebsControllerPluginDeployment.Exists()).To(BeTrue())
			Expect(ebsCSIDriver.Exists()).To(BeTrue())
			Expect(ebsNodePluginDaemonSet.Exists()).To(BeTrue())
			Expect(ebsControllerSA.Exists()).To(BeTrue())
			Expect(ebsProvisionerCR.Exists()).To(BeTrue())
			Expect(ebsProvisionerCRB.Exists()).To(BeTrue())
			Expect(ebsAttacherCR.Exists()).To(BeTrue())
			Expect(ebsAttacherCRB.Exists()).To(BeTrue())
			Expect(ebsResizerCR.Exists()).To(BeTrue())
			Expect(ebsResizerCRB.Exists()).To(BeTrue())
			Expect(ebsStorageClass.Exists()).To(BeTrue())
			Expect(ebsStorageClass.Field("metadata.annotations").String()).To(MatchYAML(`
storageclass.kubernetes.io/is-default-class: "true"
`))
		})
	})

	Context("AWS with default StorageClass specified", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderAws", moduleValues)
			f.ValuesSetFromYaml("cloudProviderAws.internal.defaultStorageClass", `iops-foo`)
			f.HelmRender()
		})

		It("Everything must render properly with proper default StorageClass", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			gp2StorageClass := f.KubernetesGlobalResource("StorageClass", "gp2")
			st1StorageClass := f.KubernetesGlobalResource("StorageClass", "st1")
			iopsStorageClass := f.KubernetesGlobalResource("StorageClass", "iops-foo")

			Expect(gp2StorageClass.Exists()).To(BeTrue())
			Expect(st1StorageClass.Exists()).To(BeTrue())
			Expect(iopsStorageClass.Exists()).To(BeTrue())

			Expect(gp2StorageClass.Field(`metadata.annotations.storageclass\.kubernetes\.io/is-default-class`).Exists()).To(BeFalse())
			Expect(st1StorageClass.Field(`metadata.annotations.storageclass\.kubernetes\.io/is-default-class`).Exists()).To(BeFalse())
			Expect(iopsStorageClass.Field("metadata.annotations").String()).To(MatchYAML(`
storageclass.kubernetes.io/is-default-class: "true"
`))
			Expect(iopsStorageClass.Field("parameters.iopsPerGB").String()).Should(Equal(`5`))
		})

		Context("Unsupported Kubernetes version", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", globalValues)
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("cloudProviderAws", moduleValues)
				f.ValuesSet("global.discovery.kubernetesVersion", "1.17.8")
				f.HelmRender()
			})

			It("CCM should not be present on unsupported Kubernetes versions", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				Expect(f.KubernetesResource("Deployment", "d8-cloud-provider-aws", "cloud-controller-manager").Exists()).To(BeFalse())
				Expect(f.KubernetesResource("Deployment", "d8-cloud-provider-aws", "csi-controller").Exists()).To(BeFalse())
			})
		})
	})

	Context("Cloud data discoverer", func() {
		deployment := func(f *Config) object_store.KubeObject {
			return f.KubernetesResource("Deployment", "d8-cloud-provider-aws", "cloud-data-discoverer")
		}

		assertEnv := func(f *Config, envName, val string) {
			d := deployment(f)
			Expect(d.Exists()).To(BeTrue())

			envs := d.Field("spec.template.spec.containers.0.env").Array()

			found := false
			for _, e := range envs {
				if e.Map()["name"].String() == envName {
					found = true

					Expect(e.Map()["value"].String()).To(Equal(val))

					break
				}
			}

			Expect(found).To(BeTrue())
		}

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderAws", moduleValues)
			f.HelmRender()
		})

		It("Should render cloud data discoverer deployment with two containers", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			d := deployment(f)
			Expect(d.Exists()).To(BeTrue())

			Expect(d.Field("spec.template.spec.containers.0.name").String()).To(Equal("cloud-data-discoverer"))
			Expect(d.Field("spec.template.spec.containers.1.name").String()).To(Equal("kube-rbac-proxy"))
		})

		It("Should render AWS_REGION env for first container", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			assertEnv(f, "AWS_REGION", "myregion")
		})

		Context("vertical-pod-autoscaler-crd module enabled", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", globalValues)
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler-crd"]`)
				f.ValuesSetFromYaml("cloudProviderAws", moduleValues)
				f.HelmRender()
			})

			It("Should render VPA resource", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				d := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-aws", "cloud-data-discoverer")
				Expect(d.Exists()).To(BeTrue())
			})
		})

		Context("vertical-pod-autoscaler-crd module disabled", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", globalValues)
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("global.enabledModules", `[]`)
				f.ValuesSetFromYaml("cloudProviderAws", moduleValues)
				f.HelmRender()
			})

			It("Should render VPA resource", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				d := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-aws", "cloud-data-discoverer")
				Expect(d.Exists()).To(BeFalse())
			})
		})
	})
})

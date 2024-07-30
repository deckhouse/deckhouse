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
2. There are applications which must be deployed — cloud-controller-manager, pd-csi-driver.

*/

package template_tests

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"sort"
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

var (
	zonesRE   = regexp.MustCompile(`--zone=([a-z]+)`)
	projectRE = regexp.MustCompile(`--project=([a-z\-]+)`)
)

const globalValues = `
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: myprefix
      provider: GCP
    clusterDomain: cluster.local
    clusterType: "Cloud"
    defaultCRI: Containerd
    kind: ClusterConfiguration
    kubernetesVersion: "1.29"
    podSubnetCIDR: 10.111.0.0/16
    podSubnetNodeCIDRPrefix: "24"
    serviceSubnetCIDR: 10.222.0.0/16
  enabledModules: ["vertical-pod-autoscaler-crd"]
  modules:
    placement: {}
  discovery:
    d8SpecificNodeCountByRole:
      worker: 1
      master: 3
    podSubnet: 10.0.1.0/16
    kubernetesVersion: 1.29.0
`

const moduleValues = `
  internal:
    storageClasses:
    - name: pd-standard-not-replicated
      type: pd-standard
      replicationType: none
    - name: pd-standard-replicated
      type: pd-standard
      replicationType: regional-pd
    - name: pd-balanced-not-replicated
      type: pd-balanced
      replicationType: none
    - name: pd-balanced-replicated
      type: pd-balanced
      replicationType: regional-pd
    - name: pd-ssd-not-replicated
      type: pd-ssd
      replicationType: none
    - name: pd-ssd-replicated
      type: pd-ssd
      replicationType: regional-pd
    providerClusterConfiguration:
      apiVersion: deckhouse.io/v1
      kind: GCPClusterConfiguration
      layout: WithoutNAT
      sshKey: "mysshkey"
      subnetworkCIDR: 10.160.0.0/16
      masterNodeGroup:
        replicas: 1
        zones:
          - test-a
        instanceClass:
          machineType: n1-standard-4
          image: ubuntu
          diskSizeGB: 20
          disableExternalIP: false
      provider:
        region: myregion
        serviceAccountJSON: '{"project_id": "test"}'
    providerDiscoveryData:
      apiVersion: deckhouse.io/v1
      kind: GCPCloudDiscoveryData
      networkName: mynetname
      subnetworkName: mysubnetname
      zones: ["zonea", "zoneb"]
      disableExternalIP: false
      instances:
        image: image
        diskSizeGb: 50
        diskType: disk-type
        networkTags: ["tag1", "tag2"]
        labels:
            test: test
`

var _ = Describe("Module :: cloud-provider-gcp :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("GCP", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
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

			pdCSISS := f.KubernetesResource("Deployment", "d8-cloud-provider-gcp", "csi-controller")
			pdCSICSIDriver := f.KubernetesGlobalResource("CSIDriver", "pd.csi.storage.gke.io")
			pdCSIDS := f.KubernetesResource("DaemonSet", "d8-cloud-provider-gcp", "csi-node")
			pdCSIControllerSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-gcp", "csi")
			pdCSIProvisionerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-gcp:csi:controller:external-provisioner")
			pdCSIProvisionerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-gcp:csi:controller:external-provisioner")
			pdCSIAttacherCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-gcp:csi:controller:external-attacher")
			pdCSIAttacherCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-gcp:csi:controller:external-attacher")
			pdCSIResizerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-gcp:csi:controller:external-resizer")
			pdCSIResizerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-gcp:csi:controller:external-resizer")
			pdCSIStandardNotReplicatedSC := f.KubernetesGlobalResource("StorageClass", "pd-standard-not-replicated")
			pdCSIStandardReplicatedSC := f.KubernetesGlobalResource("StorageClass", "pd-standard-replicated")
			pdCSIBalancedNotReplicatedSC := f.KubernetesGlobalResource("StorageClass", "pd-balanced-not-replicated")
			pdCSIBalancedReplicatedSC := f.KubernetesGlobalResource("StorageClass", "pd-balanced-replicated")
			pdCSISSDNotReplicatedSC := f.KubernetesGlobalResource("StorageClass", "pd-ssd-not-replicated")
			pdCSISSDReplicatedSC := f.KubernetesGlobalResource("StorageClass", "pd-ssd-replicated")

			userAuthzUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-gcp:user")
			userAuthzClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-gcp:cluster-admin")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())

			// user story #1
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())
			expectedProviderRegistrationJSON := `{
          "disableExternalIP": false,
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
          "serviceAccountJSON": "{\"project_id\": \"test\"}",
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
			Expect(pdCSIControllerSA.Exists()).To(BeTrue())
			Expect(pdCSIProvisionerCR.Exists()).To(BeTrue())
			Expect(pdCSIProvisionerCRB.Exists()).To(BeTrue())
			Expect(pdCSIAttacherCR.Exists()).To(BeTrue())
			Expect(pdCSIAttacherCRB.Exists()).To(BeTrue())
			Expect(pdCSIResizerCR.Exists()).To(BeTrue())
			Expect(pdCSIResizerCRB.Exists()).To(BeTrue())
			Expect(pdCSIStandardNotReplicatedSC.Exists()).To(BeTrue())
			Expect(pdCSIStandardReplicatedSC.Exists()).To(BeTrue())
			Expect(pdCSIBalancedNotReplicatedSC.Exists()).To(BeTrue())
			Expect(pdCSIBalancedReplicatedSC.Exists()).To(BeTrue())
			Expect(pdCSISSDNotReplicatedSC.Exists()).To(BeTrue())
			Expect(pdCSISSDReplicatedSC.Exists()).To(BeTrue())

			Expect(pdCSIStandardNotReplicatedSC.Field("metadata.annotations").String()).To(MatchYAML(`
storageclass.kubernetes.io/is-default-class: "true"
`))

			Expect(userAuthzUser.Exists()).To(BeTrue())
			Expect(userAuthzClusterAdmin.Exists()).To(BeTrue())
		})

		Context("Unsupported Kubernetes version", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", globalValues)
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("cloudProviderGcp", moduleValues)
				f.ValuesSet("global.discovery.kubernetesVersion", "1.17.8")
				f.HelmRender()
			})

			It("CCM and CSI controller should not be present on unsupported Kubernetes versions", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				Expect(f.KubernetesResource("Deployment", "d8-cloud-provider-gcp", "cloud-controller-manager").Exists()).To(BeFalse())
				Expect(f.KubernetesResource("Deployment", "d8-cloud-provider-gcp", "csi-controller").Exists()).To(BeFalse())
			})
		})
	})

	Context("GCP with default StorageClass specified", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderGcp", moduleValues)
			f.ValuesSetFromYaml("cloudProviderGcp.internal.defaultStorageClass", `pd-ssd-replicated`)
			f.HelmRender()
		})

		It("Everything must render properly with proper default StorageClass", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			pdCSIStandardNotReplicatedSC := f.KubernetesGlobalResource("StorageClass", "pd-standard-not-replicated")
			pdCSIStandardReplicatedSC := f.KubernetesGlobalResource("StorageClass", "pd-standard-replicated")
			pdCSIBalancedNotReplicatedSC := f.KubernetesGlobalResource("StorageClass", "pd-balanced-not-replicated")
			pdCSIBalancedReplicatedSC := f.KubernetesGlobalResource("StorageClass", "pd-balanced-replicated")
			pdCSISSDNotReplicatedSC := f.KubernetesGlobalResource("StorageClass", "pd-ssd-not-replicated")
			pdCSISSDReplicatedSC := f.KubernetesGlobalResource("StorageClass", "pd-ssd-replicated")

			Expect(pdCSIStandardNotReplicatedSC.Exists()).To(BeTrue())
			Expect(pdCSIStandardReplicatedSC.Exists()).To(BeTrue())
			Expect(pdCSIBalancedNotReplicatedSC.Exists()).To(BeTrue())
			Expect(pdCSIBalancedReplicatedSC.Exists()).To(BeTrue())
			Expect(pdCSISSDNotReplicatedSC.Exists()).To(BeTrue())
			Expect(pdCSISSDReplicatedSC.Exists()).To(BeTrue())

			Expect(pdCSIStandardNotReplicatedSC.Field(`metadata.annotations.storageclass\.kubernetes\.io/is-default-class`).Exists()).To(BeFalse())
			Expect(pdCSIStandardReplicatedSC.Field(`metadata.annotations.storageclass\.kubernetes\.io/is-default-class`).Exists()).To(BeFalse())
			Expect(pdCSIBalancedNotReplicatedSC.Field(`metadata.annotations.storageclass\.kubernetes\.io/is-default-class`).Exists()).To(BeFalse())
			Expect(pdCSIBalancedReplicatedSC.Field(`metadata.annotations.storageclass\.kubernetes\.io/is-default-class`).Exists()).To(BeFalse())
			Expect(pdCSISSDNotReplicatedSC.Field(`metadata.annotations.storageclass\.kubernetes\.io/is-default-class`).Exists()).To(BeFalse())
			Expect(pdCSISSDReplicatedSC.Field("metadata.annotations").String()).To(MatchYAML(`
storageclass.kubernetes.io/is-default-class: "true"
`))
		})
	})

	Context("Cloud data discoverer", func() {
		deployment := func(f *Config) object_store.KubeObject {
			return f.KubernetesResource("Deployment", "d8-cloud-provider-gcp", "cloud-data-discoverer")
		}

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderGcp", moduleValues)
			f.ValuesSet("cloudProviderGcp.internal.providerClusterConfiguration.provider.serviceAccountJSON", `{"project_id": "my-proj"}`)
			f.HelmRender()
		})

		It("Should render cloud data discoverer deployment with two containers", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			d := deployment(f)
			Expect(d.Exists()).To(BeTrue())

			Expect(d.Field("spec.template.spec.containers.0.name").String()).To(Equal("cloud-data-discoverer"))
			Expect(d.Field("spec.template.spec.containers.1.name").String()).To(Equal("kube-rbac-proxy"))
		})

		It("Should render all zones in arguments", func() {
			d := deployment(f)
			Expect(d.Exists()).To(BeTrue())

			args := d.Field("spec.template.spec.containers.0.args").Array()
			zones := make([]string, 0, 2)

			for _, a := range args {
				found := zonesRE.FindAllStringSubmatch(a.String(), -1)
				if len(found) > 0 {
					zones = append(zones, found[0][1])
				}
			}

			sort.Strings(zones)

			Expect(zones).To(Equal([]string{"zonea", "zoneb"}))
		})

		It("Should render project in arguments", func() {
			d := deployment(f)
			Expect(d.Exists()).To(BeTrue())

			args := d.Field("spec.template.spec.containers.0.args").Array()

			project := ""
			for _, a := range args {
				found := projectRE.FindAllStringSubmatch(a.String(), -1)
				if len(found) > 0 {
					project = found[0][1]
				}
			}
			Expect(project).To(Equal("my-proj"))
		})

		Context("vertical-pod-autoscaler-crd module enabled", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", globalValues)
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler-crd"]`)
				f.ValuesSetFromYaml("cloudProviderGcp", moduleValues)
				f.HelmRender()
			})

			It("Should render VPA resource", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				d := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-gcp", "cloud-data-discoverer")
				Expect(d.Exists()).To(BeTrue())
			})
		})

		Context("vertical-pod-autoscaler-crd module disabled", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", globalValues)
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("global.enabledModules", `[]`)
				f.ValuesSetFromYaml("cloudProviderGcp", moduleValues)
				f.HelmRender()
			})

			It("Should render VPA resource", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				d := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-gcp", "cloud-data-discoverer")
				Expect(d.Exists()).To(BeFalse())
			})
		})
	})
})

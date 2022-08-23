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
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: myprefix
      provider: GCP
    clusterDomain: cluster.local
    clusterType: "Cloud"
    defaultCRI: Docker
    kind: ClusterConfiguration
    kubernetesVersion: "1.19"
    podSubnetCIDR: 10.111.0.0/16
    podSubnetNodeCIDRPrefix: "24"
    serviceSubnetCIDR: 10.222.0.0/16
  enabledModules: ["vertical-pod-autoscaler-crd"]
  modules:
    placement: {}
  modulesImages:
    registry: registry.deckhouse.io/deckhouse/fe
    registryDockercfg: Y2ZnCg==
    tags:
      common:
        csiExternalProvisioner116: imagehash
        csiExternalAttacher116: imagehash
        csiExternalResizer116: imagehash
        csiNodeDriverRegistrar116: imagehash
        csiExternalProvisioner119: imagehash
        csiExternalAttacher119: imagehash
        csiExternalResizer119: imagehash
        csiNodeDriverRegistrar119: imagehash
        resolvWatcher: imagehash
      cloudProviderGcp:
        cloudControllerManager116: imagehash
        cloudControllerManager119: imagehash
        pdCsiPlugin: imagehash
  discovery:
    d8SpecificNodeCountByRole:
      worker: 1
      master: 3
    podSubnet: 10.0.1.0/16
    kubernetesVersion: 1.16.4
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
      sshKey: mysshkey
      provider:
        region: myregion
        serviceAccountJSON: mysvcacckey
    providerDiscoveryData:
      disableExternalIP: true
      instances:
        diskSizeGb: 50
        diskType: disk-type
        image: image
        networkTags: ["tag1", "tag2"]
        labels:
          test: test
      networkName: mynetname
      subnetworkName: mysubnetname
      zones: ["zonea", "zoneb"]
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
          "disableExternalIP": true,
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
          "serviceAccountJSON": "mysvcacckey",
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

})

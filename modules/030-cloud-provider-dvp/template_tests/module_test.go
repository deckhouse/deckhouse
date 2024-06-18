/*
Copyright 2024 Flant JSC

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
  enabledModules: ["vertical-pod-autoscaler-crd", "cloud-provider-dvp"]
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: sandbox
      provider: DVP
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
    kubernetesVersion: 1.29.0
    podSubnet: 10.111.0.0/16
    d8SpecificNodeCountByRole:
      worker: 1
      master: 3
`

const moduleValuesA = `
    internal:
      providerDiscoveryData:
        kind: DVPCloudDiscoveryData
        apiVersion: deckhouse.io/v1
        storageClasses:
        - name: sc1
          isDefault: true
        - name: sc2
      providerClusterConfiguration:
        apiVersion: deckhouse.io/v1
        kind: DVPClusterConfiguration
        layout: Standard
        sshPublicKey: rsa-aaaa
        zones:
        - zone-a
        - zone-b
        - zone-c
        region: r1
        masterNodeGroup:
          replicas: 3
          zones:
          - zone-a
          - zone-b
          - zone-c
          instanceClass:
            virtualMachine:
              cpu:
                cores: 1
                coreFraction: 100%
              memory:
                size: 4Gi
              ipAddresses:
              - 10.66.30.100
              - 10.66.30.101
              - 10.66.30.102
              additionalLabels:
                additional-vm-label: label-value
              additionalAnnotations:
                additional-vm-annotation: annotation-value
              tolerations:
              - key: "dedicated.deckhouse.io"
                operator: "Equal"
                value: "system"
              nodeSelector:
                beta.kubernetes.io/os: linux
            rootDisk:
              size: 10Gi
              storageClass: linstor-thin-r1
              image:
                kind: ClusterVirtualImage
                name: ubuntu-2204
            etcdDisk:
              size: 10Gi
              storageClass: linstor-thin-r1
        nodeGroups:
          - name: worker
            zones:
            - zone-a
            - zone-b
            - zone-c
            replicas: 1
            instanceClass:
              virtualMachine:
                cpu:
                  cores: 4
                  coreFraction: 100%
                memory:
                  size: 8Gi
              rootDisk:
                size: 10Gi
                image:
                  kind: ClusterVirtualImage
                  name: ubuntu-2204
        provider:
          kubeconfigDataBase64: ZXhhbXBsZQo=
          namespace: default
`

var _ = Describe("Module :: cloud-provider-dvp :: helm template ::", func() {
	f := SetupHelmConfig(``)
	Context("DVP", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesA)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// Common
			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-provider-dvp")
			Expect(namespace.Exists()).To(BeTrue())
			registrySecret := f.KubernetesResource("Secret", "d8-cloud-provider-dvp", "deckhouse-registry")
			Expect(registrySecret.Exists()).To(BeTrue())

			// CSI
			regSecret := f.KubernetesResource("Secret", "d8-cloud-provider-dvp", "csi-credentials")
			Expect(regSecret.Exists()).To(BeTrue())
			Expect(regSecret.Field("data.kubeconfigDataBase64").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("ZXhhbXBsZQo="))))

			csiDriver := f.KubernetesGlobalResource("CSIDriver", "disk.csi.virtualization.deckhouse.io")
			Expect(csiDriver.Exists()).To(BeTrue())

			csiControllerSS := f.KubernetesResource("Deployment", "d8-cloud-provider-dvp", "csi-controller")
			Expect(csiControllerSS.Exists()).To(BeTrue())

			csiNodeDS := f.KubernetesResource("DaemonSet", "d8-cloud-provider-dvp", "csi-node")
			Expect(csiNodeDS.Exists()).To(BeTrue())

			csiControllerSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-dvp", "csi")
			Expect(csiControllerSA.Exists()).To(BeTrue())

			csiExternalAttacherCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-dvp:csi:controller:external-attacher")
			Expect(csiExternalAttacherCR.Exists()).To(BeTrue())

			csiExternalAttacherCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-dvp:csi:controller:external-attacher")
			Expect(csiExternalAttacherCRB.Exists()).To(BeTrue())

			csiExternalResizerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-dvp:csi:controller:external-resizer")
			Expect(csiExternalResizerCR.Exists()).To(BeTrue())

			csiExternalResizerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-dvp:csi:controller:external-resizer")
			Expect(csiExternalResizerCRB.Exists()).To(BeTrue())

			storageClass1 := f.KubernetesGlobalResource("StorageClass", "sc1")
			Expect(storageClass1.Exists()).To(BeTrue())
		})
	})
})

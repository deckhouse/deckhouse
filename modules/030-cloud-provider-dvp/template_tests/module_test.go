/*
Copyright 2025 Flant JSC

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
  enabledModules: ["vertical-pod-autoscaler", "vertical-pod-autoscaler-crd", "cloud-provider-dvp"]
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: sandbox
      provider: DVP
    clusterDomain: cluster.local
    clusterType: Cloud
    defaultCRI: Containerd
    kind: ClusterConfiguration
    kubernetesVersion: "1.30"
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
    kubernetesVersion: 1.30.1
    clusterUUID: cluster
`

const moduleValuesA = `
internal:
  providerClusterConfiguration:
    apiVersion: deckhouse.io/v1
    kind: DVPClusterConfiguration
    layout: Standard
    masterNodeGroup:
      instanceClass:
        etcdDisk:
          size: 15Gi
          storageClass: ceph-pool-r2-csi-rbd-immediate
        rootDisk:
          image:
            kind: ClusterVirtualImage
            name: ubuntu-2204
          size: 50Gi
          storageClass: ceph-pool-r2-csi-rbd-immediate
        virtualMachine:
          bootloader: EFI
          cpu:
            coreFraction: 100%
            cores: 4
          ipAddresses:
            - Auto
          memory:
            size: 8Gi
      replicas: 3
    provider:
      kubeconfigDataBase64: YXBpVmV=
      namespace: cloud-provider01
    sshPublicKey: ssh-rsa AAAAB3N
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: DVPCloudDiscoveryData
    zones:
      - default
  storageClasses:
    - dvpStorageClass: 1test
      name: 1test
    - dvpStorageClass: ceph-pool-r2-csi-cephfs
      name: ceph-pool-r2-csi-cephfs
    - dvpStorageClass: ceph-pool-r2-csi-rbd
      name: ceph-pool-r2-csi-rbd
    - dvpStorageClass: ceph-pool-r2-csi-rbd-immediate
      name: ceph-pool-r2-csi-rbd-immediate
    - dvpStorageClass: ceph-pool-r2-csi-rbd-immediate-feat
      name: ceph-pool-r2-csi-rbd-immediate-feat
    - dvpStorageClass: linstor-thin-r1
      name: linstor-thin-r1
    - dvpStorageClass: linstor-thin-r2
      name: linstor-thin-r2
    - dvpStorageClass: sds-local-storage
      name: sds-local-storage
    - dvpStorageClass: xxx
      name: xxx
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

			regSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")
			Expect(regSecret.Exists()).To(BeTrue())
			Expect(regSecret.Field("data.capiClusterName").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("dvp"))))
		})
	})
})

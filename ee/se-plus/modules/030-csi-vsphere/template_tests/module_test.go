/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

/*

User-stories:
1. There are module settings. They must be exported via Secret d8-node-manager-cloud-provider.
2. There are applications which must be deployed — cloud-controller-manager, csi.
3. StorageClass must be created for every internal.storageClasses. One mentioned in value `.storageClass.default` must be annotated as default.

*/

package template_tests

import (
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
enabledModules: ["vertical-pod-autoscaler", "csi-vsphere"]
clusterConfiguration:
  apiVersion: deckhouse.io/v1
  clusterDomain: cluster.local
  clusterType: Static
  defaultCRI: Containerd
  kind: ClusterConfiguration
  kubernetesVersion: "%s"
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
  kubernetesVersion: "%s.1"
`

const moduleValuesA = `
host: myhost
username: myuname
password: myPaSsWd
vmFolderPath: dev/test
regionTagCategory: myregtagcat
zoneTagCategory: myzonetagcat
region: myreg
zones: ["zonea", "zoneb"]
internal:
  storageClasses:
  - name: mydsname1
    datastoreType: Datastore
    datastoreURL: ds:///vmfs/volumes/hash1/
    path: /my/ds/path/mydsname1
    zones: ["zonea", "zoneb"]
  - name: mydsname2
    datastoreType: Datastore
    datastoreURL: ds:///vmfs/volumes/hash2/
    path: /my/ds/path/mydsname2
    zones: ["zonea", "zoneb"]
  compatibilityFlag: ""
  providerDiscoveryData:
    datacenter: X1
    zones: ["aaa", "bbb"]
  providerClusterConfiguration:
    provider:
      server: myhost
      username: myuname
      password: myPaSsWd
      insecure: true
    regionTagCategory: myregtagcat
    zoneTagCategory: myzonetagcat
    region: myreg
    vmFolderPath: dev/test
`

const moduleValuesB = `
    host: myhost
    username: myuname
    password: myPaSsWd
    vmFolderPath: dev/test
    regionTagCategory: myregtagcat
    zoneTagCategory: myzonetagcat
    region: myreg
    zones: ["zonea", "zoneb"]
    internal:
      storageClasses:
      - name: mydsname1
        datastoreType: Datastore
        datastoreURL: ds:///vmfs/volumes/hash1/
        path: /my/ds/path/mydsname1
        zones: ["zonea", "zoneb"]
      - name: mydsname2
        datastoreType: Datastore
        datastoreURL: ds:///vmfs/volumes/hash2/
        path: /my/ds/path/mydsname2
        zones: ["zonea", "zoneb"]
      compatibilityFlag: ""
      providerDiscoveryData:
        resourcePoolPath: kubernetes-dev
        zones: ["aaa", "bbb"]
        datacenter: X1
      providerClusterConfiguration:
        provider:
          server: myhost
          username: myuname
          password: myPaSsWd
          insecure: true
        regionTagCategory: myregtagcat
        zoneTagCategory: myzonetagcat
        region: myreg
        vmFolderPath: dev/test
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
- key: node.kubernetes.io/not-ready
- key: node.kubernetes.io/out-of-disk
- key: node.kubernetes.io/memory-pressure
- key: node.kubernetes.io/disk-pressure
- key: node.kubernetes.io/pid-pressure
- key: node.kubernetes.io/unreachable
- key: node.kubernetes.io/network-unavailable`

const moduleNamespace = "d8-csi-vsphere"

var _ = Describe("Module :: csi-vsphere :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Vsphere", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.31", "1.31"))
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("csiVsphere", moduleValuesA)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", moduleNamespace)
			registrySecret := f.KubernetesResource("Secret", moduleNamespace, "deckhouse-registry")
			csiCongrollerPluginSS := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			csiDriver := f.KubernetesGlobalResource("CSIDriver", "csi.vsphere.vmware.com")
			csiNodePluginDS := f.KubernetesResource("DaemonSet", moduleNamespace, "csi-node")
			csiSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "csi")
			csiProvisionerCR := f.KubernetesGlobalResource("ClusterRole", "d8:csi-vsphere:csi:controller:external-provisioner")
			csiProvisionerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:csi-vsphere:csi:controller:external-provisioner")
			csiAttacherCR := f.KubernetesGlobalResource("ClusterRole", "d8:csi-vsphere:csi:controller:external-attacher")
			csiAttacherCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:csi-vsphere:csi:controller:external-attacher")
			csiResizerCR := f.KubernetesGlobalResource("ClusterRole", "d8:csi-vsphere:csi:controller:external-resizer")
			csiResizerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:csi-vsphere:csi:controller:external-resizer")
			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())

			// user story #2
			Expect(csiDriver.Exists()).To(BeTrue())
			Expect(csiNodePluginDS.Exists()).To(BeTrue())
			Expect(csiNodePluginDS.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(csiSA.Exists()).To(BeTrue())
			Expect(csiCongrollerPluginSS.Exists()).To(BeTrue())
			Expect(csiCongrollerPluginSS.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(csiAttacherCR.Exists()).To(BeTrue())
			Expect(csiAttacherCRB.Exists()).To(BeTrue())
			Expect(csiProvisionerCR.Exists()).To(BeTrue())
			Expect(csiProvisionerCRB.Exists()).To(BeTrue())
			Expect(csiResizerCR.Exists()).To(BeTrue())
			Expect(csiResizerCRB.Exists()).To(BeTrue())
			Expect(csiResizerCR.Exists()).To(BeTrue())
			Expect(csiResizerCRB.Exists()).To(BeTrue())

			// user story #3
			scMydsname1 := f.KubernetesGlobalResource("StorageClass", "mydsname1")
			scMydsname2 := f.KubernetesGlobalResource("StorageClass", "mydsname2")

			Expect(scMydsname1.Exists()).To(BeTrue())
			Expect(scMydsname2.Exists()).To(BeTrue())

			Expect(scMydsname2.Field(`metadata.annotations.storageclass\.kubernetes\.io/is-default-class`).Exists()).To(BeFalse())

			Expect(cddDeployment.Exists()).To(BeTrue())
			Expect(cddDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(cddDeployment.Field("spec.template.spec.tolerations").String()).To(MatchYAML(tolerationsAnyNodeWithUninitialized))
		})
	})

	Context("Vsphere", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.31", "1.31"))
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("csiVsphere", moduleValuesB)
			f.HelmRender()
		})

		Context("Unsupported Kubernetes version", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.31", "1.31"))
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("csiVsphere", moduleValuesA)
				f.ValuesSet("global.discovery.kubernetesVersion", "1.17.8")
				f.HelmRender()
			})

			It("CSI controller should not be present on unsupported Kubernetes versions", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				Expect(f.KubernetesResource("Deployment", moduleNamespace, "csi-controller").Exists()).To(BeFalse())
			})
		})
	})

	Context("Vsphere with default StorageClass specified", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.31", "1.31"))
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("csiVsphere", moduleValuesB)
			f.ValuesSetFromYaml("global.discovery.defaultStorageClass", `mydsname2`)
			f.HelmRender()
		})

		It("Everything must render properly with proper default StorageClass", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			scMydsname1 := f.KubernetesGlobalResource("StorageClass", "mydsname1")
			scMydsname2 := f.KubernetesGlobalResource("StorageClass", "mydsname2")

			Expect(scMydsname1.Exists()).To(BeTrue())
			Expect(scMydsname2.Exists()).To(BeTrue())

			Expect(scMydsname1.Field(`metadata.annotations.storageclass\.kubernetes\.io/is-default-class`).Exists()).To(BeFalse())
		})
	})
})

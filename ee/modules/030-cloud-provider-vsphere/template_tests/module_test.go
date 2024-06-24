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
	"encoding/base64"
	"fmt"
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
  enabledModules: ["vertical-pod-autoscaler-crd", "cloud-provider-vsphere"]
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: sandbox
      provider: vSphere
    clusterDomain: cluster.local
    clusterType: Cloud
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
      vsphereDiscoveryData:
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
        sshPublicKey: mysshkey1
        vmFolderPath: dev/test
        masterNodeGroup:
          instanceClass:
            datastore: dev/lun_1
            mainNetwork: k8s-msk/test_187
            memory: 8192
            numCPUs: 4
            template: dev/golden_image
          replicas: 1
`

const moduleValuesB = `
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
      vsphereDiscoveryData:
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
        sshPublicKey: mysshkey1
        vmFolderPath: dev/test
        externalNetworkNames: ["aaa", "bbb"]
        internalNetworkNames: ["ccc", "ddd"]
      providerDiscoveryData:
        resourcePoolPath: kubernetes-dev
        zones:
        - default
`

const moduleValuesC = `
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
      vsphereDiscoveryData:
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
        sshPublicKey: mysshkey1
        vmFolderPath: dev/test
        externalNetworkNames: ["aaa", "bbb"]
        internalNetworkNames: ["ccc", "ddd"]
        nsxt:
          defaultIpPoolName: main
          defaultTcpAppProfileName: default-tcp-lb-app-profile
          defaultUdpAppProfileName: default-udp-lb-app-profile
          size: SMALL
          tier1GatewayPath: /host/tier1
          user: user
          password: password
          host: 1.2.3.4
      providerDiscoveryData:
        resourcePoolPath: kubernetes-dev
`

const moduleValuesD = `
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
      vsphereDiscoveryData:
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
        sshPublicKey: mysshkey1
        vmFolderPath: dev/test
        externalNetworkNames: ["aaa", "bbb"]
        internalNetworkNames: ["ccc", "ddd"]
        nsxt:
          defaultIpPoolName: main
          size: SMALL
          defaultTcpAppProfileName: default-tcp-lb-app-profile
          defaultUdpAppProfileName: default-udp-lb-app-profile
          tier1GatewayPath: /host/tier1
          user: user
          password: password
          host: 1.2.3.4
          loadBalancerClass:
          - name: class1
            ipPoolName: pool2
            tcpAppProfileName: profile1
      providerDiscoveryData:
        resourcePoolPath: kubernetes-dev
`

var _ = Describe("Module :: cloud-provider-vsphere :: helm template ::", func() {
	f := SetupHelmConfig(``)

	BeforeSuite(func() {
		err := os.Remove("/deckhouse/ee/modules/030-cloud-provider-vsphere/candi")
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Symlink("/deckhouse/ee/candi/cloud-providers/vsphere", "/deckhouse/ee/modules/030-cloud-provider-vsphere/candi")
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterSuite(func() {
		err := os.Remove("/deckhouse/ee/modules/030-cloud-provider-vsphere/candi")
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Symlink("/deckhouse/candi/cloud-providers/vsphere", "/deckhouse/ee/modules/030-cloud-provider-vsphere/candi")
		Expect(err).ShouldNot(HaveOccurred())
	})

	Context("Vsphere", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.29", "1.29"))
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderVsphere", moduleValuesA)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-provider-vsphere")
			registrySecret := f.KubernetesResource("Secret", "d8-cloud-provider-vsphere", "deckhouse-registry")

			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")

			csiCongrollerPluginSS := f.KubernetesResource("Deployment", "d8-cloud-provider-vsphere", "csi-controller")
			csiDriver := f.KubernetesGlobalResource("CSIDriver", "csi.vsphere.vmware.com")
			csiNodePluginDS := f.KubernetesResource("DaemonSet", "d8-cloud-provider-vsphere", "csi-node")
			csiSA := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-vsphere", "csi")
			csiProvisionerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-vsphere:csi:controller:external-provisioner")
			csiProvisionerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-vsphere:csi:controller:external-provisioner")
			csiAttacherCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-vsphere:csi:controller:external-attacher")
			csiAttacherCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-vsphere:csi:controller:external-attacher")
			csiResizerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-vsphere:csi:controller:external-resizer")
			csiResizerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-vsphere:csi:controller:external-resizer")

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
		})
	})

	Context("Vsphere", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.29", "1.29"))
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderVsphere", moduleValuesB)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())
			expectedProviderRegistrationJSON := `{
          "server": "myhost",
          "insecure": true,
          "password": "myPaSsWd",
          "region": "myreg",
          "regionTagCategory": "myregtagcat",
          "instanceClassDefaults": {
            "disableTimesync": true,
            "resourcePoolPath": "kubernetes-dev"
          },
          "sshKey": "mysshkey1",
          "username": "myuname",
          "vmFolderPath": "dev/test",
          "zoneTagCategory": "myzonetagcat"
        }`

			providerRegistrationData, err := base64.StdEncoding.DecodeString(providerRegistrationSecret.Field("data.vsphere").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(providerRegistrationData)).To(MatchJSON(expectedProviderRegistrationJSON))

			cloudConfig := f.KubernetesResource("Secret", "d8-cloud-provider-vsphere", "cloud-controller-manager")
			Expect(cloudConfig.Exists()).To(BeTrue())
			expectedCloudConfigYaml := `
global:
  user: "myuname"
  password: "myPaSsWd"
  insecureFlag: true

vcenter:
  main:
    server: "myhost"
    datacenters:
      - "X1"

nodes:
  externalVmNetworkName: aaa,bbb
  internalVmNetworkName: ccc,ddd

labels:
  region: "myregtagcat"
  zone: "myzonetagcat"`

			cloudConfigData, err := base64.StdEncoding.DecodeString(cloudConfig.Field("data.cloud-config").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(cloudConfigData)).To(MatchYAML(expectedCloudConfigYaml))
		})

		Context("Unsupported Kubernetes version", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.29", "1.29"))
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("cloudProviderVsphere", moduleValuesA)
				f.ValuesSet("global.discovery.kubernetesVersion", "1.17.8")
				f.HelmRender()
			})

			It("CCM and CSI controller should not be present on unsupported Kubernetes versions", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				Expect(f.KubernetesResource("Deployment", "d8-cloud-provider-vsphere", "cloud-controller-manager").Exists()).To(BeFalse())
				Expect(f.KubernetesResource("Deployment", "d8-cloud-provider-vsphere", "csi-controller").Exists()).To(BeFalse())

			})
		})
	})

	Context("Vsphere with default StorageClass specified", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.29", "1.29"))
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderVsphere", moduleValuesB)
			f.ValuesSetFromYaml("cloudProviderVsphere.internal.defaultStorageClass", `mydsname2`)
			f.HelmRender()
		})

		It("Everything must render properly with proper default StorageClass", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			scMydsname1 := f.KubernetesGlobalResource("StorageClass", "mydsname1")
			scMydsname2 := f.KubernetesGlobalResource("StorageClass", "mydsname2")

			Expect(scMydsname1.Exists()).To(BeTrue())
			Expect(scMydsname2.Exists()).To(BeTrue())

			Expect(scMydsname1.Field(`metadata.annotations.storageclass\.kubernetes\.io/is-default-class`).Exists()).To(BeFalse())
			Expect(scMydsname2.Field("metadata.annotations").String()).To(MatchYAML(`
storageclass.deckhouse.io/volume-expansion-mode: offline
storageclass.kubernetes.io/is-default-class: "true"
`))
		})
	})

	Context("Vsphere with NSX-T specified", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.29", "1.29"))
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderVsphere", moduleValuesC)
			f.HelmRender()
		})

		It("Everything must render properly with proper secret", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmSecret := f.KubernetesResource("Secret", "d8-cloud-provider-vsphere", "cloud-controller-manager")
			Expect(ccmSecret.Exists()).To(BeTrue())

			cloudConfig, _ := base64.StdEncoding.DecodeString(ccmSecret.Field("data.cloud-config").String())
			Expect(cloudConfig).To(MatchYAML(`
global:
  user: "myuname"
  password: "myPaSsWd"
  insecureFlag: true

vcenter:
  main:
    server: "myhost"
    datacenters:
      - "X1"

labels:
  region: "myregtagcat"
  zone: "myzonetagcat"

loadBalancer:
  ipPoolName: main
  size: SMALL
  snatDisabled: true
  tcpAppProfileName: default-tcp-lb-app-profile
  tier1GatewayPath: /host/tier1
  udpAppProfileName: default-udp-lb-app-profile
nsxt:
  host: 1.2.3.4
  password: password
  user: user
nodes:
  externalVmNetworkName: aaa,bbb
  internalVmNetworkName: ccc,ddd
`))
		})
	})

	Context("Vsphere with NSX-T with LoadBalancerClass specified", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.29", "1.29"))
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderVsphere", moduleValuesD)
			f.HelmRender()
		})

		It("Everything must render properly with proper secret", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmSecret := f.KubernetesResource("Secret", "d8-cloud-provider-vsphere", "cloud-controller-manager")
			Expect(ccmSecret.Exists()).To(BeTrue())

			cloudConfig, _ := base64.StdEncoding.DecodeString(ccmSecret.Field("data.cloud-config").String())
			Expect(cloudConfig).To(MatchYAML(`
global:
  insecureFlag: true
  password: myPaSsWd
  user: myuname
labels:
  region: myregtagcat
  zone: myzonetagcat

loadBalancer:
  ipPoolName: main
  size: SMALL
  snatDisabled: true
  tcpAppProfileName: default-tcp-lb-app-profile
  tier1GatewayPath: /host/tier1
  udpAppProfileName: default-udp-lb-app-profile

loadBalancerClass:
  class1:
    ipPoolName: pool2
    tcpAppProfileName: profile1

nsxt:
  host: 1.2.3.4
  password: password
  user: user

nodes:
  externalVmNetworkName: aaa,bbb
  internalVmNetworkName: ccc,ddd

vcenter:
  main:
    datacenters:
    - X1
    server: myhost
`))
		})
	})
})

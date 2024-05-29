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

package template_tests

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
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
modules:
  placement: {}
discovery:
  d8SpecificNodeCountByRole:
    master: 3
  clusterUUID: f49dd1c3-a63a-4565-a06c-625e35587eab
  kubernetesVersion: 1.29.8
clusterConfiguration:
  apiVersion: deckhouse.io/v1
  cloud:
    prefix: sandbox
    provider: vSphere
  clusterDomain: cluster.local
  clusterType: Cloud
  defaultCRI: Docker
  kind: ClusterConfiguration
  kubernetesVersion: "1.29"
  podSubnetCIDR: 10.111.0.0/16
  podSubnetNodeCIDRPrefix: "24"
  serviceSubnetCIDR: 10.222.0.0/16
  proxy:
    httpProxy: https://example.com
    httpsProxy: https://example.com
    noProxy:
    - example.com
`

// Defaults from openapi/config-values.yaml.
const nodeManagerConfigValues = `
allowedBundles:
  - "ubuntu-lts"
  - "centos"
  - "debian"
allowedKubernetesVersions:
  - "1.25"
  - "1.26"
  - "1.27"
  - "1.28"
  - "1.29"
mcmEmergencyBrake: false
`

const nodeManagerValues = `
internal:
  capiControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  capsControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
`

const nodeManagerAWS = `
internal:
  capiControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  capsControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string

  clusterAutoscalerPriorities:
    "50":
    - ^xxx-staging-[0-9a-zA-Z]+$
    "70":
    - ^xxx-staging-spot-m5a-2xlarge-[0-9a-zA-Z]+$
    "90":
    - ^xxx-staging-spot-[0-9a-zA-Z]+$
    - ^xxx-staging-spot-m5a.8xlarge-[0-9a-zA-Z]+$
    - ^xxx-staging-spot-c5.16xlarge-[0-9a-zA-Z]+$
  machineDeployments: {}
  instancePrefix: myprefix
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  kubernetesCA: myclusterca
  cloudProvider:
    type: aws
    machineClassKind: AWSInstanceClass
    aws:
      providerAccessKeyId: myprovaccesskeyid
      providerSecretAccessKey: myprovsecretaccesskey
      region: myregion
      loadBalancerSecurityGroupID: mylbsecuritygroupid
      keyName: mykeyname
      instances:
        iamProfileName: myiamprofilename
        additionalSecurityGroups: ["mysecgroupid1", "mysecgroupid2"]
        extraTags: ["extratag1", "extratag2"]
      internal:
        zoneToSubnetIdMap:
          zonea: mysubnetida
          zoneb: mysubnetidb
  nodeGroups:
  - name: worker
    instanceClass:
      ami: myami
      diskSizeGb: 50
      diskType: gp2
      iops: 42
      instanceType: t2.medium
    nodeType: CloudEphemeral
    kubernetesVersion: "1.29"
    cri:
      type: "Containerd"
    cloudInstances:
      classReference:
        kind: AWSInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 2
      zones:
      - zonea
      - zoneb
  machineControllerManagerEnabled: true
`

const nodeManagerAzure = `
internal:
  capiControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  capsControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  machineDeployments: {}
  instancePrefix: myprefix
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  kubernetesCA: myclusterca
  cloudProvider:
    type: azure
    machineClassKind: AzureMachineClass
    azure:
      sshPublicKey: sshPublicKey
      clientId: clientId
      clientSecret: clientSecret
      subscriptionId: subscriptionId
      tenantId: tenantId
      location: location
      resourceGroupName: resourceGroupName
      vnetName: vnetName
      subnetName: subnetName
      urn: urn
      diskType: diskType
      additionalTags: []
  nodeGroups:
  - name: worker
    instanceClass:
      flavorName: m1.large
      imageName: ubuntu-18-04-cloud-amd64
      machineType: mymachinetype
      preemptible: true #optional
      diskType: superdisk #optional
      diskSizeGb: 42 #optional
    nodeType: CloudEphemeral
    kubernetesVersion: "1.29"
    cri:
      type: "Docker"
    cloudInstances:
      classReference:
        kind: AzureInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 2
      zones:
      - zonea
      - zoneb
  - name: aaa
    instanceClass:
      acceleratedNetworking: false
      machineSize: test
      urn: test
    nodeType: CloudEphemeral
    cloudInstances:
      classReference:
        kind: AzureInstanceClass
        name: aaa
      maxPerZone: 1
      minPerZone: 1
      zones:
      - zonea
  - name: bbb
    instanceClass:
      acceleratedNetworking: true
      machineSize: bbb
      urn: zzz
    nodeType: CloudEphemeral
    cloudInstances:
      classReference:
        kind: AzureInstanceClass
        name: bbb
      maxPerZone: 1
      minPerZone: 1
      zones:
      - zonea
  machineControllerManagerEnabled: true
`

const nodeManagerGCP = `
internal:
  capiControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  capsControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  machineDeployments: {}
  instancePrefix: myprefix
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  kubernetesCA: myclusterca
  cloudProvider:
    type: gcp
    machineClassKind: GCPMachineClass
    gcp:
      region: region
      sshKey: privatekey
      networkName: mynetwork
      subnetworkName: mysubnetwork
      disableExternalIP: true
      image: image
      diskSizeGb: 20
      diskType: type
      networkTags: ["tag1", "tag2"]
      labels:
        test: test
      serviceAccountJSON: '{"client_email":"client_email"}'
  nodeGroups:
  - name: worker
    instanceClass: # maximum filled
      flavorName: m1.large
      imageName: ubuntu-18-04-cloud-amd64
      machineType: mymachinetype
      preemptible: true #optional
      diskType: superdisk #optional
      diskSizeGb: 42 #optional
    nodeType: CloudEphemeral
    kubernetesVersion: "1.29"
    cri:
      type: "Containerd"
    cloudInstances:
      classReference:
        kind: GCPInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 2
      zones:
      - zonea
      - zoneb
  machineControllerManagerEnabled: true
`

const faultyNodeManagerOpenstack = `
internal:
  capiControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  capsControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  machineDeployments: {}
  instancePrefix: myprefix
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  kubernetesCA: myclusterca
  cloudProvider:
    type: openstack
    machineClassKind: OpenStackMachineClass
    openstack:
      podNetworkMode: DirectRoutingWithPortSecurityEnabled
      connection:
        authURL: https://mycloud.qqq/3/
        caCert: mycacert
        domainName: Default
        password: pPaAsS
        region: myreg
        tenantName: mytname
        username: myuname
      instances:
        securityGroups: [groupa, groupb]
        sshKeyPairName: mysshkey
        mainNetwork: shared
      internalSubnet: "10.0.0.1/24"
      internalNetworkNames: [mynetwork, mynetwork2]
      externalNetworkNames: [shared]
  nodeGroups:
  - name: worker
    instanceClass:
      flavorName: m1.large
    nodeType: CloudEphemeral
    kubernetesVersion: "1.29"
    cri:
      type: "Docker"
    cloudInstances:
      classReference:
        kind: OpenStackInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 2
      zones:
      - zonea
      - zoneb
  machineControllerManagerEnabled: true
`

const nodeManagerOpenstack = `
internal:
  capiControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  capsControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  machineDeployments: {}
  instancePrefix: myprefix
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  kubernetesCA: myclusterca
  cloudProvider:
    type: openstack
    machineClassKind: OpenStackMachineClass
    openstack:
      podNetworkMode: DirectRoutingWithPortSecurityEnabled
      connection:
        authURL: https://mycloud.qqq/3/
        caCert: mycacert
        domainName: Default
        password: pPaAsS
        region: myreg
        tenantName: mytname
        username: myuname
      instances:
        securityGroups: [groupa, groupb]
        sshKeyPairName: mysshkey
        mainNetwork: shared
        additionalNetworks: [mynetwork]
        imageName: centos
      internalSubnet: "10.0.0.1/24"
      internalNetworkNames: [mynetwork, mynetwork2]
      externalNetworkNames: [shared]
      tags:
        yyy: zzz
        aaa: xxx
  nodeGroups:
  - name: worker
    instanceClass:
      flavorName: m1.large
      imageName: ubuntu-18-04-cloud-amd64
      mainNetwork: shared
      additionalNetworks:
      - mynetwork
      - mynetwork2
    nodeType: CloudEphemeral
    kubernetesVersion: "1.29"
    cri:
      type: "Containerd"
    cloudInstances:
      classReference:
        kind: OpenStackInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 2
      zones:
      - zonea
      - zoneb
  - name: simple
    instanceClass:
      flavorName: m1.xlarge
      additionalSecurityGroups:
      - ic-groupa
      - ic-groupb
      additionalTags:
        aaa: bbb
        ccc: ddd
    nodeType: CloudEphemeral
    kubernetesVersion: "1.29"
    cri:
      type: "Docker"
    cloudInstances:
      classReference:
        kind: OpenStackInstanceClass
        name: simple
      maxPerZone: 1
      minPerZone: 1
      zones:
      - zonea
  machineControllerManagerEnabled: true
`

const nodeManagerVsphere = `
internal:
  capiControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  capsControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  machineDeployments: {}
  instancePrefix: myprefix
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  kubernetesCA: myclusterca
  cloudProvider:
    type: vsphere
    machineClassKind: VsphereMachineClass
    vsphere:
      instanceClassDefaults: {}
      server: myhost.qqq
      username: myname
      password: pAsSwOrd
      insecure: true #
      regionTagCategory: myregtagcat #
      zoneTagCategory: myzonetagcateg #
      region: myreg
      sshKeys: [key1, key2] #
      vmFolderPath: dev/test
  nodeGroups:
  - name: worker
    instanceClass:
      flavorName: m1.large
      imageName: ubuntu-18-04-cloud-amd64
      numCPUs: 3
      memory: 3
      rootDiskSize: 42
      template: dev/test
      mainNetwork: mymainnetwork
      additionalNetworks: [aaa, bbb]
      datastore: lun-111
      runtimeOptions: # optional
        nestedHardwareVirtualization: true
        memoryReservation: 42
    nodeType: CloudEphemeral
    kubernetesVersion: "1.29"
    cri:
      type: "Containerd"
    cloudInstances:
      classReference:
        kind: VsphereInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 2
      zones:
      - zonea
      - zoneb
  - name: worker-with-disabled-nested-virt
    instanceClass:
      flavorName: m1.large
      imageName: ubuntu-18-04-cloud-amd64
      numCPUs: 3
      memory: 3
      rootDiskSize: 42
      template: dev/test
      mainNetwork: mymainnetwork
      additionalNetworks: [aaa, bbb]
      datastore: lun-111
      runtimeOptions: # optional
        nestedHardwareVirtualization: false
        memoryReservation: 42
    nodeType: CloudEphemeral
    kubernetesVersion: "1.29"
    cri:
      type: "Containerd"
    cloudInstances:
      classReference:
        kind: VsphereInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 2
      zones:
      - zonea
      - zoneb
  machineControllerManagerEnabled: true
`

const nodeManagerYandex = `
internal:
  capiControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  capsControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  machineDeployments: {}
  instancePrefix: myprefix
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  kubernetesCA: myclusterca
  cloudProvider:
    type: yandex
    machineClassKind: YandexMachineClass
    yandex:
      instanceClassDefaults: {}
      serviceAccountJSON: '{"my":"svcacc"}'
      region: myreg
      folderID: myfolder
      sshKey: mysshkey
      sshUser: mysshuser
      nameservers: ["4.2.2.2"]
      dns:
        search: ["qwe"]
        nameservers: ["1.2.3.4","3.4.5.6"]
      zoneToSubnetIdMap:
        zonea: subneta
        zoneb: subnetb
  nodeGroups:
  - name: worker
    instanceClass:
      flavorName: m1.large
      imageName: ubuntu-18-04-cloud-amd64
      platformID: myplaid
      cores: 42
      coreFraction: 50 #optional
      memory: 42
      gpus: 2
      imageID: myimageid
      preemptible: true #optional
      diskType: ssd #optional
      diskSizeGB: 42 #optional
      assignPublicIPAddress: true #optional
      mainSubnet: mymainsubnet
      additionalSubnets: [aaa, bbb]
      additionalLabels: # optional
        my: label
    nodeType: CloudEphemeral
    kubernetesVersion: "1.29"
    cri:
      type: "Docker"
    cloudInstances:
      classReference:
        kind: YandexInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 2
      zones:
      - zonea
      - zoneb
  machineControllerManagerEnabled: true
`

const nodeManagerStatic = `
internal:
  capiControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  capsControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  machineDeployments: {}
  instancePrefix: myprefix
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  kubernetesCA: myclusterca
  bootstrapTokens:
    worker: myworker
  nodeGroups:
  - name: worker
    nodeType: Static
    kubernetesVersion: "1.29"
    cri:
      type: "Containerd"
`

const (
	nodeManagerStaticInstances = `
internal:
  capiControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  capsControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  machineDeployments: {}
  instancePrefix: myprefix
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  kubernetesCA: myclusterca
  bootstrapTokens:
    worker: myworker
  nodeGroups:
  - name: worker
    nodeType: Static
    staticInstances:
      labelSelector:
        matchLabels:
          node-group: worker
    kubernetesVersion: "1.23"
    cri:
      type: "Containerd"
`
	nodeManagerStaticInstancesStaticMachineTemplate = `
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: StaticMachineTemplate
metadata:
  namespace: d8-cloud-instance-manager
  name: worker
  labels:
    heritage: deckhouse
    module: node-manager
    node-group: worker
spec:
  template:
    metadata:
      labels:
        heritage: deckhouse
        module: node-manager
        node-group: worker
    spec:
      labelSelector:
        matchLabels:
          node-group: worker
`
	nodeManagerStaticInstancesMachineDeployment = `
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  namespace: d8-cloud-instance-manager
  name: worker
  labels:
    heritage: deckhouse
    module: node-manager
    node-group: worker
spec:
  clusterName: static
  replicas: 0
  template:
    spec:
      bootstrap:
        dataSecretName: manual-bootstrap-for-worker
      clusterName: static
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
        kind: StaticMachineTemplate
        name: worker
  selector: {}
`
)

const openstackCIMPath = "/deckhouse/ee/modules/030-cloud-provider-openstack/cloud-instance-manager"
const openstackCIMSymlink = "/deckhouse/modules/040-node-manager/cloud-providers/openstack"
const vsphereCIMPath = "/deckhouse/ee/modules/030-cloud-provider-vsphere/cloud-instance-manager"
const vsphereCIMSymlink = "/deckhouse/modules/040-node-manager/cloud-providers/vsphere"
const vcdCAPIPath = "/deckhouse/ee/modules/030-cloud-provider-vcd/capi"
const vcdCAPISymlink = "/deckhouse/modules/040-node-manager/capi/vcd"

var _ = Describe("Module :: node-manager :: helm template ::", func() {
	f := SetupHelmConfig(``)

	BeforeSuite(func() {
		err := os.Symlink(openstackCIMPath, openstackCIMSymlink)
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Symlink(vsphereCIMPath, vsphereCIMSymlink)
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Symlink(vcdCAPIPath, vcdCAPISymlink)
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterSuite(func() {
		err := os.Remove(openstackCIMSymlink)
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Remove(vsphereCIMSymlink)
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Remove(vcdCAPISymlink)
		Expect(err).ShouldNot(HaveOccurred())
	})

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
	})

	Context("Prometheus rules", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerValues)
			setBashibleAPIServerTLSValues(f)
			f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler-crd", "operator-prometheus-crd"]`)
		})

		assertSpecDotGroupsArray := func(rule object_store.KubeObject, shouldEmpty bool) {
			Expect(rule.Exists()).To(BeTrue())

			groups := rule.Field("spec.groups")

			Expect(groups.IsArray()).To(BeTrue())
			if shouldEmpty {
				Expect(groups.Array()).To(BeEmpty())
			} else {
				Expect(groups.Array()).ToNot(BeEmpty())
			}
		}

		Context("For cluster auto-scaler", func() {
			Context("cluster auto-scaler disabled", func() {
				BeforeEach(func() {
					f.HelmRender()
				})

				It("spec.groups should be empty array", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					rule := f.KubernetesResource("PrometheusRule", "d8-cloud-instance-manager", "node-manager-cluster-autoscaler")

					assertSpecDotGroupsArray(rule, true)
				})
			})

			Context("cluster auto-scaler enabled", func() {
				BeforeEach(func() {
					// autoscaler enabled if have none empty cloud node group
					f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerAWS)
					setBashibleAPIServerTLSValues(f)
					f.HelmRender()
				})

				It("spec.groups should be none empty array", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					rule := f.KubernetesResource("PrometheusRule", "d8-cloud-instance-manager", "node-manager-cluster-autoscaler")
					cm := f.KubernetesResource("ConfigMap", "d8-cloud-instance-manager", "cluster-autoscaler-priority-expander")
					Expect(cm.Field("data.priorities").String()).To(MatchYAML(`
50:
  - ^xxx-staging-[0-9a-zA-Z]+$
70:
  - ^xxx-staging-spot-m5a-2xlarge-[0-9a-zA-Z]+$
90:
  - ^xxx-staging-spot-[0-9a-zA-Z]+$
  - ^xxx-staging-spot-m5a.8xlarge-[0-9a-zA-Z]+$
  - ^xxx-staging-spot-c5.16xlarge-[0-9a-zA-Z]+$
`))

					assertSpecDotGroupsArray(rule, false)
				})
			})
		})

		Context("For machine controller manager", func() {
			Context("machine controller manager disabled", func() {
				BeforeEach(func() {
					f.HelmRender()
				})

				It("spec.groups should be empty array", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					rule := f.KubernetesResource("PrometheusRule", "d8-cloud-instance-manager", "node-manager-machine-controller-manager")

					assertSpecDotGroupsArray(rule, true)
				})
			})

			Context("machine controller manager enabled", func() {
				BeforeEach(func() {
					f.ValuesSetFromYaml("nodeManager.mcmEmergencyBrake", "false")
					f.ValuesSetFromYaml("nodeManager.internal.machineControllerManagerEnabled", "true")

					f.HelmRender()
				})

				It("spec.groups should be none empty array", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					rule := f.KubernetesResource("PrometheusRule", "d8-cloud-instance-manager", "node-manager-machine-controller-manager")

					assertSpecDotGroupsArray(rule, false)
				})
			})
		})
	})

	Context("AWS", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerAWS)
			setBashibleAPIServerTLSValues(f)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			registrySecret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "deckhouse-registry")

			userAuthzClusterRoleUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:user")
			userAuthzClusterRoleClusterEditor := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:cluster-editor")
			userAuthzClusterRoleClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:cluster-admin")

			mcmDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:node-manager:machine-controller-manager")
			mcmClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:machine-controller-manager")

			clusterAutoscalerDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:node-manager:cluster-autoscaler")
			clusterAutoscalerClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:cluster-autoscaler")

			machineClassA := f.KubernetesResource("AWSMachineClass", "d8-cloud-instance-manager", "worker-02320933")
			machineClassSecretA := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-02320933")
			machineDeploymentA := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
			machineClassB := f.KubernetesResource("AWSMachineClass", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineClassSecretB := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineDeploymentB := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-6bdb5b0d")

			Expect(verifyClusterAutoscalerDeploymentArgs(clusterAutoscalerDeploy, machineDeploymentA, machineDeploymentB)).To(Succeed())

			bashibleSecrets := map[string]object_store.KubeObject{}
			bashibleSecrets["bashible-bashbooster"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bashbooster")

			roles := map[string]object_store.KubeObject{}
			roles["bashible"] = f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible")
			roles["bashible-mcm-bootstrapped-nodes"] = f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible-mcm-bootstrapped-nodes")

			roleBindings := map[string]object_store.KubeObject{}
			roleBindings["bashible"] = f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible")
			roleBindings["bashible-mcm-bootstrapped-nodes"] = f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible-mcm-bootstrapped-nodes")

			Expect(registrySecret.Exists()).To(BeTrue())

			Expect(userAuthzClusterRoleUser.Exists()).To(BeTrue())
			Expect(userAuthzClusterRoleClusterEditor.Exists()).To(BeTrue())
			Expect(userAuthzClusterRoleClusterAdmin.Exists()).To(BeTrue())

			Expect(mcmDeploy.Exists()).To(BeTrue())
			Expect(mcmServiceAccount.Exists()).To(BeTrue())
			Expect(mcmRole.Exists()).To(BeTrue())
			Expect(mcmRoleBinding.Exists()).To(BeTrue())
			Expect(mcmClusterRole.Exists()).To(BeTrue())
			Expect(mcmClusterRoleBinding.Exists()).To(BeTrue())

			Expect(clusterAutoscalerDeploy.Exists()).To(BeTrue())
			Expect(clusterAutoscalerServiceAccount.Exists()).To(BeTrue())
			Expect(clusterAutoscalerRole.Exists()).To(BeTrue())
			Expect(clusterAutoscalerRoleBinding.Exists()).To(BeTrue())
			Expect(clusterAutoscalerClusterRole.Exists()).To(BeTrue())
			Expect(clusterAutoscalerClusterRoleBinding.Exists()).To(BeTrue())

			Expect(machineClassA.Exists()).To(BeTrue())
			Expect(machineClassSecretA.Exists()).To(BeTrue())
			Expect(machineDeploymentA.Exists()).To(BeTrue())

			Expect(machineClassB.Exists()).To(BeTrue())
			Expect(machineClassSecretB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Exists()).To(BeTrue())

			Expect(bashibleSecrets["bashible-bashbooster"].Exists()).To(BeTrue())

			Expect(roles["bashible"].Exists()).To(BeTrue())
			Expect(roles["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			Expect(roleBindings["bashible"].Exists()).To(BeTrue())
			Expect(roleBindings["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			assertBashibleAPIServerTLS(f)
		})
	})

	Context("GCP", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerGCP)
			setBashibleAPIServerTLSValues(f)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			registrySecret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "deckhouse-registry")

			userAuthzClusterRoleUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:user")
			userAuthzClusterRoleClusterEditor := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:cluster-editor")
			userAuthzClusterRoleClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:cluster-admin")

			mcmDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:node-manager:machine-controller-manager")
			mcmClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:machine-controller-manager")

			clusterAutoscalerDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:node-manager:cluster-autoscaler")
			clusterAutoscalerClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:cluster-autoscaler")

			machineClassA := f.KubernetesResource("GCPMachineClass", "d8-cloud-instance-manager", "worker-02320933")
			machineClassSecretA := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-02320933")
			machineDeploymentA := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
			machineClassB := f.KubernetesResource("GCPMachineClass", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineClassSecretB := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineDeploymentB := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-6bdb5b0d")

			Expect(verifyClusterAutoscalerDeploymentArgs(clusterAutoscalerDeploy, machineDeploymentA, machineDeploymentB)).To(Succeed())

			bashibleSecrets := map[string]object_store.KubeObject{}
			bashibleSecrets["bashible-bashbooster"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bashbooster")

			roles := map[string]object_store.KubeObject{}
			roles["bashible"] = f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible")
			roles["bashible-mcm-bootstrapped-nodes"] = f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible-mcm-bootstrapped-nodes")

			roleBindings := map[string]object_store.KubeObject{}
			roleBindings["bashible"] = f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible")
			roleBindings["bashible-mcm-bootstrapped-nodes"] = f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible-mcm-bootstrapped-nodes")

			Expect(registrySecret.Exists()).To(BeTrue())

			Expect(userAuthzClusterRoleUser.Exists()).To(BeTrue())
			Expect(userAuthzClusterRoleClusterEditor.Exists()).To(BeTrue())
			Expect(userAuthzClusterRoleClusterAdmin.Exists()).To(BeTrue())

			Expect(mcmDeploy.Exists()).To(BeTrue())
			Expect(mcmServiceAccount.Exists()).To(BeTrue())
			Expect(mcmRole.Exists()).To(BeTrue())
			Expect(mcmRoleBinding.Exists()).To(BeTrue())
			Expect(mcmClusterRole.Exists()).To(BeTrue())
			Expect(mcmClusterRoleBinding.Exists()).To(BeTrue())

			Expect(clusterAutoscalerDeploy.Exists()).To(BeTrue())
			Expect(clusterAutoscalerServiceAccount.Exists()).To(BeTrue())
			Expect(clusterAutoscalerRole.Exists()).To(BeTrue())
			Expect(clusterAutoscalerRoleBinding.Exists()).To(BeTrue())
			Expect(clusterAutoscalerClusterRole.Exists()).To(BeTrue())
			Expect(clusterAutoscalerClusterRoleBinding.Exists()).To(BeTrue())

			Expect(machineClassA.Exists()).To(BeTrue())
			Expect(machineClassSecretA.Exists()).To(BeTrue())
			Expect(machineDeploymentA.Exists()).To(BeTrue())

			Expect(machineClassB.Exists()).To(BeTrue())
			Expect(machineClassSecretB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Exists()).To(BeTrue())

			Expect(bashibleSecrets["bashible-bashbooster"].Exists()).To(BeTrue())

			Expect(roles["bashible"].Exists()).To(BeTrue())
			Expect(roles["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			Expect(roleBindings["bashible"].Exists()).To(BeTrue())
			Expect(roleBindings["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			assertBashibleAPIServerTLS(f)
		})
	})

	Context("Openstack", func() {
		Describe("Openstack faulty config", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+faultyNodeManagerOpenstack)
				setBashibleAPIServerTLSValues(f)
				f.HelmRender()
			})

			It("Test should fail", func() {
				Expect(f.RenderError).Should(HaveOccurred())
				Expect(f.RenderError.Error()).ShouldNot(BeEmpty())
			})
		})

		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerOpenstack)
			setBashibleAPIServerTLSValues(f)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			registrySecret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "deckhouse-registry")

			userAuthzClusterRoleUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:user")
			userAuthzClusterRoleClusterEditor := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:cluster-editor")
			userAuthzClusterRoleClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:cluster-admin")

			mcmDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:node-manager:machine-controller-manager")
			mcmClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:machine-controller-manager")

			clusterAutoscalerDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:node-manager:cluster-autoscaler")
			clusterAutoscalerClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:cluster-autoscaler")

			machineClassA := f.KubernetesResource("OpenstackMachineClass", "d8-cloud-instance-manager", "worker-02320933")
			machineClassSecretA := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-02320933")
			machineDeploymentA := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
			machineClassB := f.KubernetesResource("OpenstackMachineClass", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineClassSecretB := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineDeploymentB := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-6bdb5b0d")
			simpleMachineClassA := f.KubernetesResource("OpenstackMachineClass", "d8-cloud-instance-manager", "simple-02320933")
			simpleMachineClassSecretA := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "simple-02320933")
			simpleMachineDeploymentA := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-simple-02320933")

			Expect(verifyClusterAutoscalerDeploymentArgs(clusterAutoscalerDeploy, machineDeploymentA, machineDeploymentB, simpleMachineDeploymentA)).To(Succeed())

			bashibleSecrets := map[string]object_store.KubeObject{}
			bashibleSecrets["bashible-bashbooster"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bashbooster")

			roles := map[string]object_store.KubeObject{}
			roles["bashible"] = f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible")
			roles["bashible-mcm-bootstrapped-nodes"] = f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible-mcm-bootstrapped-nodes")

			roleBindings := map[string]object_store.KubeObject{}
			roleBindings["bashible"] = f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible")
			roleBindings["bashible-mcm-bootstrapped-nodes"] = f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible-mcm-bootstrapped-nodes")

			Expect(registrySecret.Exists()).To(BeTrue())

			Expect(userAuthzClusterRoleUser.Exists()).To(BeTrue())
			Expect(userAuthzClusterRoleClusterEditor.Exists()).To(BeTrue())
			Expect(userAuthzClusterRoleClusterAdmin.Exists()).To(BeTrue())

			Expect(mcmDeploy.Exists()).To(BeTrue())
			Expect(mcmServiceAccount.Exists()).To(BeTrue())
			Expect(mcmRole.Exists()).To(BeTrue())
			Expect(mcmRoleBinding.Exists()).To(BeTrue())
			Expect(mcmClusterRole.Exists()).To(BeTrue())
			Expect(mcmClusterRoleBinding.Exists()).To(BeTrue())

			Expect(clusterAutoscalerDeploy.Exists()).To(BeTrue())
			Expect(clusterAutoscalerServiceAccount.Exists()).To(BeTrue())
			Expect(clusterAutoscalerRole.Exists()).To(BeTrue())
			Expect(clusterAutoscalerRoleBinding.Exists()).To(BeTrue())
			Expect(clusterAutoscalerClusterRole.Exists()).To(BeTrue())
			Expect(clusterAutoscalerClusterRoleBinding.Exists()).To(BeTrue())

			Expect(machineClassA.Exists()).To(BeTrue())
			Expect(machineClassA.Field("spec.networks").String()).To(MatchYAML(`
[{name: shared}, {name: mynetwork, podNetwork: true}, {name: mynetwork2, podNetwork: true}]
`))
			Expect(machineClassA.Field("spec.securityGroups").String()).To(MatchYAML(`
[groupa, groupb]
`))
			Expect(machineClassA.Field("spec.tags").String()).To(MatchYAML(`
kubernetes.io-cluster-deckhouse-f49dd1c3-a63a-4565-a06c-625e35587eab: "1"
kubernetes.io-role-deckhouse-worker-zonea: "1"
yyy: zzz
aaa: xxx
`))
			Expect(machineClassA.Field("spec.flavorName").String()).To(MatchYAML(`m1.large`))
			Expect(machineClassA.Field("spec.imageName").String()).To(MatchYAML(`ubuntu-18-04-cloud-amd64`))

			Expect(machineClassSecretA.Exists()).To(BeTrue())
			Expect(machineDeploymentA.Exists()).To(BeTrue())

			Expect(machineClassB.Exists()).To(BeTrue())
			Expect(machineClassSecretB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Exists()).To(BeTrue())

			Expect(simpleMachineClassA.Exists()).To(BeTrue())
			Expect(simpleMachineClassA.Field("spec.networks").String()).To(MatchYAML(`
[{name: shared}, {name: mynetwork, podNetwork: true}]
`))
			Expect(simpleMachineClassA.Field("spec.securityGroups").String()).To(MatchYAML(`
[groupa, groupb, ic-groupa, ic-groupb]
`))
			Expect(simpleMachineClassA.Field("spec.tags").String()).To(MatchYAML(`
kubernetes.io-cluster-deckhouse-f49dd1c3-a63a-4565-a06c-625e35587eab: "1"
kubernetes.io-role-deckhouse-simple-zonea: "1"
yyy: zzz
aaa: bbb
ccc: ddd
`))
			Expect(simpleMachineClassA.Field("spec.flavorName").String()).To(MatchYAML(`m1.xlarge`))
			Expect(simpleMachineClassA.Field("spec.imageName").String()).To(MatchYAML(`centos`))
			Expect(simpleMachineClassSecretA.Exists()).To(BeTrue())
			Expect(simpleMachineDeploymentA.Exists()).To(BeTrue())

			Expect(bashibleSecrets["bashible-bashbooster"].Exists()).To(BeTrue())

			Expect(roles["bashible"].Exists()).To(BeTrue())
			Expect(roles["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			Expect(roleBindings["bashible"].Exists()).To(BeTrue())
			Expect(roleBindings["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			assertBashibleAPIServerTLS(f)
		})
	})

	Context("Vsphere", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerVsphere)
			setBashibleAPIServerTLSValues(f)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			registrySecret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "deckhouse-registry")

			userAuthzClusterRoleUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:user")
			userAuthzClusterRoleClusterEditor := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:cluster-editor")
			userAuthzClusterRoleClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:cluster-admin")

			mcmDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:node-manager:machine-controller-manager")
			mcmClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:machine-controller-manager")

			clusterAutoscalerDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:node-manager:cluster-autoscaler")
			clusterAutoscalerClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:cluster-autoscaler")

			machineClassA := f.KubernetesResource("VsphereMachineClass", "d8-cloud-instance-manager", "worker-02320933")
			machineClassSecretA := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-02320933")
			machineDeploymentA := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
			machineClassB := f.KubernetesResource("VsphereMachineClass", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineClassSecretB := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineDeploymentB := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-6bdb5b0d")

			machineClassAWitoutNestedVirt := f.KubernetesResource("VsphereMachineClass", "d8-cloud-instance-manager", "worker-with-disabled-nested-virt-02320933")
			machineClassSecretAWitoutNestedVirt := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-with-disabled-nested-virt-02320933")
			machineDeploymentAWitoutNestedVirt := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-with-disabled-nested-virt-02320933")
			machineClassBWitoutNestedVirt := f.KubernetesResource("VsphereMachineClass", "d8-cloud-instance-manager", "worker-with-disabled-nested-virt-6bdb5b0d")
			machineClassSecretBWitoutNestedVirt := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-with-disabled-nested-virt-6bdb5b0d")
			machineDeploymentBWitoutNestedVirt := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-with-disabled-nested-virt-6bdb5b0d")

			Expect(verifyClusterAutoscalerDeploymentArgs(clusterAutoscalerDeploy, machineDeploymentA, machineDeploymentB, machineDeploymentAWitoutNestedVirt, machineDeploymentBWitoutNestedVirt)).To(Succeed())

			bashibleSecrets := map[string]object_store.KubeObject{}
			bashibleSecrets["bashible-bashbooster"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bashbooster")

			roles := map[string]object_store.KubeObject{}
			roles["bashible"] = f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible")
			roles["bashible-mcm-bootstrapped-nodes"] = f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible-mcm-bootstrapped-nodes")

			roleBindings := map[string]object_store.KubeObject{}
			roleBindings["bashible"] = f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible")
			roleBindings["bashible-mcm-bootstrapped-nodes"] = f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible-mcm-bootstrapped-nodes")

			Expect(registrySecret.Exists()).To(BeTrue())

			Expect(userAuthzClusterRoleUser.Exists()).To(BeTrue())
			Expect(userAuthzClusterRoleClusterEditor.Exists()).To(BeTrue())
			Expect(userAuthzClusterRoleClusterAdmin.Exists()).To(BeTrue())

			Expect(mcmDeploy.Exists()).To(BeTrue())
			Expect(mcmServiceAccount.Exists()).To(BeTrue())
			Expect(mcmRole.Exists()).To(BeTrue())
			Expect(mcmRoleBinding.Exists()).To(BeTrue())
			Expect(mcmClusterRole.Exists()).To(BeTrue())
			Expect(mcmClusterRoleBinding.Exists()).To(BeTrue())

			Expect(clusterAutoscalerDeploy.Exists()).To(BeTrue())
			Expect(clusterAutoscalerServiceAccount.Exists()).To(BeTrue())
			Expect(clusterAutoscalerRole.Exists()).To(BeTrue())
			Expect(clusterAutoscalerRoleBinding.Exists()).To(BeTrue())
			Expect(clusterAutoscalerClusterRole.Exists()).To(BeTrue())
			Expect(clusterAutoscalerClusterRoleBinding.Exists()).To(BeTrue())

			Expect(machineClassA.Exists()).To(BeTrue())
			Expect(machineClassSecretA.Exists()).To(BeTrue())
			Expect(machineDeploymentA.Exists()).To(BeTrue())

			Expect(machineClassB.Exists()).To(BeTrue())
			Expect(machineClassSecretB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Exists()).To(BeTrue())

			Expect(machineClassAWitoutNestedVirt.Exists()).To(BeTrue())
			Expect(machineClassSecretAWitoutNestedVirt.Exists()).To(BeTrue())
			Expect(machineDeploymentAWitoutNestedVirt.Exists()).To(BeTrue())

			Expect(machineClassBWitoutNestedVirt.Exists()).To(BeTrue())
			Expect(machineClassSecretBWitoutNestedVirt.Exists()).To(BeTrue())
			Expect(machineDeploymentBWitoutNestedVirt.Exists()).To(BeTrue())

			nestedVirtA := machineClassAWitoutNestedVirt.Field("spec.runtimeOptions.nestedHardwareVirtualization")
			Expect(nestedVirtA.Exists()).To(BeTrue())
			Expect(nestedVirtA.Bool()).To(BeFalse())

			nestedVirtB := machineClassBWitoutNestedVirt.Field("spec.runtimeOptions.nestedHardwareVirtualization")
			Expect(nestedVirtB.Exists()).To(BeTrue())
			Expect(nestedVirtB.Bool()).To(BeFalse())

			Expect(bashibleSecrets["bashible-bashbooster"].Exists()).To(BeTrue())

			Expect(roles["bashible"].Exists()).To(BeTrue())
			Expect(roles["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			Expect(roleBindings["bashible"].Exists()).To(BeTrue())
			Expect(roleBindings["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			assertBashibleAPIServerTLS(f)
		})
	})

	Context("Yandex", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerYandex)
			setBashibleAPIServerTLSValues(f)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			registrySecret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "deckhouse-registry")

			userAuthzClusterRoleUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:user")
			userAuthzClusterRoleClusterEditor := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:cluster-editor")
			userAuthzClusterRoleClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:cluster-admin")

			mcmDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:node-manager:machine-controller-manager")
			mcmClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:machine-controller-manager")

			clusterAutoscalerDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:node-manager:cluster-autoscaler")
			clusterAutoscalerClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:cluster-autoscaler")

			machineClassA := f.KubernetesResource("YandexMachineClass", "d8-cloud-instance-manager", "worker-02320933")
			machineClassSecretA := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-02320933")
			machineDeploymentA := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
			machineClassB := f.KubernetesResource("YandexMachineClass", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineClassSecretB := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineDeploymentB := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-6bdb5b0d")

			Expect(verifyClusterAutoscalerDeploymentArgs(clusterAutoscalerDeploy, machineDeploymentA, machineDeploymentB)).To(Succeed())

			bashibleSecrets := map[string]object_store.KubeObject{}
			bashibleSecrets["bashible-bashbooster"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bashbooster")

			roles := map[string]object_store.KubeObject{}
			roles["bashible"] = f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible")
			roles["bashible-mcm-bootstrapped-nodes"] = f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible-mcm-bootstrapped-nodes")

			roleBindings := map[string]object_store.KubeObject{}
			roleBindings["bashible"] = f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible")
			roleBindings["bashible-mcm-bootstrapped-nodes"] = f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible-mcm-bootstrapped-nodes")

			Expect(registrySecret.Exists()).To(BeTrue())

			Expect(userAuthzClusterRoleUser.Exists()).To(BeTrue())
			Expect(userAuthzClusterRoleClusterEditor.Exists()).To(BeTrue())
			Expect(userAuthzClusterRoleClusterAdmin.Exists()).To(BeTrue())

			Expect(mcmDeploy.Exists()).To(BeTrue())
			Expect(mcmServiceAccount.Exists()).To(BeTrue())
			Expect(mcmRole.Exists()).To(BeTrue())
			Expect(mcmRoleBinding.Exists()).To(BeTrue())
			Expect(mcmClusterRole.Exists()).To(BeTrue())
			Expect(mcmClusterRoleBinding.Exists()).To(BeTrue())

			Expect(clusterAutoscalerDeploy.Exists()).To(BeTrue())
			Expect(clusterAutoscalerServiceAccount.Exists()).To(BeTrue())
			Expect(clusterAutoscalerRole.Exists()).To(BeTrue())
			Expect(clusterAutoscalerRoleBinding.Exists()).To(BeTrue())
			Expect(clusterAutoscalerClusterRole.Exists()).To(BeTrue())
			Expect(clusterAutoscalerClusterRoleBinding.Exists()).To(BeTrue())

			Expect(machineClassA.Exists()).To(BeTrue())
			Expect(machineClassSecretA.Exists()).To(BeTrue())
			Expect(machineDeploymentA.Exists()).To(BeTrue())

			Expect(machineClassB.Exists()).To(BeTrue())
			Expect(machineClassSecretB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Exists()).To(BeTrue())

			Expect(bashibleSecrets["bashible-bashbooster"].Exists()).To(BeTrue())

			Expect(roles["bashible"].Exists()).To(BeTrue())
			Expect(roles["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			Expect(roleBindings["bashible"].Exists()).To(BeTrue())
			Expect(roleBindings["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			assertBashibleAPIServerTLS(f)
		})
	})

	Context("Static", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerStatic)
			setBashibleAPIServerTLSValues(f)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			registrySecret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "deckhouse-registry")

			userAuthzClusterRoleUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:user")
			userAuthzClusterRoleClusterEditor := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:cluster-editor")
			userAuthzClusterRoleClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:cluster-admin")

			mcmDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:node-manager:machine-controller-manager")
			mcmClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:machine-controller-manager")

			clusterAutoscalerDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:node-manager:cluster-autoscaler")
			clusterAutoscalerClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:cluster-autoscaler")

			machineClassA := f.KubernetesResource("OpenstackMachineClass", "d8-cloud-instance-manager", "worker-02320933")
			machineClassSecretA := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-02320933")
			machineDeploymentA := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
			machineClassB := f.KubernetesResource("OpenstackMachineClass", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineClassSecretB := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineDeploymentB := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-6bdb5b0d")

			bashibleSecrets := map[string]object_store.KubeObject{}
			bashibleSecrets["bashible-bashbooster"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bashbooster")

			bootstrapSecrets := map[string]object_store.KubeObject{}
			bootstrapSecrets["manual-bootstrap-for-worker"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "manual-bootstrap-for-worker")

			roles := map[string]object_store.KubeObject{}
			roles["bashible"] = f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible")
			roles["bashible-mcm-bootstrapped-nodes"] = f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible-mcm-bootstrapped-nodes")

			roleBindings := map[string]object_store.KubeObject{}
			roleBindings["bashible"] = f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible")
			roleBindings["bashible-mcm-bootstrapped-nodes"] = f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible-mcm-bootstrapped-nodes")

			Expect(registrySecret.Exists()).To(BeTrue())

			Expect(userAuthzClusterRoleUser.Exists()).To(BeTrue())
			Expect(userAuthzClusterRoleClusterEditor.Exists()).To(BeTrue())
			Expect(userAuthzClusterRoleClusterAdmin.Exists()).To(BeTrue())

			Expect(mcmDeploy.Exists()).To(BeFalse())
			Expect(mcmServiceAccount.Exists()).To(BeFalse())
			Expect(mcmRole.Exists()).To(BeFalse())
			Expect(mcmRoleBinding.Exists()).To(BeFalse())
			Expect(mcmClusterRole.Exists()).To(BeFalse())
			Expect(mcmClusterRoleBinding.Exists()).To(BeFalse())

			Expect(clusterAutoscalerDeploy.Exists()).To(BeFalse())
			Expect(clusterAutoscalerServiceAccount.Exists()).To(BeFalse())
			Expect(clusterAutoscalerRole.Exists()).To(BeFalse())
			Expect(clusterAutoscalerRoleBinding.Exists()).To(BeFalse())
			Expect(clusterAutoscalerClusterRole.Exists()).To(BeFalse())
			Expect(clusterAutoscalerClusterRoleBinding.Exists()).To(BeFalse())

			Expect(machineClassA.Exists()).To(BeFalse())
			Expect(machineClassSecretA.Exists()).To(BeFalse())
			Expect(machineDeploymentA.Exists()).To(BeFalse())

			Expect(machineClassB.Exists()).To(BeFalse())
			Expect(machineClassSecretB.Exists()).To(BeFalse())
			Expect(machineDeploymentB.Exists()).To(BeFalse())

			Expect(bashibleSecrets["bashible-bashbooster"].Exists()).To(BeTrue())

			Expect(bootstrapSecrets["manual-bootstrap-for-worker"].Exists()).To(BeTrue())

			Expect(roles["bashible"].Exists()).To(BeTrue())
			Expect(roles["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			Expect(roleBindings["bashible"].Exists()).To(BeTrue())
			Expect(roleBindings["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			assertBashibleAPIServerTLS(f)
		})
	})

	Context("Static instances", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerStaticInstances)
			setBashibleAPIServerTLSValues(f)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			registrySecret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "deckhouse-registry")

			userAuthzClusterRoleUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:user")
			userAuthzClusterRoleClusterEditor := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:cluster-editor")
			userAuthzClusterRoleClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:cluster-admin")

			mcmDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:node-manager:machine-controller-manager")
			mcmClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:machine-controller-manager")

			clusterAutoscalerDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:node-manager:cluster-autoscaler")
			clusterAutoscalerClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:cluster-autoscaler")

			machineClassA := f.KubernetesResource("OpenstackMachineClass", "d8-cloud-instance-manager", "worker-02320933")
			machineClassSecretA := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-02320933")
			machineDeploymentA := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
			machineClassB := f.KubernetesResource("OpenstackMachineClass", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineClassSecretB := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineDeploymentB := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-6bdb5b0d")

			bashibleSecrets := map[string]object_store.KubeObject{}
			bashibleSecrets["bashible-bashbooster"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bashbooster")

			bootstrapSecrets := map[string]object_store.KubeObject{}
			bootstrapSecrets["manual-bootstrap-for-worker"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "manual-bootstrap-for-worker")

			roles := map[string]object_store.KubeObject{}
			roles["bashible"] = f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible")
			roles["bashible-mcm-bootstrapped-nodes"] = f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible-mcm-bootstrapped-nodes")

			roleBindings := map[string]object_store.KubeObject{}
			roleBindings["bashible"] = f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible")
			roleBindings["bashible-mcm-bootstrapped-nodes"] = f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible-mcm-bootstrapped-nodes")

			staticMachineTemplate := f.KubernetesResource("StaticMachineTemplate", "d8-cloud-instance-manager", "worker")
			staticMachineDeployment := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "worker")

			Expect(registrySecret.Exists()).To(BeTrue())

			Expect(userAuthzClusterRoleUser.Exists()).To(BeTrue())
			Expect(userAuthzClusterRoleClusterEditor.Exists()).To(BeTrue())
			Expect(userAuthzClusterRoleClusterAdmin.Exists()).To(BeTrue())

			Expect(mcmDeploy.Exists()).To(BeFalse())
			Expect(mcmServiceAccount.Exists()).To(BeFalse())
			Expect(mcmRole.Exists()).To(BeFalse())
			Expect(mcmRoleBinding.Exists()).To(BeFalse())
			Expect(mcmClusterRole.Exists()).To(BeFalse())
			Expect(mcmClusterRoleBinding.Exists()).To(BeFalse())

			Expect(clusterAutoscalerDeploy.Exists()).To(BeFalse())
			Expect(clusterAutoscalerServiceAccount.Exists()).To(BeFalse())
			Expect(clusterAutoscalerRole.Exists()).To(BeFalse())
			Expect(clusterAutoscalerRoleBinding.Exists()).To(BeFalse())
			Expect(clusterAutoscalerClusterRole.Exists()).To(BeFalse())
			Expect(clusterAutoscalerClusterRoleBinding.Exists()).To(BeFalse())

			Expect(machineClassA.Exists()).To(BeFalse())
			Expect(machineClassSecretA.Exists()).To(BeFalse())
			Expect(machineDeploymentA.Exists()).To(BeFalse())

			Expect(machineClassB.Exists()).To(BeFalse())
			Expect(machineClassSecretB.Exists()).To(BeFalse())
			Expect(machineDeploymentB.Exists()).To(BeFalse())

			Expect(bashibleSecrets["bashible-bashbooster"].Exists()).To(BeTrue())

			Expect(bootstrapSecrets["manual-bootstrap-for-worker"].Exists()).To(BeTrue())

			Expect(roles["bashible"].Exists()).To(BeTrue())
			Expect(roles["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			Expect(roleBindings["bashible"].Exists()).To(BeTrue())
			Expect(roleBindings["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			Expect(staticMachineTemplate.ToYaml()).To(MatchYAML(nodeManagerStaticInstancesStaticMachineTemplate))
			Expect(staticMachineDeployment.ToYaml()).To(MatchYAML(nodeManagerStaticInstancesMachineDeployment))

			assertBashibleAPIServerTLS(f)
		})
	})

	Context("Setting tags/labels to MachineClass", func() {
		providerValues := `{ "o":"provider", "z":"provider" }`
		nodeGroupValues := `{ "a":"nodegroup", "o":"nodegroup" }`
		// Basically asserting that provider entries are overwritten with nodegroup ones
		assertValues := func(machineClass object_store.KubeObject, mapPath string) {
			mapJSON := machineClass.Field(mapPath).String()
			a := machineClass.Field(mapPath + ".a").String()
			o := machineClass.Field(mapPath + ".o").String()
			z := machineClass.Field(mapPath + ".z").String()
			Expect(a).To(Equal("nodegroup"), `"a" must be "nodegroup" in `+mapPath+" "+mapJSON)
			Expect(o).To(Equal("nodegroup"), `"o" must be "nodegroup" in `+mapPath+" "+mapJSON)
			Expect(z).To(Equal("provider"), `"z" must be "provider" in `+mapPath+" "+mapJSON)
		}

		Context("AWS", func() {
			f := SetupHelmConfig(``)

			BeforeEach(func() {
				f.ValuesSetFromYaml("global", globalValues)
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerAWS)
				f.ValuesSetFromYaml("nodeManager.internal.cloudProvider.aws.tags", providerValues)
				f.ValuesSetFromYaml("nodeManager.internal.nodeGroups.0.instanceClass.additionalTags", nodeGroupValues)
				setBashibleAPIServerTLSValues(f)
				f.HelmRender()
			})

			It("spec.tags must contain tags from cloud provider and nodegroup", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				mcls := f.KubernetesResource("AWSMachineClass", "d8-cloud-instance-manager", "worker-02320933")

				Expect(mcls.Exists()).To(BeTrue())
				assertValues(mcls, "spec.tags")
			})

			// Important! If checksum changes, the MachineDeployments will re-deploy!
			// All nodes in MD will reboot! If you're not sure, don't change it.
			It("preserves checksum", func() {
				machineDeployment := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
				checksum := machineDeployment.Field("spec.template.metadata.annotations.checksum/machine-class").String()
				Expect(checksum).To(Equal("32ed026c31873a9b40c14182924c1d5d6766f025581f4562652f8ccb784898f2"))
			})
		})

		Context("Openstack", func() {
			f := SetupHelmConfig(``)

			BeforeEach(func() {
				f.ValuesSetFromYaml("global", globalValues)
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerOpenstack)
				f.ValuesSetFromYaml("nodeManager.internal.cloudProvider.openstack.tags", providerValues)
				f.ValuesSetFromYaml("nodeManager.internal.nodeGroups.0.instanceClass.additionalTags", nodeGroupValues)
				setBashibleAPIServerTLSValues(f)
				f.HelmRender()
			})

			It("spec.tags must contain tags from cloud provider and nodegroup", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				mcls := f.KubernetesResource("OpenstackMachineClass", "d8-cloud-instance-manager", "worker-02320933")

				Expect(mcls.Exists()).To(BeTrue())
				assertValues(mcls, "spec.tags")
			})

			// Important! If checksum changes, the MachineDeployments will re-deploy!
			// All nodes in MD will reboot! If you're not sure, don't change it.
			It("preserves checksum", func() {
				machineDeployment := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
				checksum := machineDeployment.Field("spec.template.metadata.annotations.checksum/machine-class").String()
				Expect(checksum).To(Equal("453963d10ea1bfa125d4186fe8a3cf9ec01cc769c694b0c0a74ed781364cb71e"))
			})
		})

		Context("Azure", func() {
			f := SetupHelmConfig(``)

			BeforeEach(func() {
				f.ValuesSetFromYaml("global", globalValues)
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerAzure)
				f.ValuesSetFromYaml("nodeManager.internal.cloudProvider.azure.additionalTags", providerValues)
				f.ValuesSetFromYaml("nodeManager.internal.nodeGroups.0.instanceClass.additionalTags", nodeGroupValues)
				setBashibleAPIServerTLSValues(f)
				f.HelmRender()
			})

			It("spec.tags must contain tags from cloud provider and nodegroup", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				mcls := f.KubernetesResource("AzureMachineClass", "d8-cloud-instance-manager", "worker-02320933")

				Expect(mcls.Exists()).To(BeTrue())
				assertValues(mcls, "spec.tags")
			})

			It("spec.properties.networkProfile.acceleratedNetworking is not set (default true)", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				t := f.KubernetesResource("AzureMachineClass", "d8-cloud-instance-manager", "worker-02320933")
				Expect(t.Exists()).To(BeTrue())
				Expect(t.Field("spec.properties.networkProfile.acceleratedNetworking").Bool()).To(Equal(true))
			})

			It("spec.properties.networkProfile.acceleratedNetworking is set to true", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				t := f.KubernetesResource("AzureMachineClass", "d8-cloud-instance-manager", "bbb-02320933")
				Expect(t.Exists()).To(BeTrue())
				Expect(t.Field("spec.properties.networkProfile.acceleratedNetworking").Bool()).To(Equal(true))
			})

			It("spec.properties.networkProfile.acceleratedNetworking is set to false", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				t := f.KubernetesResource("AzureMachineClass", "d8-cloud-instance-manager", "aaa-02320933")
				Expect(t.Exists()).To(BeTrue())
				Expect(t.Field("spec.properties.networkProfile.acceleratedNetworking").Bool()).To(Equal(false))
			})

			// Important! If checksum changes, the MachineDeployments will re-deploy!
			// All nodes in MD will reboot! If you're not sure, don't change it.
			It("preserves checksum", func() {
				machineDeployment := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
				checksum := machineDeployment.Field("spec.template.metadata.annotations.checksum/machine-class").String()
				Expect(checksum).To(Equal("891a23e39148fe1457b88ad65898164c65df2e4cd34b013e4289127091089d95"))
			})
		})

		Context("GCP", func() {
			f := SetupHelmConfig(``)

			BeforeEach(func() {
				f.ValuesSetFromYaml("global", globalValues)
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerGCP)
				f.ValuesSetFromYaml("nodeManager.internal.cloudProvider.gcp.labels", providerValues)
				f.ValuesSetFromYaml("nodeManager.internal.nodeGroups.0.instanceClass.additionalLabels", nodeGroupValues)
				setBashibleAPIServerTLSValues(f)
				f.HelmRender()
			})

			It("spec.labels must contain labels from cloud provider and nodegroup", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				mcls := f.KubernetesResource("GCPMachineClass", "d8-cloud-instance-manager", "worker-02320933")

				Expect(mcls.Exists()).To(BeTrue())
				assertValues(mcls, "spec.labels")
			})

			// Important! If checksum changes, the MachineDeployments will re-deploy!
			// All nodes in MD will reboot! If you're not sure, don't change it.
			It("preserves checksum", func() {
				machineDeployment := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
				checksum := machineDeployment.Field("spec.template.metadata.annotations.checksum/machine-class").String()
				Expect(checksum).To(Equal("c87109f7fbd4b885f754a0f3d913bbc4340e5a585449ed29e36930b6b6503ac6"))
			})
		})

		Context("Yandex", func() {
			f := SetupHelmConfig(``)

			BeforeEach(func() {
				f.ValuesSetFromYaml("global", globalValues)
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerYandex)
				f.ValuesSetFromYaml("nodeManager.internal.cloudProvider.yandex.labels", providerValues)
				f.ValuesSetFromYaml("nodeManager.internal.nodeGroups.0.instanceClass.additionalLabels", nodeGroupValues)
				setBashibleAPIServerTLSValues(f)
				f.HelmRender()
			})

			It("spec.labels must contain labels from cloud provider and nodegroup", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				mcls := f.KubernetesResource("YandexMachineClass", "d8-cloud-instance-manager", "worker-02320933")

				Expect(mcls.Exists()).To(BeTrue())
				assertValues(mcls, "spec.labels")
			})

			// Important! If checksum changes, the MachineDeployments will re-deploy!
			// All nodes in MD will reboot! If you're not sure, don't change it.
			It("preserves checksum", func() {
				machineDeployment := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
				checksum := machineDeployment.Field("spec.template.metadata.annotations.checksum/machine-class").String()
				Expect(checksum).To(Equal("55b0c5ac9c7e72252f509bc825f5046e198eab25ebd80efa3258cfb38e881359"))
			})
		})
	})

	Context("CAPI", func() {
		assertClusterResources := func(f *Config, clusterName string) {
			cluster := f.KubernetesResource("Cluster", "d8-cloud-instance-manager", clusterName)
			Expect(cluster.Exists()).To(BeTrue())

			Expect(cluster.Field("spec.clusterNetwork.pods.cidrBlocks.0").String()).To(Equal("10.111.0.0/16"))
			Expect(cluster.Field("spec.clusterNetwork.services.cidrBlocks.0").String()).To(Equal("10.222.0.0/16"))
			Expect(cluster.Field("spec.clusterNetwork.serviceDomain").String()).To(Equal("cluster.local"))

			Expect(cluster.Field("spec.controlPlaneRef.apiVersion").String()).To(Equal("infrastructure.cluster.x-k8s.io/v1alpha1"))
			Expect(cluster.Field("spec.controlPlaneRef.kind").String()).To(Equal("DeckhouseControlPlane"))
			Expect(cluster.Field("spec.controlPlaneRef.namespace").String()).To(Equal("d8-cloud-instance-manager"))
			Expect(cluster.Field("spec.controlPlaneRef.name").String()).To(Equal(fmt.Sprintf("%s-control-plane", clusterName)))

			controlPlane := f.KubernetesResource("DeckhouseControlPlane", "d8-cloud-instance-manager", fmt.Sprintf("%s-control-plane", clusterName))
			Expect(controlPlane.Exists()).To(BeTrue())

			healthCheck := f.KubernetesResource("MachineHealthCheck", "d8-cloud-instance-manager", fmt.Sprintf("%s-machine-health-check", clusterName))
			Expect(healthCheck.Exists()).To(BeTrue())
			Expect(healthCheck.Field("spec.clusterName").String()).To(Equal(clusterName))

			capiDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "capi-controller-manager")
			Expect(capiDeploy.Exists()).To(BeTrue())
		}

		Context("VCD", func() {
			const nodeManagerVCD = `
internal:
  capiControllerManagerEnabled: true
  bootstrapTokens:
    worker: mytoken
  capiControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  capsControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  machineDeployments: {}
  instancePrefix: myprefix
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  kubernetesCA: myclusterca
  cloudProvider:
    type: vcd
    machineClassKind: ""
    capiClusterKind: "VCDCluster"
    capiClusterAPIVersion: "infrastructure.cluster.x-k8s.io/v1beta2"
    capiClusterName: "app"
    capiMachineTemplateKind: "VCDMachineTemplate"
    capiMachineTemplateAPIVersion: "infrastructure.cluster.x-k8s.io/v1beta2"
    vcd:
      sshPublicKey: ssh-rsa AAAAA
      organization: org
      virtualDataCenter: dc
      virtualApplicationName: app
      server: https://localhost:5000
      username: user
      password: pass
      insecure: true
  nodeGroups:
  - name: worker
    nodeCapacity:
      cpu: "2"
      memory: "2Gi"
    instanceClass:
      rootDiskSizeGb: 20
      sizingPolicy: s-c572-MSK1-S1-vDC1
      storageProfile: vHDD
      template: Ubuntu
      placementPolicy: policy
    nodeType: CloudEphemeral
    kubernetesVersion: "1.24"
    cri:
      type: "Docker"
    cloudInstances:
      classReference:
        kind: VcdInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 4
      zones:
      - zonea
      - zoneb
  - name: worker-big
    nodeCapacity:
      cpu: "2"
      memory: "2Gi"
    instanceClass:
      rootDiskSizeGb: 20
      sizingPolicy: s-c572-MSK1-S1-vDC1
      storageProfile: vHDD
      template: catalog/Ubuntu
      placementPolicy: policy
    nodeType: CloudEphemeral
    kubernetesVersion: "1.24"
    cri:
      type: "Docker"
    cloudInstances:
      classReference:
        kind: VcdInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 4
      zones:
      - zonea
  machineControllerManagerEnabled: false
`
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", globalValues)
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerVCD)
				setBashibleAPIServerTLSValues(f)
				f.HelmRender()
			})

			It("Everything must render properly", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				assertVCDCluster := func(f *Config) {
					secret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "capi-user-credentials")
					Expect(secret.Exists()).To(BeTrue())
					Expect(secret.Field("data.username").String()).To(Equal("dXNlcg==")) // user
					Expect(secret.Field("data.password").String()).To(Equal("cGFzcw==")) // pass

					vcdCluster := f.KubernetesResource("VCDCluster", "d8-cloud-instance-manager", "app")
					Expect(vcdCluster.Exists()).To(BeTrue())
					Expect(vcdCluster.Field("spec.site").String()).To(Equal("https://localhost:5000"))
					Expect(vcdCluster.Field("spec.org").String()).To(Equal("org"))
					Expect(vcdCluster.Field("spec.ovdc").String()).To(Equal("dc"))

					Expect(vcdCluster.Field("spec.proxyConfigSpec.httpProxy").String()).To(Equal("https://example.com"))
					Expect(vcdCluster.Field("spec.proxyConfigSpec.httpsProxy").String()).To(Equal("https://example.com"))
					Expect(vcdCluster.Field("spec.proxyConfigSpec.noProxy").AsStringSlice()).To(Equal([]string{
						"127.0.0.1", "169.254.169.254", "cluster.local", "10.111.0.0/16", "10.222.0.0/16", "example.com",
					}))
				}

				type mdParams struct {
					name         string
					templateName string
				}

				assertMachineDeploymentAndItsDeps := func(f *Config, d mdParams) {
					md := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", d.name)
					Expect(md.Exists()).To(BeTrue())

					Expect(md.Field("spec.clusterName").String()).To(Equal("app"))
					Expect(md.Field("spec.template.spec.clusterName").String()).To(Equal("app"))
					Expect(md.Field("spec.template.spec.bootstrap.dataSecretName").String()).To(Equal(d.templateName))
					Expect(md.Field("spec.template.spec.infrastructureRef.name").String()).To(Equal(d.templateName))

					annotations := md.Field("metadata.annotations").Map()
					Expect(annotations["cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size"].String()).To(Equal("4"))
					Expect(annotations["cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size"].String()).To(Equal("5"))
					Expect(annotations["capacity.cluster-autoscaler.kubernetes.io/cpu"].String()).To(Equal("2"))
					Expect(annotations["capacity.cluster-autoscaler.kubernetes.io/memory"].String()).To(Equal("2Gi"))

					secret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", d.templateName)
					Expect(secret.Exists()).To(BeTrue())

					vcdTemplate := f.KubernetesResource("VCDMachineTemplate", "d8-cloud-instance-manager", d.templateName)
					Expect(vcdTemplate.Exists()).To(BeTrue())

					Expect(vcdTemplate.Field("spec.template.spec.diskSize").String()).To(Equal("21474836480"))
					Expect(vcdTemplate.Field("spec.template.spec.sizingPolicy").String()).To(Equal("s-c572-MSK1-S1-vDC1"))
					Expect(vcdTemplate.Field("spec.template.spec.placementPolicy").String()).To(Equal("policy"))
					Expect(vcdTemplate.Field("spec.template.spec.storageProfile").String()).To(Equal("vHDD"))
					Expect(vcdTemplate.Field("spec.template.spec.template").String()).To(Equal("Ubuntu"))

					Expect(vcdTemplate.Field("metadata.annotations.checksum/instance-class").String()).To(Equal("9a87428aa818245d4b86ee9438255d53e6ae2d8a76d43cfb1b7560a6f0eab02e"), "Prevent checksum changing")
					Expect(md.Field("metadata.annotations.checksum/instance-class").String()).To(Equal("9a87428aa818245d4b86ee9438255d53e6ae2d8a76d43cfb1b7560a6f0eab02e"), "Prevent checksum changing")
				}

				registrySecret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "deckhouse-registry")
				Expect(registrySecret.Exists()).To(BeTrue())

				assertClusterResources(f, "app")

				assertVCDCluster(f)

				// zonea
				assertMachineDeploymentAndItsDeps(f, mdParams{
					name:         "myprefix-worker-02320933",
					templateName: "worker-6656f66e",
				})

				// zoneb
				assertMachineDeploymentAndItsDeps(f, mdParams{
					name:         "myprefix-worker-6bdb5b0d",
					templateName: "worker-d30762c9",
				})

				vcdTemplateWithCatalog := f.KubernetesResource("VCDMachineTemplate", "d8-cloud-instance-manager", "worker-big-c10b569f")
				Expect(vcdTemplateWithCatalog.Exists()).To(BeTrue())
				Expect(vcdTemplateWithCatalog.Field("spec.template.spec.template").String()).To(Equal("Ubuntu"))
				Expect(vcdTemplateWithCatalog.Field("spec.template.spec.catalog").String()).To(Equal("catalog"))
			})
		})
	})
})

func verifyClusterAutoscalerDeploymentArgs(deployment object_store.KubeObject, mds ...object_store.KubeObject) error {
	args := deployment.Field("spec.template.spec.containers.0.args").AsStringSlice()

	var nodesArgs []string
	for _, arg := range args {
		if !strings.HasPrefix(arg, "--nodes") {
			continue
		}

		nodesArgs = append(nodesArgs, strings.Split(arg, ".")[1])
	}

	var mdsNames []string
	for _, md := range mds {
		mdsNames = append(mdsNames, md.Field("metadata.name").String())
	}

	sort.Strings(nodesArgs)
	sort.Strings(mdsNames)
	equal := cmp.Equal(nodesArgs, mdsNames)
	if !equal {
		return fmt.Errorf("cluster-autoscaler args %+v are not equal to a list of MachineDeployment names %+v", nodesArgs, mdsNames)
	}

	return nil
}

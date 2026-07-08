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
enabledModules: ["vertical-pod-autoscaler"]
modules:
  placement: {}
discovery:
  d8SpecificNodeCountByRole:
    master: 3
  clusterUUID: f49dd1c3-a63a-4565-a06c-625e35587eab
  kubernetesVersion: 1.32.8
clusterConfiguration:
  apiVersion: deckhouse.io/v1
  cloud:
    prefix: sandbox
    provider: vSphere
  clusterDomain: cluster.local
  clusterType: Cloud
  defaultCRI: Containerd
  kind: ClusterConfiguration
  kubernetesVersion: "1.32"
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
  nodeControllerWebhookCert:
    ca: string
    key: string
    crt: string
  allowedBundles:
    - "ubuntu-lts"
    - "centos"
    - "debian"
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
  nodeControllerWebhookCert:
    ca: string
    key: string
    crt: string

  clusterAutoscalerPriorities:
    "50":
    - .*xxx-staging-[0-9a-zA-Z]+$
    "70":
    - .*xxx-staging-spot-m5a-2xlarge-[0-9a-zA-Z]+$
    "90":
    - .*xxx-staging-spot-[0-9a-zA-Z]+$
    - .*xxx-staging-spot-m5a.8xlarge-[0-9a-zA-Z]+$
    - .*xxx-staging-spot-c5.16xlarge-[0-9a-zA-Z]+$
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
    kubernetesVersion: "1.32"
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
  nodeControllerWebhookCert:
    ca: string
    key: string
    crt: string
  instancePrefix: myprefix
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  kubernetesCA: myclusterca
  cloudProvider:
    type: azure
    machineClassKind: AzureMachineClass
    azure:
      sshPublicKey: ssh-rsa AAAAB...==
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
    kubernetesVersion: "1.32"
    cri:
      type: "Containerd"
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
  nodeControllerWebhookCert:
    ca: string
    key: string
    crt: string
  instancePrefix: myprefix
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  kubernetesCA: myclusterca
  cloudProvider:
    type: gcp
    machineClassKind: GCPMachineClass
    gcp:
      region: region
      sshKey: cert-authority,principals="test" ssh-rsa AAAAB...==
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
    kubernetesVersion: "1.32"
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
  nodeControllerWebhookCert:
    ca: string
    key: string
    crt: string
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
    kubernetesVersion: "1.32"
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
  nodeControllerWebhookCert:
    ca: string
    key: string
    crt: string
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
    kubernetesVersion: "1.32"
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
    kubernetesVersion: "1.32"
    cri:
      type: "Containerd"
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
  nodeControllerWebhookCert:
    ca: string
    key: string
    crt: string
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
      sshKeys: ['cert-authority,principals="test" ssh-rsa AAAAB...==', key2] #
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
    kubernetesVersion: "1.32"
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
    kubernetesVersion: "1.32"
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
  nodeControllerWebhookCert:
    ca: string
    key: string
    crt: string
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
      sshKey: cert-authority,principals="test" ssh-rsa AAAAB...==
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
    kubernetesVersion: "1.32"
    cri:
      type: "Containerd"
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
  nodeControllerWebhookCert:
    ca: string
    key: string
    crt: string
  instancePrefix: myprefix
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  kubernetesCA: myclusterca
  bootstrapTokens:
    worker: myworker
  nodeGroups:
  - name: worker
    nodeType: Static
    kubernetesVersion: "1.32"
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
  nodeControllerWebhookCert:
    ca: string
    key: string
    crt: string
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
)

const (
	openstackCIMPath    = "/deckhouse/ee/modules/030-cloud-provider-openstack/cloud-instance-manager"
	openstackCIMSymlink = "/deckhouse/modules/040-node-manager/cloud-providers/openstack"
	vsphereCIMPath      = "/deckhouse/ee/se-plus/modules/030-cloud-provider-vsphere/cloud-instance-manager"
	vsphereCIMSymlink   = "/deckhouse/modules/040-node-manager/cloud-providers/vsphere"
	vcdCAPIPath         = "/deckhouse/ee/modules/030-cloud-provider-vcd/capi"
	vcdCAPISymlink      = "/deckhouse/modules/040-node-manager/capi/vcd"
)

var nodeManagerAWSSpot = strings.Replace(nodeManagerAWS, "      instanceType: t2.medium\n", "      instanceType: t2.medium\n      spot: true\n", 1)

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
			// fake *-crd modules are required for backward compatibility with lib_helm library
			// TODO: remove fake crd modules
			f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler", "operator-prometheus", "vertical-pod-autoscaler-crd", "operator-prometheus-crd"]`)
		})

		Context("For cluster auto-scaler", func() {
			Context("cluster auto-scaler disabled", func() {
				BeforeEach(func() {
					f.HelmRender()
				})

				It("PrometheusRule does not Exists", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					rule := f.KubernetesResource("PrometheusRule", "d8-cloud-instance-manager", "node-manager-cluster-autoscaler")
					Expect(rule.Exists()).Should(BeFalse())
				})
			})

			Context("cluster auto-scaler enabled", func() {
				BeforeEach(func() {
					// autoscaler enabled if have none empty cloud node group
					f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerAWS)
					setBashibleAPIServerTLSValues(f)
					f.HelmRender()
				})

				It("PrometheusRule Exists", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					rule := f.KubernetesResource("PrometheusRule", "d8-cloud-instance-manager", "node-manager-cluster-autoscaler")
					Expect(rule.Exists()).Should(BeTrue())
					cm := f.KubernetesResource("ConfigMap", "d8-cloud-instance-manager", "cluster-autoscaler-priority-expander")
					Expect(cm.Field("data.priorities").String()).To(MatchYAML(`
50:
  - .*xxx-staging-[0-9a-zA-Z]+$
70:
  - .*xxx-staging-spot-m5a-2xlarge-[0-9a-zA-Z]+$
90:
  - .*xxx-staging-spot-[0-9a-zA-Z]+$
  - .*xxx-staging-spot-m5a.8xlarge-[0-9a-zA-Z]+$
  - .*xxx-staging-spot-c5.16xlarge-[0-9a-zA-Z]+$
`))

				})
			})

			Context("cluster auto-scaler split mode with MCM only", func() {
				BeforeEach(func() {
					f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerAWS)
					f.ValuesSetFromYaml("nodeManager.internal.deployAutoscalerMCM", "true")
					f.ValuesSetFromYaml("nodeManager.internal.autoscalerMCMNodes", `["--nodes=0:2:d8-cloud-instance-manager.myprefix-worker-02320933"]`)
					f.ValuesSetFromYaml("nodeManager.internal.deployAutoscaler", "false")
					f.ValuesSetFromYaml("nodeManager.internal.autoscalerNodes", `[]`)
					setBashibleAPIServerTLSValues(f)
					f.HelmRender()
				})

				It("renders MCM autoscaler target alerts against the MCM job", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					rule := f.KubernetesResource("PrometheusRule", "d8-cloud-instance-manager", "node-manager-cluster-autoscaler")
					Expect(rule.Exists()).Should(BeTrue())
					Expect(rule.Field("spec.groups.0.rules.2.expr").String()).To(Equal(`max by (job) (up{job=~"cluster-autoscaler-mcm", namespace="d8-cloud-instance-manager"} == 0)`))
					Expect(rule.Field("spec.groups.0.rules.3.expr").String()).To(Equal(`absent(up{job="cluster-autoscaler-mcm", namespace="d8-cloud-instance-manager"} == 1)`))
				})
			})

			Context("cluster auto-scaler split mode with MCM and CAPI", func() {
				BeforeEach(func() {
					f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerAWS)
					f.ValuesSetFromYaml("nodeManager.internal.deployAutoscaler", "true")
					f.ValuesSetFromYaml("nodeManager.internal.autoscalerNodes", `["--nodes=0:2:d8-cloud-instance-manager.myprefix-worker-02320933"]`)
					f.ValuesSetFromYaml("nodeManager.internal.deployAutoscalerMCM", "true")
					f.ValuesSetFromYaml("nodeManager.internal.autoscalerMCMNodes", `["--nodes=0:2:d8-cloud-instance-manager.myprefix-worker-6bdb5b0d"]`)
					setBashibleAPIServerTLSValues(f)
					f.HelmRender()
				})

				It("renders target alerts for both autoscaler jobs", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					rule := f.KubernetesResource("PrometheusRule", "d8-cloud-instance-manager", "node-manager-cluster-autoscaler")
					Expect(rule.Exists()).Should(BeTrue())
					Expect(rule.Field("spec.groups.0.rules.2.expr").String()).To(Equal(`max by (job) (up{job=~"cluster-autoscaler|cluster-autoscaler-mcm", namespace="d8-cloud-instance-manager"} == 0)`))
					Expect(rule.Field("spec.groups.0.rules.3.expr").String()).To(Equal(`absent(up{job="cluster-autoscaler", namespace="d8-cloud-instance-manager"} == 1) or absent(up{job="cluster-autoscaler-mcm", namespace="d8-cloud-instance-manager"} == 1)`))
				})
			})
		})

		Context("For machine controller manager", func() {
			Context("machine controller manager disabled", func() {
				BeforeEach(func() {
					f.HelmRender()
				})

				It("PrometheusRule does not Exists", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					rule := f.KubernetesResource("PrometheusRule", "d8-cloud-instance-manager", "node-manager-machine-controller-manager")
					Expect(rule.Exists()).Should(BeFalse())
				})
			})

			Context("machine controller manager enabled", func() {
				BeforeEach(func() {
					f.ValuesSetFromYaml("nodeManager.mcmEmergencyBrake", "false")
					f.ValuesSetFromYaml("nodeManager.internal.machineControllerManagerEnabled", "true")

					f.HelmRender()
				})

				It("PrometheusRule Exists", func() {
					Expect(f.RenderError).ShouldNot(HaveOccurred())

					rule := f.KubernetesResource("PrometheusRule", "d8-cloud-instance-manager", "node-manager-machine-controller-manager")
					Expect(rule.Exists()).Should(BeTrue())
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

			Expect(verifyClusterAutoscalerDeploymentArgs(clusterAutoscalerDeploy, "myprefix-worker-02320933", "myprefix-worker-6bdb5b0d")).To(Succeed())

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

			// MachineClass CR and MachineDeployment are rendered by node-controller
			// (capi.reconcileCloudMCMs), not helm; only the MachineClass Secret stays in helm.
			Expect(machineClassA.Exists()).To(BeFalse())
			Expect(machineClassSecretA.Exists()).To(BeTrue())
			Expect(machineDeploymentA.Exists()).To(BeFalse())

			Expect(machineClassB.Exists()).To(BeFalse())
			Expect(machineClassSecretB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Exists()).To(BeFalse())

			Expect(bashibleSecrets["bashible-bashbooster"].Exists()).To(BeTrue())

			Expect(roles["bashible"].Exists()).To(BeTrue())
			Expect(roles["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			Expect(roleBindings["bashible"].Exists()).To(BeTrue())
			Expect(roleBindings["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			assertBashibleAPIServerTLS(f)
		})
	})

	Context("AWS spot", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerAWSSpot)
			setBashibleAPIServerTLSValues(f)
			f.HelmRender()
		})

		// The spot creationTimeout=5m is now set by node-controller
		// (capi.buildMCMMachineDeployment); see the AWSSpot case in
		// mcm_machinedeployment_test.go. Helm no longer renders the MachineDeployment.
		It("does not render MachineDeployments in helm", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			machineDeploymentA := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
			machineDeploymentB := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-6bdb5b0d")

			Expect(machineDeploymentA.Exists()).To(BeFalse())
			Expect(machineDeploymentB.Exists()).To(BeFalse())
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

			Expect(verifyClusterAutoscalerDeploymentArgs(clusterAutoscalerDeploy, "myprefix-worker-02320933", "myprefix-worker-6bdb5b0d")).To(Succeed())

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

			// MachineClass CR and MachineDeployment are rendered by node-controller
			// (capi.reconcileCloudMCMs), not helm; only the MachineClass Secret stays in helm.
			Expect(machineClassA.Exists()).To(BeFalse())
			Expect(machineClassSecretA.Exists()).To(BeTrue())
			Expect(machineDeploymentA.Exists()).To(BeFalse())

			Expect(machineClassB.Exists()).To(BeFalse())
			Expect(machineClassSecretB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Exists()).To(BeFalse())

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

			Expect(verifyClusterAutoscalerDeploymentArgs(clusterAutoscalerDeploy, "myprefix-worker-02320933", "myprefix-worker-6bdb5b0d", "myprefix-simple-02320933")).To(Succeed())

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

			// MachineClass CR and MachineDeployment are rendered by node-controller
			// (capi.reconcileCloudMCMs), not helm; only the MachineClass Secret stays in helm.
			// OpenstackMachineClass field content (networks/securityGroups/tags/flavorName/imageName)
			// is covered by render_openstack_test.go.
			Expect(machineClassA.Exists()).To(BeFalse())
			Expect(machineClassSecretA.Exists()).To(BeTrue())
			Expect(machineDeploymentA.Exists()).To(BeFalse())

			Expect(machineClassB.Exists()).To(BeFalse())
			Expect(machineClassSecretB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Exists()).To(BeFalse())

			Expect(simpleMachineClassA.Exists()).To(BeFalse())
			Expect(simpleMachineClassSecretA.Exists()).To(BeTrue())
			Expect(simpleMachineDeploymentA.Exists()).To(BeFalse())

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

			Expect(verifyClusterAutoscalerDeploymentArgs(clusterAutoscalerDeploy, "myprefix-worker-02320933", "myprefix-worker-6bdb5b0d", "myprefix-worker-with-disabled-nested-virt-02320933", "myprefix-worker-with-disabled-nested-virt-6bdb5b0d")).To(Succeed())

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

			// MachineClass CR and MachineDeployment are rendered by node-controller
			// (capi.reconcileCloudMCMs), not helm; only the MachineClass Secret stays in helm.
			Expect(machineClassA.Exists()).To(BeFalse())
			Expect(machineClassSecretA.Exists()).To(BeTrue())
			Expect(machineDeploymentA.Exists()).To(BeFalse())

			Expect(machineClassB.Exists()).To(BeFalse())
			Expect(machineClassSecretB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Exists()).To(BeFalse())

			// VsphereMachineClass CR / MachineDeployment are controller-owned; the Secret stays in helm.
			// spec.runtimeOptions.nestedHardwareVirtualization content is covered by render_vsphere_test.go.
			Expect(machineClassAWitoutNestedVirt.Exists()).To(BeFalse())
			Expect(machineClassSecretAWitoutNestedVirt.Exists()).To(BeTrue())
			Expect(machineDeploymentAWitoutNestedVirt.Exists()).To(BeFalse())

			Expect(machineClassBWitoutNestedVirt.Exists()).To(BeFalse())
			Expect(machineClassSecretBWitoutNestedVirt.Exists()).To(BeTrue())
			Expect(machineDeploymentBWitoutNestedVirt.Exists()).To(BeFalse())

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

			Expect(verifyClusterAutoscalerDeploymentArgs(clusterAutoscalerDeploy, "myprefix-worker-02320933", "myprefix-worker-6bdb5b0d")).To(Succeed())

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

			// MachineClass CR and MachineDeployment are rendered by node-controller
			// (capi.reconcileCloudMCMs), not helm; only the MachineClass Secret stays in helm.
			Expect(machineClassA.Exists()).To(BeFalse())
			Expect(machineClassSecretA.Exists()).To(BeTrue())
			Expect(machineDeploymentA.Exists()).To(BeFalse())

			Expect(machineClassB.Exists()).To(BeFalse())
			Expect(machineClassSecretB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Exists()).To(BeFalse())

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

			// StaticMachineTemplate and MachineDeployment are created by node-controller
			// (capi.reconcileStaticMDRendered), not helm.
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

			Expect(staticMachineTemplate.Exists()).To(BeFalse())
			Expect(staticMachineDeployment.Exists()).To(BeFalse())

			assertBashibleAPIServerTLS(f)
		})
	})

	// The "Setting tags/labels to MachineClass" helm suite was removed: the
	// MachineClass CR is now rendered by node-controller (capi.reconcileCloudMCMs),
	// so provider+nodegroup tag/label merge, azure acceleratedNetworking, and the
	// per-provider render output are covered by render_*_test.go. The MachineClass
	// checksum (checksum/machine-class annotation — the only fleet-roll trigger) is
	// covered byte-for-byte by TestRenderChecksum_MCMProviderParity in the
	// node-controller machineclass package.

	Context("CAPI", func() {
		assertClusterResources := func(f *Config, clusterName string) {
			// Cluster and MachineHealthCheck (cluster.x-k8s.io/v1beta1) are no
			// longer rendered by helm — they are owned by the
			// create_capi_cluster_resources hook on a dedicated queue (see
			// hooks/create_capi_cluster_resources.go). Helm rendering used to
			// race the capi conversion webhook. Hook-level tests cover their
			// content; template tests only assert what helm still owns.
			cluster := f.KubernetesResource("Cluster", "d8-cloud-instance-manager", clusterName)
			Expect(cluster.Exists()).To(BeFalse())

			healthCheck := f.KubernetesResource("MachineHealthCheck", "d8-cloud-instance-manager", fmt.Sprintf("%s-machine-health-check", clusterName))
			Expect(healthCheck.Exists()).To(BeFalse())

			controlPlane := f.KubernetesResource("DeckhouseControlPlane", "d8-cloud-instance-manager", fmt.Sprintf("%s-control-plane", clusterName))
			Expect(controlPlane.Exists()).To(BeTrue())

			capiDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "capi-controller-manager")
			Expect(capiDeploy.Exists()).To(BeTrue())
		}

		Context("Scale from zero annotations", func() {
			const nodeManager = `
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
  nodeControllerWebhookCert:
    ca: string
    key: string
    crt: string
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
      sshPublicKey: cert-authority,principals="test" ssh-rsa AAAAB...==
      organization: org
      virtualDataCenter: dc
      virtualApplicationName: app
      server: https://localhost:5000
      username: user
      password: pass
      insecure: true
  nodeGroups:
  - name: without-labels-and-taints
    serializedLabels: ""
    serializedTaints: ""
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
      type: "Containerd"
    cloudInstances:
      classReference:
        kind: VcdInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 4
      zones:
      - zonea
      - zoneb
  - name: with-labels-only
    serializedLabels: "app=warp-drive-ai,environment=production"
    serializedTaints: ""
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
      type: "Containerd"
    cloudInstances:
      classReference:
        kind: VcdInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 4
      zones:
      - zonea
  - name: with-taints-only
    serializedLabels: ""
    serializedTaints: "b=v:NoExecute,a,d:NoExecute,c=v1:"
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
      type: "Containerd"
    cloudInstances:
      classReference:
        kind: VcdInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 4
      zones:
      - zonea
  - name: with-labels-and-taints
    serializedLabels: "app=warp-drive-ai,environment=production"
    serializedTaints: "b=v:NoExecute,a,d:NoExecute,c=v1:"
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
      type: "Containerd"
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
				f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManager)
				setBashibleAPIServerTLSValues(f)
				f.HelmRender()
			})

			It("Everything must render properly", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				// MachineDeployments are created by the capi-machine-deployment controller, not helm.
				for _, name := range []string{
					"myprefix-without-labels-and-taints-02320933",
					"myprefix-with-labels-only-02320933",
					"myprefix-with-taints-only-02320933",
					"myprefix-with-labels-and-taints-02320933",
				} {
					md := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", name)
					Expect(md.Exists()).To(BeFalse())
				}
			})
		})

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
  nodeControllerWebhookCert:
    ca: string
    key: string
    crt: string
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
      sshPublicKey: cert-authority,principals="test" ssh-rsa AAAAB...==
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
      type: "Containerd"
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
      type: "Containerd"
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
				}

				type mdParams struct {
					name                string
					templateName        string
					bootstrapSecretName string
				}

				// MachineDeployment and VCDMachineTemplate are created by node-controller
				// (capi.reconcileCloudMDsRendered), not helm. Only the bootstrap Secret
				// stays in helm.
				assertMachineDeploymentAndItsDeps := func(f *Config, d mdParams) {
					md := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", d.name)
					Expect(md.Exists()).To(BeFalse())

					// The bootstrap Secret no longer embeds the instance-class checksum:
					// its name is {ng}-{sha(clusterUUID+zone)}, independent of the template.
					secret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", d.bootstrapSecretName)
					Expect(secret.Exists()).To(BeTrue())

					vcdTemplate := f.KubernetesResource("VCDMachineTemplate", "d8-cloud-instance-manager", d.templateName)
					Expect(vcdTemplate.Exists()).To(BeFalse())
				}
				//
				registrySecret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "deckhouse-registry")
				Expect(registrySecret.Exists()).To(BeTrue())

				assertClusterResources(f, "app")

				assertVCDCluster(f)

				// zonea
				assertMachineDeploymentAndItsDeps(f, mdParams{
					name:                "myprefix-worker-02320933",
					templateName:        "worker-6656f66e",
					bootstrapSecretName: "worker-02320933",
				})

				// zoneb
				assertMachineDeploymentAndItsDeps(f, mdParams{
					name:                "myprefix-worker-6bdb5b0d",
					templateName:        "worker-d30762c9",
					bootstrapSecretName: "worker-6bdb5b0d",
				})

				vcdTemplateWithCatalog := f.KubernetesResource("VCDMachineTemplate", "d8-cloud-instance-manager", "worker-big-c10b569f")
				Expect(vcdTemplateWithCatalog.Exists()).To(BeFalse())
			})
		})

		Context("DVP", func() {
			const nodeManagerDVP = `
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
  nodeControllerWebhookCert:
    ca: string
    key: string
    crt: string
  instancePrefix: myprefix
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  kubernetesCA: myclusterca
  cloudProvider:
    type: dvp
    machineClassKind: ""
    capiClusterKind: "DeckhouseCluster"
    capiClusterAPIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1"
    capiClusterName: "dvp"
    capiMachineTemplateKind: "DeckhouseMachineTemplate"
    capiMachineTemplateAPIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1"
    dvp: {}
  nodeGroups:
    - cloudInstances:
        classReference:
          kind: DVPInstanceClass
          name: worker
        maxPerZone: 5
        minPerZone: 4
        zones:
          - default
      cri:
        type: Containerd
      instanceClass:
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
          memory:
            size: 8Gi
      kubelet:
        containerLogMaxFiles: 4
        containerLogMaxSize: 50Mi
        resourceReservation:
          mode: Auto
        topologyManager: {}
      kubernetesVersion: "1.32"
      manualRolloutID: ""
      name: worker
      nodeType: CloudEphemeral
      updateEpoch: "1746532947"
`
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", globalValues)
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerDVP)
				setBashibleAPIServerTLSValues(f)
				f.HelmRender()
			})
			It("Everything must render properly", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				type mdParams struct {
					name                string
					templateName        string
					bootstrapSecretName string
				}

				// MachineDeployment and DeckhouseMachineTemplate are created by
				// node-controller (capi.reconcileCloudMDsRendered), not helm. Only the
				// bootstrap Secret stays in helm.
				assertMachineDeploymentAndItsDeps := func(f *Config, d mdParams) {
					md := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", d.name)
					Expect(md.Exists()).To(BeFalse())

					// The bootstrap Secret no longer embeds the instance-class checksum:
					// its name is {ng}-{sha(clusterUUID+zone)}, independent of the template.
					secret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", d.bootstrapSecretName)
					Expect(secret.Exists()).To(BeTrue())

					dvpTemplate := f.KubernetesResource("DeckhouseMachineTemplate", "d8-cloud-instance-manager", d.templateName)
					Expect(dvpTemplate.Exists()).To(BeFalse())
				}

				registrySecret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "deckhouse-registry")
				Expect(registrySecret.Exists()).To(BeTrue())

				assertClusterResources(f, "dvp")

				assertMachineDeploymentAndItsDeps(f, mdParams{
					name:                "myprefix-worker-8ced91ee",
					templateName:        "worker-a6381073",
					bootstrapSecretName: "worker-8ced91ee",
				})
			})
		})
	})
})

// verifyClusterAutoscalerDeploymentArgs checks the cluster-autoscaler --nodes args
// against the expected MachineDeployment names. The MachineDeployments themselves are
// rendered by node-controller (capi.reconcileCloudMCMs), not helm, so the expected
// names are passed as literals rather than read from rendered MD objects.
func verifyClusterAutoscalerDeploymentArgs(deployment object_store.KubeObject, mdNames ...string) error {
	args := deployment.Field("spec.template.spec.containers.0.args").AsStringSlice()

	nodesArgs := make([]string, 0)
	for _, arg := range args {
		if !strings.HasPrefix(arg, "--nodes") {
			continue
		}

		nodesArgs = append(nodesArgs, strings.Split(arg, ".")[1])
	}

	expected := make([]string, len(mdNames))
	copy(expected, mdNames)

	sort.Strings(nodesArgs)
	sort.Strings(expected)
	equal := cmp.Equal(nodesArgs, expected)
	if !equal {
		return fmt.Errorf("cluster-autoscaler args %+v are not equal to a list of MachineDeployment names %+v", nodesArgs, expected)
	}

	return nil
}

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
modulesImages:
  registry: registry.deckhouse.io/deckhouse/ce
  registryDockercfg: cfg
  registryAddress: registry.deckhouse.io
  registryPath: /deckhouse/ce
  registryScheme: https
  tags:
    nodeManager:
      clusterAutoscaler: imagehash
      machineControllerManager: imagehash
    common:
      kubeRbacProxy: imagehash
      alpine: tagstring
    registrypackages:
      jq16: imagehash
discovery:
  d8SpecificNodeCountByRole:
    master: 3
  clusterUUID: f49dd1c3-a63a-4565-a06c-625e35587eab
  kubernetesVersion: 1.19.8
clusterConfiguration:
  clusterType: Hybrid
  packagesProxy:
    url: "http://aaa.bbb:80"
    username: "test"
    password: "test"
`

// Defaults from openapi/config-values.yaml.
const nodeManagerConfigValues = `
allowedBundles:
  - "ubuntu-lts"
  - "centos-7"
allowedKubernetesVersions:
  - "1.19"
  - "1.20"
  - "1.21"
  - "1.22"
mcmEmergencyBrake: false
`

const nodeManagerAWS = `
internal:
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
    kubernetesVersion: "1.19"
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
    kubernetesVersion: "1.19"
    cri:
      type: "Docker"
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

const nodeManagerGCP = `
internal:
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
    kubernetesVersion: "1.19"
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
    kubernetesVersion: "1.19"
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
    kubernetesVersion: "1.19"
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
    kubernetesVersion: "1.19"
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
    kubernetesVersion: "1.19"
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
    kubernetesVersion: "1.19"
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
  machineDeployments: {}
  instancePrefix: myprefix
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  kubernetesCA: myclusterca
  bootstrapTokens:
    worker: myworker
  nodeGroups:
  - name: worker
    nodeType: Static
    kubernetesVersion: "1.19"
    cri:
      type: "Containerd"
`

var _ = Describe("Module :: node-manager :: helm template ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
	})

	Context("AWS", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerAWS)
			setBashibleAPIServerTLSValues(f)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-instance-manager")
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

			Expect(namespace.Exists()).To(BeTrue())
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

			assertBashibleAPIServerTLS(f, nodeManagerNamespace)
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

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-instance-manager")
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

			Expect(namespace.Exists()).To(BeTrue())
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

			assertBashibleAPIServerTLS(f, nodeManagerNamespace)
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

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-instance-manager")
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

			Expect(namespace.Exists()).To(BeTrue())
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

			assertBashibleAPIServerTLS(f, nodeManagerNamespace)
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

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-instance-manager")
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

			Expect(verifyClusterAutoscalerDeploymentArgs(clusterAutoscalerDeploy, machineDeploymentA, machineDeploymentB)).To(Succeed())

			bashibleSecrets := map[string]object_store.KubeObject{}
			bashibleSecrets["bashible-bashbooster"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bashbooster")

			roles := map[string]object_store.KubeObject{}
			roles["bashible"] = f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible")
			roles["bashible-mcm-bootstrapped-nodes"] = f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible-mcm-bootstrapped-nodes")

			roleBindings := map[string]object_store.KubeObject{}
			roleBindings["bashible"] = f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible")
			roleBindings["bashible-mcm-bootstrapped-nodes"] = f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible-mcm-bootstrapped-nodes")

			Expect(namespace.Exists()).To(BeTrue())
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

			assertBashibleAPIServerTLS(f, nodeManagerNamespace)
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

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-instance-manager")
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

			Expect(namespace.Exists()).To(BeTrue())
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

			assertBashibleAPIServerTLS(f, nodeManagerNamespace)
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

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-instance-manager")
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

			Expect(namespace.Exists()).To(BeTrue())
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

			assertBashibleAPIServerTLS(f, nodeManagerNamespace)
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

package template_tests

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/deckhouse/deckhouse/testing/library/object_store"
	"github.com/google/go-cmp/cmp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const globalValues = `
enabledModules: ["vertical-pod-autoscaler-crd"]
modulesImages:
  registry: registry.flant.com
  registryDockercfg: cfg
  tags:
    nodeManager:
      clusterAutoscaler: imagehash
      machineControllerManager: imagehash
    common:
      kubeRbacProxy: imagehash
discovery:
  clusterMasterCount: "3"
  clusterUUID: f49dd1c3-a63a-4565-a06c-625e35587eab
  kubernetesVersion: 1.15.4
`

const nodeManagerAWS = `
internal:
  bashibleChecksumMigration: {}
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
        securityGroupIDs: ["mysecgroupid1", "mysecgroupid2"]
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
    nodeType: Cloud
    kubernetesVersion: "1.16"
    cloudInstances:
      classReference:
        kind: AWSInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 2
      zones:
      - zonea
      - zoneb
`

const nodeManagerGCP = `
internal:
  bashibleChecksumMigration: {}
  instancePrefix: myprefix
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  kubernetesCA: myclusterca
  cloudProvider:
    type: gcp
    machineClassKind: GCPMachineClass
    gcp:
      networkName: mynetwork
      subnetworkName: mysubnet
      region: myreg
      extraInstanceTags: [aaa,bbb] #optional
      sshKey: mysshkey
      serviceAccountKey: '{"my":"key"}'
      disableExternalIP: true
  nodeGroups:
  - name: worker
    instanceClass: # maximum filled
      flavorName: m1.large
      imageName: ubuntu-18-04-cloud-amd64
      machineType: mymachinetype
      preemptible: true #optional
      diskType: superdisk #optional
      diskSizeGb: 42 #optional
    nodeType: Cloud
    kubernetesVersion: "1.15"
    cloudInstances:
      classReference:
        kind: GCPInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 2
      zones:
      - zonea
      - zoneb
`

const nodeManagerOpenstack = `
internal:
  bashibleChecksumMigration: {}
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
      internalSubnet: "10.0.0.1/24"
      internalNetworkNames: [mynetwork, mynetwork2]
      externalNetworkNames: [shared]
  nodeGroups:
  - name: worker
    instanceClass:
      flavorName: m1.large
      imageName: ubuntu-18-04-cloud-amd64
      mainNetwork: shared
      additionalNetworks:
      - mynetwork
      - mynetwork2
      securityGroups:
      - ic-groupa
      - ic-groupb
    nodeType: Cloud
    kubernetesVersion: "1.15"
    cloudInstances:
      classReference:
        kind: OpenStackInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 2
      zones:
      - zonea
      - zoneb
`

const nodeManagerVsphere = `
internal:
  bashibleChecksumMigration: {}
  instancePrefix: myprefix
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  kubernetesCA: myclusterca
  cloudProvider:
    type: vsphere
    machineClassKind: VsphereMachineClass
    vsphere:
      host: myhost.qqq
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
    nodeType: Cloud
    kubernetesVersion: "1.15"
    cloudInstances:
      classReference:
        kind: VsphereInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 2
      zones:
      - zonea
      - zoneb
`

const nodeManagerYandex = `
internal:
  bashibleChecksumMigration: {}
  instancePrefix: myprefix
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  kubernetesCA: myclusterca
  cloudProvider:
    type: yandex
    machineClassKind: YandexMachineClass
    yandex:
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
      labels: # optional
        my: label
    nodeType: Cloud
    kubernetesVersion: "1.15"
    cloudInstances:
      classReference:
        kind: YandexInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 2
      zones:
      - zonea
      - zoneb
`

const nodeManagerStatic = `
internal:
  bashibleChecksumMigration: {}
  instancePrefix: myprefix
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  kubernetesCA: myclusterca
  bootstrapTokens:
    worker: myworker
  nodeGroups:
  - name: worker
    nodeType: Static
    kubernetesVersion: "1.15"
`

var _ = Describe("Module :: node-manager :: helm template ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
	})

	Context("AWS", func() {
		Describe("With manual-rollout-id", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("nodeManager", nodeManagerAWS)
				f.ValuesSet("nodeManager.internal.nodeGroups.0.manualRolloutID", "test")
				f.HelmRender()
			})

			It("should render correctly", func() {
				machineDeployment := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
				// Important! If checksum changes, the MachineDeployments will re-deploy! All nodes in MD will reboot! If you're not sure, don't change it.
				Expect(machineDeployment.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("7b787b33650a0f9166b6eacfdaff5d7c1e0cc508d2831d392ca938e47b7460f6"))
			})
		})

		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager", nodeManagerAWS)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

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
			bashibleSecrets["bashible-bundle-centos-7-1.14"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-1.14")
			bashibleSecrets["bashible-bundle-centos-7-1.15"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-1.15")
			bashibleSecrets["bashible-bundle-centos-7-1.16"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-1.16")
			bashibleSecrets["bashible-bundle-centos-7-worker"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-worker")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-1.14"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-1.14")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-1.15"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-1.15")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-1.16"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-1.16")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-worker"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-worker")
			bashibleSecrets["bashible-worker-centos-7"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-worker-centos-7")
			bashibleSecrets["bashible-worker-ubuntu-18.04"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-worker-ubuntu-18.04")

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
			// Important! If checksum changes, the MachineDeployments will re-deploy! All nodes in MD will reboot! If you're not sure, don't change it.
			Expect(machineDeploymentA.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("21b7f37222f1cbad6c644c0aa4eef85aa309b874ec725dc0cdc087ca06fc6c19"))

			Expect(machineClassB.Exists()).To(BeTrue())
			Expect(machineClassSecretB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Exists()).To(BeTrue())
			// Important! If checksum changes, the MachineDeployments will re-deploy! All nodes in MD will reboot! If you're not sure, don't change it.
			Expect(machineDeploymentB.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("21b7f37222f1cbad6c644c0aa4eef85aa309b874ec725dc0cdc087ca06fc6c19"))

			Expect(bashibleSecrets["bashible-bashbooster"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-1.14"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-1.15"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-1.16"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-worker"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-1.14"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-1.15"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-1.16"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-worker"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-worker-centos-7"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-worker-ubuntu-18.04"].Exists()).To(BeTrue())

			Expect(roles["bashible"].Exists()).To(BeTrue())
			Expect(roles["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			Expect(roleBindings["bashible"].Exists()).To(BeTrue())
			Expect(roleBindings["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())
		})
	})

	Context("GCP", func() {
		Describe("With manual-rollout-id", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("nodeManager", nodeManagerGCP)
				f.ValuesSet("nodeManager.internal.nodeGroups.0.manualRolloutID", "test")
				f.HelmRender()
			})

			It("should render correctly", func() {
				machineDeployment := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
				// Important! If checksum changes, the MachineDeployments will re-deploy! All nodes in MD will reboot! If you're not sure, don't change it.
				Expect(machineDeployment.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("48aa95710a1ea40e5dc26d36a8a0b2d461a85e4fc47953e94a84cef64a4060ca"))
			})
		})

		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager", nodeManagerGCP)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

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
			bashibleSecrets["bashible-bundle-centos-7-1.14"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-1.14")
			bashibleSecrets["bashible-bundle-centos-7-1.15"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-1.15")
			bashibleSecrets["bashible-bundle-centos-7-1.16"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-1.16")
			bashibleSecrets["bashible-bundle-centos-7-worker"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-worker")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-1.14"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-1.14")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-1.15"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-1.15")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-1.16"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-1.16")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-worker"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-worker")
			bashibleSecrets["bashible-worker-centos-7"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-worker-centos-7")
			bashibleSecrets["bashible-worker-ubuntu-18.04"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-worker-ubuntu-18.04")

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

			// Important! If checksum changes, the MachineDeployments will re-deploy! All nodes in MD will reboot! If you're not sure, don't change it.
			Expect(machineDeploymentA.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("a9e6ed184c6eab25aa7e47d3d4c7e5647fee9fa5bc2d35eb0232eab45749d3ae"))

			Expect(machineClassB.Exists()).To(BeTrue())
			Expect(machineClassSecretB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Exists()).To(BeTrue())
			// Important! If checksum changes, the MachineDeployments will re-deploy! All nodes in MD will reboot! If you're not sure, don't change it.
			Expect(machineDeploymentB.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("a9e6ed184c6eab25aa7e47d3d4c7e5647fee9fa5bc2d35eb0232eab45749d3ae"))

			Expect(bashibleSecrets["bashible-bashbooster"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-1.14"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-1.15"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-1.16"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-worker"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-1.14"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-1.15"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-1.16"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-worker"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-worker-centos-7"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-worker-ubuntu-18.04"].Exists()).To(BeTrue())

			Expect(roles["bashible"].Exists()).To(BeTrue())
			Expect(roles["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			Expect(roleBindings["bashible"].Exists()).To(BeTrue())
			Expect(roleBindings["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())
		})
	})

	Context("Openstack", func() {
		Describe("With manual-rollout-id", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("nodeManager", nodeManagerOpenstack)
				f.ValuesSet("nodeManager.internal.nodeGroups.0.manualRolloutID", "test")
				f.HelmRender()
			})

			It("should render correctly", func() {
				machineDeployment := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
				// Important! If checksum changes, the MachineDeployments will re-deploy! All nodes in MD will reboot! If you're not sure, don't change it.
				Expect(machineDeployment.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("9b3c57c4b09792ff626866698884907c89dc3f8d6571b81a5c226e1cae35057d"))
			})
		})

		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager", nodeManagerOpenstack)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

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

			Expect(verifyClusterAutoscalerDeploymentArgs(clusterAutoscalerDeploy, machineDeploymentA, machineDeploymentB)).To(Succeed())

			bashibleSecrets := map[string]object_store.KubeObject{}
			bashibleSecrets["bashible-bashbooster"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bashbooster")
			bashibleSecrets["bashible-bundle-centos-7-1.14"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-1.14")
			bashibleSecrets["bashible-bundle-centos-7-1.15"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-1.15")
			bashibleSecrets["bashible-bundle-centos-7-1.16"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-1.16")
			bashibleSecrets["bashible-bundle-centos-7-worker"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-worker")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-1.14"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-1.14")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-1.15"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-1.15")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-1.16"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-1.16")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-worker"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-worker")
			bashibleSecrets["bashible-worker-centos-7"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-worker-centos-7")
			bashibleSecrets["bashible-worker-ubuntu-18.04"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-worker-ubuntu-18.04")

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
[groupa, groupb, ic-groupa, ic-groupb]
`))

			Expect(machineClassSecretA.Exists()).To(BeTrue())
			Expect(machineDeploymentA.Exists()).To(BeTrue())
			// Important! If checksum changes, the MachineDeployments will re-deploy! All nodes in MD will reboot! If you're not sure, don't change it.
			Expect(machineDeploymentA.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("d4829faf5ac0babecf268f0c74a512d3d00f48533af62f337e41bd7ccd12ce23"))

			Expect(machineClassB.Exists()).To(BeTrue())
			Expect(machineClassSecretB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Exists()).To(BeTrue())
			// Important! If checksum changes, the MachineDeployments will re-deploy! All nodes in MD will reboot! If you're not sure, don't change it.
			Expect(machineDeploymentB.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("d4829faf5ac0babecf268f0c74a512d3d00f48533af62f337e41bd7ccd12ce23"))

			Expect(bashibleSecrets["bashible-bashbooster"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-1.14"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-1.15"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-1.16"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-worker"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-1.14"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-1.15"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-1.16"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-worker"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-worker-centos-7"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-worker-ubuntu-18.04"].Exists()).To(BeTrue())

			Expect(roles["bashible"].Exists()).To(BeTrue())
			Expect(roles["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			Expect(roleBindings["bashible"].Exists()).To(BeTrue())
			Expect(roleBindings["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())
		})
	})

	Context("Vsphere", func() {
		Describe("With manual-rollout-id", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("nodeManager", nodeManagerVsphere)
				f.ValuesSet("nodeManager.internal.nodeGroups.0.manualRolloutID", "test")
				f.HelmRender()
			})

			It("should render correctly", func() {
				machineDeployment := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
				// Important! If checksum changes, the MachineDeployments will re-deploy! All nodes in MD will reboot! If you're not sure, don't change it.
				Expect(machineDeployment.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("72d791f90322f67ea0f42d80fbae93c5e5dacf3b26e2dc0cf03c8bad0a0bb072"))
			})
		})

		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager", nodeManagerVsphere)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

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
			bashibleSecrets["bashible-bundle-centos-7-1.14"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-1.14")
			bashibleSecrets["bashible-bundle-centos-7-1.15"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-1.15")
			bashibleSecrets["bashible-bundle-centos-7-1.16"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-1.16")
			bashibleSecrets["bashible-bundle-centos-7-worker"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-worker")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-1.14"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-1.14")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-1.15"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-1.15")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-1.16"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-1.16")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-worker"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-worker")
			bashibleSecrets["bashible-worker-centos-7"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-worker-centos-7")
			bashibleSecrets["bashible-worker-ubuntu-18.04"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-worker-ubuntu-18.04")

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

			// Important! If checksum changes, the MachineDeployments will re-deploy! All nodes in MD will reboot! If you're not sure, don't change it.
			Expect(machineDeploymentA.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("e54154626facdf7ba3937af03fb11ac3e626cf1ebab8e36fb17c8320ed4ae906"))

			Expect(machineClassB.Exists()).To(BeTrue())
			Expect(machineClassSecretB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Exists()).To(BeTrue())
			// Important! If checksum changes, the MachineDeployments will re-deploy! All nodes in MD will reboot! If you're not sure, don't change it.
			Expect(machineDeploymentB.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("e54154626facdf7ba3937af03fb11ac3e626cf1ebab8e36fb17c8320ed4ae906"))

			Expect(bashibleSecrets["bashible-bashbooster"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-1.14"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-1.15"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-1.16"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-worker"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-1.14"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-1.15"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-1.16"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-worker"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-worker-centos-7"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-worker-ubuntu-18.04"].Exists()).To(BeTrue())

			Expect(roles["bashible"].Exists()).To(BeTrue())
			Expect(roles["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			Expect(roleBindings["bashible"].Exists()).To(BeTrue())
			Expect(roleBindings["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())
		})
	})

	Context("Yandex", func() {
		Describe("With manual-rollout-id", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("nodeManager", nodeManagerYandex)
				f.ValuesSet("nodeManager.internal.nodeGroups.0.manualRolloutID", "test")
				f.HelmRender()
			})

			It("should render correctly", func() {
				machineDeployment := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
				// Important! If checksum changes, the MachineDeployments will re-deploy! All nodes in MD will reboot! If you're not sure, don't change it.
				Expect(machineDeployment.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("f6d76633ca65e16841d11fbdb5838633f1a9dca126d503d479ad38ba1d67efdb"))
			})
		})

		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager", nodeManagerYandex)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

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
			bashibleSecrets["bashible-bundle-centos-7-1.14"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-1.14")
			bashibleSecrets["bashible-bundle-centos-7-1.15"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-1.15")
			bashibleSecrets["bashible-bundle-centos-7-1.16"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-1.16")
			bashibleSecrets["bashible-bundle-centos-7-worker"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-worker")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-1.14"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-1.14")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-1.15"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-1.15")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-1.16"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-1.16")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-worker"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-worker")
			bashibleSecrets["bashible-worker-centos-7"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-worker-centos-7")
			bashibleSecrets["bashible-worker-ubuntu-18.04"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-worker-ubuntu-18.04")

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

			// Important! If checksum changes, the MachineDeployments will re-deploy! All nodes in MD will reboot! If you're not sure, don't change it.
			Expect(machineDeploymentA.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("74795e5fe09827e6c1b0a44968e667aa93a9c1ee34e9c6f0bb6994dbdb2bb2fd"))

			Expect(machineClassB.Exists()).To(BeTrue())
			Expect(machineClassSecretB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Exists()).To(BeTrue())
			// Important! If checksum changes, the MachineDeployments will re-deploy! All nodes in MD will reboot! If you're not sure, don't change it.
			Expect(machineDeploymentB.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("74795e5fe09827e6c1b0a44968e667aa93a9c1ee34e9c6f0bb6994dbdb2bb2fd"))

			Expect(bashibleSecrets["bashible-bashbooster"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-1.14"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-1.15"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-1.16"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-worker"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-1.14"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-1.15"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-1.16"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-worker"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-worker-centos-7"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-worker-ubuntu-18.04"].Exists()).To(BeTrue())

			Expect(roles["bashible"].Exists()).To(BeTrue())
			Expect(roles["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			Expect(roleBindings["bashible"].Exists()).To(BeTrue())
			Expect(roleBindings["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())
		})
	})

	Context("Static", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager", nodeManagerStatic)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

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
			bashibleSecrets["bashible-bundle-centos-7-1.14"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-1.14")
			bashibleSecrets["bashible-bundle-centos-7-1.15"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-1.15")
			bashibleSecrets["bashible-bundle-centos-7-1.16"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-1.16")
			bashibleSecrets["bashible-bundle-centos-7-worker"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-worker")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-1.14"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-1.14")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-1.15"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-1.15")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-1.16"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-1.16")
			bashibleSecrets["bashible-bundle-ubuntu-18.04-worker"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-worker")
			bashibleSecrets["bashible-worker-centos-7"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-worker-centos-7")
			bashibleSecrets["bashible-worker-ubuntu-18.04"] = f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-worker-ubuntu-18.04")

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
			Expect(bashibleSecrets["bashible-bundle-centos-7-1.14"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-1.15"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-1.16"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-centos-7-worker"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-1.14"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-1.15"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-1.16"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-bundle-ubuntu-18.04-worker"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-worker-centos-7"].Exists()).To(BeTrue())
			Expect(bashibleSecrets["bashible-worker-ubuntu-18.04"].Exists()).To(BeTrue())

			Expect(bootstrapSecrets["manual-bootstrap-for-worker"].Exists()).To(BeTrue())

			Expect(roles["bashible"].Exists()).To(BeTrue())
			Expect(roles["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())

			Expect(roleBindings["bashible"].Exists()).To(BeTrue())
			Expect(roleBindings["bashible-mcm-bootstrapped-nodes"].Exists()).To(BeTrue())
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

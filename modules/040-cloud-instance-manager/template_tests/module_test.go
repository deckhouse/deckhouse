package template_tests

import (
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
enabledModules: ["vertical-pod-autoscaler-crd"]
modulesImages:
  registry: registry.flant.com
  registryDockercfg: cfg
  tags:
    cloudInstanceManager:
      clusterAutoscaler: imagehash
      machineControllerManager: imagehash
    common:
      kubeRbacProxy: imagehash
discovery:
  clusterMasterCount: "3"
  clusterUUID: f49dd1c3-a63a-4565-a06c-625e35587eab
  clusterVersion: 1.15.4
`

const cloudInstanceManagerAWS = `
instancePrefix: myprefix
internal:
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  clusterCA: myclusterca
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
    bashible:
      dynamicOptions: {}
      options:
        kubernetesVersion: 1.16.6
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

const cloudInstanceManagerGCP = `
instancePrefix: myprefix
internal:
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  clusterCA: myclusterca
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
    bashible:
      dynamicOptions: {}
      options:
        kubernetesVersion: 1.15.4
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

const cloudInstanceManagerOpenstack = `
instancePrefix: myprefix
internal:
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  clusterCA: myclusterca
  cloudProvider:
    type: openstack
    machineClassKind: OpenStackMachineClass
    openstack:
      addPodSubnetToPortWhitelist: true
      authURL: https://mycloud.qqq/3/
      caCert: mycacert
      domainName: Default
      internalNetworkName: mynetwork
      networkName: shared
      password: pPaAsS
      region: myreg
      securityGroups: [groupa, groupb]
      sshKeyPairName: mysshkey
      internalSubnet: "10.0.0.1/24"
      tenantName: mytname
      username: myuname
  nodeGroups:
  - name: worker
    instanceClass:
      flavorName: m1.large
      imageName: ubuntu-18-04-cloud-amd64
    nodeType: Cloud
    bashible:
      dynamicOptions: {}
      options:
        kubernetesVersion: 1.15.4
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

const cloudInstanceManagerVsphere = `
instancePrefix: myprefix
internal:
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  clusterCA: myclusterca
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
    bashible:
      dynamicOptions: {}
      options:
        kubernetesVersion: 1.15.4
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

const cloudInstanceManagerYandex = `
instancePrefix: myprefix
internal:
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  clusterCA: myclusterca
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
    bashible:
      dynamicOptions: {}
      options:
        kubernetesVersion: 1.15.4
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

const cloudInstanceManagerStatic = `
instancePrefix: myprefix
internal:
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
  clusterCA: myclusterca
  nodeGroups:
  - name: worker
    nodeType: Static
    bashible:
      dynamicOptions: {}
      options:
        kubernetesVersion: 1.15.4
`

var _ = Describe("Module :: cloud-instance-manager :: helm template ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
	})

	Context("AWS", func() {
		Describe("With manual-rollout-id", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("cloudInstanceManager", cloudInstanceManagerAWS)
				f.ValuesSet("cloudInstanceManager.internal.nodeGroups.0.manual-rollout-id", "test")
				f.HelmRender()
			})

			It("should render correctly", func() {
				machineDeployment := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
				Expect(machineDeployment.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("ceedac6e4f575da505fda1ec0abb11a60074e803537d4a162bad0200ece1a4b2"))
			})
		})

		BeforeEach(func() {
			f.ValuesSetFromYaml("cloudInstanceManager", cloudInstanceManagerAWS)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-instance-manager")
			registrySecret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "deckhouse-registry")

			userAuthzClusterRoleUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-instance-manager:user")
			userAuthzClusterRoleClusterEditor := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-instance-manager:cluster-editor")
			userAuthzClusterRoleClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-instance-manager:cluster-admin")

			mcmDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-instance-manager:machine-controller-manager")
			mcmClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-instance-manager:machine-controller-manager")

			clusterAutoscalerDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-instance-manager:cluster-autoscaler")
			clusterAutoscalerClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-instance-manager:cluster-autoscaler")

			machineClassA := f.KubernetesResource("AWSMachineClass", "d8-cloud-instance-manager", "worker-02320933")
			machineClassSecretA := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-02320933")
			machineDeploymentA := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
			machineClassB := f.KubernetesResource("AWSMachineClass", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineClassSecretB := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineDeploymentB := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-6bdb5b0d")

			bashibleRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible")
			bashibleRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible")

			bashibleBundleCentos := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7")
			bashibleBundleCentosWorker := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-worker")
			bashibleBundleUbuntu := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04")
			bashibleBundleUbuntuWorker := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-worker")

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
			Expect(machineDeploymentA.Field("spec.template.metadata.annotations.checksum/bashible-bundles-options").String()).To(Equal("d801592ae7c43d3b0fba96a805c8d9f7fd006b9726daf97ba7f7abc399a56b09"))
			Expect(machineDeploymentA.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("21b7f37222f1cbad6c644c0aa4eef85aa309b874ec725dc0cdc087ca06fc6c19"))

			Expect(machineClassB.Exists()).To(BeTrue())
			Expect(machineClassSecretB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Field("spec.template.metadata.annotations.checksum/bashible-bundles-options").String()).To(Equal("d801592ae7c43d3b0fba96a805c8d9f7fd006b9726daf97ba7f7abc399a56b09"))
			Expect(machineDeploymentB.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("21b7f37222f1cbad6c644c0aa4eef85aa309b874ec725dc0cdc087ca06fc6c19"))

			Expect(bashibleRole.Exists()).To(BeTrue())
			Expect(bashibleRoleBinding.Exists()).To(BeTrue())

			Expect(bashibleBundleCentos.Exists()).To(BeTrue())
			Expect(bashibleBundleCentosWorker.Exists()).To(BeTrue())
			Expect(bashibleBundleUbuntu.Exists()).To(BeTrue())
			Expect(bashibleBundleUbuntuWorker.Exists()).To(BeTrue())
		})
	})

	Context("GCP", func() {
		Describe("With manual-rollout-id", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("cloudInstanceManager", cloudInstanceManagerGCP)
				f.ValuesSet("cloudInstanceManager.internal.nodeGroups.0.manual-rollout-id", "test")
				f.HelmRender()
			})

			It("should render correctly", func() {
				machineDeployment := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
				Expect(machineDeployment.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("7d61bcc22c607b55c72744031271598bc8d350add1239912f66893d7e976179d"))
			})
		})

		BeforeEach(func() {
			f.ValuesSetFromYaml("cloudInstanceManager", cloudInstanceManagerGCP)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-instance-manager")
			registrySecret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "deckhouse-registry")

			userAuthzClusterRoleUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-instance-manager:user")
			userAuthzClusterRoleClusterEditor := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-instance-manager:cluster-editor")
			userAuthzClusterRoleClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-instance-manager:cluster-admin")

			mcmDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-instance-manager:machine-controller-manager")
			mcmClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-instance-manager:machine-controller-manager")

			clusterAutoscalerDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-instance-manager:cluster-autoscaler")
			clusterAutoscalerClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-instance-manager:cluster-autoscaler")

			machineClassA := f.KubernetesResource("GCPMachineClass", "d8-cloud-instance-manager", "worker-02320933")
			machineClassSecretA := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-02320933")
			machineDeploymentA := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
			machineClassB := f.KubernetesResource("GCPMachineClass", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineClassSecretB := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineDeploymentB := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-6bdb5b0d")

			bashibleRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible")
			bashibleRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible")

			bashibleBundleCentos := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7")
			bashibleBundleCentosWorker := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-worker")
			bashibleBundleUbuntu := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04")
			bashibleBundleUbuntuWorker := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-worker")

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

			Expect(machineDeploymentA.Field("spec.template.metadata.annotations.checksum/bashible-bundles-options").String()).To(Equal("d98bbed20612cd12e463d29a0d76837bb821a14810944aea2a2c19542e3d71be"))
			Expect(machineDeploymentA.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("a9e6ed184c6eab25aa7e47d3d4c7e5647fee9fa5bc2d35eb0232eab45749d3ae"))

			Expect(machineClassB.Exists()).To(BeTrue())
			Expect(machineClassSecretB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Field("spec.template.metadata.annotations.checksum/bashible-bundles-options").String()).To(Equal("d98bbed20612cd12e463d29a0d76837bb821a14810944aea2a2c19542e3d71be"))
			Expect(machineDeploymentB.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("a9e6ed184c6eab25aa7e47d3d4c7e5647fee9fa5bc2d35eb0232eab45749d3ae"))

			Expect(bashibleRole.Exists()).To(BeTrue())
			Expect(bashibleRoleBinding.Exists()).To(BeTrue())

			Expect(bashibleBundleCentos.Exists()).To(BeTrue())
			Expect(bashibleBundleCentosWorker.Exists()).To(BeTrue())
			Expect(bashibleBundleUbuntu.Exists()).To(BeTrue())
			Expect(bashibleBundleUbuntuWorker.Exists()).To(BeTrue())
		})
	})

	Context("Openstack", func() {
		Describe("With manual-rollout-id", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("cloudInstanceManager", cloudInstanceManagerOpenstack)
				f.ValuesSet("cloudInstanceManager.internal.nodeGroups.0.manual-rollout-id", "test")
				f.HelmRender()
			})

			It("should render correctly", func() {
				machineDeployment := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
				Expect(machineDeployment.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("64f2edcc9b5a394fe86fa0ac87b54a23051b9c89c5f36cc902ecbd8fd0caf95f"))
			})
		})

		BeforeEach(func() {
			f.ValuesSetFromYaml("cloudInstanceManager", cloudInstanceManagerOpenstack)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-instance-manager")
			registrySecret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "deckhouse-registry")

			userAuthzClusterRoleUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-instance-manager:user")
			userAuthzClusterRoleClusterEditor := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-instance-manager:cluster-editor")
			userAuthzClusterRoleClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-instance-manager:cluster-admin")

			mcmDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-instance-manager:machine-controller-manager")
			mcmClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-instance-manager:machine-controller-manager")

			clusterAutoscalerDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-instance-manager:cluster-autoscaler")
			clusterAutoscalerClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-instance-manager:cluster-autoscaler")

			machineClassA := f.KubernetesResource("OpenstackMachineClass", "d8-cloud-instance-manager", "worker-02320933")
			machineClassSecretA := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-02320933")
			machineDeploymentA := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
			machineClassB := f.KubernetesResource("OpenstackMachineClass", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineClassSecretB := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineDeploymentB := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-6bdb5b0d")

			bashibleRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible")
			bashibleRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible")

			bashibleBundleCentos := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7")
			bashibleBundleCentosWorker := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-worker")
			bashibleBundleUbuntu := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04")
			bashibleBundleUbuntuWorker := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-worker")

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

			Expect(machineDeploymentA.Field("spec.template.metadata.annotations.checksum/bashible-bundles-options").String()).To(Equal("d98bbed20612cd12e463d29a0d76837bb821a14810944aea2a2c19542e3d71be"))
			Expect(machineDeploymentA.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("bbfc6f35c09ffb41b71cbb1670803013cd247118a83169d6170bc5699176242f"))

			Expect(machineClassB.Exists()).To(BeTrue())
			Expect(machineClassSecretB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Field("spec.template.metadata.annotations.checksum/bashible-bundles-options").String()).To(Equal("d98bbed20612cd12e463d29a0d76837bb821a14810944aea2a2c19542e3d71be"))
			Expect(machineDeploymentB.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("bbfc6f35c09ffb41b71cbb1670803013cd247118a83169d6170bc5699176242f"))

			Expect(bashibleRole.Exists()).To(BeTrue())
			Expect(bashibleRoleBinding.Exists()).To(BeTrue())

			Expect(bashibleBundleCentos.Exists()).To(BeTrue())
			Expect(bashibleBundleCentosWorker.Exists()).To(BeTrue())
			Expect(bashibleBundleUbuntu.Exists()).To(BeTrue())
			Expect(bashibleBundleUbuntuWorker.Exists()).To(BeTrue())
		})
	})

	Context("Vsphere", func() {
		Describe("With manual-rollout-id", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("cloudInstanceManager", cloudInstanceManagerVsphere)
				f.ValuesSet("cloudInstanceManager.internal.nodeGroups.0.manual-rollout-id", "test")
				f.HelmRender()
			})

			It("should render correctly", func() {
				machineDeployment := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
				Expect(machineDeployment.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("7605f2939d00980d80f4f203c3644fb0ef3444087c35af275df51fa5904330e0"))
			})
		})

		BeforeEach(func() {
			f.ValuesSetFromYaml("cloudInstanceManager", cloudInstanceManagerVsphere)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-instance-manager")
			registrySecret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "deckhouse-registry")

			userAuthzClusterRoleUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-instance-manager:user")
			userAuthzClusterRoleClusterEditor := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-instance-manager:cluster-editor")
			userAuthzClusterRoleClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-instance-manager:cluster-admin")

			mcmDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-instance-manager:machine-controller-manager")
			mcmClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-instance-manager:machine-controller-manager")

			clusterAutoscalerDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-instance-manager:cluster-autoscaler")
			clusterAutoscalerClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-instance-manager:cluster-autoscaler")

			machineClassA := f.KubernetesResource("VsphereMachineClass", "d8-cloud-instance-manager", "worker-02320933")
			machineClassSecretA := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-02320933")
			machineDeploymentA := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
			machineClassB := f.KubernetesResource("VsphereMachineClass", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineClassSecretB := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineDeploymentB := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-6bdb5b0d")

			bashibleRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible")
			bashibleRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible")

			bashibleBundleCentos := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7")
			bashibleBundleCentosWorker := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-worker")
			bashibleBundleUbuntu := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04")
			bashibleBundleUbuntuWorker := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-worker")

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

			Expect(machineDeploymentA.Field("spec.template.metadata.annotations.checksum/bashible-bundles-options").String()).To(Equal("d98bbed20612cd12e463d29a0d76837bb821a14810944aea2a2c19542e3d71be"))
			Expect(machineDeploymentA.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("e54154626facdf7ba3937af03fb11ac3e626cf1ebab8e36fb17c8320ed4ae906"))

			Expect(machineClassB.Exists()).To(BeTrue())
			Expect(machineClassSecretB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Field("spec.template.metadata.annotations.checksum/bashible-bundles-options").String()).To(Equal("d98bbed20612cd12e463d29a0d76837bb821a14810944aea2a2c19542e3d71be"))
			Expect(machineDeploymentB.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("e54154626facdf7ba3937af03fb11ac3e626cf1ebab8e36fb17c8320ed4ae906"))

			Expect(bashibleRole.Exists()).To(BeTrue())
			Expect(bashibleRoleBinding.Exists()).To(BeTrue())

			Expect(bashibleBundleCentos.Exists()).To(BeTrue())
			Expect(bashibleBundleCentosWorker.Exists()).To(BeTrue())
			Expect(bashibleBundleUbuntu.Exists()).To(BeTrue())
			Expect(bashibleBundleUbuntuWorker.Exists()).To(BeTrue())
		})
	})

	Context("Yandex", func() {
		Describe("With manual-rollout-id", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("cloudInstanceManager", cloudInstanceManagerYandex)
				f.ValuesSet("cloudInstanceManager.internal.nodeGroups.0.manual-rollout-id", "test")
				f.HelmRender()
			})

			It("should render correctly", func() {
				machineDeployment := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
				Expect(machineDeployment.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("a84156b8921dec1023ecbd099f3da1f4da31be01a0bd164f75a86a3bd450f04e"))
			})
		})

		BeforeEach(func() {
			f.ValuesSetFromYaml("cloudInstanceManager", cloudInstanceManagerYandex)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-instance-manager")
			registrySecret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "deckhouse-registry")

			userAuthzClusterRoleUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-instance-manager:user")
			userAuthzClusterRoleClusterEditor := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-instance-manager:cluster-editor")
			userAuthzClusterRoleClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-instance-manager:cluster-admin")

			mcmDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-instance-manager:machine-controller-manager")
			mcmClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-instance-manager:machine-controller-manager")

			clusterAutoscalerDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-instance-manager:cluster-autoscaler")
			clusterAutoscalerClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-instance-manager:cluster-autoscaler")

			machineClassA := f.KubernetesResource("YandexMachineClass", "d8-cloud-instance-manager", "worker-02320933")
			machineClassSecretA := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-02320933")
			machineDeploymentA := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
			machineClassB := f.KubernetesResource("YandexMachineClass", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineClassSecretB := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineDeploymentB := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-6bdb5b0d")

			bashibleRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible")
			bashibleRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible")

			bashibleBundleCentos := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7")
			bashibleBundleCentosWorker := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-worker")
			bashibleBundleUbuntu := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04")
			bashibleBundleUbuntuWorker := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-worker")

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

			Expect(machineDeploymentA.Field("spec.template.metadata.annotations.checksum/bashible-bundles-options").String()).To(Equal("d98bbed20612cd12e463d29a0d76837bb821a14810944aea2a2c19542e3d71be"))
			Expect(machineDeploymentA.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("74795e5fe09827e6c1b0a44968e667aa93a9c1ee34e9c6f0bb6994dbdb2bb2fd"))

			Expect(machineClassB.Exists()).To(BeTrue())
			Expect(machineClassSecretB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Exists()).To(BeTrue())
			Expect(machineDeploymentB.Field("spec.template.metadata.annotations.checksum/bashible-bundles-options").String()).To(Equal("d98bbed20612cd12e463d29a0d76837bb821a14810944aea2a2c19542e3d71be"))
			Expect(machineDeploymentB.Field("spec.template.metadata.annotations.checksum/machine-class").String()).To(Equal("74795e5fe09827e6c1b0a44968e667aa93a9c1ee34e9c6f0bb6994dbdb2bb2fd"))

			Expect(bashibleRole.Exists()).To(BeTrue())
			Expect(bashibleRoleBinding.Exists()).To(BeTrue())

			Expect(bashibleBundleCentos.Exists()).To(BeTrue())
			Expect(bashibleBundleCentosWorker.Exists()).To(BeTrue())
			Expect(bashibleBundleUbuntu.Exists()).To(BeTrue())
			Expect(bashibleBundleUbuntuWorker.Exists()).To(BeTrue())
		})
	})

	Context("Static", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("cloudInstanceManager", cloudInstanceManagerStatic)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-instance-manager")
			registrySecret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "deckhouse-registry")

			userAuthzClusterRoleUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-instance-manager:user")
			userAuthzClusterRoleClusterEditor := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-instance-manager:cluster-editor")
			userAuthzClusterRoleClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-instance-manager:cluster-admin")

			mcmDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "machine-controller-manager")
			mcmClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-instance-manager:machine-controller-manager")
			mcmClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-instance-manager:machine-controller-manager")

			clusterAutoscalerDeploy := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerServiceAccount := f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "cluster-autoscaler")
			clusterAutoscalerClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-instance-manager:cluster-autoscaler")
			clusterAutoscalerClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-instance-manager:cluster-autoscaler")

			machineClassA := f.KubernetesResource("OpenstackMachineClass", "d8-cloud-instance-manager", "worker-02320933")
			machineClassSecretA := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-02320933")
			machineDeploymentA := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-02320933")
			machineClassB := f.KubernetesResource("OpenstackMachineClass", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineClassSecretB := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "worker-6bdb5b0d")
			machineDeploymentB := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "myprefix-worker-6bdb5b0d")

			bashibleRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible")
			bashibleRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible")

			bashibleBundleCentos := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7")
			bashibleBundleCentosWorker := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-centos-7-worker")
			bashibleBundleUbuntu := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04")
			bashibleBundleUbuntuWorker := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bundle-ubuntu-18.04-worker")

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

			Expect(bashibleRole.Exists()).To(BeTrue())
			Expect(bashibleRoleBinding.Exists()).To(BeTrue())

			Expect(bashibleBundleCentos.Exists()).To(BeTrue())
			Expect(bashibleBundleCentosWorker.Exists()).To(BeTrue())
			Expect(bashibleBundleUbuntu.Exists()).To(BeTrue())
			Expect(bashibleBundleUbuntuWorker.Exists()).To(BeTrue())
		})
	})
})

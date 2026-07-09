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
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const providerID = "vsphere"
const nameLabelKey = "cloud-provider\\.deckhouse\\.io/name"
const registrationLabelKey = "cloud-provider\\.deckhouse\\.io/registration"
const ephemeralNodesTemplatesLabelKey = "cloud-provider\\.deckhouse\\.io/ephemeral-nodes-templates"
const bashibleLabelKey = "cloud-provider\\.deckhouse\\.io/bashible"

const globalValues = `
  clusterIsBootstrapped: true
  enabledModules: ["vertical-pod-autoscaler", "vertical-pod-autoscaler-crd", "cloud-provider-vsphere"]
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

const hybridGlobalValues = `
  enabledModules: ["vertical-pod-autoscaler"]
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
      providerDiscoveryData:
        resourcePoolPath: kubernetes-dev
        zones: ["aaa", "bbb"]
        instances:
          mainNetwork: k8s-msk
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
      providerDiscoveryData:
        zones: ["aaa", "bbb"]
        datacenter: X1
        instances:
          mainNetwork: k8s-msk
        resourcePoolPath: kubernetes-dev
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
      providerDiscoveryData:
        zones: ["aaa", "bbb"]
        datacenter: X1
        resourcePoolPath: kubernetes-dev
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
`

const moduleValuesHybrid = `
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
        zones: ["zone-a", "zone-b"]
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

const caBundlePEM = `-----BEGIN CERTIFICATE-----
dGVzdC1jYS1idW5kbGU=
-----END CERTIFICATE-----
`

const nsxtCABundlePEM = `-----BEGIN CERTIFICATE-----
dGVzdC1uc3h0LWNhLWJ1bmRsZQ==
-----END CERTIFICATE-----
`

// Provider uses caBundle instead of insecure (mirrors moduleValuesA otherwise).
const moduleValuesProviderCABundle = `
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
          caBundle: |
            -----BEGIN CERTIFICATE-----
            dGVzdC1jYS1idW5kbGU=
            -----END CERTIFICATE-----
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

// NSX-T uses caBundle instead of insecureFlag; provider stays on insecure.
const moduleValuesNsxtCABundle = `
    internal:
      storageClasses:
      - name: mydsname1
        datastoreType: Datastore
        datastoreURL: ds:///vmfs/volumes/hash1/
        path: /my/ds/path/mydsname1
        zones: ["zonea", "zoneb"]
      compatibilityFlag: ""
      providerDiscoveryData:
        zones: ["aaa", "bbb"]
        datacenter: X1
        resourcePoolPath: kubernetes-dev
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
          caBundle: |
            -----BEGIN CERTIFICATE-----
            dGVzdC1uc3h0LWNhLWJ1bmRsZQ==
            -----END CERTIFICATE-----
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
- key: ToBeDeletedTaint
  operator: Exists
- effect: NoSchedule
  key: node.deckhouse.io/csi-not-bootstrapped
  operator: Exists
- key: node.kubernetes.io/not-ready
- key: node.kubernetes.io/out-of-disk
- key: node.kubernetes.io/memory-pressure
- key: node.kubernetes.io/disk-pressure
- key: node.kubernetes.io/pid-pressure
- key: node.kubernetes.io/unreachable
- key: node.kubernetes.io/network-unavailable`

const moduleNamespace = "d8-cloud-provider-vsphere"

// vsphereModulesImages returns module images with the vsphere digests explicitly
// populated for Kubernetes 1.32. Some earlier specs mutate the shared
// library.DefaultImagesDigests map in place, so we can not rely on GetModulesImages()
// keeping the 1.32 image keys intact.
func vsphereModulesImages() map[string]interface{} {
	images := GetModulesImages()
	if images["digests"] == nil {
		images["digests"] = make(map[string]interface{})
	}
	digests := images["digests"].(map[string]interface{})
	digests["cloudProviderVsphere"] = map[string]interface{}{
		"cloudControllerManager132": "sha256:ccm132digest",
		"cloudDataDiscoverer":       "sha256:cdddigest",
		"vsphereCsiPlugin132":       "sha256:csiplugin132digest",
		"vsphereCsiPluginLegacy":    "sha256:csipluginlegacydigest",
		"terraformManager":          "sha256:terraformdigest",
	}
	return images
}

var _ = Describe("Module :: cloud-provider-vsphere :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Vsphere", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.32", "1.32"))
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderVsphere", moduleValuesA)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", moduleNamespace)
			registrySecret := f.KubernetesResource("Secret", moduleNamespace, "deckhouse-registry")

			csiControllerPluginSS := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			csiDriver := f.KubernetesGlobalResource("CSIDriver", "csi.vsphere.vmware.com")
			csiNodePluginDS := f.KubernetesResource("DaemonSet", moduleNamespace, "csi-node")
			csiSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "csi")
			csiProvisionerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-vsphere:csi:controller:external-provisioner")
			csiProvisionerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-vsphere:csi:controller:external-provisioner")
			csiAttacherCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-vsphere:csi:controller:external-attacher")
			csiAttacherCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-vsphere:csi:controller:external-attacher")
			csiResizerCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-vsphere:csi:controller:external-resizer")
			csiResizerCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-vsphere:csi:controller:external-resizer")

			ccmSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "cloud-controller-manager")
			ccmCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-vsphere:cloud-controller-manager")
			ccmCRB := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-vsphere:cloud-controller-manager")
			ccmVPA := f.KubernetesResource("VerticalPodAutoscaler", moduleNamespace, "cloud-controller-manager")
			ccmDeploy := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			ccmSecret := f.KubernetesResource("Secret", moduleNamespace, "cloud-controller-manager")

			userAuthzUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-vsphere:user")
			userAuthzClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-vsphere:cluster-admin")

			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(userAuthzUser.Exists()).To(BeTrue())
			Expect(userAuthzClusterAdmin.Exists()).To(BeTrue())
			Expect(userAuthzUser.Field("rules").String()).To(MatchYAML(`
- apiGroups:
  - deckhouse.io
  resources:
  - vsphereinstanceclasses
  verbs:
  - get
  - list
  - watch`))
			Expect(userAuthzClusterAdmin.Field("rules").String()).To(MatchYAML(`
- apiGroups:
  - deckhouse.io
  resources:
  - vsphereinstanceclasses
  verbs:
  - create
  - delete
  - deletecollection
  - patch
  - update`))

			// user story #1
			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())
			Expect(providerRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", registrationLabelKey)).String()).To(Equal(""))
			Expect(providerRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))

			providerSpecificRegistrationSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-node-manager-cloud-provider-%s", providerID))
			Expect(providerSpecificRegistrationSecret.Exists()).To(BeTrue())
			Expect(providerSpecificRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", registrationLabelKey)).String()).To(Equal(""))
			Expect(providerSpecificRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))

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
          "instances": {
            "mainNetwork": "k8s-msk/test_187"
          },
          "sshKey": "mysshkey1",
          "username": "myuname",
          "vmFolderPath": "dev/test",
          "zoneTagCategory": "myzonetagcat"
        }`

			actualProviderRegistrationData, err := base64.StdEncoding.DecodeString(providerRegistrationSecret.Field("data.vsphere").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(actualProviderRegistrationData)).To(MatchJSON(expectedProviderRegistrationJSON))

			actualProviderSpecificRegistrationData, err := base64.StdEncoding.DecodeString(providerSpecificRegistrationSecret.Field("data.vsphere").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(actualProviderSpecificRegistrationData)).To(MatchJSON(expectedProviderRegistrationJSON))

			providerSpecificMCMSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-cloud-provider-%s-mcm", providerID))
			Expect(providerSpecificMCMSecret.Exists()).To(BeTrue())
			Expect(providerSpecificMCMSecret.Field(fmt.Sprintf("metadata.labels.%s", ephemeralNodesTemplatesLabelKey)).String()).To(Equal("mcm"))
			Expect(providerSpecificMCMSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))
			providerSpecificMCMSecretData := providerSpecificMCMSecret.Field("data").Map()
			Expect(providerSpecificMCMSecretData).To(Not(BeEmpty()))
			Expect(len(providerSpecificMCMSecretData) >= 1).To(BeTrue())
			Expect(len(providerSpecificMCMSecretData["config-for-machine-controller-manager.yaml"].String()) > 0).To(BeTrue())

			providerSpecificBashibleStepsSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-cloud-provider-%s-bashible-steps", providerID))
			Expect(providerSpecificBashibleStepsSecret.Exists()).ToNot(BeTrue())

			providerSpecificBashibleBootstrapSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-cloud-provider-%s-bashible-bootstrap", providerID))
			Expect(providerSpecificBashibleBootstrapSecret.Exists()).To(BeTrue())
			Expect(providerSpecificBashibleBootstrapSecret.Field(fmt.Sprintf("metadata.labels.%s", bashibleLabelKey)).String()).To(Equal("bootstrap"))
			Expect(providerSpecificBashibleBootstrapSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))
			providerSpecificBashibleBootstrapSecretData := providerSpecificBashibleBootstrapSecret.Field("data").Map()
			Expect(len(providerSpecificBashibleBootstrapSecretData) >= 1).To(BeTrue())
			Expect(len(providerSpecificBashibleBootstrapSecretData["bootstrap-networks.sh.tpl"].String()) > 0).To(BeTrue())

			// user story #2
			Expect(csiDriver.Exists()).To(BeTrue())
			Expect(csiNodePluginDS.Exists()).To(BeTrue())
			Expect(csiNodePluginDS.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(csiSA.Exists()).To(BeTrue())
			Expect(csiControllerPluginSS.Exists()).To(BeTrue())
			Expect(csiControllerPluginSS.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
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

			Expect(cddDeployment.Exists()).To(BeTrue())
			Expect(cddDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(cddDeployment.Field("spec.template.spec.tolerations").String()).To(MatchYAML(tolerationsAnyNodeWithUninitialized))
		})
	})

	Context("Hybrid vSphere", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", fmt.Sprintf(hybridGlobalValues, "1.32", "1.32"))
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderVsphere", moduleValuesHybrid)
			f.HelmRender()
		})

		It("renders resources for hybrid clusters", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())
			Expect(providerRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", registrationLabelKey)).String()).To(Equal(""))
			Expect(providerRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))

			providerSpecificRegistrationSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-node-manager-cloud-provider-%s", providerID))
			Expect(providerSpecificRegistrationSecret.Exists()).To(BeTrue())
			Expect(providerSpecificRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", registrationLabelKey)).String()).To(Equal(""))
			Expect(providerSpecificRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))

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
          "instances": {
            "mainNetwork": "k8s-msk/test_187"
          },
          "sshKey": "mysshkey1",
          "username": "myuname",
          "vmFolderPath": "dev/test",
          "zoneTagCategory": "myzonetagcat"
        }`

			providerRegistrationData, err := base64.StdEncoding.DecodeString(providerRegistrationSecret.Field("data.vsphere").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(providerRegistrationData)).To(MatchJSON(expectedProviderRegistrationJSON))

			providerSpecificRegistrationData, err := base64.StdEncoding.DecodeString(providerSpecificRegistrationSecret.Field("data.vsphere").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(providerSpecificRegistrationData)).To(MatchJSON(expectedProviderRegistrationJSON))

			zonesRegistrationData, err := base64.StdEncoding.DecodeString(providerRegistrationSecret.Field("data.zones").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(zonesRegistrationData)).To(Equal(`["zone-a","zone-b"]`))

			cloudDataDiscovererSecret := f.KubernetesResource("Secret", moduleNamespace, "cloud-data-discoverer")
			Expect(cloudDataDiscovererSecret.Exists()).To(BeTrue())

			zonesDiscovererData, err := base64.StdEncoding.DecodeString(cloudDataDiscovererSecret.Field("data.zones").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(zonesDiscovererData)).To(Equal("zone-a,zone-b"))

			Expect(f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-cloud-provider-%s-mcm", providerID)).Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-cloud-provider-%s-bashible-bootstrap", providerID)).Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Deployment", moduleNamespace, "csi-controller").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("CSIDriver", "csi.vsphere.vmware.com").Exists()).To(BeTrue())
		})
	})

	Context("Vsphere", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.32", "1.32"))
			images := GetModulesImages()
			if images["digests"] == nil {
				images["digests"] = make(map[string]interface{})
			}
			digests := images["digests"].(map[string]interface{})
			digests["cloudProviderVsphere"] = map[string]interface{}{
				"cloudControllerManager131": "sha256:ccm131digest",
				"cloudDataDiscoverer":       "sha256:cdddigest",
				"vsphereCsiPlugin131":       "sha256:csiplugin131digest",
				"vsphereCsiPluginLegacy":    "sha256:csipluginlegacydigest",
				"terraformManager":          "sha256:terraformdigest",
			}
			f.ValuesSet("global.modulesImages", images)
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
          "instances": {
            "mainNetwork": "k8s-msk"
          },
          "sshKey": "mysshkey1",
          "username": "myuname",
          "vmFolderPath": "dev/test",
          "zoneTagCategory": "myzonetagcat"
        }`

			providerRegistrationData, err := base64.StdEncoding.DecodeString(providerRegistrationSecret.Field("data.vsphere").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(providerRegistrationData)).To(MatchJSON(expectedProviderRegistrationJSON))

			cloudConfig := f.KubernetesResource("Secret", moduleNamespace, "cloud-controller-manager")
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
				f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.32", "1.32"))
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("cloudProviderVsphere", moduleValuesA)
				f.ValuesSet("global.discovery.kubernetesVersion", "1.17.8")
				f.HelmRender()
			})

			It("CCM and CSI controller should not be present on unsupported Kubernetes versions", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				Expect(f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager").Exists()).To(BeFalse())
				Expect(f.KubernetesResource("Deployment", moduleNamespace, "csi-controller").Exists()).To(BeFalse())

			})
		})
	})

	Context("Vsphere with default StorageClass specified", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.32", "1.32"))
			images := GetModulesImages()
			if images["digests"] == nil {
				images["digests"] = make(map[string]interface{})
			}
			digests := images["digests"].(map[string]interface{})
			digests["cloudProviderVsphere"] = map[string]interface{}{
				"cloudControllerManager131": "sha256:ccm131digest",
				"cloudDataDiscoverer":       "sha256:cdddigest",
				"vsphereCsiPlugin131":       "sha256:csiplugin131digest",
				"vsphereCsiPluginLegacy":    "sha256:csipluginlegacydigest",
				"terraformManager":          "sha256:terraformdigest",
			}
			f.ValuesSet("global.modulesImages", images)
			f.ValuesSetFromYaml("cloudProviderVsphere", moduleValuesB)
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
			Expect(scMydsname2.Field("metadata.annotations").String()).To(MatchYAML(`
storageclass.deckhouse.io/volume-expansion-mode: offline
storageclass.kubernetes.io/is-default-class: "true"
`))
		})
	})

	Context("Vsphere with NSX-T specified", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.32", "1.32"))
			images := GetModulesImages()
			if images["digests"] == nil {
				images["digests"] = make(map[string]interface{})
			}
			digests := images["digests"].(map[string]interface{})
			digests["cloudProviderVsphere"] = map[string]interface{}{
				"cloudControllerManager131": "sha256:ccm131digest",
				"cloudDataDiscoverer":       "sha256:cdddigest",
				"vsphereCsiPlugin131":       "sha256:csiplugin131digest",
				"vsphereCsiPluginLegacy":    "sha256:csipluginlegacydigest",
				"terraformManager":          "sha256:terraformdigest",
			}
			f.ValuesSet("global.modulesImages", images)
			f.ValuesSetFromYaml("cloudProviderVsphere", moduleValuesC)
			f.HelmRender()
		})

		It("Everything must render properly with proper secret", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmSecret := f.KubernetesResource("Secret", moduleNamespace, "cloud-controller-manager")
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
			f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.32", "1.32"))
			images := GetModulesImages()
			if images["digests"] == nil {
				images["digests"] = make(map[string]interface{})
			}
			digests := images["digests"].(map[string]interface{})
			digests["cloudProviderVsphere"] = map[string]interface{}{
				"cloudControllerManager131": "sha256:ccm131digest",
				"cloudDataDiscoverer":       "sha256:cdddigest",
				"vsphereCsiPlugin131":       "sha256:csiplugin131digest",
				"vsphereCsiPluginLegacy":    "sha256:csipluginlegacydigest",
				"terraformManager":          "sha256:terraformdigest",
			}
			f.ValuesSet("global.modulesImages", images)
			f.ValuesSetFromYaml("cloudProviderVsphere", moduleValuesD)
			f.HelmRender()
		})

		It("Everything must render properly with proper secret", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmSecret := f.KubernetesResource("Secret", moduleNamespace, "cloud-controller-manager")
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

	Context("Vsphere with provider caBundle specified", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.32", "1.32"))
			f.ValuesSet("global.discovery.kubernetesVersion", "1.32.1")
			f.ValuesSet("global.modulesImages", vsphereModulesImages())
			f.ValuesSetFromYaml("cloudProviderVsphere", moduleValuesProviderCABundle)
			f.HelmRender()
		})

		It("Creates the vsphere-ca-certs secret with the provider CA", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			caSecret := f.KubernetesResource("Secret", moduleNamespace, "vsphere-ca-certs")
			Expect(caSecret.Exists()).To(BeTrue())
			Expect(caSecret.Field("type").String()).To(Equal("Opaque"))

			caCrt, err := base64.StdEncoding.DecodeString(caSecret.Field("data").Get("ca\\.crt").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(caCrt)).To(Equal(caBundlePEM))

			// The nsxt CA key must be absent when only the provider CA is set.
			Expect(caSecret.Field("data").Get("ca-nsxt\\.crt").Exists()).To(BeFalse())
		})

		It("Uses caFile and disables insecure in the CCM cloud-config", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmSecret := f.KubernetesResource("Secret", moduleNamespace, "cloud-controller-manager")
			Expect(ccmSecret.Exists()).To(BeTrue())

			cloudConfig, err := base64.StdEncoding.DecodeString(ccmSecret.Field("data.cloud-config").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(cloudConfig)).To(ContainSubstring(`caFile: "/etc/vsphere-certs/ca.crt"`))
			Expect(string(cloudConfig)).To(ContainSubstring("insecureFlag: false"))
		})

		It("Uses ca-file and disables insecure-flag in the CSI cloud-config", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			csiSecret := f.KubernetesResource("Secret", moduleNamespace, "csi-controller")
			Expect(csiSecret.Exists()).To(BeTrue())

			cloudConfig, err := base64.StdEncoding.DecodeString(csiSecret.Field("data.cloud-config").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(cloudConfig)).To(ContainSubstring(`ca-file = "/etc/vsphere-certs/ca.crt"`))
			Expect(string(cloudConfig)).To(ContainSubstring("insecure-flag = false"))
		})

		It("Mounts the CA secret into the CCM deployment", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmDeploy := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			Expect(ccmDeploy.Exists()).To(BeTrue())

			volumes := ccmDeploy.Field("spec.template.spec.volumes").String()
			Expect(volumes).To(ContainSubstring("vsphere-ca-certs"))

			podAnnotations := ccmDeploy.Field("spec.template.metadata.annotations").String()
			Expect(podAnnotations).To(ContainSubstring("checksum/ca"))
		})

		It("Mounts the CA into the CSI controller as a file, not a directory", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			csiDeploy := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			Expect(csiDeploy.Exists()).To(BeTrue())

			// The CSI driver reads ca-file = /etc/vsphere-certs/ca.crt. With a
			// subPath mount the mountPath must be the full file path, otherwise
			// /etc/vsphere-certs itself becomes a file and the driver fails with
			// "open /etc/vsphere-certs/ca.crt: not a directory".
			mounts := csiDeploy.Field("spec.template.spec.containers").String()
			Expect(mounts).To(ContainSubstring("/etc/vsphere-certs/ca.crt"))
			Expect(mounts).NotTo(MatchRegexp(`"mountPath":\s*"/etc/vsphere-certs"`))
		})

		It("Passes GOVMOMI_CA_BUNDLE to the cloud-data-discoverer and disables insecure", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cddDeploy := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")
			Expect(cddDeploy.Exists()).To(BeTrue())
			Expect(cddDeploy.Field("spec.template.spec.containers.0.env").String()).To(ContainSubstring("GOVMOMI_CA_BUNDLE"))

			cddSecret := f.KubernetesResource("Secret", moduleNamespace, "cloud-data-discoverer")
			Expect(cddSecret.Exists()).To(BeTrue())
			insecure, err := base64.StdEncoding.DecodeString(cddSecret.Field("data.insecure").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(insecure)).To(Equal("false"))
		})
	})

	Context("Vsphere with NSX-T caBundle specified", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.32", "1.32"))
			f.ValuesSet("global.discovery.kubernetesVersion", "1.32.1")
			f.ValuesSet("global.modulesImages", vsphereModulesImages())
			f.ValuesSetFromYaml("cloudProviderVsphere", moduleValuesNsxtCABundle)
			f.HelmRender()
		})

		It("Creates the vsphere-ca-certs secret with the nsxt CA", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			caSecret := f.KubernetesResource("Secret", moduleNamespace, "vsphere-ca-certs")
			Expect(caSecret.Exists()).To(BeTrue())

			caNsxt, err := base64.StdEncoding.DecodeString(caSecret.Field("data").Get("ca-nsxt\\.crt").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(caNsxt)).To(Equal(nsxtCABundlePEM))

			// Provider CA is not configured here, so its key must be absent.
			Expect(caSecret.Field("data").Get("ca\\.crt").Exists()).To(BeFalse())
		})

		It("Uses nsxt caFile and disables nsxt insecureFlag in the CCM cloud-config", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmSecret := f.KubernetesResource("Secret", moduleNamespace, "cloud-controller-manager")
			cloudConfig, err := base64.StdEncoding.DecodeString(ccmSecret.Field("data.cloud-config").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(cloudConfig)).To(ContainSubstring(`caFile: "/etc/vsphere-certs/ca-nsxt.crt"`))
		})
	})

	Context("Vsphere without any caBundle", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", fmt.Sprintf(globalValues, "1.32", "1.32"))
			f.ValuesSet("global.discovery.kubernetesVersion", "1.32.1")
			f.ValuesSet("global.modulesImages", vsphereModulesImages())
			f.ValuesSetFromYaml("cloudProviderVsphere", moduleValuesA)
			f.HelmRender()
		})

		It("Does not create the vsphere-ca-certs secret and keeps insecure", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("Secret", moduleNamespace, "vsphere-ca-certs").Exists()).To(BeFalse())

			ccmSecret := f.KubernetesResource("Secret", moduleNamespace, "cloud-controller-manager")
			cloudConfig, err := base64.StdEncoding.DecodeString(ccmSecret.Field("data.cloud-config").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(cloudConfig)).To(ContainSubstring("insecureFlag: true"))
			Expect(string(cloudConfig)).ToNot(ContainSubstring("caFile"))
		})
	})
})

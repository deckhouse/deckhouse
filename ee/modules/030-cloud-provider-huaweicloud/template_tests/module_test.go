/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template_tests

import (
	"encoding/base64"
	"fmt"
	"testing"

	. "github.com/deckhouse/deckhouse/testing/helm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const providerID = "huaweicloud"
const nameLabelKey = "cloud-provider\\.deckhouse\\.io/name"
const registrationLabelKey = "cloud-provider\\.deckhouse\\.io/registration"
const ephemeralNodesTemplatesLabelKey = "cloud-provider\\.deckhouse\\.io/ephemeral-nodes-templates"
const bashibleLabelKey = "cloud-provider\\.deckhouse\\.io/bashible"

// fake *-crd modules are required for backward compatibility with lib_helm library
// TODO: remove fake crd modules
const globalValues = `
  clusterIsBootstrapped: true
  enabledModules: ["vertical-pod-autoscaler", "vertical-pod-autoscaler-crd", "cloud-provider-huaweicloud"]
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: sandbox
      provider: Huaweicloud
    clusterDomain: cluster.local
    clusterType: Cloud
    defaultCRI: Containerd
    kind: ClusterConfiguration
    kubernetesVersion: "1.31"
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
    kubernetesVersion: 1.31.0
    clusterUUID: cluster
`

const moduleValuesA = `
internal:
  cniSecretData: "base64-encoded-string-or-placeholder"
  providerClusterConfiguration:
    apiVersion: deckhouse.io/v1
    kind: HuaweiCloudClusterConfiguration
    layout: Standard
    sshPublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCu..."
    zones:
      - eu-3a
    provider:
      cloud: huaweicloud.example.com
      region: eu-3
      accessKey: "YOUR_ACCESS_KEY"
      secretKey: "YOUR_SECRET_KEY"
      domainName: "example.com"
      insecure: false
    standard:
      internalNetworkCIDR: 192.168.200.0/24
      internalNetworkDNSServers:
        - 8.8.8.8
        - 8.8.4.4
      internalNetworkSecurity: true
      enableEIP: true
    masterNodeGroup:
      replicas: 3
      instanceClass:
        flavorName: s3.xlarge.2
        imageName: "debian-11-genericcloud-amd64-20220911-1135"
        rootDiskSize: 50
        etcdDiskSizeGb: 10
      volumeTypeMap:
        eu-3a: fast-eu-3a
        eu-3b: fast-eu-3b
      serverGroup:
        policy: AntiAffinity
    nodeGroups:
      - name: front
        replicas: 2
        instanceClass:
          flavorName: m1.large
          imageName: "debian-11-genericcloud-amd64-20220911-1135"
          rootDiskSize: 50
          mainNetwork: "aaaff8f9-26af-43e3-9c49-c4d083e59c61"
          additionalNetworks:
            - "11111111-1111-1111-1111-111111111111"
        zones:
          - eu-1a
          - eu-1b
        volumeTypeMap:
          eu-1a: fast-eu-1a
          eu-1b: fast-eu-1b
        nodeTemplate:
          labels:
            role: frontend
            environment: production
          annotations:
            note: "frontend nodes"
          taints:
            - effect: NoSchedule
              key: front-node
              value: "true"
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: HuaweiCloudDiscoveryData
    layout: Standard
    zones:
      - eu-3a
    instances:
      vpcIPv4SubnetId: "00000000-0000-0000-0000-000000000000"
    volumeTypes:
      - id: "11111111-1111-1111-1111-111111111111"
        name: "ssd"
        isPublic: true
  storageClasses:
    - name: cinder-ssd
      type: ssd
      allowVolumeExpansion: true
    - name: cinder-hdd
      type: hdd
      allowVolumeExpansion: false`

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

var _ = Describe("Module :: cloud-provider-huaweicloud :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("HuaweiCloud Suite A", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderHuaweicloud", moduleValuesA)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())
			Expect(providerRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", registrationLabelKey)).String()).To(Equal(""))
			Expect(providerRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))
			providerRegistrationSecretData := providerRegistrationSecret.Field("data").Map()
			Expect(providerRegistrationSecretData).To(Not(BeEmpty()))
			Expect(providerRegistrationSecretData["capiClusterName"].String()).To(Equal(base64.StdEncoding.EncodeToString([]byte(providerID))))
			Expect(providerRegistrationSecretData["sshPublicKey"].String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCu..."))))

			providerSpecificRegistrationSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-node-manager-cloud-provider-%s", providerID))
			Expect(providerSpecificRegistrationSecret.Exists()).To(BeTrue())
			Expect(providerSpecificRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", registrationLabelKey)).String()).To(Equal(""))
			Expect(providerSpecificRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))
			providerSpecificRegistrationSecretData := providerSpecificRegistrationSecret.Field("data").Map()
			Expect(providerSpecificRegistrationSecretData).To(Not(BeEmpty()))
			Expect(providerSpecificRegistrationSecretData["capiClusterName"].String()).To(Equal(base64.StdEncoding.EncodeToString([]byte(providerID))))
			Expect(providerSpecificRegistrationSecretData["sshPublicKey"].String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCu..."))))

			providerSpecificCAPISecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-cloud-provider-%s-capi", providerID))
			Expect(providerSpecificCAPISecret.Exists()).To(BeTrue())
			Expect(providerSpecificCAPISecret.Field(fmt.Sprintf("metadata.labels.%s", ephemeralNodesTemplatesLabelKey)).String()).To(Equal("capi"))
			Expect(providerSpecificCAPISecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))
			providerSpecificCAPISecretData := providerSpecificCAPISecret.Field("data").Map()
			Expect(providerSpecificCAPISecretData).To(Not(BeEmpty()))
			Expect(len(providerSpecificCAPISecretData) >= 1).To(BeTrue())
			Expect(len(providerSpecificCAPISecretData["cluster.yaml"].String()) > 0).To(BeTrue())

			providerSpecificBashibleStepsSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-cloud-provider-%s-bashible-steps", providerID))
			Expect(providerSpecificBashibleStepsSecret.Exists()).To(BeFalse())

			providerSpecificBashibleBootstrapSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-cloud-provider-%s-bashible-bootstrap", providerID))
			Expect(providerSpecificBashibleBootstrapSecret.Exists()).To(BeFalse())

			ccmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-huaweicloud", "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.spec.containers.0.name").String()).To(Equal("cloud-controller-manager"))
			Expect(ccmDeployment.Field("spec.template.spec.serviceAccountName").String()).To(Equal("cloud-controller-manager"))
			Expect(ccmDeployment.Field("spec.template.spec.hostNetwork").Bool()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("Default"))
			Expect(ccmDeployment.Field("spec.template.spec.containers.0.args").String()).To(MatchYAML(`
- --leader-elect=true
- --bind-address=127.0.0.1
- --secure-port=10471
- --cluster-name=sandbox
- --cluster-cidr=10.0.1.0/16
- --allocate-node-cidrs=true
- --configure-cloud-routes=true
- --cloud-config=/etc/cloud/cloud-config
- --cloud-provider=huaweicloud
- --tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,TLS_RSA_WITH_AES_128_CBC_SHA,TLS_RSA_WITH_AES_256_CBC_SHA,TLS_RSA_WITH_AES_128_GCM_SHA256,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA
- --v=4`))
			Expect(ccmDeployment.Field("spec.template.spec.containers.0.env").String()).To(MatchYAML(`
- name: HOST_IP
  valueFrom:
    fieldRef:
      fieldPath: status.hostIP
- name: HUAWEICLOUD_SDK_CREDENTIALS_FILE
  value: /etc/cloud/cloud-config`))
			Expect(ccmDeployment.Field("spec.template.spec.containers.0.volumeMounts").String()).To(MatchYAML(`
- name: ccm-controller-config-volume
  mountPath: /etc/cloud
  readOnly: true`))
			Expect(ccmDeployment.Field("spec.template.spec.volumes").String()).To(MatchYAML(`
- name: ccm-controller-config-volume
  secret:
    secretName: cloud-controller-manager`))
			Expect(ccmDeployment.Field("spec.template.metadata.annotations.checksum/config").String()).NotTo(BeEmpty())
			Expect(ccmDeployment.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").Exists()).To(BeFalse())

			ccmVPA := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-huaweicloud", "cloud-controller-manager")
			Expect(ccmVPA.Exists()).To(BeTrue())
			Expect(ccmVPA.Field("spec.updatePolicy.updateMode").String()).To(Equal("InPlaceOrRecreate"))

			ccmPDB := f.KubernetesResource("PodDisruptionBudget", "d8-cloud-provider-huaweicloud", "cloud-controller-manager")
			Expect(ccmPDB.Exists()).To(BeTrue())
			Expect(ccmPDB.Field("spec.maxUnavailable").String()).To(Equal("1"))
			Expect(ccmPDB.Field("metadata.annotations.checksum/config").String()).NotTo(BeEmpty())

			caphcDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-huaweicloud", "caphc-controller-manager")
			Expect(caphcDeployment.Exists()).To(BeTrue())
			Expect(caphcDeployment.Field("spec.template.spec.containers.0.name").String()).To(Equal("caphc-controller-manager"))
			Expect(caphcDeployment.Field("spec.template.spec.containers.0.args").String()).To(MatchYAML(`
- --leader-elect
- --metrics-bind-address=:8080
- --metrics-secure=false`))
			Expect(caphcDeployment.Field("spec.template.spec.containers.0.env").String()).To(MatchYAML(`
- name: HUAWEICLOUD_CLOUD
  valueFrom:
    secretKeyRef:
      name: huaweicloud-credentials
      key: cloud
- name: HUAWEICLOUD_REGION
  valueFrom:
    secretKeyRef:
      name: huaweicloud-credentials
      key: region
- name: HUAWEICLOUD_ACCESS_KEY
  valueFrom:
    secretKeyRef:
      name: huaweicloud-credentials
      key: access-key
- name: HUAWEICLOUD_SECRET_KEY
  valueFrom:
    secretKeyRef:
      name: huaweicloud-credentials
      key: secret-key
- name: HUAWEICLOUD_PROJECT_ID
  valueFrom:
    secretKeyRef:
      name: huaweicloud-credentials
      key: project-id`))
			Expect(caphcDeployment.Field("spec.template.spec.containers.0.livenessProbe.httpGet.path").String()).To(Equal("/healthz"))
			Expect(caphcDeployment.Field("spec.template.spec.containers.0.livenessProbe.httpGet.port").String()).To(Equal("8081"))
			Expect(caphcDeployment.Field("spec.template.spec.containers.0.readinessProbe.httpGet.path").String()).To(Equal("/readyz"))
			Expect(caphcDeployment.Field("spec.template.spec.containers.0.readinessProbe.httpGet.port").String()).To(Equal("8081"))
			Expect(caphcDeployment.Field("spec.template.spec.dnsPolicy").Exists()).To(BeFalse())

			caphcVPA := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-huaweicloud", "caphc-controller-manager")
			Expect(caphcVPA.Exists()).To(BeTrue())

			caphcPDB := f.KubernetesResource("PodDisruptionBudget", "d8-cloud-provider-huaweicloud", "caphc-controller-manager")
			Expect(caphcPDB.Exists()).To(BeTrue())

			csiControllerDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-huaweicloud", "csi-controller")
			Expect(csiControllerDeployment.Exists()).To(BeTrue())
			Expect(csiControllerDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))

			csiNodeDaemonSet := f.KubernetesResource("DaemonSet", "d8-cloud-provider-huaweicloud", "csi-node")
			Expect(csiNodeDaemonSet.Exists()).To(BeTrue())
			Expect(csiNodeDaemonSet.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))

			cddDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-huaweicloud", "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())
			Expect(cddDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(cddDeployment.Field("spec.template.spec.tolerations").String()).To(MatchYAML(tolerationsAnyNodeWithUninitialized))
			Expect(cddDeployment.Field("spec.template.spec.containers.0.name").String()).To(Equal("cloud-data-discoverer"))
			Expect(cddDeployment.Field("spec.template.spec.containers.0.args").String()).To(MatchYAML(`
- --discovery-period=1h
- --listen-address=127.0.0.1:8081`))
			Expect(cddDeployment.Field("spec.template.spec.containers.0.env").String()).To(MatchYAML(`
- name: HUAWEICLOUD_CLOUD
  valueFrom:
    secretKeyRef:
      name: huaweicloud-credentials
      key: cloud
- name: HUAWEICLOUD_REGION
  valueFrom:
    secretKeyRef:
      name: huaweicloud-credentials
      key: region
- name: HUAWEICLOUD_ACCESS_KEY
  valueFrom:
    secretKeyRef:
      name: huaweicloud-credentials
      key: access-key
- name: HUAWEICLOUD_SECRET_KEY
  valueFrom:
    secretKeyRef:
      name: huaweicloud-credentials
      key: secret-key
- name: HUAWEICLOUD_PROJECT_ID
  valueFrom:
    secretKeyRef:
      name: huaweicloud-credentials
      key: project-id`))
			Expect(cddDeployment.Field("spec.template.spec.containers.1.name").String()).To(Equal("kube-rbac-proxy"))
			Expect(cddDeployment.Field("spec.template.metadata.annotations.kubectl\\.kubernetes\\.io/default-exec-container").String()).To(Equal("cloud-data-discoverer"))
			Expect(cddDeployment.Field("spec.template.metadata.annotations.kubectl\\.kubernetes\\.io/default-logs-container").String()).To(Equal("cloud-data-discoverer"))
			Expect(cddDeployment.Field("spec.template.metadata.annotations.checksum/config").String()).NotTo(BeEmpty())

			cddVPA := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-huaweicloud", "cloud-data-discoverer")
			Expect(cddVPA.Exists()).To(BeTrue())
			Expect(cddVPA.Field("spec.updatePolicy.updateMode").String()).To(Equal("Initial"))

			cddPDB := f.KubernetesResource("PodDisruptionBudget", "d8-cloud-provider-huaweicloud", "cloud-data-discoverer")
			Expect(cddPDB.Exists()).To(BeTrue())

			cddPodMonitor := f.KubernetesResource("PodMonitor", "d8-monitoring", "cloud-data-discoverer-metrics")
			Expect(cddPodMonitor.Exists()).To(BeFalse())
		})
	})

	Context("HuaweiCloud :: VPA gate compatibility", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderHuaweicloud", moduleValuesA)
			f.ValuesSet("global.enabledModules", []string{
				"vertical-pod-autoscaler",
				"vertical-pod-autoscaler-crd",
				"cloud-provider-huaweicloud",
			})
			f.HelmRender()
		})

		It("must still render VPA objects when vertical-pod-autoscaler is enabled", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-huaweicloud", "cloud-controller-manager").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-huaweicloud", "caphc-controller-manager").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-huaweicloud", "cloud-data-discoverer").Exists()).To(BeTrue())
		})
	})

	Context("HuaweiCloud :: PodMonitor gate compatibility", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderHuaweicloud", moduleValuesA)
			f.ValuesSet("global.enabledModules", []string{
				"vertical-pod-autoscaler",
				"vertical-pod-autoscaler-crd",
				"operator-prometheus",
				"operator-prometheus-crd",
				"cloud-provider-huaweicloud",
			})
			f.ValuesSet("global.discovery.prometheusScrapeInterval", 30)
			f.HelmRender()
		})

		It("must render PodMonitor when operator-prometheus is enabled", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			pm := f.KubernetesResource("PodMonitor", "d8-monitoring", "cloud-data-discoverer-metrics")
			Expect(pm.Exists()).To(BeTrue())
			Expect(pm.Field("spec.namespaceSelector.matchNames").String()).To(MatchYAML(`
- d8-cloud-provider-huaweicloud`))
			Expect(pm.Field("spec.selector.matchLabels.app").String()).To(Equal("cloud-data-discoverer"))
			Expect(pm.Field("spec.podMetricsEndpoints.0.port").String()).To(Equal("https-metrics"))
			Expect(pm.Field("spec.podMetricsEndpoints.0.path").String()).To(Equal("/metrics"))
		})
	})

	Context("HuaweiCloud :: admission-policy-engine compatibility", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderHuaweicloud", moduleValuesA)
			f.ValuesSet("global.enabledModules", []string{
				"vertical-pod-autoscaler",
				"vertical-pod-autoscaler-crd",
				"admission-policy-engine",
				"admission-policy-engine-crd",
				"cloud-provider-huaweicloud",
			})
			f.HelmRender()
		})

		It("must render Namespace labels", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-provider-huaweicloud")
			Expect(namespace.Exists()).To(BeTrue())
			Expect(namespace.Field("metadata.labels.security\\.deckhouse\\.io/enable-security-policy-check").String()).To(Equal("true"))
		})

		It("must render SecurityPolicyException for cloud-controller-manager", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-huaweicloud", "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").String()).To(Equal("cloud-controller-manager"))

			securityPolicyException := f.KubernetesResource("SecurityPolicyException", "d8-cloud-provider-huaweicloud", "cloud-controller-manager")
			Expect(securityPolicyException.Exists()).To(BeTrue())
			Expect(securityPolicyException.Field("spec.network.hostNetwork.allowedValue").Bool()).To(BeTrue())
		})

		It("must render SecurityPolicyException for csi-controller", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-huaweicloud", "csi-controller")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").String()).To(Equal("csi-controller"))

			securityPolicyException := f.KubernetesResource("SecurityPolicyException", "d8-cloud-provider-huaweicloud", "csi-controller")
			Expect(securityPolicyException.Exists()).To(BeTrue())
			Expect(securityPolicyException.Field("spec.network.hostNetwork.allowedValue").Bool()).To(BeTrue())
		})

		It("must render SecurityPolicyException for csi-node", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmDeployment := f.KubernetesResource("DaemonSet", "d8-cloud-provider-huaweicloud", "csi-node")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").String()).To(Equal("csi-node"))

			securityPolicyException := f.KubernetesResource("SecurityPolicyException", "d8-cloud-provider-huaweicloud", "csi-node")
			Expect(securityPolicyException.Exists()).To(BeTrue())
			Expect(securityPolicyException.Field("spec.network.hostNetwork.allowedValue").Bool()).To(BeTrue())
			Expect(securityPolicyException.Field("spec.securityContext.privileged.allowedValue").Bool()).To(BeTrue())
			Expect(securityPolicyException.Field("spec.securityContext.runAsNonRoot.allowedValue").Bool()).To(BeFalse())
			Expect(securityPolicyException.Field("spec.securityContext.runAsUser.allowedValues").Array()).To(HaveLen(1))
			Expect(securityPolicyException.Field("spec.securityContext.runAsUser.allowedValues").Array()[0].Int()).To(BeEquivalentTo(0))
			Expect(securityPolicyException.Field("spec.volumes.types.allowedValues").Array()).To(HaveLen(1))
			Expect(securityPolicyException.Field("spec.volumes.types.allowedValues").Array()[0].String()).To(BeEquivalentTo("hostPath"))
			Expect(securityPolicyException.Field("spec.volumes.hostPath.allowedValues").Array()).To(
				ConsistOf(
					And(
						WithTransform(func(v gjson.Result) string { return v.Get("path").String() }, Equal("/var/lib/kubelet/plugins_registry/")),
						WithTransform(func(v gjson.Result) bool { return v.Get("readOnly").Bool() }, BeFalse()),
					),
					And(
						WithTransform(func(v gjson.Result) string { return v.Get("path").String() }, Equal("/var/lib/kubelet")),
						WithTransform(func(v gjson.Result) bool { return v.Get("readOnly").Bool() }, BeFalse()),
					),
					And(
						WithTransform(func(v gjson.Result) string { return v.Get("path").String() }, Equal("/var/lib/kubelet/csi-plugins/evs.csi.huaweicloud.com/")),
						WithTransform(func(v gjson.Result) bool { return v.Get("readOnly").Bool() }, BeFalse()),
					),
					And(
						WithTransform(func(v gjson.Result) string { return v.Get("path").String() }, Equal("/dev")),
						WithTransform(func(v gjson.Result) bool { return v.Get("readOnly").Bool() }, BeFalse()),
					),
				),
			)
		})
	})

	Context("HuaweiCloud :: bootstrap compatibility", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderHuaweicloud", moduleValuesA)
			f.ValuesSet("global.clusterIsBootstrapped", false)
			f.HelmRender()
		})

		It("must keep bootstrap-specific DNS and env behavior", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-huaweicloud", "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("Default"))
			Expect(ccmDeployment.Field("spec.template.spec.containers.0.env").String()).To(MatchYAML(`
- name: KUBERNETES_SERVICE_HOST
  valueFrom:
    fieldRef:
      apiVersion: v1
      fieldPath: status.hostIP
- name: KUBERNETES_SERVICE_PORT
  value: "6443"
- name: HOST_IP
  valueFrom:
    fieldRef:
      fieldPath: status.hostIP
- name: HUAWEICLOUD_SDK_CREDENTIALS_FILE
  value: /etc/cloud/cloud-config`))

			cddDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-huaweicloud", "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())
			Expect(cddDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("Default"))

			csiControllerDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-huaweicloud", "csi-controller")
			Expect(csiControllerDeployment.Exists()).To(BeTrue())
			Expect(csiControllerDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("Default"))

			csiNodeDaemonSet := f.KubernetesResource("DaemonSet", "d8-cloud-provider-huaweicloud", "csi-node")
			Expect(csiNodeDaemonSet.Exists()).To(BeTrue())
			Expect(csiNodeDaemonSet.Field("spec.template.spec.dnsPolicy").String()).To(Equal("Default"))
		})
	})
})
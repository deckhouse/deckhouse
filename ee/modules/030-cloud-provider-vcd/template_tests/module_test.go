/*
Copyright 2023 Flant JSC
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

const providerID = "vcd"
const nameLabelKey = "cloud-provider\\.deckhouse\\.io/name"
const registrationLabelKey = "cloud-provider\\.deckhouse\\.io/registration"
const ephemeralNodesTemplatesLabelKey = "cloud-provider\\.deckhouse\\.io/ephemeral-nodes-templates"
const bashibleLabelKey = "cloud-provider\\.deckhouse\\.io/bashible"

// fake *-crd modules are required for backward compatibility with lib_helm library
// TODO: remove fake crd modules
const globalValues = `
  clusterIsBootstrapped: true
  enabledModules: ["vertical-pod-autoscaler", "vertical-pod-autoscaler-crd", "cloud-provider-vcd"]
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: sandbox
      provider: VCD
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
      capcdControllerManagerWebhookCert:
        ca: ca
        crt: crt
        key: key
      providerDiscoveryData:
        kind: VCDCloudProviderDiscoveryData
        apiVersion: deckhouse.io/v1
        zones:
        - default
      discoveryData:
        kind: VCDCloudProviderDiscoveryData
        apiVersion: deckhouse.io/v1
        vcdInstallationVersion: "10.4.2"
        vcdAPIVersion: "37.2"
      providerClusterConfiguration:
        apiVersion: deckhouse.io/v1
        kind: VCDClusterConfiguration
        provider:
          username: myuname
          password: myPaSsWd
          insecure: true
          server: "http://server/api/"
        layout: Standard
        sshPublicKey: rsa-aaaa
        organization: org
        virtualDataCenter: dc
        virtualApplicationName: v1rtual-app
        mainNetwork: internal
        masterNodeGroup:
          replicas: 1
          instanceClass:
            affinityRule:
              polarity: AntiAffinity
            template: Templates/ubuntu-focal-20.04
            sizingPolicy: 4cpu8ram
            rootDiskSizeGb: 20
            etcdDiskSizeGb: 20
            storageProfile: nvme
        nodeGroups:
        - name: front
          replicas: 3
          instanceClass:
            rootDiskSizeGb: 20
            sizingPolicy: 16cpu32ram
            template: Templates/ubuntu-focal-20.04
            storageProfile: nvme
            affinityRule:
              polarity: AntiAffinity
              required: false
      affinityRules:
      - nodeGroupName: master
        polarity: AntiAffinity
      - nodeGroupName: front
        polarity: AntiAffinity
        required: false
      - nodeGroupName: ephemeral-node
        polarity: Affinity
        required: true
`

const moduleValuesB = `
    internal:
      capcdControllerManagerWebhookCert:
        ca: ca
        crt: crt
        key: key
      providerDiscoveryData:
        kind: VCDCloudProviderDiscoveryData
        apiVersion: deckhouse.io/v1
        zones:
        - default
      discoveryData:
        kind: VCDCloudProviderDiscoveryData
        apiVersion: deckhouse.io/v1
        vcdInstallationVersion: "10.4.2"
        vcdAPIVersion: "37.2"
        loadBalancer:
          enabled: false
      providerClusterConfiguration:
        apiVersion: deckhouse.io/v1
        kind: VCDClusterConfiguration
        provider:
          username: myuname
          password: myPaSsWd
          insecure: true
          server: "http://server/api/"
        layout: Standard
        sshPublicKey: rsa-aaaa
        organization: org
        virtualDataCenter: dc
        virtualApplicationName: v1rtual-app
        mainNetwork: internal
        masterNodeGroup:
          replicas: 1
          instanceClass:
            template: Templates/ubuntu-focal-20.04
            sizingPolicy: 4cpu8ram
            rootDiskSizeGb: 20
            etcdDiskSizeGb: 20
            storageProfile: nvme
`

const moduleValuesC = `
    internal:
      capcdControllerManagerWebhookCert:
        ca: ca
        crt: crt
        key: key
      providerDiscoveryData:
        kind: VCDCloudProviderDiscoveryData
        apiVersion: deckhouse.io/v1
        zones:
        - default
      discoveryData:
        kind: VCDCloudProviderDiscoveryData
        apiVersion: deckhouse.io/v1
        vcdInstallationVersion: "10.4.2"
        vcdAPIVersion: "37.2"
        loadBalancer:
          enabled: true
      providerClusterConfiguration:
        apiVersion: deckhouse.io/v1
        kind: VCDClusterConfiguration
        provider:
          username: myuname
          password: myPaSsWd
          insecure: true
          server: "http://server/api/"
        layout: Standard
        sshPublicKey: rsa-aaaa
        organization: org
        virtualDataCenter: dc
        virtualApplicationName: v1rtual-app
        mainNetwork: internal
        masterNodeGroup:
          replicas: 1
          instanceClass:
            template: Templates/ubuntu-focal-20.04
            sizingPolicy: 4cpu8ram
            rootDiskSizeGb: 20
            etcdDiskSizeGb: 20
            storageProfile: nvme
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

var _ = Describe("Module :: cloud-provider-vcd :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("VCD Suite A", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderVcd", moduleValuesA)
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
			Expect(providerRegistrationSecretData["capiClusterName"].String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("v1rtual-app"))))
			Expect(providerRegistrationSecretData["sshPublicKey"].String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("rsa-aaaa"))))

			providerSpecificRegistrationSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-node-manager-cloud-provider-%s", providerID))
			Expect(providerSpecificRegistrationSecret.Exists()).To(BeTrue())
			Expect(providerSpecificRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", registrationLabelKey)).String()).To(Equal(""))
			Expect(providerSpecificRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))
			providerSpecificRegistrationSecretData := providerSpecificRegistrationSecret.Field("data").Map()
			Expect(providerSpecificRegistrationSecretData).To(Not(BeEmpty()))
			Expect(providerSpecificRegistrationSecretData["capiClusterName"].String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("v1rtual-app"))))
			Expect(providerSpecificRegistrationSecretData["sshPublicKey"].String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("rsa-aaaa"))))


			providerSpecificBashibleStepsSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-cloud-provider-%s-bashible-steps", providerID))
			Expect(providerSpecificBashibleStepsSecret.Exists()).To(BeFalse())

			providerSpecificBashibleBootstrapSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-cloud-provider-%s-bashible-bootstrap", providerID))
			Expect(providerSpecificBashibleBootstrapSecret.Exists()).To(BeTrue())
			providerSpecificBashibleBootstrapSecretData := providerSpecificBashibleBootstrapSecret.Field("data").Map()
			Expect(len(providerSpecificBashibleBootstrapSecretData) >= 1).To(BeTrue())
			Expect(len(providerSpecificBashibleBootstrapSecretData["bootstrap-networks.sh.tpl"].String()) > 0 ).To(BeTrue())

			providerSpecificCAPISecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-cloud-provider-%s-capi", providerID))
			Expect(providerSpecificCAPISecret.Exists()).To(BeTrue())
			Expect(providerSpecificCAPISecret.Field(fmt.Sprintf("metadata.labels.%s", ephemeralNodesTemplatesLabelKey)).String()).To(Equal("capi"))
			Expect(providerSpecificCAPISecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))
			providerSpecificCAPISecretData := providerSpecificCAPISecret.Field("data").Map()
			Expect(providerSpecificCAPISecretData).To(Not(BeEmpty()))
			Expect(len(providerSpecificCAPISecretData) >= 1).To(BeTrue())
			Expect(len(providerSpecificCAPISecretData["cluster.yaml"].String()) > 0).To(BeTrue())

			masterAffinityRule := f.KubernetesGlobalResource("VCDAffinityRule", "sandbox-master")
			Expect(masterAffinityRule.Exists()).To(BeTrue())
			Expect(masterAffinityRule.Parse().String()).To(MatchYAML(`
apiVersion: deckhouse.io/v1alpha1
kind: VCDAffinityRule
metadata:
  name: sandbox-master
  labels:
    heritage: deckhouse
    module: cloud-provider-vcd
spec:
  nodeLabelSelector:
    matchLabels:
      node.deckhouse.io/group: master
  polarity: "AntiAffinity"
  required: false
`))

			frontAffinityRule := f.KubernetesGlobalResource("VCDAffinityRule", "sandbox-front")
			Expect(frontAffinityRule.Exists()).To(BeTrue())
			Expect(frontAffinityRule.Parse().String()).To(MatchYAML(`
apiVersion: deckhouse.io/v1alpha1
kind: VCDAffinityRule
metadata:
  name: sandbox-front
  labels:
    heritage: deckhouse
    module: cloud-provider-vcd
spec:
  nodeLabelSelector:
    matchLabels:
      node.deckhouse.io/group: front
  polarity: "AntiAffinity"
  required: false
`))

			ephemeralAffinityRule := f.KubernetesGlobalResource("VCDAffinityRule", "sandbox-ephemeral-node")
			Expect(ephemeralAffinityRule.Exists()).To(BeTrue())
			Expect(ephemeralAffinityRule.Parse().String()).To(MatchYAML(`
apiVersion: deckhouse.io/v1alpha1
kind: VCDAffinityRule
metadata:
  name: sandbox-ephemeral-node
  labels:
    heritage: deckhouse
    module: cloud-provider-vcd
spec:
  nodeLabelSelector:
    matchLabels:
      node.deckhouse.io/group: ephemeral-node
  polarity: "Affinity"
  required: true
`))

			ccmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.spec.hostNetwork").Bool()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("Default"))
			Expect(ccmDeployment.Field("spec.template.spec.serviceAccountName").String()).To(Equal("cloud-controller-manager"))
			Expect(ccmDeployment.Field("spec.template.spec.containers.0.name").String()).To(Equal("cloud-controller-manager"))
			Expect(ccmDeployment.Field("spec.template.spec.containers.0.args").String()).To(MatchYAML(`
- --leader-elect=true
- --bind-address=127.0.0.1
- --secure-port=10471
- --cloud-config=/etc/cloud/cloud-config
- --cloud-provider=vmware-cloud-director
- --allow-untagged-cloud=true
- --configure-cloud-routes=false
- --controllers=cloud-node,cloud-node-lifecycle
- --v=4`))
			Expect(ccmDeployment.Field("spec.template.spec.containers.0.volumeMounts").String()).To(MatchYAML(`
- mountPath: /etc/cloud
  name: ccm-controller-config-volume
  readOnly: true
- mountPath: /etc/kubernetes/vcloud/basic-auth
  name: vcd-credentials-volume
  readOnly: true`))
			Expect(ccmDeployment.Field("spec.template.spec.volumes").String()).To(MatchYAML(`
- name: ccm-controller-config-volume
  secret:
    secretName: ccm-controller-manager
- name: vcd-credentials-volume
  secret:
    secretName: vcd-credentials`))
			Expect(ccmDeployment.Field("spec.template.metadata.annotations.checksum/config").String()).NotTo(BeEmpty())

			ccmVPA := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-vcd", "cloud-controller-manager")
			Expect(ccmVPA.Exists()).To(BeTrue())
			Expect(ccmVPA.Field("spec.updatePolicy.updateMode").String()).To(Equal("InPlaceOrRecreate"))

			ccmPDB := f.KubernetesResource("PodDisruptionBudget", "d8-cloud-provider-vcd", "cloud-controller-manager")
			Expect(ccmPDB.Exists()).To(BeTrue())
			Expect(ccmPDB.Field("metadata.annotations.checksum/config").String()).NotTo(BeEmpty())

			capcdDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "capcd-controller-manager")
			Expect(capcdDeployment.Exists()).To(BeTrue())
			Expect(capcdDeployment.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").Exists()).To(BeFalse())
			Expect(capcdDeployment.Field("spec.template.spec.hostNetwork").Bool()).To(BeTrue())
			Expect(capcdDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(capcdDeployment.Field("spec.template.spec.serviceAccountName").String()).To(Equal("capcd-controller-manager"))
			Expect(capcdDeployment.Field("spec.template.spec.containers.0.name").String()).To(Equal("capcd-controller-manager"))
			Expect(capcdDeployment.Field("spec.template.spec.containers.0.args").String()).To(MatchYAML(`
- --leader-elect
- --diagnostics-address=127.0.0.1:9446
- --insecure-diagnostics
- --health-probe-bind-address=:9445
- --zap-encoder=json`))
			Expect(capcdDeployment.Field("spec.template.spec.containers.0.env").String()).To(MatchYAML(`
- name: CAPVCD_SKIP_RDE
  value: "true"
- name: USE_K8S_ENV_AS_CONTROL_PLANE_IP
  value: "true"`))
			Expect(capcdDeployment.Field("spec.template.spec.containers.0.ports").String()).To(MatchYAML(`
- containerPort: 4201
  name: webhook-server
  protocol: TCP`))
			Expect(capcdDeployment.Field("spec.template.spec.containers.0.volumeMounts").String()).To(MatchYAML(`
- mountPath: /tmp/k8s-webhook-server/serving-certs
  name: cert
  readOnly: true`))
			Expect(capcdDeployment.Field("spec.template.spec.volumes").String()).To(MatchYAML(`
- name: cert
  secret:
    defaultMode: 420
    secretName: capcd-controller-manager-webhook-tls`))
			Expect(capcdDeployment.Field("spec.template.spec.containers.0.livenessProbe.httpGet.path").String()).To(Equal("/healthz"))
			Expect(capcdDeployment.Field("spec.template.spec.containers.0.livenessProbe.httpGet.port").String()).To(Equal("9445"))
			Expect(capcdDeployment.Field("spec.template.spec.containers.0.readinessProbe.httpGet.path").String()).To(Equal("/readyz"))
			Expect(capcdDeployment.Field("spec.template.spec.containers.0.readinessProbe.httpGet.port").String()).To(Equal("9445"))
			Expect(capcdDeployment.Field("spec.template.metadata.annotations.checksum/config").String()).NotTo(BeEmpty())

			capcdVPA := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-vcd", "capcd-controller-manager")
			Expect(capcdVPA.Exists()).To(BeTrue())
			Expect(capcdVPA.Field("spec.updatePolicy.updateMode").String()).To(Equal("InPlaceOrRecreate"))

			capcdPDB := f.KubernetesResource("PodDisruptionBudget", "d8-cloud-provider-vcd", "capcd-controller-manager")
			Expect(capcdPDB.Exists()).To(BeTrue())

			capcdWebhookTLS := f.KubernetesResource("Secret", "d8-cloud-provider-vcd", "capcd-controller-manager-webhook-tls")
			Expect(capcdWebhookTLS.Exists()).To(BeTrue())

			capcdService := f.KubernetesResource("Service", "d8-cloud-provider-vcd", "capcd-controller-manager-webhook-service")
			Expect(capcdService.Exists()).To(BeTrue())
			Expect(capcdService.Field("spec.ports").String()).To(MatchYAML(`
- port: 443
  protocol: TCP
  targetPort: webhook-server`))

			capcdMutatingWebhook := f.KubernetesGlobalResource("MutatingWebhookConfiguration", "capcd-mutating-webhook")
			Expect(capcdMutatingWebhook.Exists()).To(BeTrue())

			capcdValidatingWebhook := f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "capcd-validating-webhook")
			Expect(capcdValidatingWebhook.Exists()).To(BeTrue())

			csiControllerDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "csi-controller")
			Expect(csiControllerDeployment.Exists()).To(BeTrue())
			Expect(csiControllerDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))

			csiNodeDaemonSet := f.KubernetesResource("DaemonSet", "d8-cloud-provider-vcd", "csi-node")
			Expect(csiNodeDaemonSet.Exists()).To(BeTrue())
			Expect(csiNodeDaemonSet.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))

			cddDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())
			Expect(cddDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(cddDeployment.Field("spec.template.spec.tolerations").String()).To(MatchYAML(tolerationsAnyNodeWithUninitialized))
			Expect(cddDeployment.Field("spec.template.spec.containers.0.name").String()).To(Equal("cloud-data-discoverer"))
			Expect(cddDeployment.Field("spec.template.spec.containers.0.args").String()).To(MatchYAML(`
- --discovery-period=1h
- --listen-address=127.0.0.1:8081`))
			Expect(cddDeployment.Field("spec.template.spec.containers.0.env").String()).To(MatchYAML(`
- name: VCD_INSECURE
  valueFrom:
    secretKeyRef:
      key: insecure
      name: vcd-connection-info
- name: VCD_HREF
  valueFrom:
    secretKeyRef:
      key: host
      name: vcd-connection-info
- name: VCD_VDC
  valueFrom:
    secretKeyRef:
      key: vdc
      name: vcd-connection-info
- name: VCD_ORG
  valueFrom:
    secretKeyRef:
      key: org
      name: vcd-connection-info
- name: VCD_NETWORK
  valueFrom:
    secretKeyRef:
      key: network
      name: vcd-connection-info
- name: VCD_USER
  valueFrom:
    secretKeyRef:
      key: username
      name: vcd-credentials
      optional: true
- name: VCD_PASSWORD
  valueFrom:
    secretKeyRef:
      key: password
      name: vcd-credentials
      optional: true
- name: VCD_TOKEN
  valueFrom:
    secretKeyRef:
      key: refreshToken
      name: vcd-credentials
      optional: true`))
			Expect(cddDeployment.Field("spec.template.spec.containers.1.name").String()).To(Equal("kube-rbac-proxy"))
			Expect(cddDeployment.Field("spec.template.metadata.annotations.kubectl\\.kubernetes\\.io/default-exec-container").String()).To(Equal("cloud-data-discoverer"))
			Expect(cddDeployment.Field("spec.template.metadata.annotations.kubectl\\.kubernetes\\.io/default-logs-container").String()).To(Equal("cloud-data-discoverer"))
			Expect(cddDeployment.Field("spec.template.metadata.annotations.checksum/config").String()).NotTo(BeEmpty())

			cddVPA := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-vcd", "cloud-data-discoverer")
			Expect(cddVPA.Exists()).To(BeTrue())
			Expect(cddVPA.Field("spec.updatePolicy.updateMode").String()).To(Equal("Initial"))

			cddPDB := f.KubernetesResource("PodDisruptionBudget", "d8-cloud-provider-vcd", "cloud-data-discoverer")
			Expect(cddPDB.Exists()).To(BeTrue())

			cddPodMonitor := f.KubernetesResource("PodMonitor", "d8-monitoring", "cloud-data-discoverer-metrics")
			Expect(cddPodMonitor.Exists()).To(BeFalse())

			icmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "infra-controller-manager")
			Expect(icmDeployment.Exists()).To(BeTrue())
			Expect(icmDeployment.Field("spec.revisionHistoryLimit").Int()).To(BeEquivalentTo(2))
			Expect(icmDeployment.Field("spec.template.spec.priorityClassName").String()).To(Equal("system-cluster-critical"))
			Expect(icmDeployment.Field("spec.template.spec.nodeSelector").String()).To(MatchYAML(`
node-role.deckhouse.io/control-plane: ""`))
			Expect(icmDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(icmDeployment.Field("spec.template.spec.automountServiceAccountToken").Bool()).To(BeTrue())
			Expect(icmDeployment.Field("spec.template.spec.imagePullSecrets").String()).To(MatchYAML(`
- name: deckhouse-registry`))
			Expect(icmDeployment.Field("spec.template.spec.tolerations").String()).To(MatchYAML(tolerationsAnyNodeWithUninitialized))
			Expect(icmDeployment.Field("spec.template.spec.serviceAccountName").String()).To(Equal("infra-controller-manager"))
			Expect(icmDeployment.Field("spec.template.spec.containers.0.name").String()).To(Equal("infra-controller-manager"))
			Expect(icmDeployment.Field("spec.template.spec.containers.0.args").String()).To(MatchYAML(`
- --health-probe-bind-address=:9448
- --leader-elect
- --leader-election-namespace=d8-cloud-provider-vcd`))
			Expect(icmDeployment.Field("spec.template.spec.containers.0.env").String()).To(MatchYAML(`
- name: VCD_INSECURE
  valueFrom:
    secretKeyRef:
      key: insecure
      name: vcd-connection-info
- name: VCD_HREF
  valueFrom:
    secretKeyRef:
      key: host
      name: vcd-connection-info
- name: VCD_VDC
  valueFrom:
    secretKeyRef:
      key: vdc
      name: vcd-connection-info
- name: VCD_ORG
  valueFrom:
    secretKeyRef:
      key: org
      name: vcd-connection-info
- name: VCD_VAPP
  valueFrom:
    secretKeyRef:
      key: vAppName
      name: vcd-connection-info
- name: VCD_USER
  valueFrom:
    secretKeyRef:
      key: username
      name: vcd-credentials
      optional: true
- name: VCD_PASSWORD
  valueFrom:
    secretKeyRef:
      key: password
      name: vcd-credentials
      optional: true
- name: VCD_TOKEN
  valueFrom:
    secretKeyRef:
      key: refreshToken
      name: vcd-credentials
      optional: true`))
			Expect(icmDeployment.Field("spec.template.spec.containers.0.livenessProbe.httpGet.path").String()).To(Equal("/healthz"))
			Expect(icmDeployment.Field("spec.template.spec.containers.0.livenessProbe.httpGet.port").String()).To(Equal("9448"))
			Expect(icmDeployment.Field("spec.template.spec.containers.0.livenessProbe.httpGet.scheme").String()).To(Equal("HTTP"))
			Expect(icmDeployment.Field("spec.template.spec.containers.0.readinessProbe.httpGet.path").String()).To(Equal("/readyz"))
			Expect(icmDeployment.Field("spec.template.spec.containers.0.readinessProbe.httpGet.port").String()).To(Equal("9448"))
			Expect(icmDeployment.Field("spec.template.spec.containers.0.readinessProbe.httpGet.scheme").String()).To(Equal("HTTP"))
			Expect(icmDeployment.Field("spec.template.metadata.annotations.kubectl\\.kubernetes\\.io/default-exec-container").String()).To(Equal("infra-controller-manager"))
			Expect(icmDeployment.Field("spec.template.metadata.annotations.kubectl\\.kubernetes\\.io/default-logs-container").String()).To(Equal("infra-controller-manager"))
			Expect(icmDeployment.Field("spec.template.metadata.annotations.checksum/config").String()).NotTo(BeEmpty())

			icmVPA := f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-vcd", "infra-controller-manager")
			Expect(icmVPA.Exists()).To(BeTrue())
			Expect(icmVPA.Field("spec.targetRef.name").String()).To(Equal("infra-controller-manager"))
			Expect(icmVPA.Field("spec.updatePolicy.updateMode").String()).To(Equal("InPlaceOrRecreate"))
			Expect(icmVPA.Field("spec.resourcePolicy.containerPolicies.0.containerName").String()).To(Equal("infra-controller-manager"))
			Expect(icmVPA.Field("spec.resourcePolicy.containerPolicies.0.minAllowed.cpu").String()).To(Equal("25m"))
			Expect(icmVPA.Field("spec.resourcePolicy.containerPolicies.0.minAllowed.memory").String()).To(Equal("50Mi"))
			Expect(icmVPA.Field("spec.resourcePolicy.containerPolicies.0.maxAllowed.cpu").String()).To(Equal("50m"))
			Expect(icmVPA.Field("spec.resourcePolicy.containerPolicies.0.maxAllowed.memory").String()).To(Equal("50Mi"))

			icmPDB := f.KubernetesResource("PodDisruptionBudget", "d8-cloud-provider-vcd", "infra-controller-manager")
			Expect(icmPDB.Exists()).To(BeTrue())
			Expect(icmPDB.Field("spec.maxUnavailable").Int()).To(BeEquivalentTo(1))
			Expect(icmPDB.Field("spec.selector.matchLabels.app").String()).To(Equal("infra-controller-manager"))
		})
	})

	Context("VCD Suite B", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderVcd", moduleValuesB)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.spec.containers.0.args").String()).To(MatchYAML(`
- --leader-elect=true
- --bind-address=127.0.0.1
- --secure-port=10471
- --cloud-config=/etc/cloud/cloud-config
- --cloud-provider=vmware-cloud-director
- --allow-untagged-cloud=true
- --configure-cloud-routes=false
- --controllers=cloud-node,cloud-node-lifecycle
- --v=4
`))
		})
	})

	Context("VCD Suite C", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderVcd", moduleValuesC)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.spec.containers.0.args").String()).To(MatchYAML(`
- --leader-elect=true
- --bind-address=127.0.0.1
- --secure-port=10471
- --cloud-config=/etc/cloud/cloud-config
- --cloud-provider=vmware-cloud-director
- --allow-untagged-cloud=true
- --configure-cloud-routes=false
- --controllers=cloud-node,cloud-node-lifecycle,service
- --v=4
`))
		})
	})

	Context("VCD :: VPA gate compatibility", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderVcd", moduleValuesA)
			f.ValuesSet("global.enabledModules", []string{
				"vertical-pod-autoscaler",
				"vertical-pod-autoscaler-crd",
				"cloud-provider-vcd",
			})
			f.HelmRender()
		})

		It("must still render VPA objects when vertical-pod-autoscaler is enabled", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-vcd", "cloud-controller-manager").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-vcd", "capcd-controller-manager").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-vcd", "cloud-data-discoverer").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-cloud-provider-vcd", "infra-controller-manager").Exists()).To(BeTrue())
		})
	})

	Context("VCD :: PodMonitor gate compatibility", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderVcd", moduleValuesA)
			f.ValuesSet("global.enabledModules", []string{
				"vertical-pod-autoscaler",
				"vertical-pod-autoscaler-crd",
				"operator-prometheus",
				"operator-prometheus-crd",
				"cloud-provider-vcd",
			})
			f.ValuesSet("global.discovery.prometheusScrapeInterval", 30)
			f.HelmRender()
		})

		It("must render PodMonitor when operator-prometheus is enabled", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			pm := f.KubernetesResource("PodMonitor", "d8-monitoring", "cloud-data-discoverer-metrics")
			Expect(pm.Exists()).To(BeTrue())
			Expect(pm.Field("spec.namespaceSelector.matchNames").String()).To(MatchYAML(`
- d8-cloud-provider-vcd`))
			Expect(pm.Field("spec.selector.matchLabels.app").String()).To(Equal("cloud-data-discoverer"))
			Expect(pm.Field("spec.podMetricsEndpoints.0.port").String()).To(Equal("https-metrics"))
			Expect(pm.Field("spec.podMetricsEndpoints.0.path").String()).To(Equal("/metrics"))
		})
	})

	Context("VCD :: admission-policy-engine gate compatibility", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderVcd", moduleValuesA)
			f.ValuesSet("global.enabledModules", []string{
				"vertical-pod-autoscaler",
				"vertical-pod-autoscaler-crd",
				"admission-policy-engine",
				"admission-policy-engine-crd",
				"cloud-provider-vcd",
			})
			f.HelmRender()
		})

		It("must render Namespace labels", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cloud-provider-vcd")
			Expect(namespace.Exists()).To(BeTrue())
			Expect(namespace.Field("metadata.labels.security\\.deckhouse\\.io/enable-security-policy-check").String()).To(Equal("true"))
		})

		It("must render SecurityPolicyException for cloud-controller-manager", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").String()).To(Equal("cloud-controller-manager"))

			securityPolicyException := f.KubernetesResource("SecurityPolicyException", "d8-cloud-provider-vcd", "cloud-controller-manager")
			Expect(securityPolicyException.Exists()).To(BeTrue())
			Expect(securityPolicyException.Field("spec.network.hostNetwork.allowedValue").Bool()).To(BeTrue())
		})

		It("must render SecurityPolicyException for capcd-controller-manager", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			capcdDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "capcd-controller-manager")
			Expect(capcdDeployment.Exists()).To(BeTrue())
			Expect(capcdDeployment.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").String()).To(Equal("capcd-controller-manager"))

			securityPolicyException := f.KubernetesResource("SecurityPolicyException", "d8-cloud-provider-vcd", "capcd-controller-manager")
			Expect(securityPolicyException.Exists()).To(BeTrue())
			Expect(securityPolicyException.Field("spec.network.hostNetwork.allowedValue").Bool()).To(BeTrue())
			Expect(securityPolicyException.Field("spec.network.hostNetwork.metadata.description").String()).To(ContainSubstring("CAPCD"))
		})

		It("must render SecurityPolicyException for csi-controller", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "csi-controller")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").String()).To(Equal("csi-controller"))

			securityPolicyException := f.KubernetesResource("SecurityPolicyException", "d8-cloud-provider-vcd", "csi-controller")
			Expect(securityPolicyException.Exists()).To(BeTrue())
			Expect(securityPolicyException.Field("spec.network.hostNetwork.allowedValue").Bool()).To(BeTrue())
		})

		It("must not set security-policy-exception label on infra-controller-manager", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			icmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "infra-controller-manager")
			Expect(icmDeployment.Exists()).To(BeTrue())
			Expect(icmDeployment.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("SecurityPolicyException", "d8-cloud-provider-vcd", "infra-controller-manager").Exists()).To(BeFalse())
		})

		It("must render SecurityPolicyException for csi-node", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmDeployment := f.KubernetesResource("DaemonSet", "d8-cloud-provider-vcd", "csi-node")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").String()).To(Equal("csi-node"))

			securityPolicyException := f.KubernetesResource("SecurityPolicyException", "d8-cloud-provider-vcd", "csi-node")
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
						WithTransform(func(v gjson.Result) string { return v.Get("path").String() }, Equal("/var/lib/kubelet/csi-plugins/named-disk.csi.cloud-director.vmware.com/")),
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

	Context("VCD :: infra-controller-manager RBAC", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderVcd", moduleValuesA)
			f.HelmRender()
		})

		It("must render ServiceAccount and RBAC for infra-controller-manager", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			sa := f.KubernetesResource("ServiceAccount", "d8-cloud-provider-vcd", "infra-controller-manager")
			Expect(sa.Exists()).To(BeTrue())
			Expect(sa.Field("automountServiceAccountToken").Bool()).To(BeFalse())

			cr := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-vcd:infra-controller-manager")
			Expect(cr.Exists()).To(BeTrue())
			Expect(cr.Field("rules.0.resources.0").String()).To(Equal("vcdaffinityrules"))

			crb := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-vcd:infra-controller-manager")
			Expect(crb.Exists()).To(BeTrue())
			Expect(crb.Field("subjects.0.kind").String()).To(Equal("ServiceAccount"))
			Expect(crb.Field("subjects.0.name").String()).To(Equal("infra-controller-manager"))
			Expect(crb.Field("subjects.0.namespace").String()).To(Equal("d8-cloud-provider-vcd"))
			Expect(crb.Field("roleRef.kind").String()).To(Equal("ClusterRole"))
			Expect(crb.Field("roleRef.name").String()).To(Equal("d8:cloud-provider-vcd:infra-controller-manager"))

			role := f.KubernetesResource("Role", "d8-cloud-provider-vcd", "infra-controller-manager-leader-election")
			Expect(role.Exists()).To(BeTrue())
			Expect(role.Field("rules.0.resources.0").String()).To(Equal("leases"))

			rb := f.KubernetesResource("RoleBinding", "d8-cloud-provider-vcd", "infra-controller-manager-leader-election")
			Expect(rb.Exists()).To(BeTrue())
			Expect(rb.Field("roleRef.name").String()).To(Equal("infra-controller-manager-leader-election"))
			Expect(rb.Field("subjects.0.name").String()).To(Equal("infra-controller-manager"))
		})
	})

	Context("VCD :: bootstrap compatibility", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderVcd", moduleValuesA)
			f.ValuesSet("global.clusterIsBootstrapped", false)
			f.HelmRender()
		})

		It("must keep bootstrap-specific DNS behavior", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("Default"))

			capcdDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "capcd-controller-manager")
			Expect(capcdDeployment.Exists()).To(BeTrue())
			Expect(capcdDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))

			cddDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())
			Expect(cddDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("Default"))

			csiControllerDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "csi-controller")
			Expect(csiControllerDeployment.Exists()).To(BeTrue())
			Expect(csiControllerDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("Default"))

			csiNodeDaemonSet := f.KubernetesResource("DaemonSet", "d8-cloud-provider-vcd", "csi-node")
			Expect(csiNodeDaemonSet.Exists()).To(BeTrue())
			Expect(csiNodeDaemonSet.Field("spec.template.spec.dnsPolicy").String()).To(Equal("Default"))

			icmDeployment := f.KubernetesResource("Deployment", "d8-cloud-provider-vcd", "infra-controller-manager")
			Expect(icmDeployment.Exists()).To(BeTrue())
			Expect(icmDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("Default"))
		})
	})
})
/*
Copyright 2025 Flant JSC

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

const providerID = "dvp"
const nameLabelKey = "cloud-provider\\.deckhouse\\.io/name"
const registrationLabelKey = "cloud-provider\\.deckhouse\\.io/registration"
const ephemeralNodesTemplatesLabelKey = "cloud-provider\\.deckhouse\\.io/ephemeral-nodes-templates"

const globalValues = `
  clusterIsBootstrapped: true
  enabledModules: ["vertical-pod-autoscaler", "vertical-pod-autoscaler-crd", "cloud-provider-dvp"]
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: sandbox
      provider: DVP
    clusterDomain: cluster.local
    clusterType: Cloud
    defaultCRI: Containerd
    kind: ClusterConfiguration
    kubernetesVersion: "1.32"
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
    kubernetesVersion: 1.32.1
    clusterUUID: cluster
`

const moduleValuesA = `
nodes:
  disabled: false
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa AAAAB3N
provider:
  parameters:
    namespace: cloud-provider01
storage:
  disabled: false
  parameters: {}
ccm:
  disabled: false
internal:
  validationWebhookCert:
    crt: dGVzdC1jcnQ=
    key: dGVzdC1rZXk=
    ca: dGVzdC1jYQ==
  credentialSecrets:
    d8-credentials:
      authScheme: Kubeconfig
      secret: YXBpVmV=
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: DVPCloudDiscoveryData
    zones:
      - default
  storageClasses:
    - dvpStorageClass: 1test
      name: 1test
    - dvpStorageClass: ceph-pool-r2-csi-cephfs
      name: ceph-pool-r2-csi-cephfs
    - dvpStorageClass: ceph-pool-r2-csi-rbd
      name: ceph-pool-r2-csi-rbd
    - dvpStorageClass: ceph-pool-r2-csi-rbd-immediate
      name: ceph-pool-r2-csi-rbd-immediate
    - dvpStorageClass: ceph-pool-r2-csi-rbd-immediate-feat
      name: ceph-pool-r2-csi-rbd-immediate-feat
    - dvpStorageClass: linstor-thin-r1
      name: linstor-thin-r1
    - dvpStorageClass: linstor-thin-r2
      name: linstor-thin-r2
    - dvpStorageClass: sds-local-storage
      name: sds-local-storage
    - dvpStorageClass: xxx
      name: xxx
`

const moduleValuesStorageDisabled = `
nodes:
  disabled: false
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa AAAAB3N
provider:
  parameters:
    namespace: cloud-provider01
storage:
  disabled: true
  parameters: {}
ccm:
  disabled: false
internal:
  validationWebhookCert:
    crt: dGVzdC1jcnQ=
    key: dGVzdC1rZXk=
    ca: dGVzdC1jYQ==
  credentialSecrets:
    d8-credentials:
      authScheme: Kubeconfig
      secret: YXBpVmV=
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: DVPCloudDiscoveryData
    zones:
      - default
  storageClasses:
    - dvpStorageClass: 1test
      name: 1test
    - dvpStorageClass: ceph-pool-r2-csi-cephfs
      name: ceph-pool-r2-csi-cephfs
    - dvpStorageClass: ceph-pool-r2-csi-rbd
      name: ceph-pool-r2-csi-rbd
    - dvpStorageClass: ceph-pool-r2-csi-rbd-immediate
      name: ceph-pool-r2-csi-rbd-immediate
    - dvpStorageClass: ceph-pool-r2-csi-rbd-immediate-feat
      name: ceph-pool-r2-csi-rbd-immediate-feat
    - dvpStorageClass: linstor-thin-r1
      name: linstor-thin-r1
    - dvpStorageClass: linstor-thin-r2
      name: linstor-thin-r2
    - dvpStorageClass: sds-local-storage
      name: sds-local-storage
    - dvpStorageClass: xxx
      name: xxx
`

const moduleValuesNodesDisabled = `
nodes:
  disabled: true
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa AAAAB3N
provider:
  parameters:
    namespace: cloud-provider01
storage:
  disabled: false
  parameters: {}
ccm:
  disabled: false
internal:
  validationWebhookCert:
    crt: dGVzdC1jcnQ=
    key: dGVzdC1rZXk=
    ca: dGVzdC1jYQ==
  credentialSecrets:
    d8-credentials:
      authScheme: Kubeconfig
      secret: YXBpVmV=
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: DVPCloudDiscoveryData
    zones:
      - default
  storageClasses:
    - dvpStorageClass: 1test
      name: 1test
    - dvpStorageClass: ceph-pool-r2-csi-rbd
      name: ceph-pool-r2-csi-rbd
`

const moduleValuesBothDisabled = `
nodes:
  disabled: true
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa AAAAB3N
provider:
  parameters:
    namespace: cloud-provider01
storage:
  disabled: true
  parameters: {}
ccm:
  disabled: false
internal:
  validationWebhookCert:
    crt: test
    key: test
    ca: test
  credentialSecrets:
    d8-credentials:
      authScheme: Kubeconfig
      secret: YXBpVmV=
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: DVPCloudDiscoveryData
    zones:
      - default
  storageClasses: []
`

const moduleValuesCCMDisabled = `
nodes:
  disabled: false
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa AAAAB3N
provider:
  parameters:
    namespace: cloud-provider01
storage:
  disabled: false
  parameters: {}
ccm:
  disabled: true
internal:
  validationWebhookCert:
    crt: dGVzdC1jcnQ=
    key: dGVzdC1rZXk=
    ca: dGVzdC1jYQ==
  credentialSecrets:
    d8-credentials:
      authScheme: Kubeconfig
      secret: YXBpVmV=
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: DVPCloudDiscoveryData
    zones:
      - default
  storageClasses:
    - dvpStorageClass: 1test
      name: 1test
    - dvpStorageClass: ceph-pool-r2-csi-rbd
      name: ceph-pool-r2-csi-rbd
`

// moduleValuesNoDisabled has no 'disabled' fields in any subsection.
// This covers the case where a user creates a v2 ModuleConfig without explicit disabled flags.
const moduleValuesNoDisabled = `
nodes:

  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa AAAAB3N
provider:
  parameters:
    namespace: cloud-provider01
storage:
  parameters: {}
internal:
  validationWebhookCert:
    crt: dGVzdC1jcnQ=
    key: dGVzdC1rZXk=
    ca: dGVzdC1jYQ==
  credentialSecrets:
    d8-credentials:
      authScheme: Kubeconfig
      secret: YXBpVmV=
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: DVPCloudDiscoveryData
    zones:
      - default
  storageClasses:
    - dvpStorageClass: 1test
      name: 1test
`

const moduleValuesWithCACerts = `
nodes:
  disabled: false
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa AAAAB3N
    sshCAKeys:
      - ssh-rsa-ca-AAAA-fake-ca-key-1
      - ssh-rsa-ca-AAAA-fake-ca-key-2
provider:
  parameters:
    namespace: cloud-provider01
storage:
  disabled: false
  parameters: {}
ccm:
  disabled: false
internal:
  validationWebhookCert:
    crt: dGVzdC1jcnQ=
    key: dGVzdC1rZXk=
    ca: dGVzdC1jYQ==
  credentialSecrets:
    d8-credentials:
      authScheme: Kubeconfig
      secret: YXBpVmV=
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: DVPCloudDiscoveryData
    zones:
      - default
  storageClasses: []
`

// moduleValuesWithHeredocBreakoutCACert covers a sshCAKeys entry crafted to
// prematurely terminate the quoted heredoc that embeds it in
// templates/ngc-ssh-ca.yaml (embedded newline + a line matching the heredoc
// delimiter) - a quoted heredoc delimiter blocks variable/command expansion
// inside the body, but does not stop the body from containing a line that
// happens to equal the delimiter itself, which bash treats as the
// terminator regardless of quoting.
const moduleValuesWithHeredocBreakoutCACert = `
nodes:
  disabled: false
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa AAAAB3N
    sshCAKeys:
      - "ssh-rsa AAAA\nDVP_SSH_CA_KEYS_EOF\necho PWNED > /tmp/pwned\nssh-rsa AAAA2"
provider:
  parameters:
    namespace: cloud-provider01
storage:
  disabled: false
  parameters: {}
ccm:
  disabled: false
internal:
  validationWebhookCert:
    crt: dGVzdC1jcnQ=
    key: dGVzdC1rZXk=
    ca: dGVzdC1jYQ==
  credentialSecrets:
    d8-credentials:
      authScheme: Kubeconfig
      secret: YXBpVmV=
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: DVPCloudDiscoveryData
    zones:
      - default
  storageClasses: []
`

const moduleValuesWithAdditionalUsers = `
nodes:
  disabled: false
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa AAAAB3N
    additionalUsers:
      - alice
      - s.bob
provider:
  parameters:
    namespace: cloud-provider01
storage:
  disabled: false
  parameters: {}
ccm:
  disabled: false
internal:
  validationWebhookCert:
    crt: dGVzdC1jcnQ=
    key: dGVzdC1rZXk=
    ca: dGVzdC1jYQ==
  credentialSecrets:
    d8-credentials:
      authScheme: Kubeconfig
      secret: YXBpVmV=
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: DVPCloudDiscoveryData
    zones:
      - default
  storageClasses: []
`

// moduleValuesWithReservedAdditionalUser covers the case where "root" (an
// always-existing system account) is accidentally listed in additionalUsers.
const moduleValuesWithReservedAdditionalUser = `
nodes:
  disabled: false
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa AAAAB3N
    additionalUsers:
      - root
provider:
  parameters:
    namespace: cloud-provider01
storage:
  disabled: false
  parameters: {}
ccm:
  disabled: false
internal:
  validationWebhookCert:
    crt: dGVzdC1jcnQ=
    key: dGVzdC1rZXk=
    ca: dGVzdC1jYQ==
  credentialSecrets:
    d8-credentials:
      authScheme: Kubeconfig
      secret: YXBpVmV=
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: DVPCloudDiscoveryData
    zones:
      - default
  storageClasses: []
`

// moduleValuesWithMaliciousAdditionalUser covers a shell command-injection
// payload disguised as a user name (see the Context using this fixture for
// why schema validation alone can't be relied on to catch it).
const moduleValuesWithMaliciousAdditionalUser = `
nodes:
  disabled: false
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa AAAAB3N
    additionalUsers:
      - $(touch /tmp/pwned)
provider:
  parameters:
    namespace: cloud-provider01
storage:
  disabled: false
  parameters: {}
ccm:
  disabled: false
internal:
  validationWebhookCert:
    crt: dGVzdC1jcnQ=
    key: dGVzdC1rZXk=
    ca: dGVzdC1jYQ==
  credentialSecrets:
    d8-credentials:
      authScheme: Kubeconfig
      secret: YXBpVmV=
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: DVPCloudDiscoveryData
    zones:
      - default
  storageClasses: []
`

// moduleValuesWithReservedDefaultAdditionalUser covers "default" - the
// cloud-init keyword that already refers to the image's own default user
// (see cloudinit-merge module's static_block.users). Allowing it into
// additionalUsers would silently create an unrelated second user literally
// named "default" instead of doing what the operator meant.
const moduleValuesWithReservedDefaultAdditionalUser = `
nodes:
  disabled: false
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa AAAAB3N
    additionalUsers:
      - default
provider:
  parameters:
    namespace: cloud-provider01
storage:
  disabled: false
  parameters: {}
ccm:
  disabled: false
internal:
  validationWebhookCert:
    crt: dGVzdC1jcnQ=
    key: dGVzdC1rZXk=
    ca: dGVzdC1jYQ==
  credentialSecrets:
    d8-credentials:
      authScheme: Kubeconfig
      secret: YXBpVmV=
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: DVPCloudDiscoveryData
    zones:
      - default
  storageClasses: []
`

const moduleValuesWithoutCCM = `
nodes:
  disabled: false
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa AAAAB3N
provider:
  parameters:
    namespace: cloud-provider01
storage:
  disabled: false
  parameters: {}
internal:
  validationWebhookCert:
    crt: dGVzdC1jcnQ=
    key: dGVzdC1rZXk=
    ca: dGVzdC1jYQ==
  credentialSecrets:
    d8-credentials:
      authScheme: Kubeconfig
      secret: YXBpVmV=
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: DVPCloudDiscoveryData
    zones:
      - default
  storageClasses:
    - dvpStorageClass: 1test
      name: 1test
    - dvpStorageClass: ceph-pool-r2-csi-rbd
      name: ceph-pool-r2-csi-rbd
`

// moduleValuesNoStorageSection has no 'storage' section at all.
// This covers the case where storage is completely omitted from ModuleConfig.
const moduleValuesNoStorageSection = `
nodes:
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa AAAAB3N
provider:
  parameters:
    namespace: cloud-provider01
internal:
  validationWebhookCert:
    crt: dGVzdC1jcnQ=
    key: dGVzdC1rZXk=
    ca: dGVzdC1jYQ==
  credentialSecrets:
    d8-credentials:
      authScheme: Kubeconfig
      secret: YXBpVmV=
  providerDiscoveryData:
    apiVersion: deckhouse.io/v1
    kind: DVPCloudDiscoveryData
    zones:
      - default
  storageClasses:
    - dvpStorageClass: 1test
      name: 1test
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

const moduleNamespace = "d8-cloud-provider-dvp"

const validationWebhookName = "validation-webhook"

var validationWebhookArgs = []string{
	"--webhook-port=4330",
	"--webhook-cert-dir=/tmp/k8s-webhook-server/serving-certs",
	"--metrics-bind-address=0",
	"--health-probe-bind-address=0.0.0.0:4332",
}

var _ = Describe("Module :: cloud-provider-dvp :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("DVP", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesA)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			csiController := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			Expect(csiController.Exists()).To(BeTrue())
			Expect(csiController.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))

			csiNode := f.KubernetesResource("DaemonSet", moduleNamespace, "csi-node")
			Expect(csiNode.Exists()).To(BeTrue())
			Expect(csiNode.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))

			ccmDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.spec.hostNetwork").Bool()).To(BeTrue())
			Expect(ccmDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("Default"))

			ccmVPA := f.KubernetesResource("VerticalPodAutoscaler", moduleNamespace, "cloud-controller-manager")
			Expect(ccmVPA.Exists()).To(BeTrue())

			ccmPDB := f.KubernetesResource("PodDisruptionBudget", moduleNamespace, "cloud-controller-manager")
			Expect(ccmPDB.Exists()).To(BeTrue())

			capdvpDeployment := f.KubernetesResource("Deployment", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpDeployment.Exists()).To(BeTrue())
			Expect(capdvpDeployment.Field("spec.template.spec.hostNetwork").Bool()).To(BeTrue())
			Expect(capdvpDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))

			capdvpVPA := f.KubernetesResource("VerticalPodAutoscaler", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpVPA.Exists()).To(BeTrue())

			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())
			Expect(cddDeployment.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(cddDeployment.Field("spec.template.spec.tolerations").String()).To(MatchYAML(tolerationsAnyNodeWithUninitialized))

			cddVPA := f.KubernetesResource("VerticalPodAutoscaler", moduleNamespace, "cloud-data-discoverer")
			Expect(cddVPA.Exists()).To(BeTrue())

			validationWebhookDeployment := f.KubernetesResource("Deployment", moduleNamespace, "validation-webhook")
			Expect(validationWebhookDeployment.Exists()).To(BeTrue())
			Expect(validationWebhookDeployment.Field("spec.revisionHistoryLimit").String()).To(Equal("2"))

			validationWebhookVPA := f.KubernetesResource("VerticalPodAutoscaler", moduleNamespace, "validation-webhook")
			Expect(validationWebhookVPA.Exists()).To(BeTrue())

			validationWebhookPDB := f.KubernetesResource("PodDisruptionBudget", moduleNamespace, "validation-webhook")
			Expect(validationWebhookPDB.Exists()).To(BeTrue())

			validationWebhookService := f.KubernetesResource("Service", moduleNamespace, "validation-webhook")
			Expect(validationWebhookService.Exists()).To(BeTrue())

			validationWebhookTLS := f.KubernetesResource("Secret", moduleNamespace, "validation-webhook-tls")
			Expect(validationWebhookTLS.Exists()).To(BeTrue())

			validationWebhookSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "validation-webhook")
			Expect(validationWebhookSA.Exists()).To(BeTrue())

			validationWebhookCR := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-dvp:validation-webhook")
			Expect(validationWebhookCR.Exists()).To(BeTrue())
			Expect(validationWebhookCR.Field("rules").String()).To(MatchYAML(`
- apiGroups:
  - deckhouse.io
  resources:
  - nodegroups
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - deckhouse.io
  resources:
  - dvpinstanceclasses
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - secrets
  - configmaps
  verbs:
  - get
  - list
  - watch`))

			validationWebhookConfiguration := f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "d8-cloud-provider-dvp-validation-webhook")
			Expect(validationWebhookConfiguration.Exists()).To(BeTrue())
			Expect(validationWebhookConfiguration.Field("webhooks.0.clientConfig.service.path").String()).
				To(Equal("/validate--v1-secret"))
			Expect(validationWebhookConfiguration.Field("webhooks.0.timeoutSeconds").String()).To(Equal("30"))
			Expect(validationWebhookConfiguration.Field("webhooks.0.matchConditions").String()).To(MatchYAML(`
- expression: (object != null && object.type == 'cloud-provider.deckhouse.io/credentials') || (oldObject != null && oldObject.type == 'cloud-provider.deckhouse.io/credentials')
  name: credential-secret-type`))
			Expect(validationWebhookConfiguration.Field("webhooks.1.clientConfig.service.path").String()).
				To(Equal("/validate-deckhouse-io-v1-nodegroup"))
			Expect(validationWebhookConfiguration.Field("webhooks.1.timeoutSeconds").String()).To(Equal("30"))
			Expect(validationWebhookConfiguration.Field("webhooks.1.matchConditions").String()).To(MatchYAML(`
- expression: (object != null && object.spec.nodeType == 'CloudPermanent') || (oldObject != null && oldObject.spec.nodeType == 'CloudPermanent') || (object != null && has(object.spec.cloudInstances) && has(object.spec.cloudInstances.classReference) && object.spec.cloudInstances.classReference.kind == 'DVPInstanceClass') || (oldObject != null && has(oldObject.spec.cloudInstances) && has(oldObject.spec.cloudInstances.classReference) && oldObject.spec.cloudInstances.classReference.kind == 'DVPInstanceClass')
  name: dvp-node-group-involved`))
			Expect(validationWebhookConfiguration.Field("webhooks.2.clientConfig.service.path").String()).
				To(Equal("/validate-deckhouse-io-v1alpha1-dvpinstanceclass"))
			Expect(validationWebhookConfiguration.Field("webhooks.2.timeoutSeconds").String()).To(Equal("30"))

			userAuthzUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-dvp:user")
			Expect(userAuthzUser.Exists()).To(BeTrue())
			Expect(userAuthzUser.Field("rules").String()).To(MatchYAML(`
- apiGroups:
  - deckhouse.io
  resources:
  - dvpinstanceclasses
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - infrastructure.cluster.x-k8s.io
  resources:
  - deckhouseclusters
  - deckhousemachines
  - deckhousemachinetemplates
  verbs:
  - get
  - list
  - watch`))

			userAuthzClusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-dvp:cluster-admin")
			Expect(userAuthzClusterAdmin.Exists()).To(BeTrue())
			Expect(userAuthzClusterAdmin.Field("rules").String()).To(MatchYAML(`
- apiGroups:
  - deckhouse.io
  resources:
  - dvpinstanceclasses
  verbs:
  - create
  - delete
  - deletecollection
  - patch
  - update
- apiGroups:
  - infrastructure.cluster.x-k8s.io
  resources:
  - deckhouseclusters
  - deckhousemachines
  - deckhousemachinetemplates
  verbs:
  - patch
  - update`))

			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())
			Expect(providerRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", registrationLabelKey)).String()).To(Equal(""))
			Expect(providerRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))
			providerRegistrationSecretData := providerRegistrationSecret.Field("data").Map()
			Expect(providerRegistrationSecretData).To(Not(BeEmpty()))
			Expect(providerRegistrationSecretData["capiClusterName"].String()).To(Equal(base64.StdEncoding.EncodeToString([]byte(providerID))))
			Expect(providerRegistrationSecretData["sshPublicKey"].String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("ssh-rsa AAAAB3N"))))

			providerSpecificRegistrationSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-node-manager-cloud-provider-%s", providerID))
			Expect(providerSpecificRegistrationSecret.Exists()).To(BeTrue())
			Expect(providerSpecificRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", registrationLabelKey)).String()).To(Equal(""))
			Expect(providerSpecificRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))
			providerSpecificRegistrationSecretData := providerSpecificRegistrationSecret.Field("data").Map()
			Expect(providerSpecificRegistrationSecretData).To(Not(BeEmpty()))
			Expect(providerSpecificRegistrationSecretData["capiClusterName"].String()).To(Equal(base64.StdEncoding.EncodeToString([]byte(providerID))))
			Expect(providerSpecificRegistrationSecretData["sshPublicKey"].String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("ssh-rsa AAAAB3N"))))

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
		})

	})

	Context("DVP with storage disabled", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesStorageDisabled)
			f.HelmRender()
		})

		It("CSI components must not render; CCM, capdvp, and cloud-data-discoverer must render", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// CSI controller Deployment must be absent.
			csiController := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			Expect(csiController.Exists()).To(BeFalse())

			// CSI node DaemonSet must be absent.
			csiNode := f.KubernetesResource("DaemonSet", moduleNamespace, "csi-node")
			Expect(csiNode.Exists()).To(BeFalse())

			// CSIDriver CR must be absent.
			csiDriver := f.KubernetesGlobalResource("CSIDriver", "csi.dvp.deckhouse.io")
			Expect(csiDriver.Exists()).To(BeFalse())

			// CSI ServiceAccount (RBAC) must be absent.
			csiSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "csi")
			Expect(csiSA.Exists()).To(BeFalse())

			// StorageClass must be absent.
			storageClass := f.KubernetesGlobalResource("StorageClass", "1test")
			Expect(storageClass.Exists()).To(BeFalse())

			// CCM must still be present.
			ccmDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())

			// CCM RBAC must still be present.
			ccmSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "cloud-controller-manager")
			Expect(ccmSA.Exists()).To(BeTrue())

			// capdvp must still be present.
			capdvpDeployment := f.KubernetesResource("Deployment", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpDeployment.Exists()).To(BeTrue())

			// capdvp RBAC must still be present.
			capdvpSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpSA.Exists()).To(BeTrue())

			// cloud-data-discoverer must still be present.
			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())
		})
	})

	Context("DVP with nodes disabled", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesNodesDisabled)
			f.HelmRender()
		})

		It("CCM and capdvp must not render; CSI and cloud-data-discoverer must render", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// CCM Deployment must be absent.
			ccmDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeFalse())

			// CCM ServiceAccount (RBAC) must be absent.
			ccmSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "cloud-controller-manager")
			Expect(ccmSA.Exists()).To(BeFalse())

			// capdvp Deployment must be absent.
			capdvpDeployment := f.KubernetesResource("Deployment", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpDeployment.Exists()).To(BeFalse())

			// capdvp ServiceAccount (RBAC) must be absent.
			capdvpSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpSA.Exists()).To(BeFalse())

			// CSI controller must still be present.
			csiController := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			Expect(csiController.Exists()).To(BeTrue())

			// CSIDriver CR must still be present.
			csiDriver := f.KubernetesGlobalResource("CSIDriver", "csi.dvp.deckhouse.io")
			Expect(csiDriver.Exists()).To(BeTrue())

			// CSI RBAC must still be present.
			csiSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "csi")
			Expect(csiSA.Exists()).To(BeTrue())

			// cloud-data-discoverer must still be present.
			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())
		})
	})

	Context("DVP with both storage and nodes disabled", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesBothDisabled)
			f.HelmRender()
		})

		It("Only cloud-data-discoverer and common artifacts must render", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// CSI must be absent.
			csiController := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			Expect(csiController.Exists()).To(BeFalse())

			csiNode := f.KubernetesResource("DaemonSet", moduleNamespace, "csi-node")
			Expect(csiNode.Exists()).To(BeFalse())

			csiDriver := f.KubernetesGlobalResource("CSIDriver", "csi.dvp.deckhouse.io")
			Expect(csiDriver.Exists()).To(BeFalse())

			csiSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "csi")
			Expect(csiSA.Exists()).To(BeFalse())

			// CCM must be absent.
			ccmDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeFalse())

			ccmSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "cloud-controller-manager")
			Expect(ccmSA.Exists()).To(BeFalse())

			// capdvp must be absent.
			capdvpDeployment := f.KubernetesResource("Deployment", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpDeployment.Exists()).To(BeFalse())

			capdvpSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpSA.Exists()).To(BeFalse())

			// cloud-data-discoverer must be present.
			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())

			// Namespace must be present.
			namespace := f.KubernetesGlobalResource("Namespace", moduleNamespace)
			Expect(namespace.Exists()).To(BeTrue())

			// Registration secret must be present.
			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())

			// User-authz ClusterRole must be present.
			userAuthzUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-dvp:user")
			Expect(userAuthzUser.Exists()).To(BeTrue())
		})
	})

	Context("DVP with storage disabled", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesStorageDisabled)
			f.HelmRender()
		})

		It("CSI components must not render; CCM, capdvp, and cloud-data-discoverer must render", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// CSI controller Deployment must be absent.
			csiController := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			Expect(csiController.Exists()).To(BeFalse())

			// CSI node DaemonSet must be absent.
			csiNode := f.KubernetesResource("DaemonSet", moduleNamespace, "csi-node")
			Expect(csiNode.Exists()).To(BeFalse())

			// CSIDriver CR must be absent.
			csiDriver := f.KubernetesGlobalResource("CSIDriver", "csi.dvp.deckhouse.io")
			Expect(csiDriver.Exists()).To(BeFalse())

			// CSI ServiceAccount (RBAC) must be absent.
			csiSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "csi")
			Expect(csiSA.Exists()).To(BeFalse())

			// StorageClass must be absent.
			storageClass := f.KubernetesGlobalResource("StorageClass", "1test")
			Expect(storageClass.Exists()).To(BeFalse())

			// CCM must still be present.
			ccmDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())

			// CCM RBAC must still be present.
			ccmSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "cloud-controller-manager")
			Expect(ccmSA.Exists()).To(BeTrue())

			// capdvp must still be present.
			capdvpDeployment := f.KubernetesResource("Deployment", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpDeployment.Exists()).To(BeTrue())

			// capdvp RBAC must still be present.
			capdvpSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpSA.Exists()).To(BeTrue())

			// cloud-data-discoverer must still be present.
			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())
		})
	})

	Context("DVP with nodes disabled", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesNodesDisabled)
			f.HelmRender()
		})

		It("CCM and capdvp must not render; CSI and cloud-data-discoverer must render", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// CCM Deployment must be absent.
			ccmDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeFalse())

			// CCM ServiceAccount (RBAC) must be absent.
			ccmSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "cloud-controller-manager")
			Expect(ccmSA.Exists()).To(BeFalse())

			// capdvp Deployment must be absent.
			capdvpDeployment := f.KubernetesResource("Deployment", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpDeployment.Exists()).To(BeFalse())

			// capdvp ServiceAccount (RBAC) must be absent.
			capdvpSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpSA.Exists()).To(BeFalse())

			// CSI controller must still be present.
			csiController := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			Expect(csiController.Exists()).To(BeTrue())

			// CSIDriver CR must still be present.
			csiDriver := f.KubernetesGlobalResource("CSIDriver", "csi.dvp.deckhouse.io")
			Expect(csiDriver.Exists()).To(BeTrue())

			// CSI RBAC must still be present.
			csiSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "csi")
			Expect(csiSA.Exists()).To(BeTrue())

			// cloud-data-discoverer must still be present.
			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())
		})
	})

	Context("DVP with both storage and nodes disabled", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesBothDisabled)
			f.HelmRender()
		})

		It("Only cloud-data-discoverer and common artifacts must render", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// CSI must be absent.
			csiController := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			Expect(csiController.Exists()).To(BeFalse())

			csiNode := f.KubernetesResource("DaemonSet", moduleNamespace, "csi-node")
			Expect(csiNode.Exists()).To(BeFalse())

			csiDriver := f.KubernetesGlobalResource("CSIDriver", "csi.dvp.deckhouse.io")
			Expect(csiDriver.Exists()).To(BeFalse())

			csiSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "csi")
			Expect(csiSA.Exists()).To(BeFalse())

			// CCM must be absent.
			ccmDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeFalse())

			ccmSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "cloud-controller-manager")
			Expect(ccmSA.Exists()).To(BeFalse())

			// capdvp must be absent.
			capdvpDeployment := f.KubernetesResource("Deployment", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpDeployment.Exists()).To(BeFalse())

			capdvpSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpSA.Exists()).To(BeFalse())

			// cloud-data-discoverer must be present.
			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())

			// Namespace must be present.
			namespace := f.KubernetesGlobalResource("Namespace", moduleNamespace)
			Expect(namespace.Exists()).To(BeTrue())

			// Registration secret must be present.
			providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")
			Expect(providerRegistrationSecret.Exists()).To(BeTrue())

			// User-authz ClusterRole must be present.
			userAuthzUser := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-dvp:user")
			Expect(userAuthzUser.Exists()).To(BeTrue())
		})
	})

	Context("DVP with CCM disabled", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesCCMDisabled)
			f.HelmRender()
		})

		It("CCM Deployment must not render; capdvp, CSI, and cloud-data-discoverer must render", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// CCM Deployment must be absent.
			ccmDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeFalse())

			// capdvp must still be present (guarded only by nodes.disabled).
			capdvpDeployment := f.KubernetesResource("Deployment", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpDeployment.Exists()).To(BeTrue())

			capdvpSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpSA.Exists()).To(BeTrue())

			// CSI must still be present.
			csiController := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			Expect(csiController.Exists()).To(BeTrue())

			csiDriver := f.KubernetesGlobalResource("CSIDriver", "csi.dvp.deckhouse.io")
			Expect(csiDriver.Exists()).To(BeTrue())

			csiSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "csi")
			Expect(csiSA.Exists()).To(BeTrue())

			// cloud-data-discoverer must still be present.
			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())
		})
	})

	Context("DVP without disabled fields (all subsystems default-enabled)", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesNoDisabled)
			f.HelmRender()
		})

		It("Must render properly with all subsystems enabled by default", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// All subsystems must render when disabled is absent (defaults to false).
			csiController := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			Expect(csiController.Exists()).To(BeTrue())

			csiNode := f.KubernetesResource("DaemonSet", moduleNamespace, "csi-node")
			Expect(csiNode.Exists()).To(BeTrue())

			csiDriver := f.KubernetesGlobalResource("CSIDriver", "csi.dvp.deckhouse.io")
			Expect(csiDriver.Exists()).To(BeTrue())

			ccmDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())

			capdvpDeployment := f.KubernetesResource("Deployment", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpDeployment.Exists()).To(BeTrue())

			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())
		})
	})

	Context("DVP without storage section (storage defaults to enabled)", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesNoStorageSection)
			f.HelmRender()
		})

		It("Must render properly with storage enabled by default when section is absent", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// CSI must render because storage.disabled defaults to false even when section is absent.
			csiController := f.KubernetesResource("Deployment", moduleNamespace, "csi-controller")
			Expect(csiController.Exists()).To(BeTrue())

			csiDriver := f.KubernetesGlobalResource("CSIDriver", "csi.dvp.deckhouse.io")
			Expect(csiDriver.Exists()).To(BeTrue())

			csiSA := f.KubernetesResource("ServiceAccount", moduleNamespace, "csi")
			Expect(csiSA.Exists()).To(BeTrue())

			ccmDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())

			capdvpDeployment := f.KubernetesResource("Deployment", moduleNamespace, "capdvp-controller-manager")
			Expect(capdvpDeployment.Exists()).To(BeTrue())

			cddDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-data-discoverer")
			Expect(cddDeployment.Exists()).To(BeTrue())
		})
	})

	Context("DVP without ccm section", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesWithoutCCM)
			f.HelmRender()
		})

		It("must render CCM Deployment when ccm section is omitted", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())
		})
	})

	Context("validation webhook", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesA)
			f.HelmRender()
		})

		It("renders deployment without hostNetwork when cluster is bootstrapped", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			deploy := f.KubernetesResource("Deployment", moduleNamespace, validationWebhookName)
			Expect(deploy.Exists()).To(BeTrue())
			Expect(deploy.Field("spec.template.spec.hostNetwork").Exists()).To(BeFalse())
			Expect(deploy.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").Exists()).To(BeFalse())
			Expect(deploy.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(deploy.Field("spec.template.spec.tolerations").String()).To(MatchYAML(tolerationsAnyNodeWithUninitialized))
			Expect(deploy.Field("spec.template.spec.serviceAccountName").String()).To(Equal(validationWebhookName))

			containers := deploy.Field("spec.template.spec.containers").Array()
			Expect(containers).To(HaveLen(1))
			Expect(deploy.Field("spec.template.spec.containers.0.name").String()).To(Equal(validationWebhookName))
			for _, arg := range validationWebhookArgs {
				Expect(deploy.Field("spec.template.spec.containers.0.args").String()).To(ContainSubstring(arg))
			}
			Expect(deploy.Field("spec.template.spec.containers.0.ports.0.containerPort").Int()).To(Equal(int64(4330)))
			Expect(deploy.Field("spec.template.spec.containers.0.ports.0.name").String()).To(Equal("webhook-server"))
			Expect(deploy.Field("spec.template.spec.containers.0.volumeMounts.0.name").String()).To(Equal("cert"))
			Expect(deploy.Field("spec.template.spec.volumes.0.secret.secretName").String()).To(Equal("validation-webhook-tls"))
		})

		It("renders deployment with hostNetwork during bootstrap", func() {
			f.ValuesSet("global.clusterIsBootstrapped", false)
			f.ValuesSet("global.enabledModules", []string{
				"vertical-pod-autoscaler",
				"vertical-pod-autoscaler-crd",
				"admission-policy-engine-crd",
				"cloud-provider-dvp",
			})
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			deploy := f.KubernetesResource("Deployment", moduleNamespace, validationWebhookName)
			Expect(deploy.Field("spec.template.spec.hostNetwork").Bool()).To(BeTrue())
			Expect(deploy.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").String()).
				To(Equal(validationWebhookName))
			Expect(deploy.Field("spec.template.spec.dnsPolicy").String()).To(Equal("Default"))
		})

		It("renders VPA and PDB", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			vpa := f.KubernetesResource("VerticalPodAutoscaler", moduleNamespace, validationWebhookName)
			Expect(vpa.Exists()).To(BeTrue())
			Expect(vpa.Field("spec.targetRef.kind").String()).To(Equal("Deployment"))
			Expect(vpa.Field("spec.targetRef.name").String()).To(Equal(validationWebhookName))
			Expect(vpa.Field("spec.resourcePolicy.containerPolicies.0.containerName").String()).To(Equal(validationWebhookName))

			pdb := f.KubernetesResource("PodDisruptionBudget", moduleNamespace, validationWebhookName)
			Expect(pdb.Exists()).To(BeTrue())
			Expect(pdb.Field("spec.maxUnavailable").Int()).To(Equal(int64(1)))
		})

		It("renders Service and TLS Secret", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			svc := f.KubernetesResource("Service", moduleNamespace, validationWebhookName)
			Expect(svc.Exists()).To(BeTrue())
			Expect(svc.Field("spec.ports.0.port").Int()).To(Equal(int64(443)))
			Expect(svc.Field("spec.ports.0.targetPort").String()).To(Equal("webhook-server"))
			Expect(svc.Field("spec.selector.app").String()).To(Equal(validationWebhookName))

			secret := f.KubernetesResource("Secret", moduleNamespace, "validation-webhook-tls")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field("type").String()).To(Equal("kubernetes.io/tls"))
			Expect(secret.Field("data.tls\\.crt").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("dGVzdC1jcnQ="))))
			Expect(secret.Field("data.tls\\.key").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("dGVzdC1rZXk="))))
			Expect(secret.Field("data.ca\\.crt").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("dGVzdC1jYQ=="))))
		})

		It("renders ValidatingWebhookConfiguration with three webhooks", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			vwc := f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "d8-cloud-provider-dvp-validation-webhook")
			Expect(vwc.Exists()).To(BeTrue())

			webhooks := vwc.Field("webhooks").Array()
			Expect(webhooks).To(HaveLen(3))

			expectedCA := base64.StdEncoding.EncodeToString([]byte("dGVzdC1jYQ=="))
			for i := range webhooks {
				index := fmt.Sprintf("%d", i)
				Expect(vwc.Field("webhooks." + index + ".failurePolicy").String()).To(Equal("Fail"))
				Expect(vwc.Field("webhooks." + index + ".clientConfig.service.name").String()).To(Equal(validationWebhookName))
				Expect(vwc.Field("webhooks." + index + ".clientConfig.service.namespace").String()).To(Equal(moduleNamespace))
				Expect(vwc.Field("webhooks." + index + ".clientConfig.service.port").Int()).To(Equal(int64(443)))
				Expect(vwc.Field("webhooks." + index + ".clientConfig.caBundle").String()).To(Equal(expectedCA))
			}

			Expect(vwc.Field("webhooks.0.name").String()).To(Equal("secrets.cloud-provider-dvp.deckhouse.io"))
			Expect(vwc.Field("webhooks.0.clientConfig.service.path").String()).To(Equal("/validate--v1-secret"))
			Expect(vwc.Field("webhooks.0.namespaceSelector.matchLabels.kubernetes\\.io/metadata\\.name").String()).To(Equal(moduleNamespace))

			Expect(vwc.Field("webhooks.1.name").String()).To(Equal("nodegroups.cloud-provider-dvp.deckhouse.io"))
			Expect(vwc.Field("webhooks.1.clientConfig.service.path").String()).To(Equal("/validate-deckhouse-io-v1-nodegroup"))

			Expect(vwc.Field("webhooks.2.name").String()).To(Equal("dvpinstanceclasses.cloud-provider-dvp.deckhouse.io"))
			Expect(vwc.Field("webhooks.2.clientConfig.service.path").String()).To(Equal("/validate-deckhouse-io-v1alpha1-dvpinstanceclass"))
		})

		It("renders RBAC for validation webhook", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("ServiceAccount", moduleNamespace, validationWebhookName).Exists()).To(BeTrue())

			role := f.KubernetesGlobalResource("ClusterRole", "d8:cloud-provider-dvp:validation-webhook")
			Expect(role.Exists()).To(BeTrue())
			Expect(role.Field("rules").String()).To(MatchYAML(`
- apiGroups:
  - deckhouse.io
  resources:
  - nodegroups
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - deckhouse.io
  resources:
  - dvpinstanceclasses
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - secrets
  - configmaps
  verbs:
  - get
  - list
  - watch`))

			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cloud-provider-dvp:validation-webhook").Exists()).To(BeTrue())
		})

		It("does not render SecurityPolicyException without admission-policy-engine-crd", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			deploy := f.KubernetesResource("Deployment", moduleNamespace, validationWebhookName)
			Expect(deploy.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("SecurityPolicyException", moduleNamespace, validationWebhookName).Exists()).To(BeFalse())
		})
	})

	Context("validation webhook :: admission-policy-engine compatibility", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.clusterIsBootstrapped", false)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesA)
			f.ValuesSet("global.enabledModules", []string{
				"vertical-pod-autoscaler",
				"vertical-pod-autoscaler-crd",
				"admission-policy-engine-crd",
				"cloud-provider-dvp",
			})
			f.HelmRender()
		})

		It("renders SecurityPolicyException for hostNetwork and webhook port", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			deploy := f.KubernetesResource("Deployment", moduleNamespace, validationWebhookName)
			Expect(deploy.Field("spec.template.spec.hostNetwork").Bool()).To(BeTrue())
			Expect(deploy.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").String()).
				To(Equal(validationWebhookName))

			spe := f.KubernetesResource("SecurityPolicyException", moduleNamespace, validationWebhookName)
			Expect(spe.Exists()).To(BeTrue())
			Expect(spe.Field("metadata.namespace").String()).To(Equal(moduleNamespace))
			Expect(spe.Field("spec.network.hostNetwork.allowedValue").Bool()).To(BeTrue())
			Expect(spe.Field("spec.network.hostPorts.0.port").Int()).To(Equal(int64(4330)))
			Expect(spe.Field("spec.network.hostPorts.0.protocol").String()).To(Equal("TCP"))
		})

		It("does not render SecurityPolicyException after cluster bootstrap", func() {
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			deploy := f.KubernetesResource("Deployment", moduleNamespace, validationWebhookName)
			Expect(deploy.Field("spec.template.spec.hostNetwork").Exists()).To(BeFalse())
			Expect(deploy.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("SecurityPolicyException", moduleNamespace, validationWebhookName).Exists()).To(BeFalse())
		})
	})

	Context("DVP with LoadBalancer enabled (default)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesA)
			f.ValuesSet("cloudProviderDvp.internal.loadBalancer.disabled", false)
			f.HelmRender()
		})

		It("Should include service-lb-controller in the cloud-controller-manager controllers", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field(`spec.template.spec.containers.0.args`).String()).
				To(ContainSubstring(`--controllers=cloud-node,cloud-node-lifecycle,service-lb-controller`))
		})
	})

	Context("DVP without sshCAKeys (default)", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesA)
			f.HelmRender()
		})

		It("must not render the SSH CA NodeGroupConfiguration", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ngc := f.KubernetesGlobalResource("NodeGroupConfiguration", "dvp-ssh-ca-trust.sh")
			Expect(ngc.Exists()).To(BeFalse())
		})
	})

	Context("DVP with sshCAKeys set", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesWithCACerts)
			f.HelmRender()
		})

		It("must render the SSH CA NodeGroupConfiguration for all node groups/bundles", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ngc := f.KubernetesGlobalResource("NodeGroupConfiguration", "dvp-ssh-ca-trust.sh")
			Expect(ngc.Exists()).To(BeTrue())
			Expect(ngc.Field("spec.nodeGroups").String()).To(MatchYAML(`["*"]`))
			Expect(ngc.Field("spec.bundles").String()).To(MatchYAML(`["*"]`))

			content := ngc.Field("spec.content").String()
			Expect(content).To(ContainSubstring("ssh-rsa-ca-AAAA-fake-ca-key-1"))
			Expect(content).To(ContainSubstring("ssh-rsa-ca-AAAA-fake-ca-key-2"))
			Expect(content).To(ContainSubstring("/etc/ssh/trusted-user-ca-keys.pem"))
			Expect(content).To(ContainSubstring("TrustedUserCAKeys /etc/ssh/trusted-user-ca-keys.pem"))
			Expect(content).To(ContainSubstring("sshd -t && systemctl reload ssh"))
		})
	})

	Context("DVP with sshCAKeys containing a heredoc-breakout payload", func() {
		// Must never reach a rendered NodeGroupConfiguration at all: there is
		// no schema-level constraint that could catch this (a CA public key
		// has no fixed regex the way a user name does), so
		// templates/ngc-ssh-ca.yaml's own guard is the only line of defense.
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesWithHeredocBreakoutCACert)
			f.HelmRender()
		})

		It("must fail the whole render instead of ever emitting a line that collides with the heredoc delimiter", func() {
			Expect(f.RenderError).Should(HaveOccurred())
			Expect(f.RenderError.Error()).To(ContainSubstring("is not a valid single-line SSH CA public key"))
		})
	})

	Context("DVP without additionalUsers (default)", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesA)
			f.HelmRender()
		})

		It("must not render the additional-users NodeGroupConfiguration", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ngc := f.KubernetesGlobalResource("NodeGroupConfiguration", "dvp-additional-users.sh")
			Expect(ngc.Exists()).To(BeFalse())
		})
	})

	Context("DVP with additionalUsers set", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesWithAdditionalUsers)
			f.HelmRender()
		})

		It("must render the additional-users NodeGroupConfiguration for all node groups/bundles", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ngc := f.KubernetesGlobalResource("NodeGroupConfiguration", "dvp-additional-users.sh")
			Expect(ngc.Exists()).To(BeTrue())
			Expect(ngc.Field("spec.nodeGroups").String()).To(MatchYAML(`["*"]`))
			Expect(ngc.Field("spec.bundles").String()).To(MatchYAML(`["*"]`))

			content := ngc.Field("spec.content").String()
			Expect(content).To(ContainSubstring(`id -u "alice"`))
			Expect(content).To(ContainSubstring(`useradd --create-home --shell /bin/bash "alice"`))
			Expect(content).To(ContainSubstring("/etc/sudoers.d/90-0"))
			Expect(content).To(ContainSubstring("alice ALL=(ALL) NOPASSWD:ALL"))
			Expect(content).To(ContainSubstring(`id -u "s.bob"`))
			Expect(content).To(ContainSubstring("/etc/sudoers.d/90-1"))
			Expect(content).To(ContainSubstring("s.bob ALL=(ALL) NOPASSWD:ALL"))

			// Filename must be index-based, not name-based: sudo's
			// #includedir skips files with a "." in the name, which would
			// silently disable sudo for a dotted name like "s.bob".
			Expect(content).ToNot(ContainSubstring("/etc/sudoers.d/90-alice"))
			Expect(content).ToNot(ContainSubstring("/etc/sudoers.d/90-s.bob"))
			Expect(content).ToNot(ContainSubstring(`/etc/sudoers.d/90-"`))

			// No group membership must ever be granted here: "sudo" (Debian)
			// vs "wheel" (RHEL/SUSE/AltLinux) differs per OS family, and the
			// per-user sudoers.d entry above already grants access on its own.
			Expect(content).ToNot(ContainSubstring("usermod"))
			Expect(content).ToNot(ContainSubstring(`groups`))

			// Must refuse to grant sudo to an existing system account (e.g. if
			// someone lists "root" in additionalUsers by mistake) - guarded by
			// UID, not by "already exists", since useradd is a no-op for an
			// existing account either way.
			Expect(content).To(ContainSubstring(`-lt 1000`))
			Expect(content).To(ContainSubstring("refusing to manage"))
		})
	})

	Context("DVP with additionalUsers containing a reserved system account name", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesWithReservedAdditionalUser)
			f.HelmRender()
		})

		It("must render a UID-guarded refusal for root instead of an unconditional sudoers grant", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ngc := f.KubernetesGlobalResource("NodeGroupConfiguration", "dvp-additional-users.sh")
			Expect(ngc.Exists()).To(BeTrue())

			content := ngc.Field("spec.content").String()
			// The sudoers grant is only reachable through the UID-guarded
			// else-branch - assert the exact guard sequence around "root" as
			// one contiguous block, so a refactor that makes the grant
			// unconditional (or drops the guard) fails this test.
			Expect(content).To(ContainSubstring(
				"_DVP_UID=\"$(id -u \"root\" 2>/dev/null || true)\"\n" +
					"if [ -n \"$_DVP_UID\" ] && [ \"$_DVP_UID\" -lt 1000 ]; then\n" +
					"  echo \"dvp-additional-users: refusing to manage root - it is an existing system account (uid=$_DVP_UID), not one we created\" >&2\n" +
					"else\n",
			))
		})
	})

	Context("DVP with additionalUsers containing a shell command-injection payload", func() {
		// config-values.yaml can't carry a `pattern` here (see the dev-note
		// on NodesParameters.AdditionalUsers), so ValidateValues() lets this
		// through - confirm that, then confirm the Helm template's `fail`
		// guard is the actual boundary.
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesWithMaliciousAdditionalUser)
		})

		It("passes schema validation but is rejected by the Helm template guard before any rendering happens", func() {
			Expect(f.ValidateValues()).ShouldNot(HaveOccurred())

			f.HelmRender()
			Expect(f.RenderError).Should(HaveOccurred())
			Expect(f.RenderError.Error()).To(ContainSubstring("is not a valid Linux user name"))
		})
	})

	Context("DVP with additionalUsers containing the reserved \"default\" name", func() {
		f := SetupHelmConfig(``)

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesWithReservedDefaultAdditionalUser)
			f.HelmRender()
		})

		It("must fail the whole render instead of creating an unrelated user literally named \"default\"", func() {
			Expect(f.RenderError).Should(HaveOccurred())
			Expect(f.RenderError.Error()).To(ContainSubstring("is a reserved cloud-init keyword"))
		})
	})

	Context("DVP with LoadBalancer disabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cloudProviderDvp", moduleValuesA)
			f.ValuesSet("cloudProviderDvp.internal.loadBalancer.disabled", true)
			f.HelmRender()
		})

		It("Should exclude service-lb-controller from the cloud-controller-manager controllers", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ccmDeployment := f.KubernetesResource("Deployment", moduleNamespace, "cloud-controller-manager")
			Expect(ccmDeployment.Exists()).To(BeTrue())
			Expect(ccmDeployment.Field(`spec.template.spec.containers.0.args`).String()).
				To(ContainSubstring(`--controllers=cloud-node,cloud-node-lifecycle`))
			Expect(ccmDeployment.Field(`spec.template.spec.containers.0.args`).String()).
				ToNot(ContainSubstring(`service-lb-controller`))
		})
	})
})

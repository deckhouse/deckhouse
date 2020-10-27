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
modules:
  placement: {}
modulesImages:
  registry: registry.flant.com
  registryDockercfg: cfg
  tags:
    common:
      kubeRbacProxy: tagstring
    certManager:
      certManagerController: tagstring
      certManagerWebhook: tagstring
      certManagerCainjector: tagstring
discovery:
  clusterMasterCount: 1
  clusterUUID: f49dd1c3-a63a-4565-a06c-625e35587eab
  clusterVersion: 1.15.4
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
`

const globalValuesHa = `
enabledModules: ["vertical-pod-autoscaler-crd"]
modules:
  placement: {}
modulesImages:
  registry: registry.flant.com
  registryDockercfg: cfg
  tags:
    common:
      kubeRbacProxy: tagstring
    certManager:
      certManagerController: tagstring
      certManagerWebhook: tagstring
      certManagerCainjector: tagstring
discovery:
  clusterMasterCount: 5
  clusterControlPlaneIsHighlyAvailable: true
  clusterUUID: f49dd1c3-a63a-4565-a06c-625e35587eab
  clusterVersion: 1.15.4
  d8SpecificNodeCountByRole:
    system: 3
    master: 1
`

const globalValuesManaged = `
enabledModules: ["vertical-pod-autoscaler-crd"]
modules:
  placement: {}
modulesImages:
  registry: registry.flant.com
  registryDockercfg: cfg
  tags:
    common:
      kubeRbacProxy: tagstring
    certManager:
      certManagerController: tagstring
      certManagerWebhook: tagstring
      certManagerCainjector: tagstring
discovery:
  clusterUUID: f49dd1c3-a63a-4565-a06c-625e35587eab
  clusterVersion: 1.15.4
  d8SpecificNodeCountByRole:
    master: 1
    system: 3
`

const globalValuesManagedHa = `
highAvailability: true
modules:
  placement: {}
enabledModules: ["vertical-pod-autoscaler-crd"]
modulesImages:
  registry: registry.flant.com
  registryDockercfg: cfg
  tags:
    common:
      kubeRbacProxy: tagstring
    certManager:
      certManagerController: tagstring
      certManagerWebhook: tagstring
      certManagerCainjector: tagstring
discovery:
  clusterUUID: f49dd1c3-a63a-4565-a06c-625e35587eab
  clusterVersion: 1.15.4
  d8SpecificNodeCountByRole:
    master: 3
    system: 3
`

const certManager = `
internal:
  selfSignedCA:
    cert: string
    key: string
`

const cloudDNS = `
cloudDNSServiceAccount: ewogICJ0eXBlIjogInNlcnZpY2VfYWNjb3VudCIsCiAgInByb2plY3RfaWQiOiAicHJvamVjdC0yMDkzMTciLAogICJwcml2YXRlX2tleV9pZCI6ICJwcml2YXRlX2lkIiwKICAicHJpdmF0ZV9rZXkiOiAicHJpdmF0ZV9rZXkiLAogICJjbGllbnRfZW1haWwiOiAiZG5zMDEtc29sdmVyQHByb2plY3QtMjA5MzE3LmlhbS5nc2VydmljZWFjY291bnQuY29tIiwKICAiY2xpZW50X2lkIjogIjExNzM1MzAzMzgzOTQ2NTUzNjY3MiIsCiAgImF1dGhfdXJpIjogImh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbS9vL29hdXRoMi9hdXRoIiwKICAidG9rZW5fdXJpIjogImh0dHBzOi8vb2F1dGgyLmdvb2dsZWFwaXMuY29tL3Rva2VuIiwKICAiYXV0aF9wcm92aWRlcl94NTA5X2NlcnRfdXJsIjogImh0dHBzOi8vd3d3Lmdvb2dsZWFwaXMuY29tL29hdXRoMi92MS9jZXJ0cyIsCiAgImNsaWVudF94NTA5X2NlcnRfdXJsIjogImh0dHBzOi8vd3d3Lmdvb2dsZWFwaXMuY29tL3JvYm90L3YxL21ldGFkYXRhL3g1MDkvZG5zMDEtc29sdmVyJXByb2plY3QtMjA5MzE3LmlhbS5nc2VydmljZWFjY291bnQuY29tIgp9Cg==
`

var _ = Describe("Module :: cert-manager :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Default", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("certManager", certManager)
			f.HelmRender()
		})

		It("Everything must render properly for default cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cert-manager")
			registrySecret := f.KubernetesResource("Secret", "d8-cert-manager", "deckhouse-registry")

			cainjector := f.KubernetesResource("Deployment", "d8-cert-manager", "cainjector")
			cert_manager := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())

			Expect(cainjector.Exists()).To(BeTrue())
			Expect(cainjector.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.kubernetes.io/master\":\"\"}"))
			Expect(cainjector.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: node-role.kubernetes.io/master
- effect: NoSchedule
  key: node.deckhouse.io/uninitialized
  operator: Exists
- key: dedicated.deckhouse.io
  operator: Exists
- key: dedicated
  operator: Exists
- key: node.kubernetes.io/not-ready
- key: node.kubernetes.io/out-of-disk
- key: node.kubernetes.io/memory-pressure
- key: node.kubernetes.io/disk-pressure
`))
			Expect(cainjector.Field("spec.replicas").Int()).To(BeEquivalentTo(1))
			Expect(cainjector.Field("spec.strategy").Exists()).To(BeTrue())
			Expect(cainjector.Field("spec.template.spec.affinity").Exists()).To(BeFalse())

			Expect(cert_manager.Exists()).To(BeTrue())
			Expect(cert_manager.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.deckhouse.io/system\":\"\"}"))
			Expect(cert_manager.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: dedicated.flant.com
  operator: Equal
  value: "cert-manager"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "cert-manager"
- key: dedicated.flant.com
  operator: Equal
  value: "system"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
`))
			Expect(cert_manager.Field("spec.replicas").Int()).To(BeEquivalentTo(1))
			Expect(cert_manager.Field("spec.strategy").Exists()).To(BeFalse())
			Expect(cert_manager.Field("spec.template.spec.affinity").Exists()).To(BeFalse())
		})
	})

	Context("DefaultHA", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesHa)
			f.ValuesSetFromYaml("certManager", certManager)
			f.HelmRender()
		})

		It("Everything must render properly for default cluster with ha", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cert-manager")
			registrySecret := f.KubernetesResource("Secret", "d8-cert-manager", "deckhouse-registry")

			cainjector := f.KubernetesResource("Deployment", "d8-cert-manager", "cainjector")
			cert_manager := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(cainjector.Exists()).To(BeTrue())
			Expect(cainjector.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.kubernetes.io/master\":\"\"}"))
			Expect(cainjector.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: node-role.kubernetes.io/master
- effect: NoSchedule
  key: node.deckhouse.io/uninitialized
  operator: Exists
- key: dedicated.deckhouse.io
  operator: Exists
- key: dedicated
  operator: Exists
- key: node.kubernetes.io/not-ready
- key: node.kubernetes.io/out-of-disk
- key: node.kubernetes.io/memory-pressure
- key: node.kubernetes.io/disk-pressure
`))
			Expect(cainjector.Field("spec.replicas").Int()).To(BeEquivalentTo(5))
			Expect(cainjector.Field("spec.strategy").String()).To(MatchYAML(`
type: RollingUpdate
rollingUpdate:
  maxSurge: 0
  maxUnavailable: 2
`))
			Expect(cainjector.Field("spec.template.spec.affinity").String()).To(MatchYAML(`
podAntiAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
  - labelSelector:
      matchLabels:
        app: cainjector
    topologyKey: kubernetes.io/hostname
`))
			Expect(cert_manager.Exists()).To(BeTrue())
			Expect(cert_manager.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.deckhouse.io/system\":\"\"}"))
			Expect(cert_manager.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: dedicated.flant.com
  operator: Equal
  value: "cert-manager"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "cert-manager"
- key: dedicated.flant.com
  operator: Equal
  value: "system"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
`))
			Expect(cert_manager.Field("spec.replicas").Int()).To(BeEquivalentTo(2))
			Expect(cert_manager.Field("spec.strategy").String()).To(MatchYAML(`
type: RollingUpdate
rollingUpdate:
  maxSurge: 0
  maxUnavailable: 1
`))
			Expect(cert_manager.Field("spec.template.spec.affinity").String()).To(MatchYAML(`
podAntiAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
  - labelSelector:
      matchLabels:
        app: cert-manager
    topologyKey: kubernetes.io/hostname
`))
		})
	})

	Context("Managed", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesManaged)
			f.ValuesSetFromYaml("certManager", certManager)
			f.HelmRender()
		})

		It("Everything must render properly for managed cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cert-manager")
			registrySecret := f.KubernetesResource("Secret", "d8-cert-manager", "deckhouse-registry")

			cainjector := f.KubernetesResource("Deployment", "d8-cert-manager", "cainjector")
			cert_manager := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(cainjector.Exists()).To(BeTrue())
			Expect(cainjector.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.deckhouse.io/master\":\"\"}"))
			Expect(cainjector.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: node-role.kubernetes.io/master
- effect: NoSchedule
  key: node.deckhouse.io/uninitialized
  operator: Exists
- key: dedicated.deckhouse.io
  operator: Exists
- key: dedicated
  operator: Exists
- key: node.kubernetes.io/not-ready
- key: node.kubernetes.io/out-of-disk
- key: node.kubernetes.io/memory-pressure
- key: node.kubernetes.io/disk-pressure
`))
			Expect(cainjector.Field("spec.replicas").Int()).To(BeEquivalentTo(1))
			Expect(cainjector.Field("spec.strategy").Exists()).To(BeTrue())
			Expect(cainjector.Field("spec.template.spec.affinity").Exists()).To(BeFalse())

			Expect(cert_manager.Exists()).To(BeTrue())
			Expect(cert_manager.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.deckhouse.io/system\":\"\"}"))
			Expect(cert_manager.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: dedicated.flant.com
  operator: Equal
  value: "cert-manager"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "cert-manager"
- key: dedicated.flant.com
  operator: Equal
  value: "system"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
`))
			Expect(cert_manager.Field("spec.replicas").Int()).To(BeEquivalentTo(1))
			Expect(cert_manager.Field("spec.strategy").Exists()).To(BeFalse())
			Expect(cert_manager.Field("spec.template.spec.affinity").Exists()).To(BeFalse())
		})
	})

	Context("ManagedHa", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesManagedHa)
			f.ValuesSetFromYaml("certManager", certManager)
			f.HelmRender()
		})

		It("Everything must render properly for managed cluster with ha", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cert-manager")
			registrySecret := f.KubernetesResource("Secret", "d8-cert-manager", "deckhouse-registry")

			cainjector := f.KubernetesResource("Deployment", "d8-cert-manager", "cainjector")
			cert_manager := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(cainjector.Exists()).To(BeTrue())
			Expect(cainjector.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.deckhouse.io/master\":\"\"}"))
			Expect(cainjector.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: node-role.kubernetes.io/master
- effect: NoSchedule
  key: node.deckhouse.io/uninitialized
  operator: Exists
- key: dedicated.deckhouse.io
  operator: Exists
- key: dedicated
  operator: Exists
- key: node.kubernetes.io/not-ready
- key: node.kubernetes.io/out-of-disk
- key: node.kubernetes.io/memory-pressure
- key: node.kubernetes.io/disk-pressure
`))
			Expect(cainjector.Field("spec.replicas").Int()).To(BeEquivalentTo(3))
			Expect(cainjector.Field("spec.strategy").String()).To(MatchYAML(`
type: RollingUpdate
rollingUpdate:
  maxSurge: 0
  maxUnavailable: 2
`))
			Expect(cainjector.Field("spec.template.spec.affinity").String()).To(MatchYAML(`
podAntiAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
  - labelSelector:
      matchLabels:
        app: cainjector
    topologyKey: kubernetes.io/hostname
`))
			Expect(cert_manager.Exists()).To(BeTrue())
			Expect(cert_manager.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.deckhouse.io/system\":\"\"}"))
			Expect(cert_manager.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: dedicated.flant.com
  operator: Equal
  value: "cert-manager"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "cert-manager"
- key: dedicated.flant.com
  operator: Equal
  value: "system"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
`))
			Expect(cert_manager.Field("spec.replicas").Int()).To(BeEquivalentTo(2))
			Expect(cert_manager.Field("spec.strategy").String()).To(MatchYAML(`
type: RollingUpdate
rollingUpdate:
  maxSurge: 0
  maxUnavailable: 1
`))
			Expect(cert_manager.Field("spec.template.spec.affinity").String()).To(MatchYAML(`
podAntiAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
  - labelSelector:
      matchLabels:
        app: cert-manager
    topologyKey: kubernetes.io/hostname
`))
		})
	})

	Context("CloudDNS", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesManagedHa)
			f.ValuesSetFromYaml("certManager", certManager+cloudDNS)
			f.HelmRender()
		})

		It("Everything must render properly for CloudDNS enabled cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			secret := f.KubernetesResource("Secret", "d8-cert-manager", "clouddns")
			Expect(secret.Exists()).To(BeTrue())

			clusterIssuer := f.KubernetesResource("ClusterIssuer", "d8-cert-manager", "clouddns")
			Expect(clusterIssuer.Exists()).To(BeTrue())
			Expect(clusterIssuer.Field("spec.acme.dns01.providers.0.clouddns.project").String()).To(Equal("project-209317"))
			Expect(clusterIssuer.Field("spec.acme.dns01.providers.0.clouddns.serviceAccountSecretRef.name").String()).To(Equal("clouddns"))
			Expect(clusterIssuer.Field("spec.acme.dns01.providers.0.clouddns.serviceAccountSecretRef.key").String()).To(Equal("key.json"))

		})
	})
})

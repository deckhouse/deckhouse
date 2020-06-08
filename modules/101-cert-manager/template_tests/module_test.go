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

var _ = Describe("Module :: cert-manager :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Default", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("certManager", certManager)
			f.HelmRender()
		})

		It("Everything must render properly for default cluster", func() {
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cert-manager")
			registrySecret := f.KubernetesResource("Secret", "d8-cert-manager", "deckhouse-registry")

			cainjector := f.KubernetesResource("Deployment", "d8-cert-manager", "cainjector")
			cert_manager := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())

			Expect(cainjector.Exists()).To(BeTrue())
			Expect(cainjector.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.kubernetes.io/master\":\"\"}"))
			Expect(cainjector.Field("spec.template.spec.tolerations").String()).To(MatchJSON("[{\"operator\":\"Exists\"}]"))
			Expect(cainjector.Field("spec.replicas").Int()).To(BeEquivalentTo(1))
			Expect(cainjector.Field("spec.strategy").Exists()).To(BeFalse())
			Expect(cainjector.Field("spec.template.spec.affinity").Exists()).To(BeFalse())

			Expect(cert_manager.Exists()).To(BeTrue())
			Expect(cert_manager.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.flant.com/system\":\"\"}"))
			Expect(cert_manager.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: dedicated.flant.com
  operator: Equal
  value: "cert-manager"
- key: dedicated.flant.com
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
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cert-manager")
			registrySecret := f.KubernetesResource("Secret", "d8-cert-manager", "deckhouse-registry")

			cainjector := f.KubernetesResource("Deployment", "d8-cert-manager", "cainjector")
			cert_manager := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(cainjector.Exists()).To(BeTrue())
			Expect(cainjector.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.kubernetes.io/master\":\"\"}"))
			Expect(cainjector.Field("spec.template.spec.tolerations").String()).To(MatchJSON("[{\"operator\":\"Exists\"}]"))
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
			Expect(cert_manager.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.flant.com/system\":\"\"}"))
			Expect(cert_manager.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: dedicated.flant.com
  operator: Equal
  value: "cert-manager"
- key: dedicated.flant.com
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
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cert-manager")
			registrySecret := f.KubernetesResource("Secret", "d8-cert-manager", "deckhouse-registry")

			cainjector := f.KubernetesResource("Deployment", "d8-cert-manager", "cainjector")
			cert_manager := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(cainjector.Exists()).To(BeTrue())
			Expect(cainjector.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.flant.com/master\":\"\"}"))
			Expect(cainjector.Field("spec.template.spec.tolerations").String()).To(MatchJSON("[{\"operator\":\"Exists\"}]"))
			Expect(cainjector.Field("spec.replicas").Int()).To(BeEquivalentTo(1))
			Expect(cainjector.Field("spec.strategy").Exists()).To(BeFalse())
			Expect(cainjector.Field("spec.template.spec.affinity").Exists()).To(BeFalse())

			Expect(cert_manager.Exists()).To(BeTrue())
			Expect(cert_manager.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.flant.com/system\":\"\"}"))
			Expect(cert_manager.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: dedicated.flant.com
  operator: Equal
  value: "cert-manager"
- key: dedicated.flant.com
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
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cert-manager")
			registrySecret := f.KubernetesResource("Secret", "d8-cert-manager", "deckhouse-registry")

			cainjector := f.KubernetesResource("Deployment", "d8-cert-manager", "cainjector")
			cert_manager := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(cainjector.Exists()).To(BeTrue())
			Expect(cainjector.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.flant.com/master\":\"\"}"))
			Expect(cainjector.Field("spec.template.spec.tolerations").String()).To(MatchJSON("[{\"operator\":\"Exists\"}]"))
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
			Expect(cert_manager.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.flant.com/system\":\"\"}"))
			Expect(cert_manager.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: dedicated.flant.com
  operator: Equal
  value: "cert-manager"
- key: dedicated.flant.com
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
})

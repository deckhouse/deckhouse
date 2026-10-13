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
	"testing"

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
  kubernetesVersion: "1.19.5"
  clusterMasterCount: 1
  clusterUUID: f49dd1c3-a63a-4565-a06c-625e35587eab
  clusterVersion: 1.15.4
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
  extensionAPIServerAuthenticationRequestheaderClientCA: string
`

const globalValuesHa = `
enabledModules: ["vertical-pod-autoscaler"]
modules:
  placement: {}
discovery:
  kubernetesVersion: "1.19.5"
  clusterMasterCount: 5
  clusterControlPlaneIsHighlyAvailable: true
  clusterUUID: f49dd1c3-a63a-4565-a06c-625e35587eab
  clusterVersion: 1.15.4
  d8SpecificNodeCountByRole:
    system: 3
    master: 1
  extensionAPIServerAuthenticationRequestheaderClientCA: string
`

const globalValuesManaged = `
enabledModules: ["vertical-pod-autoscaler"]
modules:
  placement: {}
discovery:
  kubernetesVersion: "1.19.5"
  clusterUUID: f49dd1c3-a63a-4565-a06c-625e35587eab
  clusterVersion: 1.15.4
  d8SpecificNodeCountByRole:
    master: 1
    system: 3
  extensionAPIServerAuthenticationRequestheaderClientCA: string
`

const globalValuesManagedHa = `
highAvailability: true
modules:
  placement: {}
enabledModules: ["vertical-pod-autoscaler"]
discovery:
  kubernetesVersion: "1.19.5"
  clusterUUID: f49dd1c3-a63a-4565-a06c-625e35587eab
  clusterVersion: 1.15.4
  d8SpecificNodeCountByRole:
    master: 3
    system: 3
  extensionAPIServerAuthenticationRequestheaderClientCA: string
`

const globalValuesMultiMaster = `
highAvailability: false
enabledModules: ["vertical-pod-autoscaler"]
modules:
  placement: {}
discovery:
  kubernetesVersion: "1.19.5"
  clusterMasterCount: 3
  clusterUUID: f49dd1c3-a63a-4565-a06c-625e35587eab
  clusterVersion: 1.15.4
  d8SpecificNodeCountByRole:
    system: 1
    master: 3
  extensionAPIServerAuthenticationRequestheaderClientCA: string
`

const certManager = `
enableCAInjector: true
internal:
  enableCAInjector: true
  selfSignedCA:
    cert: string
    key: string
  webhookCert:
    ca: string
    key: string
    crt: string
`

const certManagerWithoutCAInjector = `
enableCAInjector: false
internal:
  enableCAInjector: false
  selfSignedCA:
    cert: string
    key: string
  webhookCert:
    ca: string
    key: string
    crt: string
`

const certManagerGatewayAPI = `
enableListenerSet: true
`

const cloudDNS = `
cloudDNSServiceAccount: ewogICJ0eXBlIjogInNlcnZpY2VfYWNjb3VudCIsCiAgInByb2plY3RfaWQiOiAicHJvamVjdC0yMDkzMTciLAogICJwcml2YXRlX2tleV9pZCI6ICJwcml2YXRlX2lkIiwKICAicHJpdmF0ZV9rZXkiOiAicHJpdmF0ZV9rZXkiLAogICJjbGllbnRfZW1haWwiOiAiZG5zMDEtc29sdmVyQHByb2plY3QtMjA5MzE3LmlhbS5nc2VydmljZWFjY291bnQuY29tIiwKICAiY2xpZW50X2lkIjogIjExNzM1MzAzMzgzOTQ2NTUzNjY3MiIsCiAgImF1dGhfdXJpIjogImh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbS9vL29hdXRoMi9hdXRoIiwKICAidG9rZW5fdXJpIjogImh0dHBzOi8vb2F1dGgyLmdvb2dsZWFwaXMuY29tL3Rva2VuIiwKICAiYXV0aF9wcm92aWRlcl94NTA5X2NlcnRfdXJsIjogImh0dHBzOi8vd3d3Lmdvb2dsZWFwaXMuY29tL29hdXRoMi92MS9jZXJ0cyIsCiAgImNsaWVudF94NTA5X2NlcnRfdXJsIjogImh0dHBzOi8vd3d3Lmdvb2dsZWFwaXMuY29tL3JvYm90L3YxL21ldGFkYXRhL3g1MDkvZG5zMDEtc29sdmVyJXByb2plY3QtMjA5MzE3LmlhbS5nc2VydmljZWFjY291bnQuY29tIgp9Cg==
`

const letsencryptResourcesValues = `
requests:
  cpu: "100m"
  memory: "128Mi"
limits:
  cpu: "200m"
  memory: "256Mi"
`

var _ = Describe("Module :: cert-manager :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Default", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("certManager", certManager)
			f.HelmRender()
		})

		It("Everything must render properly for default cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cert-manager")
			registrySecret := f.KubernetesResource("Secret", "d8-cert-manager", "deckhouse-registry")

			cainjector := f.KubernetesResource("Deployment", "d8-cert-manager", "cainjector")
			certManager := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())

			Expect(cainjector.Exists()).To(BeTrue())
			Expect(cainjector.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.kubernetes.io/control-plane\":\"\"}"))
			Expect(cainjector.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: node-role.kubernetes.io/master
- key: node-role.kubernetes.io/control-plane
- key: node.deckhouse.io/etcd-arbiter
- key: dedicated.deckhouse.io
  operator: Exists
- key: dedicated
  operator: Exists
- key: DeletionCandidateOfClusterAutoscaler
- key: ToBeDeletedByClusterAutoscaler
- effect: NoSchedule
  key: node.deckhouse.io/bashible-uninitialized
  operator: Exists
- effect: NoSchedule
  key: node.deckhouse.io/uninitialized
  operator: Exists
- key: node.kubernetes.io/not-ready
- key: node.kubernetes.io/out-of-disk
- key: node.kubernetes.io/memory-pressure
- key: node.kubernetes.io/disk-pressure
- key: node.kubernetes.io/pid-pressure
- key: node.kubernetes.io/unreachable
- key: node.kubernetes.io/network-unavailable
`))
			Expect(cainjector.Field("spec.replicas").Int()).To(BeEquivalentTo(1))
			Expect(cainjector.Field("spec.strategy").Exists()).To(BeTrue())
			Expect(cainjector.Field("spec.template.spec.affinity").Exists()).To(BeFalse())

			Expect(certManager.Exists()).To(BeTrue())
			Expect(certManager.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.deckhouse.io/system\":\"\"}"))
			Expect(certManager.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: dedicated.deckhouse.io
  operator: Equal
  value: "cert-manager"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
`))
			Expect(certManager.Field("spec.replicas").Int()).To(BeEquivalentTo(1))
			Expect(certManager.Field("spec.strategy").Exists()).To(BeFalse())
			Expect(certManager.Field("spec.template.spec.affinity").Exists()).To(BeFalse())
		})
	})

	Context("DefaultHA", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesHa)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("certManager", certManager)
			f.HelmRender()
		})

		It("Everything must render properly for default cluster with ha", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cert-manager")
			registrySecret := f.KubernetesResource("Secret", "d8-cert-manager", "deckhouse-registry")

			cainjector := f.KubernetesResource("Deployment", "d8-cert-manager", "cainjector")
			certManager := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(cainjector.Exists()).To(BeTrue())
			Expect(cainjector.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.kubernetes.io/control-plane\":\"\"}"))
			Expect(cainjector.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: node-role.kubernetes.io/master
- key: node-role.kubernetes.io/control-plane
- key: node.deckhouse.io/etcd-arbiter
- key: dedicated.deckhouse.io
  operator: Exists
- key: dedicated
  operator: Exists
- key: DeletionCandidateOfClusterAutoscaler
- key: ToBeDeletedByClusterAutoscaler
- effect: NoSchedule
  key: node.deckhouse.io/bashible-uninitialized
  operator: Exists
- effect: NoSchedule
  key: node.deckhouse.io/uninitialized
  operator: Exists
- key: node.kubernetes.io/not-ready
- key: node.kubernetes.io/out-of-disk
- key: node.kubernetes.io/memory-pressure
- key: node.kubernetes.io/disk-pressure
- key: node.kubernetes.io/pid-pressure
- key: node.kubernetes.io/unreachable
- key: node.kubernetes.io/network-unavailable
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
			Expect(certManager.Exists()).To(BeTrue())
			Expect(certManager.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.deckhouse.io/system\":\"\"}"))
			Expect(certManager.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: dedicated.deckhouse.io
  operator: Equal
  value: "cert-manager"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
`))
			Expect(certManager.Field("spec.replicas").Int()).To(BeEquivalentTo(2))
			Expect(certManager.Field("spec.strategy").String()).To(MatchYAML(`
type: RollingUpdate
rollingUpdate:
  maxSurge: 0
  maxUnavailable: 1
`))
			Expect(certManager.Field("spec.template.spec.affinity").String()).To(MatchYAML(`
podAntiAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
  - labelSelector:
      matchLabels:
        app: cert-manager
    topologyKey: kubernetes.io/hostname
`))
		})
	})

	Context("CAInjector disabled by internal value", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("certManager", certManagerWithoutCAInjector)
			f.HelmRender()
		})

		It("Should not render cainjector resources", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cainjector := f.KubernetesResource("Deployment", "d8-cert-manager", "cainjector")
			Expect(cainjector.Exists()).To(BeFalse())
		})
	})

	Context("Managed", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesManaged)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("certManager", certManager)
			f.HelmRender()
		})

		It("Everything must render properly for managed cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cert-manager")
			registrySecret := f.KubernetesResource("Secret", "d8-cert-manager", "deckhouse-registry")

			cainjector := f.KubernetesResource("Deployment", "d8-cert-manager", "cainjector")
			certManager := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(cainjector.Exists()).To(BeTrue())
			Expect(cainjector.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.deckhouse.io/control-plane\":\"\"}"))
			Expect(cainjector.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: node-role.kubernetes.io/master
- key: node-role.kubernetes.io/control-plane
- key: node.deckhouse.io/etcd-arbiter
- key: dedicated.deckhouse.io
  operator: Exists
- key: dedicated
  operator: Exists
- key: DeletionCandidateOfClusterAutoscaler
- key: ToBeDeletedByClusterAutoscaler
- effect: NoSchedule
  key: node.deckhouse.io/bashible-uninitialized
  operator: Exists
- effect: NoSchedule
  key: node.deckhouse.io/uninitialized
  operator: Exists
- key: node.kubernetes.io/not-ready
- key: node.kubernetes.io/out-of-disk
- key: node.kubernetes.io/memory-pressure
- key: node.kubernetes.io/disk-pressure
- key: node.kubernetes.io/pid-pressure
- key: node.kubernetes.io/unreachable
- key: node.kubernetes.io/network-unavailable
`))
			Expect(cainjector.Field("spec.replicas").Int()).To(BeEquivalentTo(1))
			Expect(cainjector.Field("spec.strategy").Exists()).To(BeTrue())
			Expect(cainjector.Field("spec.template.spec.affinity").Exists()).To(BeFalse())

			Expect(certManager.Exists()).To(BeTrue())
			Expect(certManager.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.deckhouse.io/system\":\"\"}"))
			Expect(certManager.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: dedicated.deckhouse.io
  operator: Equal
  value: "cert-manager"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
`))
			Expect(certManager.Field("spec.replicas").Int()).To(BeEquivalentTo(1))
			Expect(certManager.Field("spec.strategy").Exists()).To(BeFalse())
			Expect(certManager.Field("spec.template.spec.affinity").Exists()).To(BeFalse())
		})
	})

	Context("ManagedHa", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesManagedHa)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("certManager", certManager)
			f.HelmRender()
		})

		It("Everything must render properly for managed cluster with ha", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-cert-manager")
			registrySecret := f.KubernetesResource("Secret", "d8-cert-manager", "deckhouse-registry")

			cainjector := f.KubernetesResource("Deployment", "d8-cert-manager", "cainjector")
			certManager := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(cainjector.Exists()).To(BeTrue())
			Expect(cainjector.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.deckhouse.io/control-plane\":\"\"}"))
			Expect(cainjector.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: node-role.kubernetes.io/master
- key: node-role.kubernetes.io/control-plane
- key: node.deckhouse.io/etcd-arbiter
- key: dedicated.deckhouse.io
  operator: Exists
- key: dedicated
  operator: Exists
- key: DeletionCandidateOfClusterAutoscaler
- key: ToBeDeletedByClusterAutoscaler
- effect: NoSchedule
  key: node.deckhouse.io/bashible-uninitialized
  operator: Exists
- effect: NoSchedule
  key: node.deckhouse.io/uninitialized
  operator: Exists
- key: node.kubernetes.io/not-ready
- key: node.kubernetes.io/out-of-disk
- key: node.kubernetes.io/memory-pressure
- key: node.kubernetes.io/disk-pressure
- key: node.kubernetes.io/pid-pressure
- key: node.kubernetes.io/unreachable
- key: node.kubernetes.io/network-unavailable
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
			Expect(certManager.Exists()).To(BeTrue())
			Expect(certManager.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON("{\"node-role.deckhouse.io/system\":\"\"}"))
			Expect(certManager.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: dedicated.deckhouse.io
  operator: Equal
  value: "cert-manager"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
`))
			Expect(certManager.Field("spec.replicas").Int()).To(BeEquivalentTo(2))
			Expect(certManager.Field("spec.strategy").String()).To(MatchYAML(`
type: RollingUpdate
rollingUpdate:
  maxSurge: 0
  maxUnavailable: 1
`))
			Expect(certManager.Field("spec.template.spec.affinity").String()).To(MatchYAML(`
podAntiAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
  - labelSelector:
      matchLabels:
        app: cert-manager
    topologyKey: kubernetes.io/hostname
`))
		})
	})

	Context("Module highAvailability enabled with global HA disabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesMultiMaster)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("certManager", certManager)
			f.ValuesSet("certManager.highAvailability", true)
			f.HelmRender()
		})

		It("Module value must override the global one", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			for name, replicas := range map[string]int{
				"cert-manager": 2,
				"cainjector":   3,
				"webhook":      3,
			} {
				deployment := f.KubernetesResource("Deployment", "d8-cert-manager", name)
				pdb := f.KubernetesResource("PodDisruptionBudget", "d8-cert-manager", name)

				Expect(deployment.Field("spec.replicas").Int()).To(BeEquivalentTo(replicas), name)
				Expect(deployment.Field("spec.template.spec.affinity.podAntiAffinity").Exists()).To(BeTrue(), name)
				Expect(pdb.Field("spec.minAvailable").Int()).To(BeEquivalentTo(1), name)
			}
		})
	})

	Context("Module highAvailability disabled with global HA enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesManagedHa)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("certManager", certManager)
			f.ValuesSet("certManager.highAvailability", false)
			f.HelmRender()
		})

		It("Module value must override the global one", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			for _, name := range []string{"cert-manager", "cainjector", "webhook"} {
				deployment := f.KubernetesResource("Deployment", "d8-cert-manager", name)
				pdb := f.KubernetesResource("PodDisruptionBudget", "d8-cert-manager", name)

				Expect(deployment.Field("spec.replicas").Int()).To(BeEquivalentTo(1), name)
				Expect(deployment.Field("spec.template.spec.affinity").Exists()).To(BeFalse(), name)
				Expect(pdb.Field("spec.minAvailable").Int()).To(BeEquivalentTo(0), name)
			}
		})
	})

	Context("CloudDNS", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesManagedHa)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("certManager", certManager+cloudDNS)
			f.HelmRender()
		})

		It("Everything must render properly for CloudDNS enabled cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			secret := f.KubernetesResource("Secret", "d8-cert-manager", "clouddns")
			Expect(secret.Exists()).To(BeTrue())

			clusterIssuer := f.KubernetesResource("ClusterIssuer", "d8-cert-manager", "clouddns")
			Expect(clusterIssuer.Exists()).To(BeTrue())

			if clusterIssuer.Field("apiVersion").String() == "cert-manager.io/v1" {
				Expect(clusterIssuer.Field("spec.acme.solvers.0.dns01.cloudDNS.project").String()).To(Equal("project-209317"))
				Expect(clusterIssuer.Field("spec.acme.solvers.0.dns01.cloudDNS.serviceAccountSecretRef.name").String()).To(Equal("clouddns"))
				Expect(clusterIssuer.Field("spec.acme.solvers.0.dns01.cloudDNS.serviceAccountSecretRef.key").String()).To(Equal("key.json"))
			} else {
				Expect(clusterIssuer.Field("spec.acme.dns01.providers.0.clouddns.project").String()).To(Equal("project-209317"))
				Expect(clusterIssuer.Field("spec.acme.dns01.providers.0.clouddns.serviceAccountSecretRef.name").String()).To(Equal("clouddns"))
				Expect(clusterIssuer.Field("spec.acme.dns01.providers.0.clouddns.serviceAccountSecretRef.key").String()).To(Equal("key.json"))
			}
		})
	})

	Context("Cluster-wide proxy", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesManagedHa)
			f.ValuesSetFromYaml("global.clusterConfiguration", `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "Automatic"
podSubnetNodeCIDRPrefix: "24"
clusterDomain: cluster.local
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
proxy:
  httpProxy: http://proxy.example.com:3128
  httpsProxy: https://proxy.example.com:3129
  noProxy: ["example.org"]
`)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("certManager", certManager)
			f.HelmRender()
		})

		It("passes the proxy settings to the containers that reach external APIs", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			const noProxy = "127.0.0.1,169.254.169.254,registry.d8-system.svc,cluster.local,10.111.0.0/16,10.222.0.0/16,example.org"

			for _, target := range []struct{ deployment, container, noProxy string }{
				// The controller reaches issuers inside the cluster by their service names.
				{"cert-manager", "cert-manager", noProxy + ",.svc"},
			} {
				deployment := f.KubernetesResource("Deployment", "d8-cert-manager", target.deployment)
				Expect(deployment.Exists()).To(BeTrue())

				env := containerEnv(deployment, target.container)
				Expect(env).To(HaveKeyWithValue("HTTP_PROXY", "http://proxy.example.com:3128"), target.container)
				Expect(env).To(HaveKeyWithValue("http_proxy", "http://proxy.example.com:3128"), target.container)
				Expect(env).To(HaveKeyWithValue("HTTPS_PROXY", "https://proxy.example.com:3129"), target.container)
				Expect(env).To(HaveKeyWithValue("https_proxy", "https://proxy.example.com:3129"), target.container)
				Expect(env).To(HaveKeyWithValue("NO_PROXY", target.noProxy), target.container)
				Expect(env).To(HaveKeyWithValue("no_proxy", target.noProxy), target.container)
			}

			// The rest of the components talk only to the API server and keep their environment.
			for _, target := range []struct{ deployment, container string }{
				{"cert-manager", "kube-rbac-proxy"},
				{"cainjector", "cainjector"},
				{"webhook", "webhook"},
			} {
				deployment := f.KubernetesResource("Deployment", "d8-cert-manager", target.deployment)
				Expect(deployment.Exists()).To(BeTrue(), target.deployment)
				env := containerEnv(deployment, target.container)
				for _, name := range proxyEnvNames {
					Expect(env).NotTo(HaveKey(name), target.container)
				}
			}
		})
	})

	for _, clusterConfiguration := range []struct{ name, values string }{
		{"without ClusterConfiguration", ""},
		{"with ClusterConfiguration without a proxy", `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "Automatic"
podSubnetNodeCIDRPrefix: "24"
clusterDomain: cluster.local
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
`},
	} {
		Context("Without a cluster-wide proxy, "+clusterConfiguration.name, func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", globalValuesManagedHa)
				if clusterConfiguration.values != "" {
					f.ValuesSetFromYaml("global.clusterConfiguration", clusterConfiguration.values)
				}
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("certManager", certManager)
				f.HelmRender()
			})

			It("does not add proxy variables", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				for _, target := range []struct{ deployment, container, env string }{
					{"cert-manager", "cert-manager", "POD_NAMESPACE"},
				} {
					deployment := f.KubernetesResource("Deployment", "d8-cert-manager", target.deployment)
					Expect(deployment.Exists()).To(BeTrue(), target.deployment)
					env := containerEnv(deployment, target.container)
					Expect(env).To(HaveKey(target.env), target.container)
					for _, name := range proxyEnvNames {
						Expect(env).NotTo(HaveKey(name), target.container)
					}
				}
			})
		})
	}

	Context("<DisableLetsencrypt>", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesManagedHa)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("certManager", certManager)
			f.ValuesSet("certManager.disableLetsencrypt", true)
			f.HelmRender()
		})

		It("Check non-creation ClusterIssuer objects if disableLetsencrypt set to true", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			clusterIssuer := f.KubernetesGlobalResource("ClusterIssuer", "letsencrypt")
			Expect(clusterIssuer.Exists()).To(BeFalse())
			clusterIssuerStaging := f.KubernetesGlobalResource("ClusterIssuer", "letsencrypt-staging")
			Expect(clusterIssuerStaging.Exists()).To(BeFalse())
		})
	})
	Context("<Letsencrypt resources>", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesManagedHa)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("certManager", certManager)
		})

		It("should not render resources when letsencrypt.resources is not set (default)", func() {
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			issuer := f.KubernetesGlobalResource("ClusterIssuer", "letsencrypt")
			Expect(issuer.Exists()).To(BeTrue())
			Expect(issuer.Field("spec.acme.solvers.0.http01.ingress.podTemplate.spec.resources").Exists()).To(BeFalse())

		})
		It("should render resources when letsencrypt.resources is set", func() {
			f.ValuesSetFromYaml("certManager.letsencrypt.resources", letsencryptResourcesValues)
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			issuer := f.KubernetesGlobalResource("ClusterIssuer", "letsencrypt")
			Expect(issuer.Exists()).To(BeTrue())
			Expect(issuer.Field("spec.acme.solvers.0.http01.ingress.podTemplate.spec.resources").String()).
				To(MatchYAML(letsencryptResourcesValues))

		})
	})
	Context("<LetsencryptStaging resources>", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesManagedHa)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("certManager", certManager)
		})
		It("should not render resources when letsencryptStaging.resources is not set (default)", func() {
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			issuer := f.KubernetesGlobalResource("ClusterIssuer", "letsencrypt-staging")
			Expect(issuer.Exists()).To(BeTrue())
			Expect(issuer.Field("spec.acme.solvers.0.http01.ingress.podTemplate.spec.resources").Exists()).To(BeFalse())

		})
		It("should render resources when letsencryptStaging.resources is set", func() {
			f.ValuesSetFromYaml("certManager.letsencryptStaging.resources", letsencryptResourcesValues)
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			issuer := f.KubernetesGlobalResource("ClusterIssuer", "letsencrypt-staging")
			Expect(issuer.Exists()).To(BeTrue())
			// print object in yaml format
			Expect(issuer.Field("spec.acme.solvers.0.http01.ingress.podTemplate.spec.resources").String()).
				To(MatchYAML(letsencryptResourcesValues))

		})

	})

	Context("<Gateway API and ListenerSet flags>", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesManagedHa)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("global.discovery.gatewayAPIDefaultGateway", `
name: shared-gateway
namespace: d8-ingress-gateway
`)
			f.ValuesSetFromYaml("global.discovery.gatewayAPICRDs",
				gatewayAPICRDsServing("v1", "gateways", "httproutes", "listenersets"))
			f.ValuesSetFromYaml("certManager", certManager+certManagerGatewayAPI)
		})

		It("should render gateway and listenerset arguments", func() {
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			certManagerDeployment := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")
			Expect(certManagerDeployment.Exists()).To(BeTrue())

			args := certManagerDeployment.Field("spec.template.spec.containers.0.args").String()
			Expect(args).To(ContainSubstring("--feature-gates=ACMEHTTP01IngressPathTypeExact=false,ListenerSets=true"))
			Expect(args).To(ContainSubstring("--enable-gateway-api=true"))
			Expect(args).To(ContainSubstring("--enable-gateway-api-listenerset=true"))
		})

		It("still renders Gateway ClusterIssuers for the selected Gateway", func() {
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesGlobalResource("ClusterIssuer", "letsencrypt-gateway-shared-gateway").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterIssuer", "letsencrypt-staging-gateway-shared-gateway").Exists()).To(BeTrue())
		})
	})

	Context("<Gateway API CRDs without a Gateway>", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesManagedHa)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("global.discovery.gatewayAPICRDs",
				gatewayAPICRDsServing("v1", "gateways", "httproutes"))
			f.ValuesSetFromYaml("certManager", certManager)
		})

		It("enables Gateway API without rendering Gateway ClusterIssuers", func() {
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			deployment := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")
			args := deployment.Field("spec.template.spec.containers.0.args").String()
			Expect(args).To(ContainSubstring("--enable-gateway-api=true"))
			Expect(args).NotTo(ContainSubstring("--enable-gateway-api-listenerset=true"))
			Expect(f.KubernetesGlobalResource("ClusterIssuer", "letsencrypt-gateway-shared-gateway").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterIssuer", "letsencrypt-staging-gateway-shared-gateway").Exists()).To(BeFalse())
		})

		It("enables ListenerSet integration when requested", func() {
			f.ValuesSet("certManager.enableListenerSet", true)
			f.ValuesSetFromYaml("global.discovery.gatewayAPICRDs",
				gatewayAPICRDsServing("v1", "gateways", "httproutes", "listenersets"))
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			deployment := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")
			args := deployment.Field("spec.template.spec.containers.0.args").String()
			Expect(args).To(ContainSubstring("--feature-gates=ACMEHTTP01IngressPathTypeExact=false,ListenerSets=true"))
			Expect(args).To(ContainSubstring("--enable-gateway-api=true"))
			Expect(args).To(ContainSubstring("--enable-gateway-api-listenerset=true"))
		})

		// The integration is built against the stable API, so CRDs present only under an alpha or
		// beta version are not something it can be switched on for.
		It("does not enable Gateway API for CRDs served only under a version other than v1", func() {
			f.ValuesSetFromYaml("global.discovery.gatewayAPICRDs",
				gatewayAPICRDsServing("v1beta1", "gateways", "httproutes"))
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			deployment := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")
			args := deployment.Field("spec.template.spec.containers.0.args").String()
			Expect(args).NotTo(ContainSubstring("--enable-gateway-api=true"))
		})

		// v1 alongside another version is still v1 being served, so the integration is on.
		It("enables Gateway API when v1 is served next to another version", func() {
			f.ValuesSetFromYaml("global.discovery.gatewayAPICRDs", `
- name: gateways.gateway.networking.k8s.io
  versions:
  - name: v1beta1
    served: true
    storage: false
  - name: v1
    served: true
    storage: true
- name: httproutes.gateway.networking.k8s.io
  versions:
  - name: v1beta1
    served: true
    storage: false
  - name: v1
    served: true
    storage: true
`)
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			deployment := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")
			args := deployment.Field("spec.template.spec.containers.0.args").String()
			Expect(args).To(ContainSubstring("--enable-gateway-api=true"))
		})

		// Installed is not the same as usable: a CRD whose v1 is withdrawn serves nothing usable.
		It("does not enable Gateway API when v1 is present but not served", func() {
			f.ValuesSetFromYaml("global.discovery.gatewayAPICRDs",
				gatewayAPICRDsWithVersion("v1", false, "gateways", "httproutes"))
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			deployment := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")
			args := deployment.Field("spec.template.spec.containers.0.args").String()
			Expect(args).NotTo(ContainSubstring("--enable-gateway-api=true"))
		})

		It("does not enable ListenerSet integration without the ListenerSet CRD", func() {
			f.ValuesSet("certManager.enableListenerSet", true)
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			deployment := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")
			args := deployment.Field("spec.template.spec.containers.0.args").String()
			Expect(args).To(ContainSubstring("--feature-gates=ACMEHTTP01IngressPathTypeExact=false,ListenerSets=true"))
			Expect(args).To(ContainSubstring("--enable-gateway-api=true"))
			Expect(args).NotTo(ContainSubstring("--enable-gateway-api-listenerset=true"))
		})
	})

	Context("<Incomplete Gateway API CRDs>", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesManagedHa)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("certManager", certManager+certManagerGatewayAPI)
		})

		for _, plural := range []string{"gateways", "httproutes"} {
			It("does not enable Gateway API with only "+plural, func() {
				f.ValuesSetFromYaml("global.discovery.gatewayAPICRDs", gatewayAPICRDsServing("v1", plural))
				f.HelmRender()
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				deployment := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")
				args := deployment.Field("spec.template.spec.containers.0.args").String()
				Expect(args).NotTo(ContainSubstring("--enable-gateway-api=true"))
				Expect(args).NotTo(ContainSubstring("--enable-gateway-api-listenerset=true"))
			})
		}
	})

	Context("<Gateway without Gateway API CRDs>", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesManagedHa)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("global.discovery.gatewayAPIDefaultGateway", `
name: shared-gateway
namespace: d8-ingress-gateway
`)
			f.ValuesSetFromYaml("certManager", certManager)
			f.HelmRender()
		})

		It("does not render Gateway ClusterIssuers", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesGlobalResource("ClusterIssuer", "letsencrypt-gateway-shared-gateway").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterIssuer", "letsencrypt-staging-gateway-shared-gateway").Exists()).To(BeFalse())
		})
	})

	Context("<ListenerSet feature gate without Gateway API>", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesManagedHa)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("certManager", certManager+certManagerGatewayAPI)
			f.HelmRender()
		})

		It("should enable ListenerSets feature gate only", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			certManagerDeployment := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")
			Expect(certManagerDeployment.Exists()).To(BeTrue())

			args := certManagerDeployment.Field("spec.template.spec.containers.0.args").String()
			Expect(args).To(ContainSubstring("--feature-gates=ACMEHTTP01IngressPathTypeExact=false,ListenerSets=true"))
			Expect(args).NotTo(ContainSubstring("--enable-gateway-api=true"))
			Expect(args).NotTo(ContainSubstring("--enable-gateway-api-listenerset=true"))
		})
	})
})

// proxyEnvNames lists the variables helm_lib_envs_for_proxy renders.
var proxyEnvNames = []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "NO_PROXY", "no_proxy"}

// containerEnv maps the names of the container's environment variables to their literal values.
func containerEnv(deployment object_store.KubeObject, container string) map[string]string {
	env := map[string]string{}

	for _, c := range deployment.Field("spec.template.spec.containers").Array() {
		if c.Get("name").String() != container {
			continue
		}

		for _, e := range c.Get("env").Array() {
			env[e.Get("name").String()] = e.Get("value").String()
		}
	}

	return env
}

// gatewayAPICRDsServing renders global.discovery.gatewayAPICRDs with the named CRDs serving the
// given version. The value is a list of CRDs and their versions, so a spec that wants one available
// states it here rather than flipping a boolean.
func gatewayAPICRDsServing(version string, plurals ...string) string {
	return gatewayAPICRDsWithVersion(version, true, plurals...)
}

// gatewayAPICRDsWithVersion is gatewayAPICRDsServing with control over whether the version is
// served, for specs about a CRD that is installed but exposes nothing.
func gatewayAPICRDsWithVersion(version string, served bool, plurals ...string) string {
	var crds string
	for _, plural := range plurals {
		crds += fmt.Sprintf(`
- name: %s.gateway.networking.k8s.io
  versions:
  - name: %s
    served: %t
    storage: true
`, plural, version, served)
	}

	return crds
}

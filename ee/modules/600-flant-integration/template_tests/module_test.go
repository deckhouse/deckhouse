/*
Copyright 2021 Flant CJSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/ee/LICENSE
*/

package template_tests

import (
	"encoding/base64"
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
deckhouseVersion: dev
enabledModules: ["vertical-pod-autoscaler-crd", "prometheus", "flant-integration", "operator-prometheus-crd", "log-shipper"]
modulesImages:
  registry: registry.deckhouse.io
  registryDockercfg: cfg
  tags:
    flantIntegration:
      flantPricing: tagstring
      grafanaAgent: tagstring
      madisonProxy: tagstring
    common:
      alpine: tagstring
      kubeRbacProxy: imagehash
discovery:
  prometheusScrapeInterval: 30s
  clusterControlPlaneIsHighlyAvailable: true
  clusterMasterCount: 3
  d8SpecificNodeCountByRole:
    system: 1
  kubernetesVersion: 1.19.8
modules:
  placement: {}
`

const moduleValues = `
contacts: 10
doNotChargeForRockSolid: false
plan: "Standard"
planIsBoughtAsBundle: false
auxiliaryCluster: false
clusterType: Hybrid
nodesDiscount: 10
metrics: {}
kubeall:
  team: ""
  host: ""
  kubectl: "sudo kubectl"
  kubeconfig: "/root/.kube/config"
  context: ""
logs:
  url: "https://example.com/loki"
internal:
  releaseChannel: Alpha
  bundle: Default
  cloudProvider: AWS
  cloudLayout: withoutNAT
  controlPlaneVersion: 1.19
  clusterType: Hybrid
  nodeStats:
    minimalKubeletVersion: 1.19
    staticNodesCount: 1
    mastersCount: 3
    masterIsDedicated: true
    masterMinCPU: 4
    masterMinMemory: 800000
  prometheusAPIClientTLS:
    certificate: string
    key: string
  terraformManagerEnabled: true
`

const moduleValuesNoLogs = `
contacts: 10
doNotChargeForRockSolid: false
plan: "Standard"
planIsBoughtAsBundle: false
auxiliaryCluster: false
clusterType: Hybrid
nodesDiscount: 10
metrics: {}
kubeall:
  team: ""
  host: ""
  kubectl: "sudo kubectl"
  kubeconfig: "/root/.kube/config"
  context: ""
logs: false
internal:
  releaseChannel: Alpha
  bundle: Default
  cloudProvider: AWS
  controlPlaneVersion: 1.19
  clusterType: Hybrid
  nodeStats:
    minimalKubeletVersion: 1.19
    staticNodesCount: 1
    mastersCount: 3
    masterIsDedicated: true
    masterMinCPU: 4
    masterMinMemory: 800000
  prometheusAPIClientTLS:
    certificate: string
    key: string
  terraformManagerEnabled: true
`

var _ = Describe("Module :: flant-integration :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Cluster", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("flantIntegration", moduleValues)
			f.HelmRender()
		})
		nsName := "d8-flant-integration"
		chartName := "flant-integration"

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", nsName)
			registrySecret := f.KubernetesResource("Secret", nsName, "deckhouse-registry")

			sa := f.KubernetesResource("ServiceAccount", nsName, "pricing")
			ds := f.KubernetesResource("DaemonSet", nsName, "pricing")
			s := f.KubernetesResource("Secret", nsName, "grafana-agent-config")
			pm := f.KubernetesResource("PodMonitor", nsName, "pricing")
			cr := f.KubernetesGlobalResource("ClusterRole", "d8:"+chartName+":pricing")
			crb := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:"+chartName+":pricing")
			cld := f.KubernetesGlobalResource("ClusterLogDestination", "flant-integration-loki-storage")
			clc := f.KubernetesGlobalResource("ClusterLoggingConfig", "flant-integration-d8-logs")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(sa.Exists()).To(BeTrue())
			Expect(pm.Exists()).To(BeTrue())
			Expect(cr.Exists()).To(BeTrue())
			Expect(crb.Exists()).To(BeTrue())
			Expect(cld.Exists()).To(BeTrue())
			Expect(clc.Exists()).To(BeTrue())

			// user story #1
			Expect(ds.Exists()).To(BeTrue())
			expectedEnvsDS := `
- name: FP_RELEASE_CHANNEL
  value: Alpha
- name: FP_BUNDLE
  value: Default
- name: FP_CLOUD_PROVIDER
  value: AWS
- name: FP_CONTROL_PLANE_VERSION
  value: "1.19"
- name: FP_MINIMAL_KUBELET_VERSION
  value: "1.19"
- name: FP_PLAN
  value: Standard
- name: FP_CLUSTER_TYPE
  value: Hybrid
- name: FP_MASTERS_COUNT
  value: "3"
- name: FP_MASTER_IS_DEDICATED
  value: "1"
- name: FP_MASTER_MIN_CPU
  value: "4"
- name: FP_MASTER_MIN_MEMORY
  value: "800000"
- name: FP_PLAN_IS_BOUGHT_AS_BUNDLE
  value: "0"
- name: FP_AUXILIARY_CLUSTER
  value: "0"
- name: FP_NODES_DISCOUNT
  value: "10"
- name: FP_DO_NOT_CHARGE_FOR_ROCK_SOLID
  value: "0"
- name: FP_CONTACTS
  value: "10"
- name: FP_DECKHOUSE_VERSION
  value: dev
- name: FP_TERRAFORM_MANAGER_EBABLED
  value: "true"
- name: DEBUG_UNIX_SOCKET
  value: /tmp/shell-operator-debug.socket
- name: FP_KUBEALL_TEAM
  value: ""
- name: FP_KUBEALL_HOST
  value: ""
- name: FP_KUBEALL_KUBECTL
  value: sudo kubectl
- name: FP_KUBEALL_KUBECONFIG
  value: /root/.kube/config
- name: FP_KUBEALL_CONTEXT
  value: ""
`

			Expect(ds.Field("spec.template.spec.containers.0.env").String()).To(MatchYAML(expectedEnvsDS))

			Expect(s.Exists()).To(BeTrue())
			config, err := base64.StdEncoding.DecodeString(s.Field(`data.agent-scraping-service\.yaml`).String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(config)).To(ContainSubstring("remote_write"))
		})
	})

	Context("Cluster", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("flantIntegration", moduleValuesNoLogs)
			f.HelmRender()
		})
		nsName := "d8-flant-integration"
		chartName := "flant-integration"

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", nsName)
			registrySecret := f.KubernetesResource("Secret", nsName, "deckhouse-registry")

			sa := f.KubernetesResource("ServiceAccount", nsName, "pricing")
			pm := f.KubernetesResource("PodMonitor", nsName, "pricing")
			cr := f.KubernetesGlobalResource("ClusterRole", "d8:"+chartName+":pricing")
			crb := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:"+chartName+":pricing")
			cld := f.KubernetesGlobalResource("ClusterLogDestination", "flant-integration-loki-storage")
			clc := f.KubernetesGlobalResource("ClusterLoggingConfig", "flant-integration-d8-logs")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(sa.Exists()).To(BeTrue())
			Expect(pm.Exists()).To(BeTrue())
			Expect(cr.Exists()).To(BeTrue())
			Expect(crb.Exists()).To(BeTrue())
			Expect(cld.Exists()).To(BeFalse())
			Expect(clc.Exists()).To(BeFalse())
		})
	})
})

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
project: my_project
clusterName: my_cluster
enabledModules: ["vertical-pod-autoscaler-crd", "prometheus", "flant-pricing", "operator-prometheus-crd"]
modulesImages:
  registry: registry.flant.com
  registryDockercfg: cfg
  tags:
    flantPricing:
      flantPricing: tagstring
      grafanaAgent: tagstring
    common:
      alpine: tagstring
discovery:
  prometheusScrapeInterval: 30s
  clusterControlPlaneIsHighlyAvailable: true
  clusterMasterCount: 3
  d8SpecificNodeCountByRole:
    system: 1
modules:
  placement: {}
`

const moduleValues = `
contacts: 10
doNotChargeForRockSolid: false
plan: "Standard"
planIsBoughtAsBundle: false
internal:
  releaseChannel: Alpha
  bundle: Default
  cloudProvider: AWS
  controlPlaneVersion: 1.14
  minimalKubeletVersion: 1.14
  clusterType: Cloud
  mastersCount: 3
  kops: true
  convergeIsCompleted: true
  deprecatedResourcesInHelmReleases: 100
  masterIsDedicated: true
  masterMinCPU: 4
  masterMinMemory: 800000
`

var _ = Describe("Module :: flant-pricing :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Kops cluster", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("flantPricing", moduleValues)
			f.HelmRender()
		})
		nsName := "d8-flant-pricing"
		chartName := "flant-pricing"

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", nsName)
			registrySecret := f.KubernetesResource("Secret", nsName, "deckhouse-registry")

			sa := f.KubernetesResource("ServiceAccount", nsName, chartName)
			ds := f.KubernetesResource("DaemonSet", nsName, chartName)
			s := f.KubernetesResource("Secret", nsName, "grafana-agent-config")
			pm := f.KubernetesResource("PodMonitor", nsName, chartName)
			cr := f.KubernetesGlobalResource("ClusterRole", "d8:"+chartName+":flant-pricing")
			crb := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:"+chartName+":flant-pricing")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(sa.Exists()).To(BeTrue())
			Expect(pm.Exists()).To(BeTrue())
			Expect(cr.Exists()).To(BeTrue())
			Expect(crb.Exists()).To(BeTrue())

			// user story #1
			Expect(ds.Exists()).To(BeTrue())
			expectedEnvsDS := `
- name: FP_PROJECT
  value: my_project
- name: FP_CLUSTER
  value: my_cluster
- name: FP_RELEASE_CHANNEL
  value: Alpha
- name: FP_BUNDLE
  value: Default
- name: FP_CLOUD_PROVIDER
  value: AWS
- name: FP_CONTROL_PLANE_VERSION
  value: "1.14"
- name: FP_MINIMAL_KUBELET_VERSION
  value: "1.14"
- name: FP_PLAN
  value: Standard
- name: FP_CLUSTER_TYPE
  value: Cloud
- name: FP_MASTERS_COUNT
  value: "3"
- name: FP_KOPS
  value: "1"
- name: FP_ALL_MANAGED_NODES_UP_TO_DATE
  value: "0"
- name: FP_CONVERGE_IS_COMPLETED
  value: "1"
- name: FP_DEPRECATED_RESOURCES_IN_HELM_RELEASES
  value: "100"
- name: FP_MASTER_IS_DEDICATED
  value: "1"
- name: FP_MASTER_MIN_CPU
  value: "4"
- name: FP_MASTER_MIN_MEMORY
  value: "800000"
- name: FP_PLAN_IS_BOUGHT_AS_BUNDLE
  value: "0"
- name: FP_DO_NOT_CHARGE_FOR_ROCK_SOLID
  value: "0"
- name: FP_CONTACTS
  value: "10"
- name: DEBUG_UNIX_SOCKET
  value: /tmp/shell-operator-debug.socket`

			Expect(ds.Field("spec.template.spec.containers.0.env").String()).To(MatchYAML(expectedEnvsDS))

			Expect(s.Exists()).To(BeTrue())
			config, err := base64.StdEncoding.DecodeString(s.Field(`data.agent-scraping-service\.yaml`).String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(config)).To(ContainSubstring("remote_write"))
		})
	})
})

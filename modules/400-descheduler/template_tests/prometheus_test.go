/*
Copyright 2026 Flant JSC

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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const prometheusDeschedulers = `
  deschedulers:
  - name: average
    strategies:
      lowNodeUtilization:
        enabled: true
        metricsSource: Prometheus
        useDeviationThresholds: true
        thresholds:
          cpu: 5
          memory: 10
        targetThresholds:
          cpu: 5
          memory: 10
        evictionLimits:
          node: 2
  - name: instant
    strategies:
      lowNodeUtilization:
        enabled: true
        thresholds:
          cpu: 20
        targetThresholds:
          cpu: 50
`

const instantProfilePolicy = `
- name: instant
  pluginConfig:
  - args:
      evictFailedBarePods: true
      evictLocalStoragePods: false
      evictSystemCriticalPods: false
      ignorePvcPods: false
      nodeFit: true
    name: DefaultEvictor
  - args:
      metricsUtilization:
        source: KubernetesMetrics
      targetThresholds:
        cpu: 50
      thresholds:
        cpu: 20
    name: LowNodeUtilization
  plugins:
    balance:
      enabled:
      - LowNodeUtilization
    filter:
      enabled:
      - DefaultEvictor
    preEvictionFilter:
      enabled:
      - DefaultEvictor
`

var _ = Describe("Module :: descheduler :: helm template :: LowNodeUtilization on Prometheus", func() {
	f := SetupHelmConfig(``)

	render := func(enabledModules string, isMetricsServerEnabled bool) {
		f.ValuesSetFromYaml("global", `
  enabledModules: `+enabledModules+`
  modules:
    placement: {}
  discovery:
    clusterDomain: cluster.local
    kubernetesVersion: 1.33.1
    d8SpecificNodeCountByRole:
      master: 3
`)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("descheduler", fmt.Sprintf("internal:\n  isMetricsServerEnabled: %t", isMetricsServerEnabled)+prometheusDeschedulers)
		f.HelmRender()
	}

	insecureEnv := func() string {
		return f.KubernetesResource("Deployment", "d8-descheduler", "descheduler").
			Field(`spec.template.spec.containers.#(name=="descheduler").env.#(name=="DESCHEDULER_PROMETHEUS_INSECURE_SKIP_VERIFY").value`).String()
	}

	Context("prometheus module enabled", func() {
		BeforeEach(func() { render(`["vertical-pod-autoscaler", "prometheus"]`, true) })

		It("Should add the Prometheus provider and the 15-minute query only to the Prometheus profile", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			cm := f.KubernetesResource("ConfigMap", "d8-descheduler", "descheduler-policy")
			Expect(cm.Field(`data.policy\.yaml`)).To(MatchYAML(`---
apiVersion: descheduler/v1alpha2
kind: DeschedulerPolicy
metricsProviders:
- source: KubernetesMetrics
- source: Prometheus
  prometheus:
    url: https://aggregating-proxy.d8-monitoring.svc.cluster.local
profiles:
- name: average
  pluginConfig:
  - args:
      evictFailedBarePods: true
      evictLocalStoragePods: false
      evictSystemCriticalPods: false
      ignorePvcPods: false
      nodeFit: true
    name: DefaultEvictor
  - args:
      evictionLimits:
        node: 2
      metricsUtilization:
        prometheus:
          query: 'label_replace(clamp(1 - avg by (node) (rate(node_cpu_seconds_total{job="node-exporter",mode="idle"}[15m])), 0, 1), "instance", "$1", "node", "(.+)")'
        source: Prometheus
      targetThresholds:
        MetricResource: 5
      thresholds:
        MetricResource: 5
      useDeviationThresholds: true
    name: LowNodeUtilization
  plugins:
    balance:
      enabled:
      - LowNodeUtilization
    filter:
      enabled:
      - DefaultEvictor
    preEvictionFilter:
      enabled:
      - DefaultEvictor
` + instantProfilePolicy))
		})

		It("Should skip the TLS verification of aggregating-proxy and grant read access to it", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(insecureEnv()).To(Equal("true"))

			role := f.KubernetesGlobalResource("ClusterRole", "d8:descheduler:prometheus-reader")
			Expect(role.Field("rules")).To(MatchYAML(`
- apiGroups: ["apps"]
  resources: ["deployments/http"]
  resourceNames: ["aggregating-proxy"]
  verbs: ["get", "create"]
`))
			binding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:descheduler:prometheus-reader")
			Expect(binding.Field("subjects")).To(MatchYAML(`
- kind: ServiceAccount
  name: descheduler
  namespace: d8-descheduler
`))
		})
	})

	Context("prometheus module disabled", func() {
		BeforeEach(func() { render(`["vertical-pod-autoscaler"]`, true) })

		It("Should render the Prometheus profile as Default", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			cm := f.KubernetesResource("ConfigMap", "d8-descheduler", "descheduler-policy")
			Expect(cm.Field(`data.policy\.yaml`)).To(MatchYAML(`---
apiVersion: descheduler/v1alpha2
kind: DeschedulerPolicy
metricsProviders:
- source: KubernetesMetrics
profiles:
- name: average
  pluginConfig:
  - args:
      evictFailedBarePods: true
      evictLocalStoragePods: false
      evictSystemCriticalPods: false
      ignorePvcPods: false
      nodeFit: true
    name: DefaultEvictor
  - args:
      evictionLimits:
        node: 2
      metricsUtilization:
        source: KubernetesMetrics
      targetThresholds:
        cpu: 5
        memory: 10
      thresholds:
        cpu: 5
        memory: 10
      useDeviationThresholds: true
    name: LowNodeUtilization
  plugins:
    balance:
      enabled:
      - LowNodeUtilization
    filter:
      enabled:
      - DefaultEvictor
    preEvictionFilter:
      enabled:
      - DefaultEvictor
` + instantProfilePolicy))
		})

		It("Should neither skip TLS verification nor grant access to aggregating-proxy", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(insecureEnv()).To(BeEmpty())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:descheduler:prometheus-reader").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:descheduler:prometheus-reader").Exists()).To(BeFalse())
		})
	})

	Context("prometheus module enabled, metrics.k8s.io absent", func() {
		BeforeEach(func() { render(`["vertical-pod-autoscaler", "prometheus"]`, false) })

		It("Should list only the Prometheus provider and render the Default profile without metricsUtilization", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			cm := f.KubernetesResource("ConfigMap", "d8-descheduler", "descheduler-policy")
			Expect(cm.Field(`data.policy\.yaml`)).To(MatchYAML(`---
apiVersion: descheduler/v1alpha2
kind: DeschedulerPolicy
metricsProviders:
- source: Prometheus
  prometheus:
    url: https://aggregating-proxy.d8-monitoring.svc.cluster.local
profiles:
- name: average
  pluginConfig:
  - args:
      evictFailedBarePods: true
      evictLocalStoragePods: false
      evictSystemCriticalPods: false
      ignorePvcPods: false
      nodeFit: true
    name: DefaultEvictor
  - args:
      evictionLimits:
        node: 2
      metricsUtilization:
        prometheus:
          query: 'label_replace(clamp(1 - avg by (node) (rate(node_cpu_seconds_total{job="node-exporter",mode="idle"}[15m])), 0, 1), "instance", "$1", "node", "(.+)")'
        source: Prometheus
      targetThresholds:
        MetricResource: 5
      thresholds:
        MetricResource: 5
      useDeviationThresholds: true
    name: LowNodeUtilization
  plugins:
    balance:
      enabled:
      - LowNodeUtilization
    filter:
      enabled:
      - DefaultEvictor
    preEvictionFilter:
      enabled:
      - DefaultEvictor
- name: instant
  pluginConfig:
  - args:
      evictFailedBarePods: true
      evictLocalStoragePods: false
      evictSystemCriticalPods: false
      ignorePvcPods: false
      nodeFit: true
    name: DefaultEvictor
  - args:
      targetThresholds:
        cpu: 50
      thresholds:
        cpu: 20
    name: LowNodeUtilization
  plugins:
    balance:
      enabled:
      - LowNodeUtilization
    filter:
      enabled:
      - DefaultEvictor
    preEvictionFilter:
      enabled:
      - DefaultEvictor
`))
		})
	})
})

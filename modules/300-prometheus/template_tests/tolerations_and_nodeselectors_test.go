/*
Copyright 2023 Flant JSC

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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	. "github.com/deckhouse/deckhouse/testing/library/object_store"
)

var _ = Describe("Module :: prometheus :: helm template :: render Prometheus main and longterm node selectors and tolerations in CR", func() {
	const (
		customNodeSelector = `
nodeSelector:
  main-prometheus: ""
`
		customLongtermNodeSelector = `
longtermNodeSelector:
  longterm-prometheus: ""
`

		prometheusConfigTemplate = `
auth: {}
vpa: {}
grafana: {}
https:
  mode: CustomCertificate
internal:
  alertmanagers:
    byAddress: []
    byService: []
    internal: []
  auth: {}
  deployDexAuthenticator: true
  grafana:
    additionalDatasources: []
    alertsChannelsConfig:
      notifiers: []
  prometheusAPIClientTLS: {}
  prometheusLongterm:
    diskSizeGigabytes: 40
    effectiveStorageClass: ceph-ssd
    retentionGigabytes: 32
  prometheusMain:
    diskSizeGigabytes: 35
    effectiveStorageClass: default
    retentionGigabytes: 28
  prometheusScraperIstioMTLS: {}
  prometheusScraperTLS: {}
  vpa:
    longtermMaxCPU: 2933m
    longtermMaxMemory: 2200Mi
    maxCPU: 8800m
    maxMemory: 6600Mi
longtermMaxDiskSizeGigabytes: 300
longtermRetentionDays: 1
longtermScrapeInterval: 5m
%s
%s
mainMaxDiskSizeGigabytes: 300
retentionDays: 15
scrapeInterval: 30s
`

		defaultTolerationsTemplate = `
[
  {
    "key": "dedicated.deckhouse.io",
    "operator": "Equal",
    "value": "%s"
  },
  {
    "key": "dedicated.deckhouse.io",
    "operator": "Equal",
    "value": "monitoring"
  },
  {
    "key": "dedicated.deckhouse.io",
    "operator": "Equal",
    "value": "system"
  }
]
`
		tolerations = `
tolerations:
  - key: "dedicated.deckhouse.io"
    operator: "Equal"
    value: "my-tolerations"
`

		longtermTolerations = `
longtermTolerations:
  - key: "dedicated.deckhouse.io"
    operator: "Equal"
    value: "my-longterm-tolerations"
`
		expectedTolerationsTemplate = `
[
  {
    "key": "dedicated.deckhouse.io",
    "operator": "Equal",
    "value": "%s"
  }
]
`
	)

	expectedDefaultTolerations := func(value string) string {
		return fmt.Sprintf(defaultTolerationsTemplate, value)
	}

	expectedTolerations := func(value string) string {
		return fmt.Sprintf(expectedTolerationsTemplate, value)
	}

	deckhouseNodeRole := func(node string) string {
		return fmt.Sprintf(`{"node-role.deckhouse.io/%s": ""}`, node)
	}

	getGlobalValues := func(nodeCount string) string {
		return fmt.Sprintf(`
enabledModules: ["vertical-pod-autoscaler-crd", "prometheus"]
modules:
  https:
    mode: CustomCertificate
  publicDomainTemplate: "%%s.example.com"
  placement: {}
discovery:
  d8SpecificNodeCountByRole:
    %s
    system: 1
    master: 1
`, nodeCount)
	}

	prometheusWithOptions := func(option1, option2 string) string {
		return fmt.Sprintf(prometheusConfigTemplate, option1, option2)
	}

	prometheusInstance := func(h *Config, name string) KubeObject {
		prometheus := h.KubernetesResource("Prometheus", "d8-monitoring", name)
		Expect(prometheus.Exists()).To(BeTrue())

		return prometheus
	}

	f := SetupHelmConfig(``)

	DescribeTable(
		"Node selectors for main and longterm Prometheus deployments were rendered correctly",
		func(nodeCount, nodeSelector, longtermNodeSelector, expectedMainSelector, expectedLongtermNodeSelector string) {
			f.ValuesSetFromYaml("global", getGlobalValues(nodeCount))
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("prometheus", prometheusWithOptions(nodeSelector, longtermNodeSelector))
			f.HelmRender()

			Expect(f.RenderError).ShouldNot(HaveOccurred())
			prometheusMain := prometheusInstance(f, "main")
			prometheusLongterm := prometheusInstance(f, "longterm")

			Expect(prometheusMain.Field("spec.nodeSelector").String()).To(MatchJSON(expectedMainSelector))
			Expect(prometheusLongterm.Field("spec.nodeSelector").String()).To(MatchJSON(expectedLongtermNodeSelector))
		},

		Entry("Single node configuration", "", "", "", deckhouseNodeRole("system"), deckhouseNodeRole("system")),
		Entry("Separate prometheus-longterm node", "prometheus-longterm: 1", "", "", deckhouseNodeRole("system"), deckhouseNodeRole("prometheus-longterm")),
		Entry("Separate monitoring node", "monitoring: 1", "", "", deckhouseNodeRole("monitoring"), deckhouseNodeRole("monitoring")),
		Entry("Custom main and longterm node selectors", "", customNodeSelector, customLongtermNodeSelector, `{"main-prometheus": ""}`, `{"longterm-prometheus": ""}`),
	)

	DescribeTable(
		"Tolerations for main and longterm Prometheus deployments were rendered correctly",
		func(tolerations, longtermTolerations, expectedTolerations, expectedLongtermTolerations string) {
			f.ValuesSetFromYaml("global", getGlobalValues(""))
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("prometheus", prometheusWithOptions(tolerations, longtermTolerations))
			f.HelmRender()

			Expect(f.RenderError).ShouldNot(HaveOccurred())
			prometheusMain := prometheusInstance(f, "main")
			prometheusLongterm := prometheusInstance(f, "longterm")

			Expect(prometheusMain.Field("spec.tolerations").String()).To(MatchJSON(expectedTolerations))
			Expect(prometheusLongterm.Field("spec.tolerations").String()).To(MatchJSON(expectedLongtermTolerations))
		},

		Entry("No custom tolerations specified for main and longterm", "", "", expectedDefaultTolerations("prometheus"), expectedDefaultTolerations("prometheus-longterm")),
		Entry("Custom tolerations specified for main and longterm", tolerations, longtermTolerations, expectedTolerations("my-tolerations"), expectedTolerations("my-longterm-tolerations")),
	)
})

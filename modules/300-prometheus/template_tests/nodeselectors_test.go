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
)

var _ = Describe("Module :: prometheus :: helm template :: render Prometheus main and longterm node selectors in CR", func() {
	deckhouseNodeRole := func(node string) string {
		return fmt.Sprintf(`{"node-role.deckhouse.io/%s": ""}`, node)
	}

	const (
		customNodeSelector = `
nodeSelector:
  main-prometheus: ""
`
		customLongtermNodeSelector = `
longtermNodeSelector:
  longterm-prometheus: ""
`
	)

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

	getPrometheusValues := func(nodeSelector, longtermNodeSelector string) string {
		return fmt.Sprintf(`
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
`, nodeSelector, longtermNodeSelector)
	}

	f := SetupHelmConfig(``)

	DescribeTable(
		"Node selectors for main and longterm Prometheus deployments were rendered correctly",
		func(nodeCount, nodeSelector, longtermNodeSelector, expectedMainSelector, expectedLongtermNodeSelector string) {
			f.ValuesSetFromYaml("global", getGlobalValues(nodeCount))
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("prometheus", getPrometheusValues(nodeSelector, longtermNodeSelector))
			f.HelmRender()

			Expect(f.RenderError).ShouldNot(HaveOccurred())
			prometheusMain := f.KubernetesResource("Prometheus", "d8-monitoring", "main")
			prometheusLongterm := f.KubernetesResource("Prometheus", "d8-monitoring", "longterm")
			Expect(prometheusMain.Exists()).To(BeTrue())
			Expect(prometheusLongterm.Exists()).To(BeTrue())

			Expect(prometheusMain.Field("spec.nodeSelector").String()).To(MatchJSON(expectedMainSelector))
			Expect(prometheusLongterm.Field("spec.nodeSelector").String()).To(MatchJSON(expectedLongtermNodeSelector))
		},

		Entry("Single node configuration", "", "", "", deckhouseNodeRole("system"), deckhouseNodeRole("system")),
		Entry("Separate monitoring-longterm node", "monitoring-longterm: 1", "", "", deckhouseNodeRole("system"), deckhouseNodeRole("monitoring-longterm")),
		Entry("Separate monitoring node", "monitoring: 1", "", "", deckhouseNodeRole("monitoring"), deckhouseNodeRole("monitoring")),
		Entry("Custom main and longterm node selectors", "", customNodeSelector, customLongtermNodeSelector, `{"main-prometheus": ""}`, `{"longterm-prometheus": ""}`),
	)
})

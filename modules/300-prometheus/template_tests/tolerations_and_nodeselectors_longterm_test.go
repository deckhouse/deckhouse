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

var _ = Describe("Module :: prometheus :: helm template :: render Prometheus longterm node selectors and tolerations in CR", func() {
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
  customCertificateData:
    tls.crt: |
      -----BEGIN CERTIFICATE-----
      TEST
      -----END CERTIFICATE-----
    tls.key: |
      -----BEGIN PRIVATE KEY-----
      TEST
      -----END PRIVATE KEY-----
  alertmanagers:
    byAddress: []
    byService: []
    internal: []
  auth: {}
  deployDexAuthenticator: true
  grafana:
    enabled: true
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

		globalValues = `
enabledModules: ["vertical-pod-autoscaler", "prometheus"]
modules:
  https:
    mode: CustomCertificate
  publicDomainTemplate: "%s.example.com"
  placement: {}
discovery:
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
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

	prometheusWithOptions := func(option1, option2 string) string {
		return fmt.Sprintf(prometheusConfigTemplate, option1, option2)
	}

	f := SetupHelmConfig(``)

	DescribeTable(
		"Node selectors for longterm Prometheus deployments were rendered correctly",
		func(nodeSelector, longtermNodeSelector, expectedNodeSelector string) {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("prometheus", prometheusWithOptions(nodeSelector, longtermNodeSelector))
			f.HelmRender()

			Expect(f.RenderError).ShouldNot(HaveOccurred())
			prometheus := f.KubernetesResource("Prometheus", "d8-monitoring", "longterm")
			Expect(prometheus.Exists()).To(BeTrue())

			Expect(prometheus.Field("spec.nodeSelector").String()).To(MatchJSON(expectedNodeSelector))
		},

		Entry("Single node configuration", "", "", deckhouseNodeRole("system")),
		Entry("No custom main and set longterm node selectors", "", customLongtermNodeSelector, `{"longterm-prometheus": ""}`),
		Entry("Custom main and no longterm node selectors", customNodeSelector, "", `{"main-prometheus": ""}`),
		Entry("Custom main and longterm node selectors", customNodeSelector, customLongtermNodeSelector, `{"longterm-prometheus": ""}`),
	)

	DescribeTable(
		"Tolerations for longterm Prometheus deployments were rendered correctly",
		func(tolerations, longtermTolerations, expectedTolerations string) {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("prometheus", prometheusWithOptions(tolerations, longtermTolerations))
			f.HelmRender()

			Expect(f.RenderError).ShouldNot(HaveOccurred())
			prometheus := f.KubernetesResource("Prometheus", "d8-monitoring", "longterm")
			Expect(prometheus.Exists()).To(BeTrue())

			Expect(prometheus.Field("spec.tolerations").String()).To(MatchJSON(expectedTolerations))
		},

		Entry("No custom tolerations specified for main and longterm", "", "", expectedDefaultTolerations("prometheus")),
		Entry("Custom main and no longterm tolerations", tolerations, "", expectedTolerations("my-tolerations")),
		Entry("No custom main and set longterm tolerations", "", longtermTolerations, expectedTolerations("my-longterm-tolerations")),
		Entry("Custom main and longterm tolerations", tolerations, longtermTolerations, expectedTolerations("my-longterm-tolerations")),
	)
})

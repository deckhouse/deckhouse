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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// Free-form values coming from ModuleConfig and from the PrometheusRemoteWrite /
// CustomAlertmanager CRs must be rendered as quoted YAML scalars. A value carrying a
// newline and a "---" separator must stay inside its scalar instead of adding documents
// to the module release.
var _ = Describe("Module :: prometheus :: helm template :: yaml injection", func() {
	f := SetupHelmConfig(``)

	Context("Payloads in user-controlled values", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", `
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
`)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("prometheus", `
auth: {}
vpa: {}
grafana:
  customPlugins:
  - |-
    evil-plugin
    ---
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: pwned-plugins
      namespace: d8-monitoring
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
    byAddress:
    - name: evil
      scheme: http
      path: /
      target: |-
        127.0.0.1:9093
        ---
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: pwned-alertmanager-target
          namespace: d8-monitoring
      bearerToken: |-
        token
        ---
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: pwned-alertmanager-token
          namespace: d8-monitoring
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
  remoteWrite:
  - name: evil-remote-write
    spec:
      url: |-
        https://example.com/api/v1/write
        ---
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: pwned-remote-write-url
          namespace: d8-monitoring
      bearerToken: |-
        token
        ---
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: pwned-remote-write-token
          namespace: d8-monitoring
  vpa:
    longtermMaxCPU: 2933m
    longtermMaxMemory: 2200Mi
    maxCPU: 8800m
    maxMemory: 6600Mi
longtermMaxDiskSizeGigabytes: 300
longtermRetentionDays: 0
longtermScrapeInterval: 5m
mainMaxDiskSizeGigabytes: 300
retentionDays: 15
scrapeInterval: 30s
`)
			f.HelmRender()
		})

		It("Must render and must not produce injected documents", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Prometheus", "d8-monitoring", "main").Exists()).To(BeTrue())

			for _, name := range []string{
				"pwned-plugins",
				"pwned-alertmanager-target",
				"pwned-alertmanager-token",
				"pwned-remote-write-url",
				"pwned-remote-write-token",
			} {
				Expect(f.KubernetesResource("ConfigMap", "d8-monitoring", name).Exists()).To(BeFalse(), name)
			}
		})
	})
})

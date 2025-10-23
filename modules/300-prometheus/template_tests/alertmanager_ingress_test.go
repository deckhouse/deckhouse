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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: prometheus :: helm template :: alertmanager external auth", func() {
	f := SetupHelmConfig(``)

	const values = `
- name: test-alert-emailer
  receivers:
    - emailConfigs:
        - from: test
          requireTLS: false
          sendResolved: false
          smarthost: test
          to: test@test.ru
      name: test-alert-emailer
  route:
    groupBy:
      - job
    groupInterval: 5m
    groupWait: 30s
    receiver: test-alert-emailer
    repeatInterval: 4h
    routes:
      - matchers:
          - name: namespace
            regex: false
            value: app-airflow
        receiver: test-alert-emailer
`

	Context("Default", func() {
		const prometheusValues = `
auth: {}
vpa: {}
grafana: {}
https:
  mode: CustomCertificate
internal:
  vpa: {}
  prometheusMain: {}
  grafana:
    enabled: true
  customCertificateData: {}
  alertmanagers: {}
  prometheusAPIClientTLS: {}
  prometheusScraperIstioMTLS: {}
  prometheusScraperTLS: {}
  auth: {}
`
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("prometheus", prometheusValues)
			f.ValuesSetFromYaml("prometheus.internal.alertmanagers.internal", values)
			f.HelmRender()
		})

		It("Ingress must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			resource := f.KubernetesResource("Ingress", "d8-monitoring", "alertmanager-test-alert-emailer")
			Expect(resource.Exists()).To(BeTrue())
			Expect(resource.Field(`metadata.annotations.nginx\.ingress\.kubernetes\.io/auth-type`).String()).To(Equal("basic"))
			Expect(resource.Field(`metadata.annotations.nginx\.ingress\.kubernetes\.io/auth-secret`).String()).To(Equal("basic-auth"))
			Expect(resource.Field(`metadata.annotations.nginx\.ingress\.kubernetes\.io/auth-realm`).String()).To(Equal("Authentication Required"))
		})
	})

	Context("External Authentication", func() {
		const prometheusValues = `
auth:
  externalAuthentication:
    authSignInURL: "https://auth.sign.in/url"
    authURL: "https://auth.url"
vpa: {}
grafana: {}
https:
  mode: CustomCertificate
internal:
  vpa: {}
  prometheusMain: {}
  grafana: {}
  customCertificateData:
    tls.crt: |
      -----BEGIN CERTIFICATE-----
      TEST
      -----END CERTIFICATE-----
    tls.key: |
      -----BEGIN PRIVATE KEY-----
      TEST
      -----END PRIVATE KEY-----
  alertmanagers: {}
  prometheusAPIClientTLS: {}
  prometheusScraperIstioMTLS: {}
  prometheusScraperTLS: {}
  auth: {}
`
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("prometheus", prometheusValues)
			f.ValuesSetFromYaml("prometheus.internal.alertmanagers.internal", values)
			f.HelmRender()
		})

		It("Ingress must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			resource := f.KubernetesResource("Ingress", "d8-monitoring", "alertmanager-test-alert-emailer")
			Expect(resource.Exists()).To(BeTrue())
			Expect(resource.Field(`metadata.annotations.nginx\.ingress\.kubernetes\.io/auth-signin`).String()).To(Equal("https://auth.sign.in/url"))
			Expect(resource.Field(`metadata.annotations.nginx\.ingress\.kubernetes\.io/auth-url`).String()).To(Equal("https://auth.url"))
		})

	})

})

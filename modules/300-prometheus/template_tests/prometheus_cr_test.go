/*
Copyright 2022 Flant JSC

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
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: prometheus :: helm template :: render prometheus cr", func() {
	f := SetupHelmConfig(``)

	Context("Default", func() {
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
  remoteWrite:
  - name: test-remote-write-custom-auth
    spec:
      tlsConfig:
        insecureSkipVerify: true
      url: https://test-remote-write-custom-auth.domain.com/api/v1/write
      customAuthToken: Test
  - name: test-remote-write-basic-auth
    spec:
      url: https://test-remote-write-basic-auth.domain.com/api/v1/write
      basicAuth:
        password: pass
        username: user
  - name: test-remote-write-bearer-token
    spec:
      url: https://test-remote-write-bearer-token.domain.com/api/v1/write
      bearerToken: xxx
  - name: test-remote-write-custom-ca
    spec:
      url: https://test-remote-write-custom-ca.domain.com/api/v1/write
      tlsConfig:
        ca: CRTCRTCRT
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

		It("Prometheus main must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			prometheusMain := f.KubernetesResource("Prometheus", "d8-monitoring", "main")
			Expect(prometheusMain.Exists()).To(BeTrue())
			Expect(prometheusMain.Field("spec.remoteWrite").String()).To(MatchYAML(`
- tlsConfig:
    insecureSkipVerify: true
  url: https://test-remote-write-custom-auth.domain.com/api/v1/write
  headers:
    X-Auth-Token: Test
- url: https://test-remote-write-basic-auth.domain.com/api/v1/write
  basicAuth:
    username:
      name: d8-prometheus-remote-write-test-remote-write-basic-auth
      key: username
    password:
      name: d8-prometheus-remote-write-test-remote-write-basic-auth
      key: password
- url: https://test-remote-write-bearer-token.domain.com/api/v1/write
  bearerToken: xxx
- url: https://test-remote-write-custom-ca.domain.com/api/v1/write
  tlsConfig:
    ca:
      configMap:
        key: ca.crt
        name: d8-prometheus-remote-write-ca-test-remote-write-custom-ca
`))
			rwSecret := f.KubernetesResource("Secret", "d8-monitoring", "d8-prometheus-remote-write-test-remote-write-basic-auth")
			Expect(rwSecret.Exists()).To(BeTrue())
			Expect(rwSecret.Field("data.username").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("user"))))
			Expect(rwSecret.Field("data.password").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("pass"))))

			rwCAConfigMap := f.KubernetesResource("ConfigMap", "d8-monitoring", "d8-prometheus-remote-write-ca-test-remote-write-custom-ca")
			Expect(rwCAConfigMap.Exists()).To(BeTrue())
			Expect(rwCAConfigMap.Field("data").String()).To(Equal(`{"ca.crt":"CRTCRTCRT"}`))
		})
	})
})

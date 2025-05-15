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

var _ = Describe("Module :: prometheus :: helm template :: AlertmanagerConfig", func() {
	f := SetupHelmConfig(``)

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
  customCertificateData:
    tls.crt: CRTCRTCRT
    tls.key: KEYKEYKEY
  alertmanagers: {}
  prometheusAPIClientTLS:
    certificate: CRTCRTCRT
    key: KEYKEYKEY
  prometheusScraperIstioMTLS:
    certificate: CRTCRTCRT
    key: KEYKEYKEY
  prometheusScraperTLS:
    certificate: CRTCRTCRT
    key: KEYKEYKEY
  auth: {}
`

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
- name: airflow-alert-emailer
  receivers:
    - emailConfigs:
        - from: test
          requireTLS: false
          sendResolved: false
          smarthost: test
          to: test@test.ru
      name: airflow-alert-emailer
  route:
    groupBy:
      - job
    groupInterval: 5m
    groupWait: 30s
    receiver: airflow-alert-emailer
    repeatInterval: 4h
    routes:
      - matchers:
          - name: namespace
            regex: false
            value: app-airflow
        receiver: airflow-alert-emailer
`
	var tests = []struct {
		name         string
		expectedYaml string
	}{
		{
			name: "test-alert-emailer",
			expectedYaml: `apiVersion: monitoring.coreos.com/v1alpha1
kind: AlertmanagerConfig
metadata:
  labels:
    alertmanagerConfig: test-alert-emailer
    app: alertmanager
    heritage: deckhouse
    module: prometheus
  name: test-alert-emailer
  namespace: d8-monitoring
spec:
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
`,
		},
		{
			name: "airflow-alert-emailer",
			expectedYaml: `apiVersion: monitoring.coreos.com/v1alpha1
kind: AlertmanagerConfig
metadata:
  labels:
    alertmanagerConfig: airflow-alert-emailer
    app: alertmanager
    heritage: deckhouse
    module: prometheus
  name: airflow-alert-emailer
  namespace: d8-monitoring
spec:
  receivers:
  - emailConfigs:
    - from: test
      requireTLS: false
      sendResolved: false
      smarthost: test
      to: test@test.ru
    name: airflow-alert-emailer
  route:
    groupBy:
    - job
    groupInterval: 5m
    groupWait: 30s
    receiver: airflow-alert-emailer
    repeatInterval: 4h
    routes:
    - matchers:
      - name: namespace
        regex: false
        value: app-airflow
      receiver: airflow-alert-emailer
`,
		},
	}

	Context("Default", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("prometheus", prometheusValues)
			f.ValuesSetFromYaml("prometheus.internal.alertmanagers.internal", values)
			f.HelmRender()
		})

		It("AlertmanagerConfig must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			for _, test := range tests {
				resource := f.KubernetesResource("AlertmanagerConfig", "d8-monitoring", test.name)
				Expect(resource.Exists()).To(BeTrue())
				Expect(resource.ToYaml()).To(MatchYAML(test.expectedYaml))
			}
		})
	})
})

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

	Context("Default", func() {
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

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("prometheus", prometheusValues)
			f.ValuesSetFromYaml("prometheus.internal.alertmanagers.internal", values)
			f.HelmRender()
		})

		It("Ingress must render properly", func() {
			var tests = []struct {
				name         string
				expectedYaml string
			}{
				{
					"alertmanager-test-alert-emailer",
					`apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/affinity: cookie
    nginx.ingress.kubernetes.io/app-root: alertmanager/test-alert-emailer
    nginx.ingress.kubernetes.io/auth-signin: https://auth.sign.in/url
    nginx.ingress.kubernetes.io/auth-url: https://auth.url
    nginx.ingress.kubernetes.io/backend-protocol: HTTPS
    nginx.ingress.kubernetes.io/configuration-snippet: |
      proxy_ssl_certificate /etc/nginx/ssl/client.crt;
      proxy_ssl_certificate_key /etc/nginx/ssl/client.key;
      proxy_ssl_protocols TLSv1.2;
      proxy_ssl_session_reuse on;
    nginx.ingress.kubernetes.io/rewrite-target: /$2
    web.deckhouse.io/export-icon: /public/img/alertmanager.ico
    web.deckhouse.io/export-name: alertmanager/test-alert-emailer
  labels:
    app: alertmanager
    heritage: deckhouse
    module: prometheus
  name: alertmanager-test-alert-emailer
  namespace: d8-monitoring
spec:
  ingressClassName: ""
  rules:
  - host: grafana.example.com
    http:
      paths:
      - backend:
          service:
            name: test-alert-emailer
            port:
              name: https
        path: /alertmanager/test-alert-emailer(/|$)(.*)
        pathType: ImplementationSpecific
  tls:
  - hosts:
    - grafana.example.com
    secretName: ingress-tls-customcertificate
`,
				},
				{
					"alertmanager-airflow-alert-emailer",
					`apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/affinity: cookie
    nginx.ingress.kubernetes.io/app-root: alertmanager/airflow-alert-emailer
    nginx.ingress.kubernetes.io/auth-signin: https://auth.sign.in/url
    nginx.ingress.kubernetes.io/auth-url: https://auth.url
    nginx.ingress.kubernetes.io/backend-protocol: HTTPS
    nginx.ingress.kubernetes.io/configuration-snippet: |
      proxy_ssl_certificate /etc/nginx/ssl/client.crt;
      proxy_ssl_certificate_key /etc/nginx/ssl/client.key;
      proxy_ssl_protocols TLSv1.2;
      proxy_ssl_session_reuse on;
    nginx.ingress.kubernetes.io/rewrite-target: /$2
    web.deckhouse.io/export-icon: /public/img/alertmanager.ico
    web.deckhouse.io/export-name: alertmanager/airflow-alert-emailer
  labels:
    app: alertmanager
    heritage: deckhouse
    module: prometheus
  name: alertmanager-airflow-alert-emailer
  namespace: d8-monitoring
spec:
  ingressClassName: ""
  rules:
  - host: grafana.example.com
    http:
      paths:
      - backend:
          service:
            name: airflow-alert-emailer
            port:
              name: https
        path: /alertmanager/airflow-alert-emailer(/|$)(.*)
        pathType: ImplementationSpecific
  tls:
  - hosts:
    - grafana.example.com
    secretName: ingress-tls-customcertificate
`,
				},
			}

			Expect(f.RenderError).ShouldNot(HaveOccurred())

			for _, test := range tests {
				resource := f.KubernetesResource("Ingress", "d8-monitoring", test.name)
				Expect(resource.Exists()).To(BeTrue())
				Expect(resource.ToYaml()).To(MatchYAML(test.expectedYaml))
			}
		})

	})

})

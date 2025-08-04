/*
Copyright 2024 Flant JSC

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

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Prometheus hooks :: deprecate ingress with client cert ::", func() {
	f := HookExecutionConfigInit(``, ``)
	f.RegisterCRD("deckhouse.io", "v1", "GrafanaDashboardDefinition", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		Context("After adding some ingress with the client certificate", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/backend-protocol: HTTPS
    nginx.ingress.kubernetes.io/configuration-snippet: |
      proxy_ssl_certificate /etc/nginx/ssl/client.crt;
      proxy_ssl_certificate_key /etc/nginx/ssl/client.key;
      proxy_ssl_protocols TLSv1.2;
      proxy_ssl_session_reuse on;
    web.deckhouse.io/export-icon: /public/img/prometheus.ico
    web.deckhouse.io/export-name: prometheus
  name: grafana
  namespace: d8-monitoring
spec:
  ingressClassName: nginx
  rules:
  - host: example.com
    http:
      paths:
      - backend:
          service:
            name: grafana-v10
            port:
              name: https
        path: /
        pathType: ImplementationSpecific
`, 1))
				f.RunHook()
			})

			It("Should not start exposing metrics about deprecation", func() {
				Expect(f).To(ExecuteSuccessfully())
				m := f.MetricsCollector.CollectedMetrics()
				Expect(m).To(HaveLen(0))
			})
		})
	})

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		Context("After adding prometheus ingress with the client certificate", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/backend-protocol: HTTPS
    nginx.ingress.kubernetes.io/configuration-snippet: |
      proxy_ssl_certificate /etc/nginx/ssl/client.crt;
      proxy_ssl_certificate_key /etc/nginx/ssl/client.key;
      proxy_ssl_protocols TLSv1.2;
      proxy_ssl_session_reuse on;
    nginx.ingress.kubernetes.io/rewrite-target: /$2
    web.deckhouse.io/export-icon: /public/img/prometheus.ico
    web.deckhouse.io/export-name: prometheus
  name: prometheus
  namespace: d8-monitoring
spec:
  ingressClassName: nginx
  rules:
  - host: example.com
    http:
      paths:
      - backend:
          service:
            name: prometheus-main-0
            port:
              name: https
        path: /prometheus-main-0(/|$)(.*)
        pathType: ImplementationSpecific
`, 1))
				f.RunHook()
			})

			It("Should start exposing metrics about deprecation", func() {
				Expect(f).To(ExecuteSuccessfully())
				m := f.MetricsCollector.CollectedMetrics()
				Expect(m).To(HaveLen(1))
				Expect(m[0].Name).To(Equal(deprecatedIngressWithClientCertMetric))
				Expect(m[0].Labels).To(Equal(map[string]string{
					"name": "prometheus",
				}))
			})

			Context("And after deleting the prometheus ingress should stop exposing metric", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(``, 1))
					f.RunHook()
				})

				It("Should have zero collected metrics", func() {
					Expect(f).To(ExecuteSuccessfully())
					m := f.MetricsCollector.CollectedMetrics()
					Expect(m).To(HaveLen(0))
				})
			})
		})
	})
})

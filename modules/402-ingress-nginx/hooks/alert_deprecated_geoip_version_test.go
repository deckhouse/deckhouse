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
	"context"

	"github.com/flant/shell-operator/pkg/metric_storage/operation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("ingress-nginx :: hooks :: deprecated_geoip_version ::", func() {
	f := HookExecutionConfigInit(`{"ingressNginx":{"defaultControllerVersion": "1.9", "internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "IngressNginxController", true)

	Context("An empty cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateScheduleContext("0 * * * *"))
			f.RunGoHook()
		})

		Context("check there is no metrics", func() {
			It("must have no metrics", func() {
				Expect(f).To(ExecuteSuccessfully())
				metrics := f.MetricsCollector.CollectedMetrics()
				Expect(metrics).To(HaveLen(1))
				Expect(metrics[0]).To(BeEquivalentTo(operation.MetricOperation{
					Group:  "d8_deprecated_geoip_version",
					Action: "expire",
				}))
			})
		})
	})

	Context("Cluster with up-to-date controller", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: first
spec:
  controllerVersion: "1.10"
  ingressClass: "test"
`))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 * * * *"))
			f.RunGoHook()
		})

		Context("check there is no metrics", func() {
			It("must have no metrics", func() {
				Expect(f).To(ExecuteSuccessfully())
				metrics := f.MetricsCollector.CollectedMetrics()
				Expect(metrics).To(HaveLen(1))
				Expect(metrics[0]).To(BeEquivalentTo(operation.MetricOperation{
					Group:  "d8_deprecated_geoip_version",
					Action: "expire",
				}))
			})
		})
	})

	Context("Cluster with outdated controller implementing geoip variables", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: first
spec:
  controllerVersion: "1.10"
  ingressClass: "test"
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: second
spec:
  controllerVersion: "1.9"
  ingressClass: "test"
`))
			err := createNs(d8IngressNginxNamespace)
			Expect(err).To(BeNil())

			err = createCm(configMapWithGeoIP)
			Expect(err).To(BeNil())

			err = createCm(customHeaderWithGeoIP)
			Expect(err).To(BeNil())

			f.BindingContexts.Set(f.GenerateScheduleContext("0 * * * *"))
			f.RunGoHook()
		})

		Context("check there are metrics", func() {
			It("must have metrics", func() {
				Expect(f).To(ExecuteSuccessfully())
				metrics := f.MetricsCollector.CollectedMetrics()
				Expect(metrics).To(HaveLen(3))
				Expect(metrics[0]).To(BeEquivalentTo(operation.MetricOperation{
					Group:  "d8_deprecated_geoip_version",
					Action: "expire",
				}))
				Expect(metrics[1].Group).To(BeEquivalentTo("d8_deprecated_geoip_version"))
				Expect(metrics[1].Action).To(BeEquivalentTo("set"))
				Expect(metrics[1].Value).To(BeEquivalentTo(pointer.Float64(1)))
				Expect(metrics[1].Labels).To(BeEquivalentTo(map[string]string{
					"kind":               "IngressNginxController",
					"resource_namespace": "",
					"resource_name":      "second",
					"resource_key":       "log-format-upstream",
				}))
				Expect(metrics[2].Group).To(BeEquivalentTo("d8_deprecated_geoip_version"))
				Expect(metrics[2].Action).To(BeEquivalentTo("set"))
				Expect(metrics[2].Value).To(BeEquivalentTo(pointer.Float64(1)))
				Expect(metrics[2].Labels).To(BeEquivalentTo(map[string]string{
					"kind":               "IngressNginxController",
					"resource_namespace": "",
					"resource_name":      "second",
					"resource_key":       "geoheader",
				}))
			})
		})
	})

	Context("Cluster with outdated controller and ingresses implmeneting geoip variables", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: second
spec:
  controllerVersion: "1.6"
  ingressClass: "test"
`))
			err := createNs(d8IngressNginxNamespace)
			Expect(err).To(BeNil())

			err = createNs(defaultNamespace)
			Expect(err).To(BeNil())

			err = createIngress(defaultNsIngress, "default")
			Expect(err).To(BeNil())

			err = createNs(testNamespace)
			Expect(err).To(BeNil())

			err = createIngress(testNsIngress, "test")
			Expect(err).To(BeNil())

			err = createCm(configMapWithGeoIP)
			Expect(err).To(BeNil())

			err = createCm(customHeaderWithGeoIP)
			Expect(err).To(BeNil())

			f.BindingContexts.Set(f.GenerateScheduleContext("0 * * * *"))
			f.RunGoHook()
		})

		Context("check there are metrics", func() {
			It("must have metrics", func() {
				Expect(f).To(ExecuteSuccessfully())
				metrics := f.MetricsCollector.CollectedMetrics()
				Expect(metrics).To(HaveLen(5))
				Expect(metrics[0]).To(BeEquivalentTo(operation.MetricOperation{
					Group:  "d8_deprecated_geoip_version",
					Action: "expire",
				}))
				Expect(metrics[1].Group).To(BeEquivalentTo("d8_deprecated_geoip_version"))
				Expect(metrics[1].Action).To(BeEquivalentTo("set"))
				Expect(metrics[1].Value).To(BeEquivalentTo(pointer.Float64(1)))
				Expect(metrics[1].Labels).To(BeEquivalentTo(map[string]string{
					"kind":               "IngressNginxController",
					"resource_namespace": "",
					"resource_name":      "second",
					"resource_key":       "log-format-upstream",
				}))
				Expect(metrics[2].Group).To(BeEquivalentTo("d8_deprecated_geoip_version"))
				Expect(metrics[2].Action).To(BeEquivalentTo("set"))
				Expect(metrics[2].Value).To(BeEquivalentTo(pointer.Float64(1)))
				Expect(metrics[2].Labels).To(BeEquivalentTo(map[string]string{
					"kind":               "IngressNginxController",
					"resource_namespace": "",
					"resource_name":      "second",
					"resource_key":       "geoheader",
				}))
				Expect(metrics[3].Group).To(BeEquivalentTo("d8_deprecated_geoip_version"))
				Expect(metrics[3].Action).To(BeEquivalentTo("set"))
				Expect(metrics[3].Value).To(BeEquivalentTo(pointer.Float64(1)))
				Expect(metrics[3].Labels).To(BeEquivalentTo(map[string]string{
					"kind":               "Ingress",
					"resource_namespace": "default",
					"resource_name":      "nginx",
					"resource_key":       "nginx.ingress.kubernetes.io/server-snippet",
				}))
				Expect(metrics[4].Group).To(BeEquivalentTo("d8_deprecated_geoip_version"))
				Expect(metrics[4].Action).To(BeEquivalentTo("set"))
				Expect(metrics[4].Value).To(BeEquivalentTo(pointer.Float64(1)))
				Expect(metrics[4].Labels).To(BeEquivalentTo(map[string]string{
					"kind":               "Ingress",
					"resource_namespace": "test",
					"resource_name":      "nginx",
					"resource_key":       "nginx.ingress.kubernetes.io/canary-by-header",
				}))
			})
		})
	})
})

const configMapWithGeoIP = `
apiVersion: v1
data:
  allow-snippet-annotations: "true"
  body-size: 64m
  gzip-level: "1"
  hsts: "false"
  http-redirect-code: "301"
  large-client-header-buffers: 4 16k
  log-format-escape-json: "true"
  log-format-upstream: '{ "time": "$time_iso8601", "request_id": "$request_id", "user":
    "$remote_user", "address": "$remote_addr", "bytes_received": $request_length,
    "bytes_sent": $bytes_sent, "protocol": "$server_protocol", "scheme": "$scheme",
    "method": "$request_method", "host": "$host", "path": "$uri", "request_query":
    "$args", "referrer": "$http_referer", "user_agent": "$http_user_agent", "request_time":
    $request_time, "status": $formatted_status, "content_kind": "$content_kind", "upstream_response_time":
    $total_upstream_response_time, "upstream_retries": $upstream_retries, "namespace":
    "$namespace", "ingress": "$ingress_name", "service": "$service_name", "service_port":
    "$service_port", "vhost": "$server_name", "location": "$location_path", "nginx_upstream_addr":
    "$upstream_addr", "nginx_upstream_bytes_received": "$upstream_bytes_received",
    "nginx_upstream_response_time": "$upstream_response_time", "nginx_upstream_status":
    "$upstream_status", "geo": "$geoip_city" }'
  proxy-connect-timeout: "2"
  proxy-next-upstream: error timeout invalid_header http_502 http_503 http_504
  proxy-read-timeout: "3600"
  proxy-send-timeout: "3600"
  proxy-set-headers: d8-ingress-nginx/main-custom-headers
  server-name-hash-bucket-size: "256"
  server-tokens: "false"
  use-gzip: "true"
  variables-hash-bucket-size: "256"
  worker-shutdown-timeout: "300"
kind: ConfigMap
metadata:
  annotations:
    meta.helm.sh/release-name: ingress-nginx
    meta.helm.sh/release-namespace: d8-system
  creationTimestamp: "2024-06-09T17:20:21Z"
  labels:
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: ingress-nginx
  name: second-config
  namespace: d8-ingress-nginx
`

const customHeaderWithGeoIP = `
apiVersion: v1
data:
  geoheader: $geoip_country_code
kind: ConfigMap
metadata:
  annotations:
    meta.helm.sh/release-name: ingress-nginx
    meta.helm.sh/release-namespace: d8-system
  creationTimestamp: "2024-06-09T17:20:21Z"
  labels:
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: ingress-nginx
  name: second-custom-headers
  namespace: d8-ingress-nginx
`

const d8IngressNginxNamespace = `
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    meta.helm.sh/release-name: ingress-nginx
    meta.helm.sh/release-namespace: d8-system
  creationTimestamp: "2024-06-09T17:20:21Z"
  labels:
    app.kubernetes.io/managed-by: Helm
    extended-monitoring.deckhouse.io/enabled: ""
    heritage: deckhouse
    kubernetes.io/metadata.name: d8-ingress-nginx
    module: ingress-nginx
    prometheus.deckhouse.io/rules-watcher-enabled: "true"
  name: d8-ingress-nginx
spec:
  finalizers:
  - kubernetes
`

const defaultNamespace = `
apiVersion: v1
kind: Namespace
metadata:
  annotations:
  name: default
`

const testNamespace = `
apiVersion: v1
kind: Namespace
metadata:
  annotations:
  name: test
`

const defaultNsIngress = `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/server-snippet: |
      location @toprod {
          proxy_pass "https://google.com";
          proxy_set_header Host "google.com";
          proxy_set_header City $geoip_country_code3;
          proxy_intercept_errors off;
          add_header x-source-s3 "prod" always;
      }
  name: nginx
  namespace: default
spec:
  ingressClassName: nginx
  rules:
  - host: example.com
    http:
      paths:
      - backend:
          service:
            name: nginx
            port:
              number: 80
        path: /
        pathType: Exact
`

const testNsIngress = `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/canary-by-header: $geoip_city_continent_code
    nginx.ingress.kubernetes.io/server-snippet: |
      location @toprod {
          proxy_pass "https://google.com";
          proxy_set_header Host "google.com";
          proxy_set_header City $geoip_fake_var;
          proxy_intercept_errors off;
          add_header x-source-s3 "prod" always;
      }
  name: nginx
  namespace: test
spec:
  ingressClassName: nginx
  rules:
  - host: example.com
    http:
      paths:
      - backend:
          service:
            name: nginx
            port:
              number: 80
        path: /
        pathType: Exact
`

func createNs(namespace string) error {
	var ns corev1.Namespace
	_ = yaml.Unmarshal([]byte(namespace), &ns)

	_, err := dependency.TestDC.MustGetK8sClient().
		CoreV1().
		Namespaces().
		Create(context.TODO(), &ns, metav1.CreateOptions{})
	return err
}

func createCm(configMap string) error {
	var cm corev1.ConfigMap
	_ = yaml.Unmarshal([]byte(configMap), &cm)

	_, err := dependency.TestDC.MustGetK8sClient().
		CoreV1().
		ConfigMaps("d8-ingress-nginx").
		Create(context.TODO(), &cm, metav1.CreateOptions{})
	return err
}

func createIngress(ingress, namespace string) error {
	var i netv1.Ingress
	_ = yaml.Unmarshal([]byte(ingress), &i)

	_, err := dependency.TestDC.MustGetK8sClient().
		NetworkingV1().
		Ingresses(namespace).
		Create(context.TODO(), &i, metav1.CreateOptions{})
	return err
}

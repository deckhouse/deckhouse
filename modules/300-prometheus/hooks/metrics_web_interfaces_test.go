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

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: prometheus :: hooks ::  metrics_web_interfaces ::", func() {
	const (
		ingressMetrics = `
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test-1
  labels:
    heritage: "deckhouse"
    module: "a"
  annotations:
    web.deckhouse.io/export-name: "test-1"
    web.deckhouse.io/export-host: "test1-example.com"
    web.deckhouse.io/export-path: "/abc"
spec: {}
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test-2
  labels:
    heritage: "deckhouse"
    module: "b"
  annotations:
    web.deckhouse.io/export-name: "test-2"
    web.deckhouse.io/export-host: "test1-example.com/abc"
    web.deckhouse.io/export-path: "abc"
    web.deckhouse.io/export-icon: "/custom/icon"
spec: {}
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test-3
  labels:
    heritage: "deckhouse"
    module: "c"
  annotations:
    web.deckhouse.io/export-name: "test 3"
    web.deckhouse.io/export-path: "/abc/def"
spec:
  rules:
  - host: test3-example.com
    http:
      path: /test
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test-4
  labels:
    heritage: "deckhouse"
    module: "d"
  annotations:
    web.deckhouse.io/export-name: "test@4"
    web.deckhouse.io/export-icon: "https://example.com/custom/icon"
spec:
  rules:
  - host: test4-example.com
    http:
      path: /test
  tls:
  - hosts:
    - test4-example.com
    secretName: test
`
	)
	f := HookExecutionConfigInit(
		`{"prometheus":{"internal":{}},"global":{"enabledModules":[]}}`,
		`{}`,
	)
	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
			ops := f.MetricsCollector.CollectedMetrics()
			Expect(len(ops)).To(BeEquivalentTo(1)) // expiration event
		})
	})

	Context("Cluster containing some services", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(ingressMetrics))
			f.RunHook()
		})

		It("Hook must not fail, should get metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			ops := f.MetricsCollector.CollectedMetrics()
			Expect(len(ops)).To(BeEquivalentTo(5))

			Expect(ops[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "deckhouse_web_interfaces",
				Action: operation.ActionGaugeSet,
				Labels: map[string]string{
					"icon": "/public/img/unknown.png",
					"name": "test-1",
					"url":  "http://test1-example.com/abc",
				},
				Value: ptr.To(1.0),
				Group: "deckhouse_exported_domains",
			}))
			Expect(ops[2]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "deckhouse_web_interfaces",
				Action: operation.ActionGaugeSet,
				Labels: map[string]string{
					"icon": "/custom/icon",
					"name": "test-2",
					"url":  "http://test1-example.com%2Fabc/abc",
				},
				Value: ptr.To(1.0),
				Group: "deckhouse_exported_domains",
			}))
			Expect(ops[3]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "deckhouse_web_interfaces",
				Action: operation.ActionGaugeSet,
				Labels: map[string]string{
					"icon": "/public/img/unknown.png",
					"name": "test 3",
					"url":  "http://test3-example.com/abc/def",
				},
				Value: ptr.To(1.0),
				Group: "deckhouse_exported_domains",
			}))
			Expect(ops[4]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "deckhouse_web_interfaces",
				Action: operation.ActionGaugeSet,
				Labels: map[string]string{
					"icon": "https://example.com/custom/icon",
					"name": "test@4",
					"url":  "https://test4-example.com",
				},
				Value: ptr.To(1.0),
				Group: "deckhouse_exported_domains",
			}))
		})
	})

	Context("Cluster containing some services and global module https.mode:OnlyInURI", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(ingressMetrics))
			f.ConfigValuesSet("global.modules.https.mode", "OnlyInURI")
			f.RunHook()

		})

		It("Hook must not fail, should get metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			ops := f.MetricsCollector.CollectedMetrics()
			Expect(len(ops)).To(BeEquivalentTo(5))

			Expect(ops[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "deckhouse_web_interfaces",
				Action: operation.ActionGaugeSet,
				Labels: map[string]string{
					"icon": "/public/img/unknown.png",
					"name": "test-1",
					"url":  "https://test1-example.com/abc",
				},
				Value: ptr.To(1.0),
				Group: "deckhouse_exported_domains",
			}))
			Expect(ops[2]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "deckhouse_web_interfaces",
				Action: operation.ActionGaugeSet,
				Labels: map[string]string{
					"icon": "/custom/icon",
					"name": "test-2",
					"url":  "https://test1-example.com%2Fabc/abc",
				},
				Value: ptr.To(1.0),
				Group: "deckhouse_exported_domains",
			}))
			Expect(ops[3]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "deckhouse_web_interfaces",
				Action: operation.ActionGaugeSet,
				Labels: map[string]string{
					"icon": "/public/img/unknown.png",
					"name": "test 3",
					"url":  "https://test3-example.com/abc/def",
				},
				Value: ptr.To(1.0),
				Group: "deckhouse_exported_domains",
			}))
			Expect(ops[4]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "deckhouse_web_interfaces",
				Action: operation.ActionGaugeSet,
				Labels: map[string]string{
					"icon": "https://example.com/custom/icon",
					"name": "test@4",
					"url":  "https://test4-example.com",
				},
				Value: ptr.To(1.0),
				Group: "deckhouse_exported_domains",
			}))
		})
	})

})

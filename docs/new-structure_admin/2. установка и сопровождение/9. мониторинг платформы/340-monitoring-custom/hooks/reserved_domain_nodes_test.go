/*
Copyright 2021 Flant JSC

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
	"github.com/flant/shell-operator/pkg/metric_storage/operation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: monitoring-custom :: hooks :: reserved_domain_nodes ::", func() {
	const (
		properResources = `
---
apiVersion: v1
kind: Node
metadata:
  name: system
  labels:
    node-role.deckhouse.io/system: ""
spec:
  taints:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    value: system
`
		resourcesWithReservedLabels = `
---
apiVersion: v1
kind: Node
metadata:
  name: stateful
  labels:
    node-role.deckhouse.io/stateful: ""
`
		resourcesWithFewLabels = `
---
apiVersion: v1
kind: Node
metadata:
  name: double
  labels:
    node-role.deckhouse.io/invalid: ""
    node-role.deckhouse.io/system: ""
`

		resourcesWithReservedTaints = `
---
apiVersion: v1
kind: Node
metadata:
  name: database
spec:
  taints:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    value: database
`
	)
	f := HookExecutionConfigInit(
		`{"monitoringKubernetes":{"internal":{}},"global":{"enabledModules":[]}}`,
		`{}`,
	)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster containing proper Node", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(properResources))
			f.RunHook()
		})

		It("Hook must not fail, Only expire metrics should be sent", func() {
			Expect(f).To(ExecuteSuccessfully())
			ops := f.MetricsCollector.CollectedMetrics()
			Expect(len(ops)).To(BeEquivalentTo(1))

			// first is expiration
			Expect(ops[0]).To(BeEquivalentTo(operation.MetricOperation{
				Action: "expire",
			}))
		})
	})

	Context("Cluster with Node having reserved `metadata.labels`", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(resourcesWithReservedLabels))
			f.RunHook()
		})

		It("Hook must not fail, should get 2 metrics - expire and about stateful node", func() {
			Expect(f).To(ExecuteSuccessfully())
			ops := f.MetricsCollector.CollectedMetrics()
			Expect(len(ops)).To(BeEquivalentTo(2))

			// first is expiration
			Expect(ops[0]).To(BeEquivalentTo(operation.MetricOperation{
				Action: "expire",
			}))

			// second is metrics
			expectedMetric := operation.MetricOperation{
				Name:   "reserved_domain_nodes",
				Action: "set",
				Value:  pointer.Float64(1.0),
				Labels: map[string]string{
					"name": "stateful",
				},
			}
			Expect(ops[1]).To(BeEquivalentTo(expectedMetric))
		})
	})

	Context("Cluster with Node having reserved a few labels", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(resourcesWithFewLabels))
			f.RunHook()
		})

		It("Hook must not fail, should get 2 metrics - expire and about invalid node label", func() {
			Expect(f).To(ExecuteSuccessfully())
			ops := f.MetricsCollector.CollectedMetrics()
			Expect(len(ops)).To(BeEquivalentTo(2))

			// first is expiration
			Expect(ops[0]).To(BeEquivalentTo(operation.MetricOperation{
				Action: "expire",
			}))

			// second is metrics
			expectedMetric := operation.MetricOperation{
				Name:   "reserved_domain_nodes",
				Action: "set",
				Value:  pointer.Float64(1.0),
				Labels: map[string]string{
					"name": "double",
				},
			}
			Expect(ops[1]).To(BeEquivalentTo(expectedMetric))
		})
	})

	Context("Cluster with Node having reserved `spec.taints`", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(resourcesWithReservedTaints))
			f.RunHook()
		})

		It("Hook must not fail, should get 2 metrics - expire and about database node", func() {
			Expect(f).To(ExecuteSuccessfully())
			ops := f.MetricsCollector.CollectedMetrics()
			Expect(len(ops)).To(BeEquivalentTo(2))

			// first is expiration
			Expect(ops[0]).To(BeEquivalentTo(operation.MetricOperation{
				Action: "expire",
			}))

			// second is metrics
			expectedMetric := operation.MetricOperation{
				Name:   "reserved_domain_nodes",
				Action: "set",
				Value:  pointer.Float64(1.0),
				Labels: map[string]string{
					"name": "database",
				},
			}
			Expect(ops[1]).To(BeEquivalentTo(expectedMetric))
		})
	})

})

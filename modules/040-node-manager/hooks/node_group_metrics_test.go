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
	"k8s.io/utils/pointer"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: node group metrics ", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

	const nodeGroupWith1Instance = `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    minPerZone: 1
    maxPerZone: 1
status:
  ready: 1
  nodes: 1
  instances: 1
  desired: 1
  min: 1
  max: 1
  upToDate: 1
  standby: 0
`

	const nodeGroupWith5to10Instances = `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    minPerZone: 5
    maxPerZone: 10
status:
  ready: 4
  nodes: 4
  instances: 4
  desired: 5
  min: 5
  max: 10
  upToDate: 1
  standby: 0
  conditions:
    - type: Error
      status: "True"
      message: "Quota exceeded"
`

	const nodeGroupWith2to4Instances = `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    minPerZone: 2
    maxPerZone: 4
status:
  ready: 2
  nodes: 2
  instances: 2
  desired: 2
  min: 2
  max: 4
  upToDate: 2
  standby: 1
`

	assertMetric := func(f *HookExecutionConfig, name string, expected float64) {
		metrics := f.MetricsCollector.CollectedMetrics()

		ok := false
		for _, m := range metrics {
			if m.Name == name {
				Expect(m.Value).To(Equal(pointer.Float64(expected)))
				Expect(m.Labels).To(HaveKey("node_group_name"))
				Expect(m.Labels["node_group_name"]).To(Equal("worker"))

				ok = true

				break
			}
		}

		Expect(ok).To(BeTrue())
	}

	Context("NodeGroup with 1 instance", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupWith1Instance))

			f.RunHook()
		})

		It("Test metrics", func() {
			Expect(f).To(ExecuteSuccessfully())

			tests := []struct {
				name  string
				value float64
			}{
				{
					name:  nodeGroupMetricReadyName,
					value: 1.0,
				},
				{
					name:  nodeGroupMetricNodesName,
					value: 1.0,
				},
				{
					name:  nodeGroupMetricInstancesName,
					value: 1.0,
				},
				{
					name:  nodeGroupMetricDesiredName,
					value: 1.0,
				},
				{
					name:  nodeGroupMetricMinName,
					value: 1.0,
				},
				{
					name:  nodeGroupMetricMaxName,
					value: 1.0,
				},
				{
					name:  nodeGroupMetricUpToDateName,
					value: 1.0,
				},
				{
					name:  nodeGroupMetricStandbyName,
					value: 0.0,
				},
				{
					name:  nodeGroupMetricHasErrorsName,
					value: 0.0,
				},
			}

			for _, test := range tests {
				assertMetric(f, test.name, test.value)
			}
		})
	})

	Context("NodeGroup with 5 to 10 instances", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupWith5to10Instances))

			f.RunHook()
		})

		It("Test metrics", func() {
			Expect(f).To(ExecuteSuccessfully())

			tests := []struct {
				name  string
				value float64
			}{
				{
					name:  nodeGroupMetricReadyName,
					value: 4.0,
				},
				{
					name:  nodeGroupMetricNodesName,
					value: 4.0,
				},
				{
					name:  nodeGroupMetricInstancesName,
					value: 4.0,
				},
				{
					name:  nodeGroupMetricDesiredName,
					value: 5.0,
				},
				{
					name:  nodeGroupMetricMinName,
					value: 5.0,
				},
				{
					name:  nodeGroupMetricMaxName,
					value: 10.0,
				},
				{
					name:  nodeGroupMetricUpToDateName,
					value: 1.0,
				},
				{
					name:  nodeGroupMetricStandbyName,
					value: 0.0,
				},
				{
					name:  nodeGroupMetricHasErrorsName,
					value: 1.0,
				},
			}

			for _, test := range tests {
				assertMetric(f, test.name, test.value)
			}
		})
	})

	Context("NodeGroup with 2 to 4 instances", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupWith2to4Instances))

			f.RunHook()
		})

		It("Test metrics", func() {
			Expect(f).To(ExecuteSuccessfully())

			tests := []struct {
				name  string
				value float64
			}{
				{
					name:  nodeGroupMetricReadyName,
					value: 2.0,
				},
				{
					name:  nodeGroupMetricNodesName,
					value: 2.0,
				},
				{
					name:  nodeGroupMetricInstancesName,
					value: 2.0,
				},
				{
					name:  nodeGroupMetricDesiredName,
					value: 2.0,
				},
				{
					name:  nodeGroupMetricMinName,
					value: 2.0,
				},
				{
					name:  nodeGroupMetricMaxName,
					value: 4.0,
				},
				{
					name:  nodeGroupMetricUpToDateName,
					value: 2.0,
				},
				{
					name:  nodeGroupMetricStandbyName,
					value: 1.0,
				},
				{
					name:  nodeGroupMetricHasErrorsName,
					value: 0.0,
				},
			}

			for _, test := range tests {
				assertMetric(f, test.name, test.value)
			}
		})
	})
})

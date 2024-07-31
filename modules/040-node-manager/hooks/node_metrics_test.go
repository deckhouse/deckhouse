package hooks_test

import (
	. "github.com/deckhouse/deckhouse/testing/hooks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const NodeMetricsSample = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-1
  labels:
    node.deckhouse.io/group: "true"
status:
  conditions:
  - type: Ready
    status: "True"
`

const NodeNotReadyMetricsSample = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-2
  labels:
    node.deckhouse.io/group: "true"
status:
  conditions:
  - type: Ready
    status: "False"
`

const InstanceMetricsSample = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  name: instance-1
status:
  currentStatus:
    phase: Running
  nodeRef:
    name: node-1
`

var _ = Describe("Node Manager :: hooks :: node_instance_metrics ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "Instance", true)

	assertMetric := func(f *HookExecutionConfig, metricName, nodeName, expectedStatus string) {
		metrics := f.MetricsCollector.CollectedMetrics()
		ok := false
		for _, m := range metrics {
			if m.Name == metricName && m.Labels["node_name"] == nodeName {
				Expect(m.Labels["status"]).To(Equal(expectedStatus))
				ok = true
				break
			}
		}
		Expect(ok).To(BeTrue(), "Expected metric not found")
	}

	Context("Node and Instance metrics", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(NodeMetricsSample + NodeNotReadyMetricsSample + InstanceMetricsSample))
			f.RunHook()
		})

		It("Should set correct metrics for nodes and instances", func() {
			Expect(f).To(ExecuteSuccessfully())

			assertMetric(f, "d8_node_status", "node-1", "Ready")
			assertMetric(f, "d8_node_status", "node-2", "NotReady")

			metrics := f.MetricsCollector.CollectedMetrics()
			ok := false
			for _, m := range metrics {
				if m.Name == "d8_instance_status" {
					Expect(m.Labels["instance_name"]).To(Equal("instance-1"))
					Expect(m.Labels["node_name"]).To(Equal("node-1"))
					Expect(m.Labels["status"]).To(Equal("Running"))
					ok = true
					break
				}
			}
			Expect(ok).To(BeTrue(), "Expected instance metric not found")
		})
	})
})

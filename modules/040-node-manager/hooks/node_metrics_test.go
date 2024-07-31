package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: node and instance metrics", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "Instance", false)

	const nodeStatusReady = `
apiVersion: v1
kind: Node
metadata:
  name: node-1
  labels:
    node.deckhouse.io/group: worker
status:
  phase: Ready
`

	const instanceStatusRunning = `
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  name: instance-1
  labels:
    node.deckhouse.io/group: worker
status:
  phase: Running
  nodeRef:
    name: node-1
`

	assertMetric := func(f *HookExecutionConfig, name string, expected float64, labels map[string]string) {
		metrics := f.MetricsCollector.CollectedMetrics()

		ok := false
		for _, m := range metrics {
			if m.Name == name {
				Expect(m.Value).To(Equal(pointer.Float64(expected)))
				for k, v := range labels {
					Expect(m.Labels).To(HaveKeyWithValue(k, v))
				}

				ok = true
				break
			}
		}

		Expect(ok).To(BeTrue())
	}

	Context("Node and Instance metrics", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeStatusReady + instanceStatusRunning))
			f.RunHook()
		})

		It("Should collect metrics for nodes and instances", func() {
			Expect(f).To(ExecuteSuccessfully())

			nodeLabels := map[string]string{"node_name": "node-1", "status": "Ready"}
			instanceLabels := map[string]string{"instance_name": "instance-1", "status": "Running", "node_ref": "node-1"}

			assertMetric(f, nodeMetricName, 1.0, nodeLabels)
			assertMetric(f, instanceMetricName, 1.0, instanceLabels)
		})
	})
})

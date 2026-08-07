/*
Copyright 2026 Flant JSC

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

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Module hooks :: control-plane-manager :: resources_requests_autotune_sync", func() {
	f := HookExecutionConfigInit(
		`{"controlPlaneManager":{"internal":{"resourcesRequests":{}}},"global":{"enabledModules":["prometheus","prometheus-metrics-adapter"]}}`,
		`{}`,
	)

	Context("OnBeforeHelm: projects ConfigMap components into values", func() {
		BeforeEach(func() {
			st := autotuneState{
				resourceCPU: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver: {AppliedMilliCPU: ptr.To(int64(700))},
						componentEtcd:          {AppliedMilliCPU: ptr.To(int64(800))},
					},
				},
				resourceMemory: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver: {AppliedBytes: ptr.To(int64(1024000000))},
					},
				},
			}
			f.KubeStateSet(autotuneStateYAML(st))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("sets components without calling metrics or touching combined budgets", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeApiserver.milliCPU").Int()).To(Equal(int64(700)))
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.etcd.milliCPU").Int()).To(Equal(int64(800)))
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeApiserver.memoryBytes").String()).To(Equal("1024000000"))
			for _, m := range f.MetricsCollector.CollectedMetrics() {
				Expect(m.Name).NotTo(Equal(autotuneMetricName))
			}
		})
	})

	Context("OnBeforeHelm: missing ConfigMap clears components", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("controlPlaneManager.internal.resourcesRequests.components", []byte(`
kubeApiserver:
  milliCPU: 700
`))
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("removes components from values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components").Exists()).To(BeFalse())
		})
	})
})

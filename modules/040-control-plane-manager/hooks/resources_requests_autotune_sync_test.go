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
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Module hooks :: control-plane-manager :: resources_requests_autotune_sync", func() {
	f := HookExecutionConfigInit(
		`{"controlPlaneManager":{"internal":{"resourcesRequests":{"milliCpuControlPlane":2000,"memoryControlPlane":4294967296}}},"global":{"enabledModules":["prometheus","prometheus-metrics-adapter"]}}`,
		`{}`,
	)

	Context("Synchronization: repopulate from ConfigMap without metrics API", func() {
		var called bool

		BeforeEach(func() {
			called = false
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["prometheus","prometheus-metrics-adapter"]`))
			fetchComponentUsage = func(_ context.Context, _ dependency.Container, _ string, _ resourceKind) (float64, bool, error) {
				called = true
				return 0, false, nil
			}

			st := autotuneState{
				CPU: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver: {AppliedMilliCPU: ptr.To(int64(700)), LastChange: "2026-07-01T00:00:00Z"},
						componentEtcd:          {AppliedMilliCPU: ptr.To(int64(800)), LastChange: "2026-07-01T00:00:00Z"},
					},
					CapacityBlocked: &capacityBlocked{Since: "2026-07-20T00:00:00Z", Deficit: 500},
				},
				Memory: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver: {AppliedMB: ptr.To(int64(1024)), LastChange: "2026-07-01T00:00:00Z"},
					},
				},
			}
			f.BindingContexts.Set(f.KubeStateSet(masterNodeYAML() + autotuneStateYAML(st)))
			f.RunHook()
		})

		AfterEach(func() {
			fetchComponentUsage = fetchComponentUsageFromMetricsAPI
		})

		It("repopulates components and re-emits capacityBlocked metric without metrics API", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(called).To(BeFalse())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeApiserver.milliCPU").Int()).To(Equal(int64(700)))
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.etcd.milliCPU").Int()).To(Equal(int64(800)))
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeApiserver.memoryMB").Int()).To(Equal(int64(1024)))
			found := false
			for _, m := range f.MetricsCollector.CollectedMetrics() {
				if m.Name == autotuneMetricName {
					found = true
					Expect(m.Labels).To(HaveKeyWithValue("resource", "cpu"))
					Expect(*m.Value).To(Equal(float64(500)))
				}
			}
			Expect(found).To(BeTrue())
		})
	})

	Context("Synchronization: PMA disabled discards autotune for legacy fallback", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[]`))
			f.ValuesSetFromYaml("controlPlaneManager.internal.resourcesRequests.components", []byte(`
kubeApiserver:
  milliCPU: 700
`))
			st := autotuneState{
				CPU: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver: {AppliedMilliCPU: ptr.To(int64(700)), LastChange: "2026-07-01T00:00:00Z"},
					},
					CapacityBlocked: &capacityBlocked{Since: "2026-07-20T00:00:00Z", Deficit: 500},
				},
			}
			f.BindingContexts.Set(f.KubeStateSet(masterNodeYAML() + autotuneStateYAML(st)))
			f.RunHook()
		})

		It("removes components, expires capacityBlocked metric, deletes ConfigMap", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ConfigMap", "kube-system", autotuneStateCMName).Exists()).To(BeFalse())
			for _, m := range f.MetricsCollector.CollectedMetrics() {
				Expect(m.Name).NotTo(Equal(autotuneMetricName))
			}
		})
	})
})

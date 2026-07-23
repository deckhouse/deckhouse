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
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func autotuneStateYAML(state autotuneState) string {
	raw, err := json.Marshal(state)
	Expect(err).ToNot(HaveOccurred())
	// Embed JSON as a single-line string value for the ConfigMap.
	escaped, err := json.Marshal(string(raw))
	Expect(err).ToNot(HaveOccurred())
	return fmt.Sprintf(`
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: kube-system
data:
  state: %s
`, autotuneStateCMName, string(escaped))
}

func masterNodeYAML() string {
	return generateMasterNodesConfig([]masterNode{{
		cpu:    "8",
		memory: "16Gi",
		capCPU: "8",
		capMem: "16Gi",
	}})
}

var _ = Describe("Module hooks :: control-plane-manager :: autotune_resources_requests :: decide", func() {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)

	DescribeTable("asymmetric deadband + cooldown",
		func(rec, applied int64, lastChangeAgo time.Duration, zeroLast bool, want decideAction) {
			var last time.Time
			if !zeroLast {
				last = now.Add(-lastChangeAgo)
			}
			Expect(decide(rec, applied, last, now)).To(Equal(want))
		},
		Entry("first commit (no applied)", int64(500), int64(0), time.Duration(0), true, decideRaise),
		Entry("inside deadband", int64(110), int64(100), 48*time.Hour, false, decideSkip),
		Entry("raise above threshold after cooldown", int64(130), int64(100), 25*time.Hour, false, decideRaise),
		Entry("raise blocked by cooldown", int64(130), int64(100), 12*time.Hour, false, decideSkip),
		Entry("lower below threshold after cooldown", int64(60), int64(100), 73*time.Hour, false, decideLower),
		Entry("lower blocked by cooldown", int64(60), int64(100), 48*time.Hour, false, decideSkip),
		Entry("lower inside deadband (−20%)", int64(80), int64(100), 73*time.Hour, false, decideSkip),
	)
})

var _ = Describe("Module hooks :: control-plane-manager :: autotune_resources_requests", func() {
	f := HookExecutionConfigInit(
		`{"controlPlaneManager":{"internal":{"resourcesRequests":{"milliCpuControlPlane":2000,"memoryControlPlane":4294967296}}},"global":{"enabledModules":["prometheus-metrics-adapter"]}}`,
		`{}`,
	)

	var usage map[string]map[string]float64

	BeforeEach(func() {
		usage = map[string]map[string]float64{}
		fetchComponentUsage = func(_ context.Context, _ dependency.Container, component, resourceName string) (float64, bool, error) {
			if byRes, ok := usage[component]; ok {
				if v, ok := byRes[resourceName]; ok {
					return v, true, nil
				}
			}
			return 0, false, nil
		}
	})

	AfterEach(func() {
		fetchComponentUsage = fetchComponentUsageFromMetricsAPI
	})

	Context("Schedule: raise after cooldown", func() {
		BeforeEach(func() {
			now := dependency.TestDC.GetClock().Now()
			st := autotuneState{
				CPU: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver:         {AppliedMilliCPU: ptr.To(int64(100)), LastChange: now.Add(-48 * time.Hour).Format(time.RFC3339)},
						componentEtcd:                  {AppliedMilliCPU: ptr.To(int64(100)), LastChange: now.Add(-48 * time.Hour).Format(time.RFC3339)},
						componentKubeControllerManager: {AppliedMilliCPU: ptr.To(int64(100)), LastChange: now.Add(-48 * time.Hour).Format(time.RFC3339)},
						componentKubeScheduler:         {AppliedMilliCPU: ptr.To(int64(100)), LastChange: now.Add(-48 * time.Hour).Format(time.RFC3339)},
					},
				},
			}
			usage[componentKubeApiserver] = map[string]float64{resourceCPU: 0.25}
			f.KubeStateSet(masterNodeYAML() + autotuneStateYAML(st))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 3 * * *"))
			f.RunHook()
		})

		It("commits raised milliCPU for kube-apiserver", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeApiserver.milliCPU").Int()).To(Equal(int64(250)))
		})
	})

	Context("Schedule: raise blocked by capacity gate", func() {
		BeforeEach(func() {
			now := dependency.TestDC.GetClock().Now()
			tiny := generateMasterNodesConfig([]masterNode{{
				cpu: "1", memory: "2Gi", capCPU: "1", capMem: "2Gi",
			}})
			st := autotuneState{
				CPU: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver:         {AppliedMilliCPU: ptr.To(int64(50)), LastChange: now.Add(-48 * time.Hour).Format(time.RFC3339)},
						componentEtcd:                  {AppliedMilliCPU: ptr.To(int64(50)), LastChange: now.Add(-48 * time.Hour).Format(time.RFC3339)},
						componentKubeControllerManager: {AppliedMilliCPU: ptr.To(int64(50)), LastChange: now.Add(-48 * time.Hour).Format(time.RFC3339)},
						componentKubeScheduler:         {AppliedMilliCPU: ptr.To(int64(50)), LastChange: now.Add(-48 * time.Hour).Format(time.RFC3339)},
					},
				},
			}
			for _, c := range controlPlaneComponents {
				usage[c] = map[string]float64{resourceCPU: 0.5}
			}
			f.KubeStateSet(tiny + autotuneStateYAML(st))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 3 * * *"))
			f.RunHook()
		})

		It("keeps applied values and emits insufficient-capacity metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeApiserver.milliCPU").Int()).To(Equal(int64(50)))
			found := false
			for _, m := range f.MetricsCollector.CollectedMetrics() {
				if m.Name == autotuneMetricName {
					found = true
					Expect(m.Labels).To(HaveKeyWithValue("resource", "cpu"))
				}
			}
			Expect(found).To(BeTrue())
		})
	})

	Context("Schedule: manual CPU override deletes cpu state branch", func() {
		BeforeEach(func() {
			f.ValuesSet("controlPlaneManager.resourcesRequests.cpu", "1500m")
			st := autotuneState{
				CPU: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver: {AppliedMilliCPU: ptr.To(int64(700)), LastChange: "2026-07-01T00:00:00Z"},
						componentEtcd:          {AppliedMilliCPU: ptr.To(int64(800)), LastChange: "2026-07-01T00:00:00Z"},
					},
				},
				Memory: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver: {AppliedBytes: ptr.To(int64(512 * 1024 * 1024)), LastChange: "2026-07-01T00:00:00Z"},
					},
				},
			}
			f.KubeStateSet(masterNodeYAML() + autotuneStateYAML(st))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 3 * * *"))
			f.RunHook()
		})

		It("clears cpu components from values but keeps memory", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeApiserver.milliCPU").Exists()).To(BeFalse())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.etcd.milliCPU").Exists()).To(BeFalse())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeApiserver.memoryBytes").Int()).To(Equal(int64(512 * 1024 * 1024)))

			ops := f.KubernetesResource("ConfigMap", "kube-system", autotuneStateCMName)
			Expect(ops.Exists()).To(BeTrue())
			var st autotuneState
			Expect(json.Unmarshal([]byte(ops.Field("data.state").String()), &st)).To(Succeed())
			Expect(st.CPU).To(BeNil())
			Expect(st.Memory).ToNot(BeNil())
		})
	})

	Context("Schedule: PMA disabled still repopulates applied state", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[]`))
			st := autotuneState{
				CPU: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver: {AppliedMilliCPU: ptr.To(int64(420)), LastChange: "2026-07-01T00:00:00Z"},
					},
				},
			}
			f.KubeStateSet(masterNodeYAML() + autotuneStateYAML(st))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 3 * * *"))
			f.RunHook()
		})

		It("repopulates without calling metrics API", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeApiserver.milliCPU").Int()).To(Equal(int64(420)))
		})
	})

	Context("Managed cloud (no master nodes)", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateScheduleContext("0 3 * * *"))
			f.RunHook()
		})

		It("exits without writing components", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components").Exists()).To(BeFalse())
		})
	})
})

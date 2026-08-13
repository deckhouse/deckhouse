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

package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func autotuneStateYAML(state autotuneState) string {
	raw, err := json.Marshal(state)
	Expect(err).ToNot(HaveOccurred())
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

func brokenAutotuneStateYAML() string {
	return fmt.Sprintf(`
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: kube-system
data:
  state: "}{ not json"
`, autotuneStateCMName)
}

func autotuneCMState(f *HookExecutionConfig) autotuneState {
	ops := f.KubernetesResource("ConfigMap", "kube-system", autotuneStateCMName)
	ExpectWithOffset(1, ops.Exists()).To(BeTrue())
	var st autotuneState
	ExpectWithOffset(1, json.Unmarshal([]byte(ops.Field("data.state").String()), &st)).To(Succeed())
	return st
}

func hasDegradedReason(f *HookExecutionConfig, reason string) bool {
	for _, m := range f.MetricsCollector.CollectedMetrics() {
		if m.Name == autotuneDegradedMetricName && m.Labels["reason"] == reason {
			return true
		}
	}
	return false
}

func completeAutotuneState(milliCPU, bytes int64, lastChange time.Time) autotuneState {
	ts := lastChange.Format(time.RFC3339)
	st := autotuneState{
		resourceCPU:    &autotuneMeasurementState{Components: map[string]autotuneComponentState{}},
		resourceMemory: &autotuneMeasurementState{Components: map[string]autotuneComponentState{}},
	}
	for _, comp := range controlPlaneComponents {
		st[resourceCPU].Components[comp] = autotuneComponentState{AppliedMilliCPU: ptr.To(milliCPU), LastChange: ts}
		st[resourceMemory].Components[comp] = autotuneComponentState{AppliedBytes: ptr.To(bytes), LastChange: ts}
	}
	return st
}

func masterNodeYAML() string {
	return generateMasterNodesConfig([]masterNode{{
		cpu:    "8",
		memory: "16Gi",
		capCPU: "8",
		capMem: "16Gi",
	}})
}

func cpmResourcesRequestsMC(cpu, memory string) string {
	settings := ""
	if cpu != "" || memory != "" {
		settings = "  settings:\n    resourcesRequests:\n"
		if cpu != "" {
			settings += fmt.Sprintf("      cpu: %q\n", cpu)
		}
		if memory != "" {
			settings += fmt.Sprintf("      memory: %q\n", memory)
		}
	}
	return fmt.Sprintf(`
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  enabled: true
  version: 1
%s`, settings)
}

var _ = Describe("Module hooks :: control-plane-manager :: resources_requests_autotune :: decide", func() {
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
		Entry("raise blocked by cooldown", int64(130), int64(100), 2*time.Hour, false, decideSkip),
		Entry("lower below threshold after cooldown", int64(60), int64(100), 73*time.Hour, false, decideLower),
		Entry("lower blocked by cooldown", int64(60), int64(100), 24*time.Hour, false, decideSkip),
		Entry("lower inside deadband (−20%)", int64(80), int64(100), 73*time.Hour, false, decideSkip),
	)
})

var _ = Describe("Module hooks :: control-plane-manager :: resources_requests_autotune", func() {
	f := HookExecutionConfigInit(
		`{"controlPlaneManager":{"internal":{"resourcesRequests":{}}},"global":{"enabledModules":["prometheus","prometheus-metrics-adapter"]}}`,
		`{}`,
	)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	var usage map[string]map[resourceKind]float64
	var usageCalls int

	BeforeEach(func() {
		usage = map[string]map[resourceKind]float64{}
		usageCalls = 0
		f.ValuesSetFromYaml("global.enabledModules", []byte(`["prometheus","prometheus-metrics-adapter"]`))
		fetchComponentUsage = func(_ context.Context, _ dependency.Container, component string, resourceName resourceKind) (float64, bool, error) {
			usageCalls++
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
		listPodsOnNode = listPodsOnNodeFromAPI
	})

	Context("Schedule: raise after cooldown", func() {
		BeforeEach(func() {
			now := dependency.TestDC.GetClock().Now()
			st := autotuneState{
				resourceCPU: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver:         {AppliedMilliCPU: ptr.To(int64(100)), LastChange: now.Add(-25 * time.Hour).Format(time.RFC3339)},
						componentEtcd:                  {AppliedMilliCPU: ptr.To(int64(100)), LastChange: now.Add(-25 * time.Hour).Format(time.RFC3339)},
						componentKubeControllerManager: {AppliedMilliCPU: ptr.To(int64(100)), LastChange: now.Add(-25 * time.Hour).Format(time.RFC3339)},
						componentKubeScheduler:         {AppliedMilliCPU: ptr.To(int64(100)), LastChange: now.Add(-25 * time.Hour).Format(time.RFC3339)},
					},
				},
				resourceMemory: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver:         {AppliedBytes: ptr.To(int64(100000000)), LastChange: now.Add(-25 * time.Hour).Format(time.RFC3339)},
						componentEtcd:                  {AppliedBytes: ptr.To(int64(100000000)), LastChange: now.Add(-25 * time.Hour).Format(time.RFC3339)},
						componentKubeControllerManager: {AppliedBytes: ptr.To(int64(100000000)), LastChange: now.Add(-25 * time.Hour).Format(time.RFC3339)},
						componentKubeScheduler:         {AppliedBytes: ptr.To(int64(100000000)), LastChange: now.Add(-25 * time.Hour).Format(time.RFC3339)},
					},
				},
			}
			usage[componentKubeApiserver] = map[resourceKind]float64{resourceCPU: 0.25, resourceMemory: 100000000}
			usage[componentEtcd] = map[resourceKind]float64{resourceCPU: 0.10, resourceMemory: 100000000}
			usage[componentKubeControllerManager] = map[resourceKind]float64{resourceCPU: 0.10, resourceMemory: 100000000}
			usage[componentKubeScheduler] = map[resourceKind]float64{resourceCPU: 0.10, resourceMemory: 100000000}
			f.KubeStateSet(masterNodeYAML() + autotuneStateYAML(st) + cpmResourcesRequestsMC("", ""))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 3 * * *"))
			f.RunHook()
		})

		It("writes raised milliCPU into ConfigMap", func() {
			Expect(f).To(ExecuteSuccessfully())
			ops := f.KubernetesResource("ConfigMap", "kube-system", autotuneStateCMName)
			Expect(ops.Exists()).To(BeTrue())
			var st autotuneState
			Expect(json.Unmarshal([]byte(ops.Field("data.state").String()), &st)).To(Succeed())
			Expect(*st[resourceCPU].Components[componentKubeApiserver].AppliedMilliCPU).To(Equal(int64(250)))
		})
	})

	Context("Schedule: raise blocked by capacity gate", func() {
		BeforeEach(func() {
			now := dependency.TestDC.GetClock().Now()
			tiny := generateMasterNodesConfig([]masterNode{{
				cpu: "1", memory: "2Gi", capCPU: "1", capMem: "2Gi",
			}})
			st := autotuneState{
				resourceCPU: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver:         {AppliedMilliCPU: ptr.To(int64(50)), LastChange: now.Add(-25 * time.Hour).Format(time.RFC3339)},
						componentEtcd:                  {AppliedMilliCPU: ptr.To(int64(50)), LastChange: now.Add(-25 * time.Hour).Format(time.RFC3339)},
						componentKubeControllerManager: {AppliedMilliCPU: ptr.To(int64(50)), LastChange: now.Add(-25 * time.Hour).Format(time.RFC3339)},
						componentKubeScheduler:         {AppliedMilliCPU: ptr.To(int64(50)), LastChange: now.Add(-25 * time.Hour).Format(time.RFC3339)},
					},
				},
				resourceMemory: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver:         {AppliedBytes: ptr.To(int64(50000000)), LastChange: now.Add(-25 * time.Hour).Format(time.RFC3339)},
						componentEtcd:                  {AppliedBytes: ptr.To(int64(50000000)), LastChange: now.Add(-25 * time.Hour).Format(time.RFC3339)},
						componentKubeControllerManager: {AppliedBytes: ptr.To(int64(50000000)), LastChange: now.Add(-25 * time.Hour).Format(time.RFC3339)},
						componentKubeScheduler:         {AppliedBytes: ptr.To(int64(50000000)), LastChange: now.Add(-25 * time.Hour).Format(time.RFC3339)},
					},
				},
			}
			for _, c := range controlPlaneComponents {
				usage[c] = map[resourceKind]float64{resourceCPU: 0.5, resourceMemory: 50000000}
			}
			f.KubeStateSet(tiny + autotuneStateYAML(st) + cpmResourcesRequestsMC("", ""))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 3 * * *"))
			f.RunHook()
		})

		It("keeps applied values, stores pendingRaiseSum, emits deficit metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			ops := f.KubernetesResource("ConfigMap", "kube-system", autotuneStateCMName)
			var st autotuneState
			Expect(json.Unmarshal([]byte(ops.Field("data.state").String()), &st)).To(Succeed())
			Expect(*st[resourceCPU].Components[componentKubeApiserver].AppliedMilliCPU).To(Equal(int64(50)))
			Expect(st[resourceCPU].PendingRaiseSum).To(BeNumerically(">", 0))
			Expect(st[resourceCPU].Components[componentKubeApiserver]).ToNot(BeNil())
			found := false
			for _, m := range f.MetricsCollector.CollectedMetrics() {
				if m.Name == autotuneMetricName {
					found = true
					Expect(m.Labels).To(HaveKeyWithValue("resource", "cpu"))
					Expect(*m.Value).To(BeNumerically(">", 0))
				}
			}
			Expect(found).To(BeTrue())
		})
	})

	Context("Schedule: ModuleConfig CPU override writes %-split into CM", func() {
		BeforeEach(func() {
			f.KubeStateSet(masterNodeYAML() + cpmResourcesRequestsMC("2000m", ""))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 3 * * *"))
			f.RunHook()
		})

		It("splits combined CPU override across components", func() {
			Expect(f).To(ExecuteSuccessfully())
			ops := f.KubernetesResource("ConfigMap", "kube-system", autotuneStateCMName)
			var st autotuneState
			Expect(json.Unmarshal([]byte(ops.Field("data.state").String()), &st)).To(Succeed())
			Expect(*st[resourceCPU].Components[componentKubeApiserver].AppliedMilliCPU).To(Equal(fallbackSplit(2000, 45)))
			Expect(*st[resourceCPU].Components[componentEtcd].AppliedMilliCPU).To(Equal(fallbackSplit(2000, 35)))
		})
	})

	Context("Schedule: PMA off replaces previous metric-based applied with discovery split", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[]`))
			st := autotuneState{
				resourceCPU: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver:         {AppliedMilliCPU: ptr.To(int64(900)), LastChange: "2026-07-01T00:00:00Z"},
						componentEtcd:                  {AppliedMilliCPU: ptr.To(int64(800)), LastChange: "2026-07-01T00:00:00Z"},
						componentKubeControllerManager: {AppliedMilliCPU: ptr.To(int64(100)), LastChange: "2026-07-01T00:00:00Z"},
						componentKubeScheduler:         {AppliedMilliCPU: ptr.To(int64(100)), LastChange: "2026-07-01T00:00:00Z"},
					},
					PendingRaiseSum: 5000,
				},
			}
			f.KubeStateSet(masterNodeYAML() + autotuneStateYAML(st) + cpmResourcesRequestsMC("", ""))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 3 * * *"))
			f.RunHook()
		})

		It("overwrites applied* with discovery %-split and clears pendingRaiseSum", func() {
			Expect(f).To(ExecuteSuccessfully())
			// 8-CPU master is capped by hardLimitMilliCPU before the 40%/35% carve-out.
			expectCPU := int64(hardLimitMilliCPU-configEveryNodeMilliCPU) * controlPlanePercent / 100
			ops := f.KubernetesResource("ConfigMap", "kube-system", autotuneStateCMName)
			var st autotuneState
			Expect(json.Unmarshal([]byte(ops.Field("data.state").String()), &st)).To(Succeed())
			Expect(*st[resourceCPU].Components[componentKubeApiserver].AppliedMilliCPU).To(Equal(fallbackSplit(expectCPU, 45)))
			Expect(*st[resourceCPU].Components[componentEtcd].AppliedMilliCPU).To(Equal(fallbackSplit(expectCPU, 35)))
			Expect(st[resourceCPU].PendingRaiseSum).To(Equal(int64(0)))
		})
	})

	Context("Cluster without master nodes", func() {
		BeforeEach(func() {
			f.KubeStateSet(cpmResourcesRequestsMC("", ""))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 3 * * *"))
			f.RunHook()
		})

		It("is a no-op", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("ConfigMap", "kube-system", autotuneStateCMName).Exists()).To(BeFalse())
		})
	})

	Context("Discovery: kubelet floor applied when Capacity == Allocatable", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[]`))
			f.KubeStateSet(generateMasterNodesConfig([]masterNode{{cpu: "4", memory: "8Gi"}}) + cpmResourcesRequestsMC("", ""))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 3 * * *"))
			f.RunHook()
		})

		It(fmt.Sprintf("writes CM components as %d/%d/%d/%d split of discovery budget", 45, 35, 10, 10), func() {
			Expect(f).To(ExecuteSuccessfully())
			expectCPU := int64((4000-kubeletResourceReservationCPUFloor-configEveryNodeMilliCPU)*controlPlanePercent) / 100
			expectMem := int64((8*1024*1024*1024-kubeletResourceReservationMemoryFloor-configEveryNodeMemory)*controlPlanePercent) / 100

			ops := f.KubernetesResource("ConfigMap", "kube-system", autotuneStateCMName)
			Expect(ops.Exists()).To(BeTrue())
			var st autotuneState
			Expect(json.Unmarshal([]byte(ops.Field("data.state").String()), &st)).To(Succeed())
			Expect(*st[resourceCPU].Components[componentKubeApiserver].AppliedMilliCPU).To(Equal(fallbackSplit(expectCPU, 45)))
			Expect(*st[resourceCPU].Components[componentEtcd].AppliedMilliCPU).To(Equal(fallbackSplit(expectCPU, 35)))
			Expect(*st[resourceCPU].Components[componentKubeControllerManager].AppliedMilliCPU).To(Equal(fallbackSplit(expectCPU, 10)))
			Expect(*st[resourceCPU].Components[componentKubeScheduler].AppliedMilliCPU).To(Equal(fallbackSplit(expectCPU, 10)))
			Expect(*st[resourceMemory].Components[componentKubeApiserver].AppliedBytes).To(Equal(fallbackSplit(expectMem, 45)))
		})
	})

	Context("Discovery: CPM ModuleConfig override", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[]`))
			f.KubeStateSet(generateMasterNodesConfig([]masterNode{{cpu: "4", memory: "8Gi"}}) + cpmResourcesRequestsMC("1000m", "1Gi"))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 3 * * *"))
			f.RunHook()
		})

		It("uses ModuleConfig combined budget and splits into CM", func() {
			Expect(f).To(ExecuteSuccessfully())
			ops := f.KubernetesResource("ConfigMap", "kube-system", autotuneStateCMName)
			var st autotuneState
			Expect(json.Unmarshal([]byte(ops.Field("data.state").String()), &st)).To(Succeed())
			Expect(*st[resourceCPU].Components[componentKubeApiserver].AppliedMilliCPU).To(Equal(int64(450)))
			Expect(*st[resourceCPU].Components[componentEtcd].AppliedMilliCPU).To(Equal(int64(350)))
			Expect(*st[resourceCPU].Components[componentKubeControllerManager].AppliedMilliCPU).To(Equal(int64(100)))
			Expect(*st[resourceCPU].Components[componentKubeScheduler].AppliedMilliCPU).To(Equal(int64(100)))
			Expect(*st[resourceMemory].Components[componentKubeApiserver].AppliedBytes).To(Equal(fallbackSplit(1024*1024*1024, 45)))
			Expect(*st[resourceMemory].Components[componentEtcd].AppliedBytes).To(Equal(fallbackSplit(1024*1024*1024, 35)))
		})
	})

	Context("Schedule: listing pods on masters fails", func() {
		BeforeEach(func() {
			listPodsOnNode = func(_ context.Context, _ dependency.Container, _ string) ([]v1.Pod, error) {
				return nil, fmt.Errorf("connection refused")
			}
			for _, c := range controlPlaneComponents {
				usage[c] = map[resourceKind]float64{resourceCPU: 0.5, resourceMemory: 500000000}
			}
			f.KubeStateSet(masterNodeYAML() + cpmResourcesRequestsMC("", ""))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 3 * * *"))
			f.RunHook()
		})

		It("still writes a complete ConfigMap, skips the metrics path, reports degradation", func() {
			Expect(f).To(ExecuteSuccessfully())
			st := autotuneCMState(f)
			Expect(measurementIsComplete(st[resourceCPU], resourceCPU)).To(BeTrue())
			Expect(measurementIsComplete(st[resourceMemory], resourceMemory)).To(BeTrue())
			Expect(usageCalls).To(Equal(0))
			Expect(st[resourceCPU].LastMetricsRun).To(BeEmpty())
			Expect(hasDegradedReason(f, degradedReasonListPods)).To(BeTrue())
		})
	})

	Context("Schedule: master nodes below the discovery threshold", func() {
		BeforeEach(func() {
			tiny := generateMasterNodesConfig([]masterNode{{cpu: "300m", memory: "1Gi"}})
			f.KubeStateSet(tiny + cpmResourcesRequestsMC("", ""))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 3 * * *"))
			f.RunHook()
		})

		It("writes no budget rather than an invented one", func() {
			Expect(f).To(ExecuteSuccessfully())
			// The every-node reservation is honoured, so nothing is left to split and
			// the templates keep applying their own fallback.
			st := autotuneCMState(f)
			Expect(st[resourceCPU]).To(BeNil())
			Expect(st[resourceMemory]).To(BeNil())
			Expect(hasDegradedReason(f, degradedReasonNodesTooSmall)).To(BeTrue())
		})
	})

	Context("Schedule: unreadable state ConfigMap", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[]`))
			f.KubeStateSet(masterNodeYAML() + brokenAutotuneStateYAML() + cpmResourcesRequestsMC("", ""))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 3 * * *"))
			f.RunHook()
		})

		It("recomputes the state from scratch and reports degradation", func() {
			Expect(f).To(ExecuteSuccessfully())
			st := autotuneCMState(f)
			Expect(measurementIsComplete(st[resourceCPU], resourceCPU)).To(BeTrue())
			Expect(measurementIsComplete(st[resourceMemory], resourceMemory)).To(BeTrue())
			Expect(hasDegradedReason(f, degradedReasonBadState)).To(BeTrue())
		})
	})

	Context("Schedule: complete state with a fresh LastMetricsRun", func() {
		BeforeEach(func() {
			now := dependency.TestDC.GetClock().Now().UTC()
			st := completeAutotuneState(500, 500000000, now.Add(-100*time.Hour))
			st[resourceCPU].LastMetricsRun = now.Add(-1 * time.Hour).Format(time.RFC3339)
			st[resourceMemory].LastMetricsRun = now.Add(-1 * time.Hour).Format(time.RFC3339)
			for _, c := range controlPlaneComponents {
				usage[c] = map[resourceKind]float64{resourceCPU: 2, resourceMemory: 2000000000}
			}
			f.KubeStateSet(masterNodeYAML() + autotuneStateYAML(st) + cpmResourcesRequestsMC("", ""))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 3 * * *"))
			f.RunHook()
		})

		It("does not touch the metrics API and keeps the applied values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(usageCalls).To(Equal(0))
			st := autotuneCMState(f)
			Expect(*st[resourceCPU].Components[componentKubeApiserver].AppliedMilliCPU).To(Equal(int64(500)))
		})
	})

	Context("Schedule: a run that changes nothing", func() {
		var lastChange string

		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[]`))
			now := dependency.TestDC.GetClock().Now().UTC()
			lastChange = now.Add(-200 * time.Hour).Format(time.RFC3339)
			st := autotuneState{
				resourceCPU:    &autotuneMeasurementState{Components: map[string]autotuneComponentState{}},
				resourceMemory: &autotuneMeasurementState{Components: map[string]autotuneComponentState{}},
			}
			for _, comp := range controlPlaneComponents {
				pct := componentMeta[comp].percent
				st[resourceCPU].Components[comp] = autotuneComponentState{
					AppliedMilliCPU: ptr.To(fallbackSplit(1000, pct)), LastChange: lastChange,
				}
				st[resourceMemory].Components[comp] = autotuneComponentState{
					AppliedBytes: ptr.To(fallbackSplit(1024*1024*1024, pct)), LastChange: lastChange,
				}
			}
			st[resourceCPU].AppliedOverride = ptr.To(int64(1000))
			st[resourceMemory].AppliedOverride = ptr.To(int64(1024 * 1024 * 1024))
			f.KubeStateSet(masterNodeYAML() + autotuneStateYAML(st) + cpmResourcesRequestsMC("1000m", "1Gi"))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 3 * * *"))
			f.RunHook()
		})

		It("keeps LastChange untouched", func() {
			Expect(f).To(ExecuteSuccessfully())
			st := autotuneCMState(f)
			for _, comp := range controlPlaneComponents {
				Expect(st[resourceCPU].Components[comp].LastChange).To(Equal(lastChange))
				Expect(st[resourceMemory].Components[comp].LastChange).To(Equal(lastChange))
			}
		})
	})

	Context("Schedule: ModuleConfig override removed", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[]`))
			now := dependency.TestDC.GetClock().Now().UTC()
			st := autotuneState{
				resourceCPU:    &autotuneMeasurementState{Components: map[string]autotuneComponentState{}},
				resourceMemory: &autotuneMeasurementState{Components: map[string]autotuneComponentState{}},
			}
			for _, comp := range controlPlaneComponents {
				pct := componentMeta[comp].percent
				st[resourceCPU].Components[comp] = autotuneComponentState{
					AppliedMilliCPU: ptr.To(fallbackSplit(2000, pct)),
					LastChange:      now.Add(-200 * time.Hour).Format(time.RFC3339),
				}
			}
			st[resourceCPU].AppliedOverride = ptr.To(int64(2000))
			f.KubeStateSet(masterNodeYAML() + autotuneStateYAML(st) + cpmResourcesRequestsMC("", ""))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 3 * * *"))
			f.RunHook()
		})

		It("rebases on the discovery budget immediately and clears AppliedOverride", func() {
			Expect(f).To(ExecuteSuccessfully())
			// 8-CPU master is capped by hardLimitMilliCPU before the carve-out.
			expectCPU := int64(hardLimitMilliCPU-configEveryNodeMilliCPU) * controlPlanePercent / 100
			st := autotuneCMState(f)
			Expect(st[resourceCPU].AppliedOverride).To(BeNil())
			Expect(*st[resourceCPU].Components[componentKubeApiserver].AppliedMilliCPU).
				To(Equal(fallbackSplit(expectCPU, 45)))
		})
	})

	Context("Schedule: cold start without a ConfigMap", func() {
		BeforeEach(func() {
			for _, c := range controlPlaneComponents {
				usage[c] = map[resourceKind]float64{resourceCPU: 0.3, resourceMemory: 300000000}
			}
			f.KubeStateSet(masterNodeYAML() + cpmResourcesRequestsMC("", ""))
			f.BindingContexts.Set(f.GenerateScheduleContext("0 3 * * *"))
			f.RunHook()
		})

		It("runs the metrics path at once and gates the immediate next run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(usageCalls).To(BeNumerically(">", 0))
			st := autotuneCMState(f)
			Expect(st[resourceCPU].LastMetricsRun).ToNot(BeEmpty())
			Expect(st[resourceMemory].LastMetricsRun).ToNot(BeEmpty())

			// The ConfigMap the hook just wrote is now in the cluster.
			callsAfterFirst := usageCalls
			f.BindingContexts.Set(f.GenerateScheduleContext("0 3 * * *"))
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(usageCalls).To(Equal(callsAfterFirst))
		})
	})
})

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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func masterNodeYAML() string {
	return generateMasterNodesConfig([]masterNode{{
		cpu:    "8",
		memory: "16Gi",
		capCPU: "8",
		capMem: "16Gi",
	}})
}

func setNearFallbackUsage(usage map[string]map[resourceKind]float64) {
	// Memory stubs are MB (PodMetric unit); clampRecommendation converts to bytes.
	usage[componentKubeApiserver] = map[resourceKind]float64{
		resourceCPU:    0.66,
		resourceMemory: 1417.34,
	}
	usage[componentEtcd] = map[resourceKind]float64{
		resourceCPU:    0.70,
		resourceMemory: 1503.24,
	}
	usage[componentKubeControllerManager] = map[resourceKind]float64{
		resourceCPU:    0.40,
		resourceMemory: 858.99,
	}
	usage[componentKubeScheduler] = map[resourceKind]float64{
		resourceCPU:    0.20,
		resourceMemory: 429.50,
	}
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
		Entry("raise above threshold after cooldown", int64(130), int64(100), 6*time.Minute, false, decideRaise),
		Entry("raise blocked by cooldown", int64(130), int64(100), 2*time.Minute, false, decideSkip),
		Entry("lower below threshold after cooldown", int64(60), int64(100), 16*time.Minute, false, decideLower),
		Entry("lower blocked by cooldown", int64(60), int64(100), 5*time.Minute, false, decideSkip),
		Entry("lower inside deadband (−20%)", int64(80), int64(100), 16*time.Minute, false, decideSkip),
	)
})

var _ = Describe("Module hooks :: control-plane-manager :: resources_requests_autotune", func() {
	f := HookExecutionConfigInit(
		`{"controlPlaneManager":{"internal":{"resourcesRequests":{"milliCpuControlPlane":2000,"memoryControlPlane":4294967296}}},"global":{"enabledModules":["prometheus","prometheus-metrics-adapter"]}}`,
		`{}`,
	)

	var usage map[string]map[resourceKind]float64

	BeforeEach(func() {
		usage = map[string]map[resourceKind]float64{}
		f.ValuesDelete("controlPlaneManager.resourcesRequests")
		f.ValuesSetFromYaml("global.enabledModules", []byte(`["prometheus","prometheus-metrics-adapter"]`))
		fetchComponentUsage = func(_ context.Context, _ dependency.Container, component string, resourceName resourceKind) (float64, bool, error) {
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
				resourceCPU: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver:         {AppliedMilliCPU: ptr.To(int64(100)), LastChange: now.Add(-20 * time.Minute).Format(time.RFC3339)},
						componentEtcd:                  {AppliedMilliCPU: ptr.To(int64(100)), LastChange: now.Add(-20 * time.Minute).Format(time.RFC3339)},
						componentKubeControllerManager: {AppliedMilliCPU: ptr.To(int64(100)), LastChange: now.Add(-20 * time.Minute).Format(time.RFC3339)},
						componentKubeScheduler:         {AppliedMilliCPU: ptr.To(int64(100)), LastChange: now.Add(-20 * time.Minute).Format(time.RFC3339)},
					},
				},
			}
			usage[componentKubeApiserver] = map[resourceKind]float64{resourceCPU: 0.25}
			f.KubeStateSet(masterNodeYAML() + autotuneStateYAML(st))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
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
				resourceCPU: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver:         {AppliedMilliCPU: ptr.To(int64(50)), LastChange: now.Add(-20 * time.Minute).Format(time.RFC3339)},
						componentEtcd:                  {AppliedMilliCPU: ptr.To(int64(50)), LastChange: now.Add(-20 * time.Minute).Format(time.RFC3339)},
						componentKubeControllerManager: {AppliedMilliCPU: ptr.To(int64(50)), LastChange: now.Add(-20 * time.Minute).Format(time.RFC3339)},
						componentKubeScheduler:         {AppliedMilliCPU: ptr.To(int64(50)), LastChange: now.Add(-20 * time.Minute).Format(time.RFC3339)},
					},
				},
			}
			for _, c := range controlPlaneComponents {
				usage[c] = map[resourceKind]float64{resourceCPU: 0.5}
			}
			f.KubeStateSet(tiny + autotuneStateYAML(st))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
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

	Context("Schedule: raise blocked by other pods on master", func() {
		BeforeEach(func() {
			now := dependency.TestDC.GetClock().Now()
			// 8 CPU master: effective ≈ 7900m after kubelet floor. Other pods take 7000m,
			// so only ~900m free — four 500m raises (2000m) do not fit.
			st := autotuneState{
				resourceCPU: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver:         {AppliedMilliCPU: ptr.To(int64(100)), LastChange: now.Add(-20 * time.Minute).Format(time.RFC3339)},
						componentEtcd:                  {AppliedMilliCPU: ptr.To(int64(100)), LastChange: now.Add(-20 * time.Minute).Format(time.RFC3339)},
						componentKubeControllerManager: {AppliedMilliCPU: ptr.To(int64(100)), LastChange: now.Add(-20 * time.Minute).Format(time.RFC3339)},
						componentKubeScheduler:         {AppliedMilliCPU: ptr.To(int64(100)), LastChange: now.Add(-20 * time.Minute).Format(time.RFC3339)},
					},
				},
			}
			for _, c := range controlPlaneComponents {
				usage[c] = map[resourceKind]float64{resourceCPU: 0.5}
			}
			// client-go fakes do not index spec.nodeName; stub the per-node list.
			listPodsOnNode = func(_ context.Context, _ dependency.Container, nodeName string) ([]v1.Pod, error) {
				if nodeName != "sandbox-0" {
					return nil, nil
				}
				return []v1.Pod{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-api-proxy",
						Namespace: "kube-system",
						Labels:    map[string]string{"tier": "control-plane", "component": "kube-api-proxy"},
					},
					Spec: v1.PodSpec{
						NodeName: "sandbox-0",
						Containers: []v1.Container{{
							Name: "proxy",
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("7"),
									v1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
						}},
					},
				}}, nil
			}
			f.KubeStateSet(masterNodeYAML() + autotuneStateYAML(st))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})

		AfterEach(func() {
			listPodsOnNode = listPodsOnNodeFromAPI
		})

		It("blocks raises that would not fit beside kube-api-proxy and other non-autotuned requests", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeApiserver.milliCPU").Int()).To(Equal(int64(100)))
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
				resourceCPU: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver: {AppliedMilliCPU: ptr.To(int64(700)), LastChange: "2026-07-01T00:00:00Z"},
						componentEtcd:          {AppliedMilliCPU: ptr.To(int64(800)), LastChange: "2026-07-01T00:00:00Z"},
					},
				},
				resourceMemory: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver: {AppliedMB: ptr.To(int64(512)), LastChange: "2026-07-01T00:00:00Z"},
					},
				},
			}
			f.KubeStateSet(masterNodeYAML() + autotuneStateYAML(st))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})

		It("clears cpu components from values but keeps memory", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeApiserver.milliCPU").Exists()).To(BeFalse())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.etcd.milliCPU").Exists()).To(BeFalse())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeApiserver.memoryMB").Int()).To(Equal(int64(512)))

			ops := f.KubernetesResource("ConfigMap", "kube-system", autotuneStateCMName)
			Expect(ops.Exists()).To(BeTrue())
			var st autotuneState
			Expect(json.Unmarshal([]byte(ops.Field("data.state").String()), &st)).To(Succeed())
			Expect(st[resourceCPU]).To(BeNil())
			Expect(st[resourceMemory]).ToNot(BeNil())
		})
	})

	Context("OnBeforeHelm: manual CPU override clears cpu without waiting for cron", func() {
		BeforeEach(func() {
			f.ValuesSet("controlPlaneManager.resourcesRequests.cpu", "1500m")
			st := autotuneState{
				resourceCPU: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver: {AppliedMilliCPU: ptr.To(int64(700)), LastChange: "2026-07-01T00:00:00Z"},
						componentEtcd:          {AppliedMilliCPU: ptr.To(int64(800)), LastChange: "2026-07-01T00:00:00Z"},
					},
				},
				resourceMemory: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver: {AppliedMB: ptr.To(int64(512)), LastChange: "2026-07-01T00:00:00Z"},
					},
				},
			}
			f.KubeStateSet(masterNodeYAML() + autotuneStateYAML(st))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("clears cpu components from values but keeps memory", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeApiserver.milliCPU").Exists()).To(BeFalse())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.etcd.milliCPU").Exists()).To(BeFalse())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeApiserver.memoryMB").Int()).To(Equal(int64(512)))

			ops := f.KubernetesResource("ConfigMap", "kube-system", autotuneStateCMName)
			Expect(ops.Exists()).To(BeTrue())
			var st autotuneState
			Expect(json.Unmarshal([]byte(ops.Field("data.state").String()), &st)).To(Succeed())
			Expect(st[resourceCPU]).To(BeNil())
			Expect(st[resourceMemory]).ToNot(BeNil())
		})
	})

	Context("Schedule: both measurements overridden clears components without merge error", func() {
		BeforeEach(func() {
			// Pre-seed components so Remove is exercised (the previous double-Remove
			// bug only appeared when this key already existed in module values).
			f.ValuesSetFromYaml("controlPlaneManager.internal.resourcesRequests.components", []byte(`
kubeApiserver:
  milliCPU: 700
  memoryMB: 536
etcd:
  milliCPU: 800
`))
			f.ValuesSet("controlPlaneManager.resourcesRequests.cpu", "1500m")
			f.ValuesSet("controlPlaneManager.resourcesRequests.memory", "2Gi")
			st := autotuneState{
				resourceCPU: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver: {AppliedMilliCPU: ptr.To(int64(700)), LastChange: "2026-07-01T00:00:00Z"},
					},
				},
				resourceMemory: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver: {AppliedMB: ptr.To(int64(512)), LastChange: "2026-07-01T00:00:00Z"},
					},
				},
			}
			f.KubeStateSet(masterNodeYAML() + autotuneStateYAML(st))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})

		It("removes components and both state branches", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components").Exists()).To(BeFalse())

			ops := f.KubernetesResource("ConfigMap", "kube-system", autotuneStateCMName)
			Expect(ops.Exists()).To(BeTrue())
			var st autotuneState
			Expect(json.Unmarshal([]byte(ops.Field("data.state").String()), &st)).To(Succeed())
			Expect(st[resourceCPU]).To(BeNil())
			Expect(st[resourceMemory]).To(BeNil())
		})
	})

	Context("Schedule: first memory commit", func() {
		BeforeEach(func() {
			setNearFallbackUsage(usage)
			// Apiserver recommendations differ enough from %-split (660m / ~1.4Gi)
			// to pass the deadband; others stay near fallback.
			usage[componentKubeApiserver] = map[resourceKind]float64{
				resourceCPU:    0.25,
				resourceMemory: 256,
			}
			f.KubeStateSet(masterNodeYAML())
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})

		It("commits milliCPU and memoryMB together", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeApiserver.milliCPU").Int()).To(Equal(int64(250)))
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeApiserver.memoryMB").Int()).To(Equal(int64(256)))
			// Full initial snapshot materializes every component in one values write.
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.etcd.milliCPU").Exists()).To(BeTrue())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.etcd.memoryMB").Exists()).To(BeTrue())
		})
	})

	Context("Schedule: incomplete initial metrics wait without writing components", func() {
		BeforeEach(func() {
			usage[componentKubeApiserver] = map[resourceKind]float64{
				resourceCPU:    0.25,
				resourceMemory: 256,
			}
			f.KubeStateSet(masterNodeYAML())
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})

		It("leaves components unset until the full set is available", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components").Exists()).To(BeFalse())
		})
	})

	Context("Schedule: empty-string memory override does not skip memory autotune", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("controlPlaneManager.resourcesRequests", []byte("memory: \"\"\n"))
			setNearFallbackUsage(usage)
			usage[componentKubeApiserver] = map[resourceKind]float64{
				resourceCPU:    0.25,
				resourceMemory: 256,
			}
			f.KubeStateSet(masterNodeYAML())
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})

		It("commits memoryMB despite empty memory key", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeApiserver.milliCPU").Int()).To(Equal(int64(250)))
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeApiserver.memoryMB").Int()).To(Equal(int64(256)))
		})
	})

	Context("Schedule: PMA disabled discards autotune for legacy fallback", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["prometheus"]`))
			f.ValuesSetFromYaml("controlPlaneManager.internal.resourcesRequests.components", []byte(`
kubeApiserver:
  milliCPU: 420
`))
			st := autotuneState{
				resourceCPU: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver: {AppliedMilliCPU: ptr.To(int64(420)), LastChange: "2026-07-01T00:00:00Z"},
					},
				},
			}
			f.KubeStateSet(masterNodeYAML() + autotuneStateYAML(st))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})

		It("removes components and deletes autotune ConfigMap", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ConfigMap", "kube-system", autotuneStateCMName).Exists()).To(BeFalse())
		})
	})

	Context("Managed cloud (no master nodes)", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})

		It("exits without writing components", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components").Exists()).To(BeFalse())
		})
	})
})

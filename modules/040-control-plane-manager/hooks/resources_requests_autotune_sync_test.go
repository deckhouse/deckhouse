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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
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

		It("removes components from values without reporting an anomaly", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components").Exists()).To(BeFalse())
			Expect(hasDegradedReason(f, degradedReasonReadThrough)).To(BeFalse())
		})
	})

	Context("OnBeforeHelm: empty snapshot, ConfigMap present in the API", func() {
		BeforeEach(func() {
			st := autotuneState{
				resourceCPU:    &autotuneMeasurementState{Components: map[string]autotuneComponentState{}},
				resourceMemory: &autotuneMeasurementState{Components: map[string]autotuneComponentState{}},
			}
			for _, comp := range controlPlaneComponents {
				st[resourceCPU].Components[comp] = autotuneComponentState{AppliedMilliCPU: ptr.To(int64(600))}
				st[resourceMemory].Components[comp] = autotuneComponentState{AppliedBytes: ptr.To(int64(700000000))}
			}
			f.KubeStateSet(``)
			// The ConfigMap created right after this line is visible through the API
			// but absent from the snapshot — the race read-through exists for.
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())

			raw, err := json.Marshal(st)
			Expect(err).ToNot(HaveOccurred())
			client, err := dependency.TestDC.GetK8sClient()
			Expect(err).ToNot(HaveOccurred())
			_, err = client.CoreV1().ConfigMaps(kubeSystemNS).Create(context.Background(), &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: autotuneStateCMName, Namespace: kubeSystemNS},
				Data:       map[string]string{autotuneStateKey: string(raw)},
			}, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			f.RunHook()
		})

		It("reads through and sets values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeApiserver.milliCPU").Int()).To(Equal(int64(600)))
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeScheduler.memoryBytes").String()).To(Equal("700000000"))
		})
	})

	Context("OnBeforeHelm: empty snapshot, API error", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("controlPlaneManager.internal.resourcesRequests.components", []byte(`
kubeApiserver:
  milliCPU: 700
`))
			getAutotuneStateCM = func(context.Context, dependency.Container) (autotuneState, error) {
				return nil, fmt.Errorf("connection refused")
			}
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		AfterEach(func() {
			getAutotuneStateCM = getAutotuneStateFromAPI
		})

		It("keeps the last known values and reports degradation", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeApiserver.milliCPU").Int()).To(Equal(int64(700)))
			Expect(hasDegradedReason(f, degradedReasonReadThrough)).To(BeTrue())
		})
	})

	Context("OnBeforeHelm: incomplete components map", func() {
		BeforeEach(func() {
			st := autotuneState{
				resourceCPU: &autotuneMeasurementState{
					Components: map[string]autotuneComponentState{
						componentKubeApiserver: {AppliedMilliCPU: ptr.To(int64(700))},
						componentEtcd:          {AppliedMilliCPU: ptr.To(int64(800))},
					},
				},
			}
			f.KubeStateSet(autotuneStateYAML(st))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("applies what is available and reports incompleteness", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeApiserver.milliCPU").Int()).To(Equal(int64(700)))
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.components.kubeScheduler").Exists()).To(BeFalse())

			resources := map[string]bool{}
			for _, m := range f.MetricsCollector.CollectedMetrics() {
				if m.Name == autotuneIncompleteMetricName {
					resources[m.Labels["resource"]] = true
				}
			}
			Expect(resources).To(HaveKey("cpu"))
			Expect(resources).To(HaveKey("memory"))
		})
	})
})

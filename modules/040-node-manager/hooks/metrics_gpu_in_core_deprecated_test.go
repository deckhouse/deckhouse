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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	gpuInCoreNGsYaml = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker-gpu
spec:
  gpu:
    sharing: time-slicing
  nodeType: Static
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
`
	gpuInCoreNGWithoutGPUYaml = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
`
	gpuInCoreMigNGYaml = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker-gpu-mig
spec:
  gpu:
    sharing: mig
    mig:
      partedConfig: all-1g.5gb
  nodeType: Static
`
)

var _ = Describe("Modules :: nodeManager :: hooks :: metrics_gpu_in_core_deprecated ::", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)
	var nodeGroupResource = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "nodegroups"}
	f.RegisterCRD(nodeGroupResource.Group, nodeGroupResource.Version, "NodeGroup", false)

	expireOnly := func() operation.MetricOperation {
		return operation.MetricOperation{
			Group:  "d8_node_group_gpu_in_core_deprecated",
			Action: operation.ActionExpireMetrics,
		}
	}

	deprecatedMetric := func(name string) operation.MetricOperation {
		return operation.MetricOperation{
			Name:   "d8_node_group_gpu_in_core_deprecated",
			Group:  "d8_node_group_gpu_in_core_deprecated",
			Action: operation.ActionGaugeSet,
			Value:  ptr.To(1.0),
			Labels: map[string]string{"name": name},
		}
	}

	Context("Hook bindings", func() {
		It("Must be triggered both by NodeGroup events and by OnAfterHelm", func() {
			Expect(f.GoHook).NotTo(BeNil())

			config := f.GoHook.GetConfig()
			// Without this binding, enabling or disabling the `gpu` module (which changes
			// global.enabledModules only) would never re-run the hook, so the metric would
			// never be expired and the alert would keep asking to enable an enabled module.
			Expect(config.OnAfterHelm).NotTo(BeNil())
			Expect(config.Kubernetes).To(HaveLen(1))
			Expect(config.Kubernetes[0].Kind).To(Equal("NodeGroup"))
		})
	})

	Context("An empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunGoHook()
		})

		It("Must expire the metric group and export nothing", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(expireOnly()))
		})
	})

	Context("NodeGroup with spec.gpu and the gpu module is disabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["vertical-pod-autoscaler", "prometheus"]`))
			f.BindingContexts.Set(f.KubeStateSet(gpuInCoreNGsYaml))
			f.RunGoHook()
		})

		It("Must export the metric only for the GPU NodeGroup", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(expireOnly()))
			Expect(m[1]).To(BeEquivalentTo(deprecatedMetric("worker-gpu")))
		})
	})

	Context("MIG NodeGroup without spec.gpu.sharing and the gpu module is disabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[]`))
			f.BindingContexts.Set(f.KubeStateSet(gpuInCoreMigNGYaml))
			f.RunGoHook()
		})

		It("Must export the metric for the MIG NodeGroup", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(expireOnly()))
			Expect(m[1]).To(BeEquivalentTo(deprecatedMetric("worker-gpu-mig")))
		})
	})

	Context("NodeGroup with spec.gpu and the gpu module is enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["gpu"]`))
			f.BindingContexts.Set(f.KubeStateSet(gpuInCoreNGsYaml))
			f.RunGoHook()
		})

		It("Must export no metrics", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(expireOnly()))
		})
	})

	Context("NodeGroup with spec.gpu and the gpu module is enabled among other modules", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["vertical-pod-autoscaler", "gpu", "prometheus"]`))
			f.BindingContexts.Set(f.KubeStateSet(gpuInCoreNGsYaml))
			f.RunGoHook()
		})

		It("Must export no metrics", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(expireOnly()))
		})
	})

	Context("NodeGroups without spec.gpu and the gpu module is disabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["prometheus"]`))
			f.BindingContexts.Set(f.KubeStateSet(gpuInCoreNGWithoutGPUYaml))
			f.RunGoHook()
		})

		It("Must export no metrics", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(expireOnly()))
		})
	})

	Context("The gpu module is enabled between two hook runs", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["prometheus"]`))
			f.BindingContexts.Set(f.KubeStateSet(gpuInCoreNGsYaml))
			f.RunGoHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(2))

			// Enabling the module changes global.enabledModules only, no NodeGroup is
			// touched, so the metric may only be expired thanks to the OnAfterHelm binding.
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["prometheus", "gpu"]`))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunGoHook()
		})

		It("Must expire the metric group and export nothing", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(expireOnly()))
		})
	})

	Context("The gpu module is disabled between two hook runs", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["prometheus", "gpu"]`))
			f.BindingContexts.Set(f.KubeStateSet(gpuInCoreNGsYaml))
			f.RunGoHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(1))

			f.ValuesSetFromYaml("global.enabledModules", []byte(`["prometheus"]`))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunGoHook()
		})

		It("Must export the metric for the GPU NodeGroup again", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(expireOnly()))
			Expect(m[1]).To(BeEquivalentTo(deprecatedMetric("worker-gpu")))
		})
	})

	Context("GPU NodeGroup is deleted", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["prometheus"]`))
			f.BindingContexts.Set(f.KubeStateSet(gpuInCoreNGsYaml))
			f.RunGoHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(2))

			f.BindingContexts.Set(f.KubeStateSet(gpuInCoreNGWithoutGPUYaml))
			f.RunGoHook()
		})

		It("Must expire the metric group and export nothing", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(expireOnly()))
		})
	})
})

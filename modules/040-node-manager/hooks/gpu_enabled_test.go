/*
Copyright 2025 Flant JSC

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

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	gpuNode0Yaml = `
---
apiVersion: v1
kind: Node
metadata:
  name: worker-gpu-0
  labels:
    node.deckhouse.io/group: worker-gpu
spec:
  providerID: static:///22d24f3645e885e88693cb5b235977af5acdc6c21efac9c075b56b618a1b539
`
	gpuNode1Yaml = `
---
apiVersion: v1
kind: Node
metadata:
  name: worker-gpu-1
  labels:
    node.deckhouse.io/group: worker-gpu
    node.deckhouse.io/gpu: ""
    node.deckhouse.io/device-gpu.config: time-slicing
    nvidia.com/mig.config: all-1g.5gb
spec:
  providerID: static:///22d24f3645e885e88693cb5b235977af5acdc6c21efac9c075b56b618a1b5338
`
	gpuNode2Yaml = `
---
apiVersion: v1
kind: Node
metadata:
  name: worker-gpu-2
  labels:
    node.deckhouse.io/group: worker-gpu-1
spec:
  providerID: static:///22d24f3645e885e88693cb5b235977af5acdc6c21efac9c075b56b618a1b536
`
	workerNodeYaml = `
---
apiVersion: v1
kind: Node
metadata:
  name: worker-0
  labels:
    node.deckhouse.io/group: worker
spec:
  providerID: static:///22d24f3645e885e88693cb5b235977af5acdc6c21efac9c075b56b618a1b5337
`
	gpuNodeCustomYaml = `
---
apiVersion: v1
kind: Node
metadata:
  name: worker-gpu-custom
  labels:
    node.deckhouse.io/group: worker-gpu-custom
spec:
  providerID: static:///22d24f3645e885e88693cb5b235977af5acdc6c21efac9c075b56b618a1b537
`
	ngsYaml = `
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
  name: worker-gpu-1
spec:
  gpu:
    sharing: mig
    mig:
      partedConfig: all-1g.5gb
  nodeType: Static
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker-gpu-custom
spec:
  gpu:
    sharing: mig
    mig:
      partedConfig: custom
      customConfigs:
        - index: 0
          slices:
            - profile: 1g.10gb
  nodeType: Static
`
)

var _ = Describe("Modules :: nodeManager :: hooks :: gpu_enabled ::", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)
	var nodeGroupResource = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "nodegroups"}
	f.RegisterCRD(nodeGroupResource.Group, nodeGroupResource.Version, "NodeGroup", false)

	Context("GPU module is enabled", func() {
		BeforeEach(func() {
			f.KubeStateSet(ngsYaml + gpuNode0Yaml + workerNodeYaml)
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["gpu"]`))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunGoHook()
		})

		It("Must be executed successfully and skip labeling", func() {
			Expect(f).To(ExecuteSuccessfully())
			// Node should not have GPU labels since hook was skipped
			workerGpu0 := f.KubernetesGlobalResource("Node", "worker-gpu-0")
			Expect(workerGpu0.Field("metadata.labels").Map()).NotTo(HaveKey("node.deckhouse.io/gpu"))
			Expect(workerGpu0.Field("metadata.labels").Map()).NotTo(HaveKey("node.deckhouse.io/device-gpu.config"))
		})
	})

	Context("GPU module is enabled among other modules", func() {
		BeforeEach(func() {
			f.KubeStateSet(ngsYaml + gpuNode0Yaml + workerNodeYaml)
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["vertical-pod-autoscaler", "gpu", "prometheus"]`))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunGoHook()
		})

		It("Must be executed successfully and skip labeling", func() {
			Expect(f).To(ExecuteSuccessfully())
			// Node should not have GPU labels since hook was skipped
			workerGpu0 := f.KubernetesGlobalResource("Node", "worker-gpu-0")
			Expect(workerGpu0.Field("metadata.labels").Map()).NotTo(HaveKey("node.deckhouse.io/gpu"))
			Expect(workerGpu0.Field("metadata.labels").Map()).NotTo(HaveKey("node.deckhouse.io/device-gpu.config"))
		})
	})

	Context("GPU module is not enabled", func() {
		BeforeEach(func() {
			f.KubeStateSet(ngsYaml + gpuNode0Yaml + workerNodeYaml)
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["vertical-pod-autoscaler", "prometheus"]`))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunGoHook()
		})

		It("Must be executed successfully and apply labels", func() {
			Expect(f).To(ExecuteSuccessfully())
			// Node should have GPU labels since hook was not skipped
			workerGpu0 := f.KubernetesGlobalResource("Node", "worker-gpu-0")
			Expect(workerGpu0.Field("metadata.labels.node\\.deckhouse\\.io/gpu").Exists()).To(BeTrue())
			Expect(workerGpu0.Field("metadata.labels.node\\.deckhouse\\.io/device-gpu\\.config").String()).To(Equal("time-slicing"))
		})
	})

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Set GPU label on node", func() {
		BeforeEach(func() {
			f.KubeStateSet(ngsYaml + gpuNode0Yaml + gpuNode1Yaml + workerNodeYaml + gpuNode2Yaml + gpuNodeCustomYaml)
			f.ValuesSet("nodeManager.internal.customMIGNames.worker-gpu-custom", "custom-worker-gpu-custom-12345678")
			f.BindingContexts.Set(f.GenerateAfterHelmContext())

			f.RunGoHook()
		})

		It("Must be executed successfully; new labels must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectedWorkerGpu0Labels := `
        {
            "labels": {
              "node.deckhouse.io/gpu": "",
              "node.deckhouse.io/group": "worker-gpu",
              "node.deckhouse.io/device-gpu.config": "time-slicing"
            },
            "name": "worker-gpu-0"
        }
      `
			expectedWorkerGpu1Labels := `
        {
            "labels": {
              "node.deckhouse.io/gpu": "",
              "node.deckhouse.io/group": "worker-gpu",
              "node.deckhouse.io/device-gpu.config": "time-slicing",
              "nvidia.com/mig.config": "all-disabled"
            },
            "name": "worker-gpu-1"
        }
      `
			expectedWorkerGpu2Labels := `
        {
            "labels": {
              "node.deckhouse.io/gpu": "",
              "node.deckhouse.io/group": "worker-gpu-1",
              "node.deckhouse.io/device-gpu.config": "mig",
              "nvidia.com/mig.config": "all-1g.5gb" 
            },
            "name": "worker-gpu-2"
        }
      `
			expectedWorkerLabels := `
        {
            "labels": {
              "node.deckhouse.io/group": "worker"
            },
            "name": "worker-0"
        }
      `
			expectedWorkerGpuCustomLabels := `
        {
            "labels": {
              "node.deckhouse.io/gpu": "",
              "node.deckhouse.io/group": "worker-gpu-custom",
              "node.deckhouse.io/device-gpu.config": "mig",
              "nvidia.com/mig.config": "custom-worker-gpu-custom-12345678"
            },
            "name": "worker-gpu-custom"
        }
      `
			workerGpu0 := f.KubernetesGlobalResource("Node", "worker-gpu-0")
			workerGpu1 := f.KubernetesGlobalResource("Node", "worker-gpu-1")
			workerGpu2 := f.KubernetesGlobalResource("Node", "worker-gpu-2")
			workerGpuCustom := f.KubernetesGlobalResource("Node", "worker-gpu-custom")
			worker := f.KubernetesGlobalResource("Node", "worker-0")

			Expect(workerGpu0.Field("metadata")).To(MatchJSON(expectedWorkerGpu0Labels))
			Expect(workerGpu1.Field("metadata")).To(MatchJSON(expectedWorkerGpu1Labels))
			Expect(workerGpu2.Field("metadata")).To(MatchJSON(expectedWorkerGpu2Labels))
			Expect(workerGpuCustom.Field("metadata")).To(MatchJSON(expectedWorkerGpuCustomLabels))
			Expect(worker.Field("metadata")).To(MatchJSON(expectedWorkerLabels))

		})
	})
})

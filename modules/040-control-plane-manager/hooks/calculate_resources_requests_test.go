// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

type masterNode struct {
	cpu    string
	memory string
	capCPU string // optional; falls back to cpu (kubelet not yet settled)
	capMem string // optional; falls back to memory
}

func generateMasterNodesConfig(nodes []masterNode) string {
	var stateMasterNode = `
---
apiVersion: v1
kind: Node
metadata:
  name: sandbox-%d
  labels:
    node-role.kubernetes.io/control-plane: ""
status:
  capacity:
    cpu: "%s"
    memory: "%s"
  allocatable:
    cpu: "%s"
    memory: "%s"
`
	var state string
	for i, n := range nodes {
		capCPU := n.capCPU
		if capCPU == "" {
			capCPU = n.cpu
		}
		capMem := n.capMem
		if capMem == "" {
			capMem = n.memory
		}
		state += fmt.Sprintf(stateMasterNode, i, capCPU, capMem, n.cpu, n.memory)
	}
	return state
}

var _ = Describe("Module hooks :: control-plane-manager :: calculate_resources_requests", func() {

	f := HookExecutionConfigInit(`{"controlPlaneManager": {"internal": {"resourcesRequests": {}}}}`, `{}`)

	Context("Cluster without master nodes (unmanaged)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook should run, control-plane resource values should be equal 0, because we do not admin control plane resources in unmanaged clusters (GKE for example)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(int64(0)))
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.memoryControlPlane").Int()).To(Equal(int64(0)))
		})
	})

	Context("Cluster with one master node (auto mode, Capacity == Allocatable)", func() {
		// Auto mode: the hook leaves the pool at zero and the static-pod templates
		// size each component by cluster node count. The hook only validates that
		// the master is big enough to host a control plane.
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesConfig([]masterNode{{cpu: "4", memory: "8Gi"}})))
			f.RunHook()
		})

		It("Hook should run and keep the control-plane pool at zero (per-component sizing in templates)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(int64(0)))
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.memoryControlPlane").Int()).To(Equal(int64(0)))
		})
	})

	Context("Cluster with one master node, kubelet settled (auto mode)", func() {
		// Capacity=4 CPU/8 GiB, kubelet has reserved 200m CPU / 1 GiB memory.
		// The master is large enough, so the hook succeeds and stays in auto mode.
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesConfig([]masterNode{
				{cpu: "3800m", memory: "7Gi", capCPU: "4", capMem: "8Gi"},
			})))
			f.RunHook()
		})

		It("Hook should run and keep the control-plane pool at zero", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(int64(0)))
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.memoryControlPlane").Int()).To(Equal(int64(0)))
		})
	})

	Context("Cluster with master node and both CPM and global resourcesRequests set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesConfig([]masterNode{{cpu: "4", memory: "8Gi"}})))
			f.ValuesSet("global.modules.resourcesRequests.controlPlane.cpu", "2000m")
			f.ValuesSet("global.modules.resourcesRequests.controlPlane.memory", "2Gi")
			f.ValuesSet("controlPlaneManager.resourcesRequests.cpu", "1500m")
			f.ValuesSet("controlPlaneManager.resourcesRequests.memory", "1Gi")
			f.RunHook()
		})

		It("Hook should prefer control-plane-manager resourcesRequests", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(int64(1500)))
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.memoryControlPlane").Int()).To(Equal(int64(1024 * 1024 * 1024)))
		})

		It("Hook should not raise the obsolete global resources metric when CPM config takes priority over global fallback", func() {
			Expect(f).To(ExecuteSuccessfully())

			metrics := f.MetricsCollector.CollectedMetrics()
			found := false
			for _, m := range metrics {
				if m.Name == obsoleteGlobalResourcesRequestsMetricName {
					found = true
					break
				}
			}
			Expect(found).To(BeFalse())
		})
	})

	Context("Cluster with master node, with only global resourcesRequests set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesConfig([]masterNode{{cpu: "4", memory: "8Gi"}})))
			f.ValuesSet("global.modules.resourcesRequests.controlPlane.cpu", "2000m")
			f.ValuesSet("global.modules.resourcesRequests.controlPlane.memory", "2Gi")
			f.RunHook()
		})

		It("Hook should run, control-plane resource values should be equal global modules resourcesRequests for control-plane as a fallback path", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(int64(2000)))
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.memoryControlPlane").Int()).To(Equal(int64(2 * 1024 * 1024 * 1024)))
		})

		It("Hook should raise the obsolete global resources metric when global fallback is used", func() {
			Expect(f).To(ExecuteSuccessfully())

			metrics := f.MetricsCollector.CollectedMetrics()
			found := false
			value := 0.0
			for _, m := range metrics {
				if m.Name == obsoleteGlobalResourcesRequestsMetricName {
					found = true
					value = *m.Value
					break
				}
			}
			Expect(found).To(BeTrue())
			Expect(value).To(Equal(1.0))
		})
	})

	Context("Cluster with master node and only CPM resourcesRequests set (post-migration state)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesConfig([]masterNode{{cpu: "4", memory: "8Gi"}})))
			f.ValuesSet("controlPlaneManager.resourcesRequests.cpu", "1500m")
			f.ValuesSet("controlPlaneManager.resourcesRequests.memory", "1Gi")
			f.RunHook()
		})
		It("Hook uses CPM values and does not emit the obsolete metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(int64(1500)))
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.memoryControlPlane").Int()).To(Equal(int64(1024 * 1024 * 1024)))
			found := false
			for _, m := range f.MetricsCollector.CollectedMetrics() {
				if m.Name == obsoleteGlobalResourcesRequestsMetricName {
					found = true
					break
				}
			}
			Expect(found).To(BeFalse())
		})
	})

	Context("Cluster with one master node and only CPU override set (independent override)", func() {
		// Only CPU is overridden: the CPU pool is set and the templates split it
		// by the historical share, while memory stays at zero (auto/per-component).
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesConfig([]masterNode{{cpu: "4", memory: "8Gi"}})))
			f.ValuesSet("controlPlaneManager.resourcesRequests.cpu", "1500m")
			f.RunHook()
		})

		It("Hook should set only the CPU pool and keep memory at zero", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(int64(1500)))
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.memoryControlPlane").Int()).To(Equal(int64(0)))
		})
	})

	Context("Cluster with two master nodes, with different resources, auto mode", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesConfig([]masterNode{{cpu: "4", memory: "8Gi"}, {cpu: "2000m", memory: "4Gi"}})))
			f.RunHook()
		})

		It("Hook should run and keep the control-plane pool at zero (per-component sizing in templates)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(int64(0)))
			Expect(f.ValuesGet("controlPlaneManager.internal.resourcesRequests.memoryControlPlane").Int()).To(Equal(int64(0)))
		})

	})

	Context("Cluster with two master nodes, but with very small resources", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesConfig([]masterNode{{cpu: "300m", memory: "500Mi"}, {cpu: "2000m", memory: "4Gi"}})))
			f.RunHook()
		})

		It("Hook should fail", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})

	})

	Context("absDiff", func() {
		It("Correct calc", func() {
			Expect(absDiff(2, 1)).To(Equal(int64(1)))
			Expect(absDiff(1, 2)).To(Equal(int64(1)))
			Expect(absDiff(1, 1)).To(Equal(int64(0)))
		})
	})
})

func absDiff(a, b int64) int64 {
	d := a - b
	if d > 0 {
		return d
	}
	return b - a
}

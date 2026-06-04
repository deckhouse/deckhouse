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
	cpu      string
	memory   string
	capCPU   string // optional; falls back to cpu (kubelet not yet settled)
	capMem   string // optional; falls back to memory
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

var _ = Describe("Global hooks :: calculate_resources_requests", func() {

	f := HookExecutionConfigInit(`{"global": {"internal": {"modules": {"resourcesRequests": {}}}}}`, `{}`)

	Context("Cluster without master nodes (unmanaged)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook should run, control-plane resource values should be equal 0, because we do not admin control plane resources in unmanaged clusters (GKE for example)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(int64(0)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryControlPlane").Int()).To(Equal(int64(0)))
		})
	})

	Context("Cluster with one master node (Capacity == Allocatable, kubelet not yet settled)", func() {
		// Reservation floor is applied because Capacity == Allocatable signals
		// kubelet has not yet subtracted its own reserved bucket.
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesConfig([]masterNode{{cpu: "4", memory: "8Gi"}})))
			f.RunHook()
		})

		It(fmt.Sprintf("Hook should run, CP values = %d%% of (Capacity - kubelet reservation floor - configEveryNode*)", controlPlanePercent), func() {
			Expect(f).To(ExecuteSuccessfully())
			expectCPU := int64((4000-kubeletResourceReservationCPUFloor-configEveryNodeMilliCPU)*controlPlanePercent) / 100
			expectMem := int64((8*1024*1024*1024-kubeletResourceReservationMemoryFloor-configEveryNodeMemory)*controlPlanePercent) / 100
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(expectCPU))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryControlPlane").Int()).To(Equal(expectMem))
		})
	})

	Context("Cluster with one master node, kubelet settled (Capacity > Allocatable, reservation > floor)", func() {
		// Capacity=4 CPU/8 GiB, kubelet has reserved 200m CPU / 1 GiB memory.
		// Both exceed the floor, so the hook uses the reported reservation as-is.
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesConfig([]masterNode{
				{cpu: "3800m", memory: "7Gi", capCPU: "4", capMem: "8Gi"},
			})))
			f.RunHook()
		})

		It("Hook should run, CP values reflect the actual kubelet reservation (200m, 1 GiB)", func() {
			Expect(f).To(ExecuteSuccessfully())
			// effectiveCPU = Capacity(4000) - max(actualRes=200, floor=100) = 3800
			// effectiveMem = 8 GiB - max(1 GiB, 900 MiB) = 7 GiB
			expectCPU := int64((3800-configEveryNodeMilliCPU)*controlPlanePercent) / 100
			expectMem := int64((7*1024*1024*1024-configEveryNodeMemory)*controlPlanePercent) / 100
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(expectCPU))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryControlPlane").Int()).To(Equal(expectMem))
		})
	})

	Context("Cluster with one master node, kubelet settled but reservation under floor", func() {
		// Capacity=4 CPU/8 GiB, kubelet reserved only 50m / 100 MiB. Floor wins
		// — the hook treats the reservation as 100m / 900 MiB so the value is
		// identical to the pre-settled state and a second hook run on a real
		// cluster bootstrap doesn't re-render the CP manifests.
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesConfig([]masterNode{
				{cpu: "3950m", memory: "8090Mi", capCPU: "4", capMem: "8Gi"},
			})))
			f.RunHook()
		})

		It("Hook should run, CP values reflect the reservation FLOOR not the smaller actual", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectCPU := int64((4000-kubeletResourceReservationCPUFloor-configEveryNodeMilliCPU)*controlPlanePercent) / 100
			expectMem := int64((8*1024*1024*1024-kubeletResourceReservationMemoryFloor-configEveryNodeMemory)*controlPlanePercent) / 100
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(expectCPU))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryControlPlane").Int()).To(Equal(expectMem))
		})
	})

	Context("Cluster with master node, with set global modules resourcesRequests for control-plane", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesConfig([]masterNode{{cpu: "4", memory: "8Gi"}})))
			f.ValuesSet("global.modules.resourcesRequests.controlPlane.cpu", "2000m")
			f.ValuesSet("global.modules.resourcesRequests.controlPlane.memory", "2Gi")
			f.RunHook()
		})

		It("Hook should run, control-plane resource values should be equal global modules resourcesRequests for control-plane", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(int64(2000)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryControlPlane").Int()).To(Equal(int64(2 * 1024 * 1024 * 1024)))
		})
	})

	Context("Cluster with two master nodes, with different resources, but without set global modules resourcesRequests for control-plane", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesConfig([]masterNode{{cpu: "4", memory: "8Gi"}, {cpu: "2000m", memory: "4Gi"}})))
			f.RunHook()
		})

		It(fmt.Sprintf("Hook should run, control-plane resource values should be equal %d%% of (smaller master - resources for components working on every node)", controlPlanePercent), func() {
			Expect(f).To(ExecuteSuccessfully())
			// Smaller master: Capacity=Allocatable=2000m/4 GiB → floor applies.
			expectCPU := int64((2000-kubeletResourceReservationCPUFloor-configEveryNodeMilliCPU)*controlPlanePercent) / 100
			expectMem := int64((4*1024*1024*1024-kubeletResourceReservationMemoryFloor-configEveryNodeMemory)*controlPlanePercent) / 100
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(expectCPU))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryControlPlane").Int()).To(Equal(expectMem))
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

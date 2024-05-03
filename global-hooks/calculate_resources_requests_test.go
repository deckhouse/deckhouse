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
  allocatable:
    cpu: "%s"
    memory: "%s"
`
	var state string
	for i, n := range nodes {
		state += fmt.Sprintf(stateMasterNode, i, n.cpu, n.memory)
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

	Context("Cluster with one master node, but without set global modules resourcesRequests for control-plane", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesConfig([]masterNode{{cpu: "4", memory: "8Gi"}})))
			f.RunHook()
		})

		It(fmt.Sprintf("Hook should run, control-plane resource values should be equal %d%% of (master-node resources - resources for components working on every node)", controlPlanePercent), func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(int64((4000 - configEveryNodeMilliCPU) * controlPlanePercent / 100)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryControlPlane").Int()).To(Equal(int64((8192*1024*1024 - configEveryNodeMemory) * controlPlanePercent / 100)))
		})
	})

	Context("Cluster with one master node, but without set global modules resourcesRequests for control-plane and little oscillations from maximum", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesConfig([]masterNode{{cpu: "3930m", memory: "7717366089760m"}})))
			f.RunHook()
		})

		It(fmt.Sprintf("Hook should run, control-plane resource values should be equal %d%% of (master-node resources - resources for components working on every node)", controlPlanePercent), func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(int64((4000 - configEveryNodeMilliCPU) * controlPlanePercent / 100)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryControlPlane").Int()).To(Equal(int64((8192*1024*1024 - configEveryNodeMemory) * controlPlanePercent / 100)))
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

		It(fmt.Sprintf("Hook should run, control-plane resource values should be equal %d%% of (master-node with less resources - resources for components working on every node)", controlPlanePercent), func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(int64((2000 - configEveryNodeMilliCPU) * controlPlanePercent / 100)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryControlPlane").Int()).To(Equal(int64((4096*1024*1024 - configEveryNodeMemory) * controlPlanePercent / 100)))
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

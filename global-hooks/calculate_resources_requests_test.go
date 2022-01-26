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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: calculate_resources_requests", func() {
	const (
		stateMasterNode = `
---
apiVersion: v1
kind: Node
metadata:
  name: sandbox-21-master
  labels:
    node-role.kubernetes.io/master: ""
status:
  allocatable:
    cpu: "4"
    memory: "8589934592"
`
		stateMasterNode2 = `
---
apiVersion: v1
kind: Node
metadata:
  name: sandbox-22-master
  labels:
    node-role.kubernetes.io/master: ""
status:
  allocatable:
    cpu: "2048m"
    memory: "4Gi"
`
	)

	f := HookExecutionConfigInit(`{"global": {"internal": {"modules": {"resourcesRequests": {}}}}}`, `{}`)
	Context("Cluster without master nodes (managed)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.ValuesSet("global.modules.resourcesRequests.everyNode.cpu", "300m")
			f.ValuesSet("global.modules.resourcesRequests.everyNode.memory", "512Mi")
			f.RunHook()
		})

		It("Hook should not run, because nodes resources dont exist", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(int64(0)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryControlPlane").Int()).To(Equal(int64(0)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuMaster").Int()).To(Equal(int64(700)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryMaster").Int()).To(Equal(int64(512 * 1024 * 1024)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuEveryNode").Int()).To(Equal(int64(300)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryEveryNode").Int()).To(Equal(int64(512 * 1024 * 1024)))
		})

	})

	Context("Cluster with master node, but without set global modules resourcesRequests", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMasterNode))
			f.RunHook()
		})

		It("Hook should not run, because needed global variables dont exist", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})
	})

	Context("Incorrectly set global.modules.resourcesRequests variables (everyNode.cpu + masterNode.cpu > allocatable master cpu)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMasterNode))
			f.ValuesSet("global.modules.resourcesRequests.masterNode.cpu", "5")
			f.ValuesSet("global.modules.resourcesRequests.masterNode.memory", "4Gi")
			f.ValuesSet("global.modules.resourcesRequests.everyNode.cpu", "4")
			f.ValuesSet("global.modules.resourcesRequests.everyNode.memory", "500Mi")
			f.RunHook()
		})

		It("Hook should not run, and print error message", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})

	})

	Context("Incorrectly set global.modules.resourcesRequests variables (everyNode.memory + masterNode.memory > allocatable master memory)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMasterNode))
			f.ValuesSet("global.modules.resourcesRequests.masterNode.cpu", "2")
			f.ValuesSet("global.modules.resourcesRequests.masterNode.memory", "4Gi")
			f.ValuesSet("global.modules.resourcesRequests.everyNode.cpu", "300m")
			f.ValuesSet("global.modules.resourcesRequests.everyNode.memory", "5Gi")
			f.RunHook()
		})

		It("Hook should not run, and print error message", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})

	})

	Context("Correctly set, global.modules.resourcesRequests.masterNode not set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMasterNode))
			f.ValuesSet("global.modules.resourcesRequests.everyNode.cpu", "300m")
			f.ValuesSet("global.modules.resourcesRequests.everyNode.memory", "512Mi")
			f.RunHook()
		})

		It("Hook should run and set global internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(int64(1850)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryControlPlane").Int()).To(Equal(int64(3840 * 1024 * 1024)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuMaster").Int()).To(Equal(int64(1850)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryMaster").Int()).To(Equal(int64(3840 * 1024 * 1024)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuEveryNode").Int()).To(Equal(int64(300)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryEveryNode").Int()).To(Equal(int64(512 * 1024 * 1024)))
		})

	})

	Context("Correctly set, global.modules.resourcesRequests.masterNode set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMasterNode))
			f.ValuesSet("global.modules.resourcesRequests.everyNode.cpu", "500m")
			f.ValuesSet("global.modules.resourcesRequests.everyNode.memory", "1Gi")
			f.ValuesSet("global.modules.resourcesRequests.masterNode.cpu", "1")
			f.ValuesSet("global.modules.resourcesRequests.masterNode.memory", "1Gi")
			f.RunHook()
		})

		It("Hook should run and set global internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(int64(500)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryControlPlane").Int()).To(Equal(int64(512 * 1024 * 1024)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuMaster").Int()).To(Equal(int64(500)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryMaster").Int()).To(Equal(int64(512 * 1024 * 1024)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuEveryNode").Int()).To(Equal(int64(500)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryEveryNode").Int()).To(Equal(int64(1 * 1024 * 1024 * 1024)))
		})

	})

	Context("Correctly set, two master nodes, global.modules.resourcesRequests.masterNode not set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMasterNode + stateMasterNode2))
			f.ValuesSet("global.modules.resourcesRequests.everyNode.cpu", "300m")
			f.ValuesSet("global.modules.resourcesRequests.everyNode.memory", "512Mi")
			f.RunHook()
		})

		It("Hook should run and set global internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(int64(874)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryControlPlane").Int()).To(Equal(int64(1792 * 1024 * 1024)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuMaster").Int()).To(Equal(int64(874)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryMaster").Int()).To(Equal(int64(1792 * 1024 * 1024)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuEveryNode").Int()).To(Equal(int64(300)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryEveryNode").Int()).To(Equal(int64(512 * 1024 * 1024)))
		})

	})

	Context("Correctly set, two master nodes, global.modules.resourcesRequests.masterNode set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMasterNode + stateMasterNode2))
			f.ValuesSet("global.modules.resourcesRequests.everyNode.cpu", "500m")
			f.ValuesSet("global.modules.resourcesRequests.everyNode.memory", "1Gi")
			f.ValuesSet("global.modules.resourcesRequests.masterNode.cpu", "1")
			f.ValuesSet("global.modules.resourcesRequests.masterNode.memory", "1Gi")
			f.RunHook()
		})

		It("Hook should run and set global internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuControlPlane").Int()).To(Equal(int64(500)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryControlPlane").Int()).To(Equal(int64(512 * 1024 * 1024)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuMaster").Int()).To(Equal(int64(500)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryMaster").Int()).To(Equal(int64(512 * 1024 * 1024)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.milliCpuEveryNode").Int()).To(Equal(int64(500)))
			Expect(f.ValuesGet("global.internal.modules.resourcesRequests.memoryEveryNode").Int()).To(Equal(int64(1 * 1024 * 1024 * 1024)))
		})

	})

})

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: node_resources", func() {
	const stateMasterAndReadyNode = `
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
    memory: "8264986585"
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: sandbox-21-worker-1
  labels:
    node-role.kubernetes.io/worker: ""
status:
  allocatable:
    cpu: "2"
    memory: "4132493292"
  conditions:
  - status: "True"
    type: Ready
`
	const stateMasterAndReadyNode2 = `
---
apiVersion: v1
kind: Node
metadata:
  name: sandbox-21-master
  labels:
    node-role.kubernetes.io/master: ""
status:
  allocatable:
    cpu: "8"
    memory: "8264986585"
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: sandbox-21-worker-1
  labels:
    node-role.kubernetes.io/worker: ""
status:
  allocatable:
    cpu: "4"
    memory: "8264986585"
  conditions:
  - status: "True"
    type: Ready
`
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook should not run, because nodes resources dont exist", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})

	})

	Context("Cluster with master and worker nodes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMasterAndReadyNode))
			f.RunHook()
		})

		It("Hook should run and set global values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.allocatableMilliCpuControlPlane").String()).To(Equal("819"))
			Expect(f.ValuesGet("global.allocatableMemoryControlPlane").String()).To(Equal("858993459"))
			Expect(f.ValuesGet("global.allocatableMilliCpuMaster").String()).To(Equal("819"))
			Expect(f.ValuesGet("global.allocatableMemoryMaster").String()).To(Equal("858993459"))
			Expect(f.ValuesGet("global.allocatableMilliCpuAnyNode").String()).To(Equal("400"))
			Expect(f.ValuesGet("global.allocatableMemoryAnyNode").String()).To(Equal("429496729"))
		})
	})

	Context("Cluster with master and worker nodes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMasterAndReadyNode2))
			f.ConfigValuesSet("global.controlPlaneRequestsCpu", "1")
			f.ConfigValuesSet("global.controlPlaneRequestsMemory", "1024Mi")
			f.ConfigValuesSet("global.anyNodeRequestsCpu", "300m")
			f.ConfigValuesSet("global.anyNodeRequestsMemory", "128Mi")
			f.RunHook()
		})

		It("Hook should run and set global values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.allocatableMilliCpuControlPlane").String()).To(Equal("500"))
			Expect(f.ValuesGet("global.allocatableMemoryControlPlane").String()).To(Equal("536870912"))
			Expect(f.ValuesGet("global.allocatableMilliCpuMaster").String()).To(Equal("500"))
			Expect(f.ValuesGet("global.allocatableMemoryMaster").String()).To(Equal("536870912"))
			Expect(f.ValuesGet("global.allocatableMilliCpuAnyNode").String()).To(Equal("300"))
			Expect(f.ValuesGet("global.allocatableMemoryAnyNode").String()).To(Equal("134217728"))
		})

	})

})

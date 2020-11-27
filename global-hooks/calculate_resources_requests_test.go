package hooks

import (
	. "github.com/deckhouse/deckhouse/testing/hooks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
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
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.ValuesSet("global.modules.resourcesRequests.everyNode.cpu", "300m")
			f.ValuesSet("global.modules.resourcesRequests.everyNode.memory", "512Mi")
			f.RunHook()
		})

		It("Hook should not run, because nodes resources dont exist", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: input value null must be in Quantity format !`))
		})

	})

	Context("Cluster with master node, but without set global modules resourcesRequests", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMasterNode))
			f.RunHook()
		})

		It("Hook should not run, because needed global variables dont exist", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
			Expect(f.Session.Err).Should(gbytes.Say(`Error: Value global.modules.resourcesRequests.everyNode.cpu required, but doesn't exist`))
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
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: everyNode CPU \+ masterNode CPU must be less than discovered minimal master node CPU`))
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
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: everyNode memory \+ masterNode memory must be less than discovered minimal master node memory`))
		})

	})

	Context("Correctly set, global.modules.resourcesRequests.masterNode not set)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMasterNode))
			f.ValuesSet("global.modules.resourcesRequests.everyNode.cpu", "300m")
			f.ValuesSet("global.modules.resourcesRequests.everyNode.memory", "512Mi")
			f.RunHook()
		})

		It("Hook should run and set global internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.modules.resourcesRequests.internal.milliCpuControlPlane").Int()).To(Equal(int64(1850)))
			Expect(f.ValuesGet("global.modules.resourcesRequests.internal.memoryControlPlane").Int()).To(Equal(int64(3840 * 1024 * 1024)))
			Expect(f.ValuesGet("global.modules.resourcesRequests.internal.milliCpuMaster").Int()).To(Equal(int64(1850)))
			Expect(f.ValuesGet("global.modules.resourcesRequests.internal.memoryMaster").Int()).To(Equal(int64(3840 * 1024 * 1024)))
			Expect(f.ValuesGet("global.modules.resourcesRequests.internal.milliCpuEveryNode").Int()).To(Equal(int64(300)))
			Expect(f.ValuesGet("global.modules.resourcesRequests.internal.memoryEveryNode").Int()).To(Equal(int64(512 * 1024 * 1024)))
		})

	})

	Context("Correctly set, global.modules.resourcesRequests.masterNode set)", func() {
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
			Expect(f.ValuesGet("global.modules.resourcesRequests.internal.milliCpuControlPlane").Int()).To(Equal(int64(500)))
			Expect(f.ValuesGet("global.modules.resourcesRequests.internal.memoryControlPlane").Int()).To(Equal(int64(512 * 1024 * 1024)))
			Expect(f.ValuesGet("global.modules.resourcesRequests.internal.milliCpuMaster").Int()).To(Equal(int64(500)))
			Expect(f.ValuesGet("global.modules.resourcesRequests.internal.memoryMaster").Int()).To(Equal(int64(512 * 1024 * 1024)))
			Expect(f.ValuesGet("global.modules.resourcesRequests.internal.milliCpuEveryNode").Int()).To(Equal(int64(500)))
			Expect(f.ValuesGet("global.modules.resourcesRequests.internal.memoryEveryNode").Int()).To(Equal(int64(1 * 1024 * 1024 * 1024)))
		})

	})

	Context("Correctly set with Ki in memory, global.modules.resourcesRequests.masterNode set)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
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
    memory: "8589934592Ki"
`))
			f.ValuesSet("global.modules.resourcesRequests.everyNode.cpu", "500m")
			f.ValuesSet("global.modules.resourcesRequests.everyNode.memory", "1Gi")
			f.ValuesSet("global.modules.resourcesRequests.masterNode.cpu", "1")
			f.ValuesSet("global.modules.resourcesRequests.masterNode.memory", "1Gi")
			f.RunHook()
		})

		It("Hook should run and set global internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.modules.resourcesRequests.internal.milliCpuControlPlane").Int()).To(Equal(int64(500)))
			Expect(f.ValuesGet("global.modules.resourcesRequests.internal.memoryControlPlane").Int()).To(Equal(int64(536870912)))
			Expect(f.ValuesGet("global.modules.resourcesRequests.internal.milliCpuMaster").Int()).To(Equal(int64(500)))
			Expect(f.ValuesGet("global.modules.resourcesRequests.internal.memoryMaster").Int()).To(Equal(int64(536870912)))
			Expect(f.ValuesGet("global.modules.resourcesRequests.internal.milliCpuEveryNode").Int()).To(Equal(int64(500)))
			Expect(f.ValuesGet("global.modules.resourcesRequests.internal.memoryEveryNode").Int()).To(Equal(int64(1 * 1024 * 1024 * 1024)))
		})

	})

})

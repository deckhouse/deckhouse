/*

User-stories:
1. Hook must discover number of addresses in Endpoint default/kubernetes and save to global.discovery.clusterMasterCount,
2. If number of addresses in Endpoint default/kubernetes is more than one — hook must set global.discovery.clusterControlPlaneIsHighlyAvailable to true, else — to false.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery/cluster_ha ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		stateFirstMasterNode = `
apiVersion: v1
kind: Node
metadata:
  name: master-0
  labels:
    node-role.kubernetes.io/master: ""`

		stateSecondMasterNode = `
---
apiVersion: v1
kind: Node
metadata:
  name: master-1
  labels:
    node-role.kubernetes.io/master: ""`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("`global.discovery.clusterControlPlaneIsHighlyAvailable` must be false; `global.discovery.clusterMasterCount` must be 0", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("0"))
			Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeFalse())

		})
	})

	Context("One master node in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateFirstMasterNode))
			f.RunHook()
		})

		It("`global.discovery.clusterControlPlaneIsHighlyAvailable` must be false; `global.discovery.clusterMasterCount` must be 1", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("1"))
			Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeFalse())

		})

		Context("Two master nodes in cluster", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateFirstMasterNode + stateSecondMasterNode))
				f.RunHook()
			})

			It("`global.discovery.clusterControlPlaneIsHighlyAvailable` must be true; `global.discovery.clusterMasterCount` must be 2", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("2"))
				Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeTrue())
			})
		})
	})
})

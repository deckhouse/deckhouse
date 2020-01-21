/*

User-stories:
1. There are nodes in cluster with annotation like 'node-role.flant.com/xxx', hook must group, count them and store to `global.discovery.d8SpecificNodeCountByRole`.

*/

package hooks

import (
	"sort"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery/cluster_count_node_roles ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		stateOnlyMaster = `
apiVersion: v1
kind: Node
metadata:
  name: master
`
		stateMasterAndSpecialNodes = `
apiVersion: v1
kind: Node
metadata:
  name: master
---
apiVersion: v1
kind: Node
metadata:
  name: front-1
  labels:
    node-role.flant.com/frontend: ""
    node-role.deckhouse.io/frontendbyd8: ""
    node-role.kubernetes.io/frontendbykubernetes: ""
---
apiVersion: v1
kind: Node
metadata:
  name: front-2
  labels:
    node-role.kubernetes.io/frontend: ""
---
apiVersion: v1
kind: Node
metadata:
  name: system
  labels:
    node-role.deckhouse.io/system: ""
    node-role.kubernetes.io/systembykubernetes: ""
`
		stateMasterAndSpecialNodesModified = `
apiVersion: v1
kind: Node
metadata:
  name: master
---
apiVersion: v1
kind: Node
metadata:
  name: front-1
  labels:
    node-role.flant.com/frontend: ""
---
apiVersion: v1
kind: Node
metadata:
  name: front-2
  labels:
    node-role.deckhouse.io/system: ""
    node-role.kubernetes.io/systembykubernetes: ""
---
apiVersion: v1
kind: Node
metadata:
  name: system
  labels:
    node-role.deckhouse.io/system: ""
    node-role.kubernetes.io/systembykubernetes: ""
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("There is only master in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateOnlyMaster))
			f.RunHook()
		})

		It("filterResult of master must be null; `global.discovery.d8SpecificNodeCountByRole` must be empty map", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(len(f.BindingContexts.Get("0.objects").Array())).To(Equal(1))
			Expect(f.BindingContexts.Get("0.objects.0.filterResult").Value()).To(BeNil())
			Expect(f.ValuesGet("global.discovery.d8SpecificNodeCountByRole").Map()).To(BeEmpty())
		})

		Context("Special nodes added", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateMasterAndSpecialNodes))
				f.RunHook()
			})

			It("filterResults must contain single '', two 'frontend' and one 'system' ; `global.discovery.d8SpecificNodeCountByRole` must contain map of nodes", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Parse().Array()).ShouldNot(BeEmpty())
				Expect(len(f.BindingContexts.Get("2.snapshots.node_roles").Array())).To(Equal(4))

				frSlice := []string{}
				frSlice = append(frSlice, f.BindingContexts.Get("2.snapshots.node_roles.0.filterResult").String())
				frSlice = append(frSlice, f.BindingContexts.Get("2.snapshots.node_roles.1.filterResult").String())
				frSlice = append(frSlice, f.BindingContexts.Get("2.snapshots.node_roles.2.filterResult").String())
				frSlice = append(frSlice, f.BindingContexts.Get("2.snapshots.node_roles.3.filterResult").String())
				sort.Strings(frSlice)

				Expect(frSlice).To(Equal([]string{"", "frontend", "frontend", "system"}))
				Expect(f.ValuesGet("global.discovery.d8SpecificNodeCountByRole").String()).To(MatchJSON(`{"system": 1, "frontend": 2}`))
			})

			Context("Special nodes modified", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(stateMasterAndSpecialNodesModified))
					f.RunHook()
				})

				It("filterResults must contain single '', one 'frontend' and two 'system' ; `global.discovery.d8SpecificNodeCountByRole` must contain map of nodes", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.BindingContexts.Parse().Array()).ShouldNot(BeEmpty())
					Expect(len(f.BindingContexts.Get("0.snapshots.node_roles").Array())).To(Equal(4))

					frSlice := []string{}
					frSlice = append(frSlice, f.BindingContexts.Get("0.snapshots.node_roles.0.filterResult").String())
					frSlice = append(frSlice, f.BindingContexts.Get("0.snapshots.node_roles.1.filterResult").String())
					frSlice = append(frSlice, f.BindingContexts.Get("0.snapshots.node_roles.2.filterResult").String())
					frSlice = append(frSlice, f.BindingContexts.Get("0.snapshots.node_roles.3.filterResult").String())
					sort.Strings(frSlice)

					Expect(frSlice).To(Equal([]string{"", "frontend", "system", "system"}))
					Expect(f.ValuesGet("global.discovery.d8SpecificNodeCountByRole").String()).To(MatchJSON(`{"system": 2, "frontend": 1}`))
				})

			})

		})

	})

	Context("There are special nodes in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMasterAndSpecialNodes))
			f.RunHook()
		})

		It("filterResults must contain single '', two 'frontend' and one 'system' ; `global.discovery.d8SpecificNodeCountByRole` must contain map of nodes", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Parse().Array()).ShouldNot(BeEmpty())
			Expect(len(f.BindingContexts.Get("0.snapshots.node_roles").Array())).To(Equal(4))

			frSlice := []string{}
			frSlice = append(frSlice, f.BindingContexts.Get("0.snapshots.node_roles.0.filterResult").String())
			frSlice = append(frSlice, f.BindingContexts.Get("0.snapshots.node_roles.1.filterResult").String())
			frSlice = append(frSlice, f.BindingContexts.Get("0.snapshots.node_roles.2.filterResult").String())
			frSlice = append(frSlice, f.BindingContexts.Get("0.snapshots.node_roles.3.filterResult").String())
			sort.Strings(frSlice)

			Expect(frSlice).To(Equal([]string{"", "frontend", "frontend", "system"}))
			Expect(f.ValuesGet("global.discovery.d8SpecificNodeCountByRole").String()).To(MatchJSON(`{"system": 1, "frontend": 2}`))
		})

	})

})

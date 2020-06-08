/*

User-stories:
1. There are nodes in cluster with annotation like 'node-role.flant.com/xxx', hook must group, count them and store to `global.discovery.d8SpecificNodeCountByRole`.

*/

package hooks

import (
	. "github.com/deckhouse/deckhouse/testing/hooks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
  labels:
    node-role.flant.com/frontend: ""
    node-role.kubernetes.io/master: ""
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
    node-role.flant.com/system: ""
    node-role.deckhouse.io/system: ""
    node-role.kubernetes.io/systembykubernetes: ""
`
		stateMasterAndSpecialNodesModified = `
apiVersion: v1
kind: Node
metadata:
  name: master
  labels:
    node-role.flant.com/master: ""
    node-role.kubernetes.io/master: ""
---
apiVersion: v1
kind: Node
metadata:
  name: front-1
  labels:
    node-role.flant.com/frontend: ""
    node-role.flant.com/system: ""
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
    node-role.flant.com/system: ""
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
			Expect(f.ValuesGet("global.discovery.d8SpecificNodeCountByRole").Map()).To(BeEmpty())
		})

		Context("Special nodes added", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateMasterAndSpecialNodes))
				f.RunHook()
			})

			It("filterResults must contain single '', two 'frontend' and one 'system' ; `global.discovery.d8SpecificNodeCountByRole` must contain map of nodes", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.d8SpecificNodeCountByRole").String()).To(MatchJSON(`
{
"frontend": 2,
"system": 1
}`))
			})

			Context("Special nodes modified", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(stateMasterAndSpecialNodesModified))
					f.RunHook()
				})

				It("filterResults must contain single '', one 'frontend' and two 'system' ; `global.discovery.d8SpecificNodeCountByRole` must contain map of nodes", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("global.discovery.d8SpecificNodeCountByRole").String()).To(MatchJSON(`
{
"frontend": 1,
"master": 1,
"system": 2
}
`))
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
			Expect(f.ValuesGet("global.discovery.d8SpecificNodeCountByRole").String()).To(MatchJSON(`
{
"frontend": 2,
"system": 1
}`))
		})

	})

})

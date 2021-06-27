// Copyright 2021 Flant CJSC
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

/*

User-stories:
1. There are nodes in cluster with annotation like 'node-role.deckhouse.io/xxx', hook must group, count them and store to `global.discovery.d8SpecificNodeCountByRole`.

*/

package hooks

import (
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
  labels:
    node-role.deckhouse.io/frontend: ""
    node-role.kubernetes.io/master: ""
---
apiVersion: v1
kind: Node
metadata:
  name: front-1
  labels:
    node-role.deckhouse.io/frontend: ""
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
  labels:
    node-role.deckhouse.io/master: ""
    node-role.kubernetes.io/master: ""
---
apiVersion: v1
kind: Node
metadata:
  name: front-1
  labels:
    node-role.deckhouse.io/frontend: ""
    node-role.deckhouse.io/system: ""
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

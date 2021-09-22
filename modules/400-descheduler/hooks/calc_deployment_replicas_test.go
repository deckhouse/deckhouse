/*
Copyright 2021 Flant CJSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: descheduler :: hooks :: calc_deployment_replicas ::", func() {
	f := HookExecutionConfigInit(`{"descheduler":{"internal":{}}}`, ``)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})
		It("nodeCount must be 0", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("descheduler.internal.replicas").String()).To(Equal("0"))
		})
	})

	Context("Cluster with one node", func() {
		BeforeEach(func() {
			nodes := `
---
apiVersion: v1
kind: Node
metadata:
  name: node-1
`
			f.BindingContexts.Set(f.KubeStateSet(nodes))
			f.RunHook()
		})
		It("nodeCount must be 0", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("descheduler.internal.replicas").String()).To(Equal("0"))
		})
	})

	Context("Cluster with two node", func() {
		BeforeEach(func() {
			nodes := `
---
apiVersion: v1
kind: Node
metadata:
  name: node-1
---
apiVersion: v1
kind: Node
metadata:
  name: node-2
`
			f.BindingContexts.Set(f.KubeStateSet(nodes))
			f.RunHook()
		})
		It("nodeCount must be 2", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("descheduler.internal.replicas").String()).To(Equal("1"))
		})
	})

})

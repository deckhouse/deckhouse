/*
Copyright 2025 Flant JSC

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

var _ = Describe("Modules :: node-manager :: hooks :: handle_spot_instance_deletion ::", func() {
	const (
		nodeDrainedSpot = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-spot-1
  labels:
    node.deckhouse.io/group: worker
    node.deckhouse.io/termination-in-progress: "true"
  annotations:
    update.node.deckhouse.io/drained: aws-node-termination-handler
spec:
  unschedulable: true
status:
  conditions:
  - status: "True"
    type: Ready
`

		nodeDrainedBashible = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-bashible-1
  labels:
    node.deckhouse.io/group: worker
  annotations:
    update.node.deckhouse.io/drained: bashible
spec:
  unschedulable: true
status:
  conditions:
  - status: "True"
    type: Ready
`

		normalNode = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-normal-1
  labels:
    node.deckhouse.io/group: worker
spec:
  unschedulable: false
status:
  conditions:
  - status: "True"
    type: Ready
`
	)

	f := HookExecutionConfigInit(`{}`, `{}`)

	Context("Node drained due to spot termination", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeDrainedSpot))
			f.RunHook()
		})

		It("Should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Node drained due to bashible update", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeDrainedBashible))
			f.RunHook()
		})

		It("Should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Normal node without drained annotation", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(normalNode))
			f.RunHook()
		})

		It("Should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Multiple nodes with different drain sources", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeDrainedSpot + nodeDrainedBashible + normalNode))
			f.RunHook()
		})

		It("Should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})
})

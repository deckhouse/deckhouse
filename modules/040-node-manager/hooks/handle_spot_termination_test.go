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

var _ = Describe("Modules :: node-manager :: hooks :: handle_spot_termination ::", func() {
	const (
		nodeWithSpotTaint = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-1
  labels:
    node.deckhouse.io/group: worker
spec:
  taints:
  - key: aws-node-termination-handler/spot-itn
    effect: NoSchedule
    value: "1234567890"
status:
  conditions:
  - status: "True"
    type: Ready
`

		nodeWithSpotTaintAndDraining = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-2
  labels:
    node.deckhouse.io/group: worker
  annotations:
    update.node.deckhouse.io/draining: spot-termination
spec:
  taints:
  - key: aws-node-termination-handler/spot-itn
    effect: NoSchedule
    value: "1234567890"
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
  name: node-3
  labels:
    node.deckhouse.io/group: worker
spec:
  taints: []
status:
  conditions:
  - status: "True"
    type: Ready
`
	)

	f := HookExecutionConfigInit(`{}`, `{}`)

	Context("Node with spot termination taint", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeWithSpotTaint))
			f.RunHook()
		})

		It("Should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Node with spot taint and draining annotation", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeWithSpotTaintAndDraining))
			f.RunHook()
		})

		It("Should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Normal node without spot taint", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(normalNode))
			f.RunHook()
		})

		It("Should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Multiple nodes with mixed states", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeWithSpotTaint + nodeWithSpotTaintAndDraining + normalNode))
			f.RunHook()
		})

		It("Should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})
})

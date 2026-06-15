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

var _ = Describe("cloud-provider-aws :: add_termination_metadata ::", func() {
	const (
		nodeWithSpotTaint = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-1
spec:
  taints:
  - key: aws-node-termination-handler/spot-itn
    value: "1234567890"
    effect: NoSchedule
`
		nodeWithSpotTaintAndMetadata = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-2
  labels:
    node.deckhouse.io/termination-in-progress: "true"
  annotations:
    update.node.deckhouse.io/draining: "aws-node-termination-handler"
spec:
  taints:
  - key: aws-node-termination-handler/spot-itn
    value: "1234567890"
    effect: NoSchedule
`
		normalNode = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-3
`
	)

	f := HookExecutionConfigInit(`{}`, `{}`)

	Context("Node with spot taint but no metadata", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeWithSpotTaint))
			f.RunHook()
		})

		It("Should add termination label and draining annotation", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.PatchCollector.Operations()).To(HaveLen(1))
		})
	})

	Context("Node with spot taint AND metadata (already processed)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeWithSpotTaintAndMetadata))
			f.RunHook()
		})

		It("Should NOT add metadata again (idempotent)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.PatchCollector.Operations()).To(HaveLen(0)) // No patches
		})
	})

	Context("Normal node without spot taint", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(normalNode))
			f.RunHook()
		})

		It("Should do nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.PatchCollector.Operations()).To(HaveLen(0))
		})
	})

	Context("Multiple nodes with different states", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeWithSpotTaint + nodeWithSpotTaintAndMetadata + normalNode))
			f.RunHook()
		})

		It("Should only process nodes that need metadata", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.PatchCollector.Operations()).To(HaveLen(1)) // Only node-1
		})
	})
})

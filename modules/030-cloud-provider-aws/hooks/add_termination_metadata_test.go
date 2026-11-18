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
		nodeWithSpotITNTaint = `
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
		nodeWithRebalanceRecommendationTaint = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-rr
spec:
  taints:
  - key: aws-node-termination-handler/rebalance-recommendation
    value: "1234567890"
    effect: NoSchedule
`
		nodeWithSpotITNTaintAndMetadata = `
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
		nodeWithRebalanceRecommendationTaintAndMetadata = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-rr-done
  labels:
    node.deckhouse.io/termination-in-progress: "true"
  annotations:
    update.node.deckhouse.io/draining: "aws-node-termination-handler"
spec:
  taints:
  - key: aws-node-termination-handler/rebalance-recommendation
    value: "1234567890"
    effect: NoSchedule
`
		nodeWithRebalanceRecommendationTaintAfterDrain = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-rr-drained
  labels:
    node.deckhouse.io/termination-in-progress: "true"
  annotations:
    update.node.deckhouse.io/drained: "aws-node-termination-handler"
spec:
  taints:
  - key: aws-node-termination-handler/rebalance-recommendation
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

	Context("Node with spot-itn taint but no metadata", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeWithSpotITNTaint))
			f.RunHook()
		})

		It("Should add termination label and draining annotation", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.PatchCollector.Operations()).To(HaveLen(1))
		})
	})

	Context("Node with rebalance-recommendation taint but no metadata", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeWithRebalanceRecommendationTaint))
			f.RunHook()
		})

		It("Should add termination label and draining annotation", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.PatchCollector.Operations()).To(HaveLen(1))
		})
	})

	Context("Node with spot-itn taint AND metadata (already processed)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeWithSpotITNTaintAndMetadata))
			f.RunHook()
		})

		It("Should NOT add metadata again (idempotent)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.PatchCollector.Operations()).To(HaveLen(0))
		})
	})

	Context("Node with rebalance-recommendation taint AND metadata (already processed)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeWithRebalanceRecommendationTaintAndMetadata))
			f.RunHook()
		})

		It("Should NOT add metadata again (idempotent)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.PatchCollector.Operations()).To(HaveLen(0))
		})
	})

	Context("Node with termination taint AFTER drain (label present, draining annotation gone)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeWithRebalanceRecommendationTaintAfterDrain))
			f.RunHook()
		})

		It("Should NOT re-add draining annotation and avoid a drain loop", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.PatchCollector.Operations()).To(HaveLen(0))
		})
	})

	Context("Normal node without any AWS termination taint", func() {
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
			f.BindingContexts.Set(f.KubeStateSet(
				nodeWithSpotITNTaint +
					nodeWithRebalanceRecommendationTaint +
					nodeWithSpotITNTaintAndMetadata +
					nodeWithRebalanceRecommendationTaintAndMetadata +
					normalNode,
			))
			f.RunHook()
		})

		It("Should only process nodes that need metadata", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.PatchCollector.Operations()).To(HaveLen(2))
		})
	})
})

/*
Copyright 2021 Flant JSC

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

var _ = Describe("Modules :: node-manager :: hooks :: remove_csi_taints ::", func() {
	const (
		stateNodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-1
spec:
  taints:
  - key: somekey-1
    effect: PreferNoSchedule
  - effect: NoSchedule
    key: node.deckhouse.io/csi-not-bootstrapped
    value: ""
---
apiVersion: v1
kind: Node
metadata:
  name: node-2
spec:
  taints:
  - key: somekey-2
    effect: PreferNoSchedule
  - effect: NoSchedule
    key: node.deckhouse.io/csi-not-bootstrapped
    value: ""
---
apiVersion: v1
kind: Node
metadata:
  name: node-3
spec:
  taints:
  - key: somekey-3
    effect: PreferNoSchedule
---
apiVersion: v1
kind: Node
metadata:
  name: node-4
spec:
  taints:
  - effect: NoSchedule
    key: node.deckhouse.io/csi-not-bootstrapped
    value: ""
---
apiVersion: v1
kind: Node
metadata:
  name: node-5
spec: {}
`
		stateCSINode1 = `
---
apiVersion: storage.k8s.io/v1
kind: CSINode
metadata:
  name: node-1
spec:
  drivers:
  - name: test
`
		stateCSINode2 = `
---
apiVersion: storage.k8s.io/v1
kind: CSINode
metadata:
  name: node-2
spec:
  drivers:
  - name: test
`
		stateCSINode4 = `
---
apiVersion: storage.k8s.io/v1
kind: CSINode
metadata:
  name: node-4
`
	)

	f := HookExecutionConfigInit(`{"nodeManager":{"internal": {}}}`, `{}`)

	Context("Cluster is empty", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(``, 1))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster has five nodes and single CSINode", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateNodes+stateCSINode1, 1))
			f.RunHook()
		})

		It("node-1 must lose taint 'node.deckhouse.io/csi-not-bootstrapped'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Node", "node-1").Field("spec.taints").String()).To(MatchJSON(`[{"effect": "PreferNoSchedule","key":"somekey-1"}]`))
			Expect(f.KubernetesGlobalResource("Node", "node-2").Field("spec.taints").String()).To(MatchJSON(`[{"effect": "PreferNoSchedule","key":"somekey-2"},{"effect":"NoSchedule","key":"node.deckhouse.io/csi-not-bootstrapped","value":""}]`))
			Expect(f.KubernetesGlobalResource("Node", "node-3").Field("spec.taints").String()).To(MatchJSON(`[{"effect": "PreferNoSchedule","key":"somekey-3"}]`))
		})

		Context("CSINode for node-2 added", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateNodes+stateCSINode1+stateCSINode2, 1))
				f.RunHook()
			})

			It("node-2 must lose taint 'node.deckhouse.io/csi-not-bootstrapped'", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.KubernetesGlobalResource("Node", "node-1").Field("spec.taints").String()).To(MatchJSON(`[{"effect": "PreferNoSchedule","key":"somekey-1"}]`))
				Expect(f.KubernetesGlobalResource("Node", "node-2").Field("spec.taints").String()).To(MatchJSON(`[{"effect": "PreferNoSchedule","key":"somekey-2"}]`))
				Expect(f.KubernetesGlobalResource("Node", "node-3").Field("spec.taints").String()).To(MatchJSON(`[{"effect": "PreferNoSchedule","key":"somekey-3"}]`))
			})
		})

		Context("CSINode for node-4 added", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateNodes+stateCSINode1+stateCSINode4, 1))
				f.RunHook()
			})

			It("node-4 must not get spec.taints", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.KubernetesGlobalResource("Node", "node-1").Field("spec.taints").String()).To(MatchJSON(`[{"effect": "PreferNoSchedule","key":"somekey-1"}]`))
				Expect(f.KubernetesGlobalResource("Node", "node-2").Field("spec.taints").String()).To(MatchJSON(`[{"effect": "PreferNoSchedule","key": "somekey-2"},{"effect": "NoSchedule","key": "node.deckhouse.io/csi-not-bootstrapped","value": ""}]`))
				Expect(f.KubernetesGlobalResource("Node", "node-3").Field("spec.taints").String()).To(MatchJSON(`[{"effect": "PreferNoSchedule","key":"somekey-3"}]`))
				Expect(f.KubernetesGlobalResource("Node", "node-4").Field("spec.taints").String()).To(MatchJSON(`[{"effect":"NoSchedule","key":"node.deckhouse.io/csi-not-bootstrapped","value":""}]`))
			})
		})

	})
})

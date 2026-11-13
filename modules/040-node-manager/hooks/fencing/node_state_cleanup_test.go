/*
Copyright 2026 Flant JSC

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

package fencing

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	fencedNode = `
---
apiVersion: v1
kind: Node
metadata:
  name: worker-1
  annotations:
    node-manager.deckhouse.io/fencing-profile: slow
    node-manager.deckhouse.io/fencing-cp-source: proxy
    node-manager.deckhouse.io/fencing-disable: ""
status:
  conditions:
  - type: Ready
    status: "True"
    reason: KubeletReady
  - type: Fencing
    status: "True"
    reason: Armed
`
	disabledGroupNode = `
---
apiVersion: v1
kind: Node
metadata:
  name: master-0
  annotations:
    node-manager.deckhouse.io/fencing-profile: disabled
    node-manager.deckhouse.io/fencing-cp-source: self
status:
  conditions:
  - type: Ready
    status: "True"
    reason: KubeletReady
  - type: Fencing
    status: "False"
    reason: GroupDisabled
`
	plainNode = `
---
apiVersion: v1
kind: Node
metadata:
  name: worker-2
status:
  conditions:
  - type: Ready
    status: "True"
    reason: KubeletReady
`
	slowGroup = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  fencing:
    mode: Slow
`
	disabledGroups = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  fencing:
    mode: Disabled
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: Static
`
)

var _ = Describe("Modules :: node-manager :: hooks :: fencing :: node_state_cleanup ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{"fencing":{}}}}`, `{}`)
	registerNodeGroupCRD(f)

	conditionTypes := func(name string) []string {
		var types []string
		for _, condition := range f.KubernetesGlobalResource("Node", name).Field("status.conditions").Array() {
			types = append(types, condition.Get("type").String())
		}
		return types
	}

	keepsTheState := func() {
		Expect(f).To(ExecuteSuccessfully())
		Expect(conditionTypes("worker-1")).To(Equal([]string{"Ready", "Fencing"}))
		annotations := f.KubernetesGlobalResource("Node", "worker-1").Field("metadata.annotations").Map()
		Expect(annotations).To(HaveKey("node-manager.deckhouse.io/fencing-profile"))
		Expect(annotations).To(HaveKey("node-manager.deckhouse.io/fencing-cp-source"))
	}

	Context("A node group runs the agents", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(slowGroup + fencedNode + plainNode + agentPodV2YAML))
			f.RunGoHook()
		})

		It("Must keep the state of the agent", keepsTheState)
	})

	Context("A node group is fenced again, and the agents do not run yet", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(slowGroup + fencedNode + plainNode))
			f.RunGoHook()
		})

		It("Must keep the state of the agent", keepsTheState)
	})

	Context("All node groups are Disabled, and an agent pod still exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(disabledGroups + fencedNode + plainNode + agentPodV2YAML))
			f.RunGoHook()
		})

		It("Must keep the state of the agent", keepsTheState)

		Context("The last agent pod is gone", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(disabledGroups + fencedNode + plainNode))
				f.RunGoHook()
			})

			It("Must remove the condition and the annotations of the agent", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(conditionTypes("worker-1")).To(Equal([]string{"Ready"}))
				annotations := f.KubernetesGlobalResource("Node", "worker-1").Field("metadata.annotations").Map()
				Expect(annotations).NotTo(HaveKey("node-manager.deckhouse.io/fencing-profile"))
				Expect(annotations).NotTo(HaveKey("node-manager.deckhouse.io/fencing-cp-source"))
			})
		})
	})

	Context("All node groups are Disabled", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(disabledGroups + fencedNode + disabledGroupNode + plainNode))
			f.RunGoHook()
		})

		It("Must remove the condition and the annotations of the agent only", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(conditionTypes("worker-1")).To(Equal([]string{"Ready"}))
			annotations := f.KubernetesGlobalResource("Node", "worker-1").Field("metadata.annotations").Map()
			Expect(annotations).NotTo(HaveKey("node-manager.deckhouse.io/fencing-profile"))
			Expect(annotations).NotTo(HaveKey("node-manager.deckhouse.io/fencing-cp-source"))
			Expect(annotations).To(HaveKey("node-manager.deckhouse.io/fencing-disable"))
			Expect(conditionTypes("worker-2")).To(Equal([]string{"Ready"}))
		})

		It("Must remove the profile annotation with the value disabled", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(conditionTypes("master-0")).To(Equal([]string{"Ready"}))
			Expect(f.KubernetesGlobalResource("Node", "master-0").Field("metadata.annotations").Map()).To(BeEmpty())
		})
	})
})

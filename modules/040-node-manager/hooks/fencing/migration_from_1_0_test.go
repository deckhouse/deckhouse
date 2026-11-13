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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	nodeWithLabel = `
---
apiVersion: v1
kind: Node
metadata:
  name: worker-1
  labels:
    node-manager.deckhouse.io/fencing-enabled: ""
status:
  conditions:
  - type: Ready
    status: "True"
`
	nodeWithoutLabel = `
---
apiVersion: v1
kind: Node
metadata:
  name: worker-1
status:
  conditions:
  - type: Ready
    status: "True"
`
	// fencing 1.0 stopped the node s1, and the node is off.
	stoppedNodeWithLabel = `
---
apiVersion: v1
kind: Node
metadata:
  name: s1
  labels:
    node-manager.deckhouse.io/fencing-enabled: ""
status:
  conditions:
  - type: Ready
    status: Unknown
`
	stoppedNodeAgentPod1 = `
---
apiVersion: v1
kind: Pod
metadata:
  name: fencing-agent-worker-klmno
  namespace: d8-cloud-instance-manager
  labels:
    app: fencing-agent
    node-group-name: worker
spec:
  nodeName: s1
`
	agentPod1 = `
---
apiVersion: v1
kind: Pod
metadata:
  name: fencing-agent-worker-abcde
  namespace: d8-cloud-instance-manager
  labels:
    app: fencing-agent
    node-group-name: worker
spec:
  nodeName: worker-1
`
	agentPod2 = `
---
apiVersion: v1
kind: Pod
metadata:
  name: fencing-agent-worker-vwxyz
  namespace: d8-cloud-instance-manager
  labels:
    app: fencing-agent
    node-group-name: worker
    node-manager.deckhouse.io/fencing-agent-version: "2"
spec:
  nodeName: worker-1
`
)

var _ = Describe("Modules :: node-manager :: hooks :: fencing :: migration_from_1_0 ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{"fencing":{}}}}`, `{}`)

	flag := func() bool { return f.ValuesGet("nodeManager.internal.fencing.migrationFrom10").Bool() }

	DescribeTable("The flag follows the traces of fencing 1.0",
		func(state string, want bool) {
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunGoHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.fencing.migrationFrom10").Exists()).To(BeTrue())
			Expect(flag()).To(Equal(want))
		},
		Entry("empty cluster", ``, false),
		Entry("a Node has the label of fencing 1.0", nodeWithLabel, true),
		Entry("only an agent pod of version 1 exists", nodeWithoutLabel+agentPod1, true),
		Entry("the label and a pod of version 1 exist", nodeWithLabel+agentPod1, true),
		Entry("only agent pods of version 2 exist", nodeWithoutLabel+agentPod2, false),
		Entry("no label, no pods", nodeWithoutLabel, false),
		Entry("the traces are only on a Node that is not Ready", nodeWithoutLabel+agentPod2+stoppedNodeWithLabel+stoppedNodeAgentPod1, false),
		Entry("only a pod of version 1 is on a Node that is not Ready", nodeWithoutLabel+stoppedNodeAgentPod1, false),
		Entry("a Ready Node has the label, another Node is not Ready", nodeWithLabel+stoppedNodeWithLabel, true),
	)

	Context("The last trace of fencing 1.0 disappears", func() {
		It("Must set the flag and then remove it", func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeWithLabel))
			f.RunGoHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(flag()).To(BeTrue())

			f.KubeStateSet(agentPod2)
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
			f.RunGoHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(flag()).To(BeFalse())
		})
	})
})

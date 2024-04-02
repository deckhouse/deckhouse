/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package pricing

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Flant integration :: hooks :: envs_from_nodes", func() {
	f := HookExecutionConfigInit(`{"flantIntegration":{"internal":{}}}`, `{}`)

	Context("Cluster with one master", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Node
metadata:
  name: master-0
  labels:
    node.deckhouse.io/group: master
    node-role.kubernetes.io/control-plane: ""
status:
  allocatable:
    cpu: "4"
    memory: "16560077788"
  nodeInfo:
    kubeletVersion: v1.16.15
`))
			f.RunHook()
		})

		It("Should run correctly on single master", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("flantIntegration.internal.nodeStats.minimalKubeletVersion").String()).To(Equal(`1.16`))
			Expect(f.ValuesGet("flantIntegration.internal.nodeStats.staticNodesCount").String()).To(Equal(`0`))
			Expect(f.ValuesGet("flantIntegration.internal.nodeStats.mastersCount").String()).To(Equal(`1`))
			Expect(f.ValuesGet("flantIntegration.internal.nodeStats.masterIsDedicated").String()).To(Equal(`false`))
			Expect(f.ValuesGet("flantIntegration.internal.nodeStats.masterMinCPU").String()).To(Equal(`4`))
			Expect(f.ValuesGet("flantIntegration.internal.nodeStats.masterMinMemory").String()).To(Equal(`16560077788`))
		})
	})

	Context("Cluster with two masters and multiple nodes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Node
metadata:
  name: master-0
  labels:
    node.deckhouse.io/group: master
    node-role.kubernetes.io/control-plane: ""
spec:
  taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/control-plane
status:
  allocatable:
    cpu: "4"
    memory: "16560077788"
  nodeInfo:
    kubeletVersion: v1.16.15
---
apiVersion: v1
kind: Node
metadata:
  name: master-1
  labels:
    node.deckhouse.io/group: master
    node-role.kubernetes.io/control-plane: ""
spec:
  taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/control-plane
status:
  allocatable:
    cpu: "2"
    memory: "8280038894"
  nodeInfo:
    kubeletVersion: v1.16.15
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    node.deckhouse.io/group: ng0
    node.deckhouse.io/type: Static
status:
  nodeInfo:
    kubeletVersion: v1.16.15
---
apiVersion: v1
kind: Node
metadata:
  name: node2
  labels:
    node.deckhouse.io/group: ng0
    node.deckhouse.io/type: Static
status:
  nodeInfo:
    kubeletVersion: v1.15.12
---
apiVersion: v1
kind: Node
metadata:
  name: node3
  labels:
    node.deckhouse.io/group: ng0
    node.deckhouse.io/type: CloudEphemeral
status:
  nodeInfo:
    kubeletVersion: v1.16.15
`))
			f.RunHook()
		})
		It("Should run correctly on multi master", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("flantIntegration.internal.nodeStats.minimalKubeletVersion").String()).To(Equal(`1.15`))
			Expect(f.ValuesGet("flantIntegration.internal.nodeStats.staticNodesCount").String()).To(Equal(`2`))
			Expect(f.ValuesGet("flantIntegration.internal.nodeStats.mastersCount").String()).To(Equal(`2`))
			Expect(f.ValuesGet("flantIntegration.internal.nodeStats.masterIsDedicated").String()).To(Equal(`true`))
			Expect(f.ValuesGet("flantIntegration.internal.nodeStats.masterMinCPU").String()).To(Equal(`2`))
			Expect(f.ValuesGet("flantIntegration.internal.nodeStats.masterMinMemory").String()).To(Equal(`8280038894`))
		})
	})

	Context("Cluster with one master defined allocatable in units ", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Node
metadata:
 name: master-0
 labels:
   node.deckhouse.io/group: master
   node-role.kubernetes.io/control-plane: ""
status:
 allocatable:
   cpu: 1900m
   memory: 7763032Ki
 nodeInfo:
   kubeletVersion: v1.19.5
---
apiVersion: v1
kind: Node
metadata:
 name: master-1
 labels:
   node.deckhouse.io/group: master
   node-role.kubernetes.io/control-plane: ""
status:
 allocatable:
   cpu: 4
   memory: 17763032Ki
 nodeInfo:
   kubeletVersion: v1.19.5
`))
			f.RunHook()
		})

		It("Should run correctly on single master", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("flantIntegration.internal.nodeStats.minimalKubeletVersion").String()).To(Equal(`1.19`))
			Expect(f.ValuesGet("flantIntegration.internal.nodeStats.staticNodesCount").String()).To(Equal(`0`))
			Expect(f.ValuesGet("flantIntegration.internal.nodeStats.mastersCount").String()).To(Equal(`2`))
			Expect(f.ValuesGet("flantIntegration.internal.nodeStats.masterIsDedicated").String()).To(Equal(`false`))
			Expect(f.ValuesGet("flantIntegration.internal.nodeStats.masterMinCPU").String()).To(Equal(`2`))
			Expect(f.ValuesGet("flantIntegration.internal.nodeStats.masterMinMemory").String()).To(Equal(`7949344768`))
		})
	})
})

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: flant-pricing :: hooks :: envs_from_nodes", func() {
	f := HookExecutionConfigInit(`{"flantPricing":{"internal":{}}}`, `{}`)

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
    node-role.kubernetes.io/master: ""
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
			Expect(f.ValuesGet("flantPricing.internal.minimalKubeletVersion").String()).To(Equal(`1.16`))
			Expect(f.ValuesGet("flantPricing.internal.mastersCount").String()).To(Equal(`1`))
			Expect(f.ValuesGet("flantPricing.internal.masterIsDedicated").String()).To(Equal(`false`))
			Expect(f.ValuesGet("flantPricing.internal.masterMinCPU").String()).To(Equal(`4`))
			Expect(f.ValuesGet("flantPricing.internal.masterMinMemory").String()).To(Equal(`16560077788`))
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
    node-role.kubernetes.io/master: ""
spec:
  taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/master
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
    node-role.kubernetes.io/master: ""
spec:
  taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/master
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
status:
  nodeInfo:
    kubeletVersion: v1.16.15
`))
			f.RunHook()
		})
		It("Should run correctly on multi master", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("flantPricing.internal.minimalKubeletVersion").String()).To(Equal(`1.15`))
			Expect(f.ValuesGet("flantPricing.internal.mastersCount").String()).To(Equal(`2`))
			Expect(f.ValuesGet("flantPricing.internal.masterIsDedicated").String()).To(Equal(`true`))
			Expect(f.ValuesGet("flantPricing.internal.masterMinCPU").String()).To(Equal(`2`))
			Expect(f.ValuesGet("flantPricing.internal.masterMinMemory").String()).To(Equal(`8280038894`))
		})
	})
})

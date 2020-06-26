package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: metrics ::", func() {
	const (
		stateMachineDeploymentProper = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  finalizers:
  - machine.sapcloud.io/machine-controller-manager
  labels:
    heritage: deckhouse
    node-group: proper
    module: node-manager
  name: dev-proper-297926a1
  namespace: d8-cloud-instance-manager
spec:
  minReadySeconds: 300
  replicas: 1
  selector:
    matchLabels:
      instance-group: proper-nova
  strategy:
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
    type: RollingUpdate
  template:
    metadata:
      annotations:
        checksum/machine-class: 95ad1a35397453b647c1d25d78264bca4605ff522e719e8f605159a9351d8c2a
      creationTimestamp: null
      labels:
        instance-group: bad-nova
    spec:
      class:
        kind: OpenStackMachineClass
        name: bad-297926a1
      nodeTemplate:
        metadata:
          creationTimestamp: null
          labels:
            node.deckhouse.io/group: bad
            node-role.kubernetes.io/bad: ""
        spec: {}
`
		stateNodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node0
  labels:
    node.deckhouse.io/group: ng0
  annotations: {}
   # status should be "ToBeUpdated"
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    node.deckhouse.io/group: ng0
  annotations:
    "node.deckhouse.io/configuration-checksum": abc # "abc" is stored in configuration-checksums-secret
    # status should be "UpToDate"
---
apiVersion: v1
kind: Node
metadata:
  name: node2
  labels:
    node.deckhouse.io/group: ng0
  annotations:
    "node.deckhouse.io/configuration-checksum": xyz # not desired
    # status should be "ToBeUpdated"
---
apiVersion: v1
kind: Node
metadata:
  name: node3
  labels:
    node.deckhouse.io/group: ng0
  annotations:
    node.deckhouse.io/configuration-checksum: xyz # not desired
    update.node.deckhouse.io/waiting-for-approval: ""  # status should be "WaitingForApproval"
---
apiVersion: v1
kind: Node
metadata:
  name: node4
  labels:
    node.deckhouse.io/group: ng0
  annotations:
    node.deckhouse.io/configuration-checksum: xyz # not desired
    update.node.deckhouse.io/approved: ""  # status should be "Approved"
---
apiVersion: v1
kind: Node
metadata:
  name: node50
  labels:
    node.deckhouse.io/group: ng0
  annotations:
    node.deckhouse.io/configuration-checksum: xyz # not desired
    update.node.deckhouse.io/approved: ""
    update.node.deckhouse.io/disruption-required: "" # status should be "WaitingForDisruptionApproval" due to NG.disruptions.approvalMode = "Automatic"
---
apiVersion: v1
kind: Node
metadata:
  name: node51
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: xyz # not desired
    update.node.deckhouse.io/approved: ""
    update.node.deckhouse.io/disruption-required: "" # status should be "WaitingForManualDisruptionApproval" due to NG.disruptions.approvalMode = "Manual"
---
apiVersion: v1
kind: Node
metadata:
  name: node6
  labels:
    node.deckhouse.io/group: ng0
  annotations:
    node.deckhouse.io/configuration-checksum: xyz # not desired
    update.node.deckhouse.io/approved: ""
    update.node.deckhouse.io/disruption-required: ""
    update.node.deckhouse.io/draining: "" # status should be "DrainingForDisruption"
---
apiVersion: v1
kind: Node
metadata:
  name: node7
  labels:
    node.deckhouse.io/group: ng0
  annotations:
    node.deckhouse.io/configuration-checksum: xyz # not desired
    update.node.deckhouse.io/approved: ""
    update.node.deckhouse.io/disruption-required: ""
    update.node.deckhouse.io/drained: "" # status should be "DrainedForDisruption"
---
apiVersion: v1
kind: Node
metadata:
  name: node8
  labels:
    node.deckhouse.io/group: ng0
  annotations:
    node.deckhouse.io/configuration-checksum: xyz # not desired
    update.node.deckhouse.io/approved: ""
    update.node.deckhouse.io/disruption-approved: "" # status should be "DisruptionApproved"
`

		stateNodeGroups = `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: ng0
spec: {}
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: ng1
spec:
  disruptions:
    approvalMode: Manual
`

		stateConfigurationChecksumsSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: configuration-checksums
  namespace: d8-cloud-instance-manager
type: Opaque
data:
  ng0: YWJj #abc
  ng1: YWJj #abc
`
	)

	f := HookExecutionConfigInit(`{"nodeManager":{"internal": {}}}`, `{}`)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineDeployment", true)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "NodeGroup", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with proper machine deployment", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMachineDeploymentProper))
			f.RunHook()
		})

		It("Hook must not fail, metrics must render", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.machine_deployments.0.filterResult.labels").String()).To(MatchJSON(`{"node_group":"proper","name":"dev-proper-297926a1"}`))
		})
	})

	Context("All kinds of nodes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNodes + stateNodeGroups + stateConfigurationChecksumsSecret))
			f.RunHook()
		})

		It("Hook must not fail, metrics must render", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

})

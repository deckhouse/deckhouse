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
    instance-group: proper
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
        checksum/bashible-bundles-options: d801592ae7c43d3b0fba96a805c8d9f7fd006b9726daf97ba7f7abc399a56b09
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
            cloud-instance-manager.deckhouse.io/cloud-instance-group: bad
            node-role.kubernetes.io/bad: ""
        spec: {}


`
	)

	f := HookExecutionConfigInit(`{"nodeManager":{"internal": {}}}`, `{}`)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineDeployment", true)

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
})

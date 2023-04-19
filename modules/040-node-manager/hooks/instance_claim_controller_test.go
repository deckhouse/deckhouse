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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: instance_claim_controller ::", func() {
	f := HookExecutionConfigInit(`{"global": {"discovery": {"kubernetesVersion": "1.23.1"}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "InstanceClaim", false)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineDeployment", true)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "Machine", true)

	const ng = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
  uid: 87233806-25b3-41b4-8c15-46b7212326b4
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
`

	assertFinalizersExists := func(f *HookExecutionConfig, claimName string) {
		finalizers := f.KubernetesGlobalResource("InstanceClaim", claimName).Field("metadata.finalizers")
		Expect(finalizers.AsStringSlice()).To(Equal([]string{"hooks.deckhouse.io/node-manager/instance_claim_controller"}))
	}

	assertCurrentStatus := func(f *HookExecutionConfig, claimName string) {
		ic := f.KubernetesGlobalResource("InstanceClaim", claimName)
		machine := f.KubernetesResource("Machine", "d8-cloud-instance-manager", claimName)

		Expect(ic.Field("status.currentStatus.lastUpdateTime").Exists()).To(BeTrue())
		Expect(machine.Field("status.currentStatus.lastUpdateTime").Exists()).To(BeTrue())
		icTime, err := time.Parse(time.RFC3339, ic.Field("status.currentStatus.lastUpdateTime").String())
		Expect(err).ToNot(HaveOccurred())
		machineTime, err := time.Parse(time.RFC3339, machine.Field("status.currentStatus.lastUpdateTime").String())
		Expect(err).ToNot(HaveOccurred())
		Expect(icTime.Equal(machineTime)).To(BeTrue())

		Expect(ic.Field("status.currentStatus.phase").Exists()).To(BeTrue())
		Expect(machine.Field("status.currentStatus.phase").Exists()).To(BeTrue())
		Expect(ic.Field("status.currentStatus.phase").String()).To(Equal(machine.Field("status.currentStatus.phase").String()))
	}

	assertLastOperation := func(f *HookExecutionConfig, claimName string) {
		ic := f.KubernetesGlobalResource("InstanceClaim", claimName)
		machine := f.KubernetesResource("Machine", "d8-cloud-instance-manager", claimName)

		Expect(ic.Field("status.lastOperation.lastUpdateTime").Exists()).To(BeTrue())
		Expect(machine.Field("status.lastOperation.lastUpdateTime").Exists()).To(BeTrue())
		icTime, err := time.Parse(time.RFC3339, ic.Field("status.lastOperation.lastUpdateTime").String())
		Expect(err).ToNot(HaveOccurred())
		machineTime, err := time.Parse(time.RFC3339, machine.Field("status.lastOperation.lastUpdateTime").String())
		Expect(err).ToNot(HaveOccurred())
		Expect(icTime.Equal(machineTime)).To(BeTrue())

		Expect(ic.Field("status.lastOperation.description").Exists()).To(BeTrue())
		Expect(ic.Field("status.lastOperation.description").String()).To(Equal(machine.Field("status.lastOperation.description").String()))

		Expect(ic.Field("status.lastOperation.state").Exists()).To(BeTrue())
		Expect(ic.Field("status.lastOperation.state").String()).To(Equal(machine.Field("status.lastOperation.state").String()))

		Expect(ic.Field("status.lastOperation.type").Exists()).To(BeTrue())
		Expect(ic.Field("status.lastOperation.type").String()).To(Equal(machine.Field("status.lastOperation.type").String()))
	}

	assertMachineRef := func(f *HookExecutionConfig, claimName string) {
		ic := f.KubernetesGlobalResource("InstanceClaim", claimName)

		Expect(ic.Field("status.machineRef.kind").Exists()).To(BeTrue())
		Expect(ic.Field("status.machineRef.kind").String()).To(Equal("Machine"))

		Expect(ic.Field("status.machineRef.apiVersion").Exists()).To(BeTrue())
		Expect(ic.Field("status.machineRef.apiVersion").String()).To(Equal("machine.sapcloud.io/v1alpha1"))

		Expect(ic.Field("status.machineRef.namespace").Exists()).To(BeTrue())
		Expect(ic.Field("status.machineRef.namespace").String()).To(Equal("d8-cloud-instance-manager"))

		Expect(ic.Field("status.machineRef.name").Exists()).To(BeTrue())
		Expect(ic.Field("status.machineRef.name").String()).To(Equal(claimName))
	}

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Adding instance claims", func() {
		const (
			ic1 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: InstanceClaim
metadata:
  name: worker-ac32h
  finalizers:
  - hooks.deckhouse.io/node-manager/instance_claim_controller
status: {}
`
			machine1 = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: worker-ac32h
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: ng1-nova
spec:
  nodeTemplate:
    metadata:
      labels:
        node-role.kubernetes.io/ng1: ""
        node.deckhouse.io/group: ng1
        node.deckhouse.io/type: CloudEphemeral
status: {}
`

			machine2 = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: worker-fac21
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: ng1-nova
spec:
  nodeTemplate:
    metadata:
      labels:
        node-role.kubernetes.io/ng1: ""
        node.deckhouse.io/group: ng1
        node.deckhouse.io/type: CloudEphemeral
status:
  currentStatus:
    lastUpdateTime: "2023-04-18T15:54:55Z"
    phase: Pending
  lastOperation:
    description: Create machine in the cloud provider
    lastUpdateTime: "2023-04-18T15:54:55Z"
    state: Processing
    type: Create
`
		)

		Context("does not have instance classes but have machine", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(ng + ic1 + machine1 + machine2))
				f.RunHook()
			})

			It("Should keep 'as is' instance claim with machine", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Exists()).To(BeTrue())
				ic := f.KubernetesGlobalResource("InstanceClaim", "worker-ac32h")
				machine := f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-ac32h")

				Expect(ic.Exists()).To(BeTrue())
				Expect(machine.Exists()).To(BeTrue())

				Expect(ic.ToYaml()).To(MatchYAML(ic1))
				Expect(machine.ToYaml()).To(MatchYAML(machine1))
			})

			It("Should create instance claim for machine", func() {
				Expect(f).To(ExecuteSuccessfully())

				ic := f.KubernetesGlobalResource("InstanceClaim", "worker-fac21")
				machine := f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-fac21")

				Expect(ic.Exists()).To(BeTrue())
				Expect(machine.Exists()).To(BeTrue())

				Expect(ic.Field(`metadata.labels.node\.deckhouse\.io/group`).String()).To(Equal("ng1"))
				assertCurrentStatus(f, "worker-fac21")
				assertLastOperation(f, "worker-fac21")
				assertMachineRef(f, "worker-fac21")
			})
		})
	})

	Context("Updating instance claims status", func() {
		const (
			ic = `
---
apiVersion: deckhouse.io/v1alpha1
kind: InstanceClaim
metadata:
  labels:
    node.deckhouse.io/group: "ng1"
  name: worker-ac32h
  finalizers:
  - hooks.deckhouse.io/node-manager/instance_claim_controller
status:
  currentStatus:
    lastUpdateTime: "2023-04-18T15:54:55Z"
    phase: Pending
  lastOperation:
    description: Create machine in the cloud provider
    lastUpdateTime: "2023-04-18T15:54:55Z"
    state: Processing
    type: Create
  machineRef:
    kind: Machine
    apiVersion: machine.sapcloud.io/v1alpha1
    namespace: d8-cloud-instance-manager
    name: worker-ac32h
`
			machine = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: worker-ac32h
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: ng1-nova
    node: worker-ac32h
spec:
  nodeTemplate:
    metadata:
      labels:
        node-role.kubernetes.io/ng1: ""
        node.deckhouse.io/group: ng1
        node.deckhouse.io/type: CloudEphemeral
status:
  currentStatus:
    lastUpdateTime: "2023-04-19T15:54:55Z"
    phase: Running
  lastOperation:
    description: Machine sandbox-stage-8ef4a622-6655b-wbsfg successfully re-joined the cluster
    lastUpdateTime: "2023-04-18T16:54:55Z"
    state: Successful
    type: HealthCheck
`
		)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(ng + ic + machine))
			f.RunHook()
		})

		It("Should update instance claim status from machine status", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Exists()).To(BeTrue())
			ic := f.KubernetesGlobalResource("InstanceClaim", "worker-ac32h")
			machine := f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-ac32h")

			Expect(ic.Exists()).To(BeTrue())
			Expect(machine.Exists()).To(BeTrue())

			Expect(ic.Field(`metadata.labels.node\.deckhouse\.io/group`).String()).To(Equal("ng1"))

			Expect(ic.Field(`status.nodeRef.name`).Exists()).To(BeTrue())
			Expect(ic.Field(`status.nodeRef.name`).String()).To(Equal("worker-ac32h"))

			assertMachineRef(f, "worker-ac32h")
			assertCurrentStatus(f, "worker-ac32h")
			assertLastOperation(f, "worker-ac32h")
		})
	})

	Context("Deleting instance claims (have instance claims but do not have machines)", func() {
		const (
			ic1 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: InstanceClaim
metadata:
  name: worker-ac32h
  finalizers:
  - hooks.deckhouse.io/node-manager/instance_claim_controller
status: {}
`
			machine = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: worker-ac32h
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: ng1-nova
spec:
  nodeTemplate:
    metadata:
      labels:
        node-role.kubernetes.io/ng1: ""
        node.deckhouse.io/group: ng1
        node.deckhouse.io/type: CloudEphemeral
`
		)

		Context("does not start deletion instance claim (without deletion timestamp", func() {
			const ic2 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: InstanceClaim
metadata:
  name: worker-bg11u
  finalizers:
  - hooks.deckhouse.io/node-manager/instance_claim_controller
status: {}
`
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(ng + ic1 + ic2 + machine))
				f.RunHook()
			})

			It("Should keep instance claim with machine", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Exists()).To(BeTrue())
				Expect(f.KubernetesGlobalResource("InstanceClaim", "worker-ac32h").Exists()).To(BeTrue())
				Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-ac32h").Exists()).To(BeTrue())
				assertFinalizersExists(f, "worker-ac32h")
			})

			It("Should delete instance claim without machine", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(f.KubernetesGlobalResource("InstanceClaim", "worker-bg11u").Exists()).To(BeFalse())
			})
		})

		Context("start deletion instance claim (with deletion timestamp)", func() {
			Context("does not have machine for deleted instance claim", func() {
				const ic2 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: InstanceClaim
metadata:
  name: worker-bg11u
  finalizers:
  - hooks.deckhouse.io/node-manager/instance_claim_controller
  deletionTimestamp: "1970-01-01T00:00:00Z"
status: {}
`
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(ng + ic1 + ic2 + machine))
					f.RunHook()
				})

				It("Should keep instance claim with machine", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Exists()).To(BeTrue())
					Expect(f.KubernetesGlobalResource("InstanceClaim", "worker-ac32h").Exists()).To(BeTrue())
					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-ac32h").Exists()).To(BeTrue())
					assertFinalizersExists(f, "worker-ac32h")
				})

				It("Should remove finalizers from instance claim without machine", func() {
					Expect(f).To(ExecuteSuccessfully())

					ic := f.KubernetesGlobalResource("InstanceClaim", "worker-bg11u")
					Expect(ic.Exists()).To(BeTrue())
					Expect(ic.Field("metadata.finalizers").Array()).To(BeEmpty())
				})
			})

			Context("have machine for deleted instance claim", func() {
				const (
					ic2 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: InstanceClaim
metadata:
  name: worker-bg11u
  finalizers:
  - hooks.deckhouse.io/node-manager/instance_claim_controller
  deletionTimestamp: "1970-01-01T00:00:00Z"
status: {}
`
					machine2 = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: worker-bg11u
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: ng1-nova
spec:
  nodeTemplate:
    metadata:
      labels:
        node-role.kubernetes.io/ng1: ""
        node.deckhouse.io/group: ng1
        node.deckhouse.io/type: CloudEphemeral
`
				)
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(ng + ic1 + ic2 + machine + machine2))
					f.RunHook()
				})

				It("Should keep another instance claims", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Exists()).To(BeTrue())
					Expect(f.KubernetesGlobalResource("InstanceClaim", "worker-ac32h").Exists()).To(BeTrue())
					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-ac32h").Exists()).To(BeTrue())
					assertFinalizersExists(f, "worker-ac32h")
				})

				It("Should remove machine", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-bg11u").Exists()).To(BeFalse())
				})

				It("Should keep instance claim with finalizers", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(f.KubernetesGlobalResource("InstanceClaim", "worker-bg11u").Exists()).To(BeTrue())
					assertFinalizersExists(f, "worker-bg11u")
				})
			})
		})
	})
})

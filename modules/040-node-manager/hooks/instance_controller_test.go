/*
Copyright 2023 Flant JSC

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

var _ = Describe("Modules :: node-manager :: hooks :: instance_controller ::", func() {
	f := HookExecutionConfigInit(`{"global": {"discovery": {"kubernetesVersion": "1.29.1"}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "Instance", false)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineDeployment", true)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "Machine", true)
	f.RegisterCRD("cluster.x-k8s.io", "v1beta1", "Machine", true)

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
    classReference:
      kind: YandexInstanceClass
      name: worker
    maxPerZone: 5
    minPerZone: 1
`

	assertFinalizersExists := func(f *HookExecutionConfig, instanceName string) {
		finalizers := f.KubernetesGlobalResource("Instance", instanceName).Field("metadata.finalizers")
		Expect(finalizers.AsStringSlice()).To(Equal([]string{"node-manager.hooks.deckhouse.io/instance-controller"}))
	}

	assertCurrentStatus := func(f *HookExecutionConfig, instanceName string) {
		ic := f.KubernetesGlobalResource("Instance", instanceName)
		machine := f.KubernetesResource("Machine", "d8-cloud-instance-manager", instanceName)

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

	assertLastOperation := func(f *HookExecutionConfig, instanceName string) {
		ic := f.KubernetesGlobalResource("Instance", instanceName)
		machine := f.KubernetesResource("Machine", "d8-cloud-instance-manager", instanceName)

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

	assertMachineRef := func(f *HookExecutionConfig, instanceName string) {
		ic := f.KubernetesGlobalResource("Instance", instanceName)

		Expect(ic.Field("status.machineRef.kind").Exists()).To(BeTrue())
		Expect(ic.Field("status.machineRef.kind").String()).To(Equal("Machine"))

		Expect(ic.Field("status.machineRef.apiVersion").Exists()).To(BeTrue())
		Expect(ic.Field("status.machineRef.apiVersion").String()).To(Equal("machine.sapcloud.io/v1alpha1"))

		Expect(ic.Field("status.machineRef.namespace").Exists()).To(BeTrue())
		Expect(ic.Field("status.machineRef.namespace").String()).To(Equal("d8-cloud-instance-manager"))

		Expect(ic.Field("status.machineRef.name").Exists()).To(BeTrue())
		Expect(ic.Field("status.machineRef.name").String()).To(Equal(instanceName))
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

	Context("Adding instances", func() {
		const (
			ic1 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  name: worker-ac32h
  finalizers:
  - node-manager.hooks.deckhouse.io/instance-controller
status:
  classReference:
    kind: YandexInstanceClass
    name: worker
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

			It("Should keep 'as is' instance with machine", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Exists()).To(BeTrue())
				ic := f.KubernetesGlobalResource("Instance", "worker-ac32h")
				machine := f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-ac32h")

				Expect(ic.Exists()).To(BeTrue())
				Expect(machine.Exists()).To(BeTrue())

				Expect(ic.ToYaml()).To(MatchYAML(ic1))
				Expect(machine.ToYaml()).To(MatchYAML(machine1))
			})

			It("Should create instance for machine", func() {
				Expect(f).To(ExecuteSuccessfully())

				ic := f.KubernetesGlobalResource("Instance", "worker-fac21")
				machine := f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-fac21")

				Expect(ic.Exists()).To(BeTrue())
				Expect(machine.Exists()).To(BeTrue())

				Expect(ic.Field(`metadata.labels.node\.deckhouse\.io/group`).String()).To(Equal("ng1"))
				assertCurrentStatus(f, "worker-fac21")
				assertLastOperation(f, "worker-fac21")
				assertMachineRef(f, "worker-fac21")

				Expect(ic.Field("metadata.ownerReferences").IsArray()).To(BeTrue())
				Expect(ic.Field("metadata.ownerReferences").Array()).To(HaveLen(1))

				Expect(ic.Field("metadata.ownerReferences.0.kind").String()).To(Equal("NodeGroup"))
				Expect(ic.Field("metadata.ownerReferences.0.name").String()).To(Equal("ng1"))
				Expect(ic.Field("metadata.ownerReferences.0.uid").String()).To(Equal("87233806-25b3-41b4-8c15-46b7212326b4"))
			})
		})
	})

	Context("Updating instances status", func() {
		Context("Phase is Running and bootstrapStatus is not empty", func() {
			const (
				ic1 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  labels:
    node.deckhouse.io/group: "ng1"
  name: worker-dde21
  finalizers:
  - node-manager.hooks.deckhouse.io/instance-controller
status:
  classReference:
    kind: YandexInstanceClass
    name: worker
  currentStatus:
    lastUpdateTime: "2023-04-18T15:54:55Z"
    phase: Pending
  lastOperation:
    description: AAAA
    lastUpdateTime: "2023-04-18T15:54:55Z"
    state: Processing
    type: Create
  machineRef:
    kind: Machine
    apiVersion: machine.sapcloud.io/v1alpha1
    namespace: d8-cloud-instance-manager
    name: worker-dde21
  bootstrapStatus:
    logsEndpoint: 127.0.0.0:1111
    description: Use nc
`
				machine1 = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: worker-dde21
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: ng1-nova
    node: worker-dde21
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
    description: AAAA
    lastUpdateTime: "2023-04-18T15:54:55Z"
    state: Processing
    type: Create
`
				ic2 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  labels:
    node.deckhouse.io/group: "ng1"
  name: worker-ac32h
  finalizers:
  - node-manager.hooks.deckhouse.io/instance-controller
status:
  classReference:
    kind: YandexInstanceClass
    name: worker
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
  bootstrapStatus:
    logsEndpoint: 127.0.0.0:1111
    description: Use nc
`
				machine2 = `
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
				f.BindingContexts.Set(f.KubeStateSet(ng + ic1 + machine1 + ic2 + machine2))
				f.RunHook()
			})

			It("Should keep instance bootstrapStatus for none running machines", func() {
				Expect(f).To(ExecuteSuccessfully())

				ic := f.KubernetesGlobalResource("Instance", "worker-dde21")

				Expect(ic.Exists()).To(BeTrue())
				Expect(ic.Field(`status.bootstrapStatus.logsEndpoint`).String()).To(Equal("127.0.0.0:1111"))
				Expect(ic.Field(`status.bootstrapStatus.description`).String()).To(Equal("Use nc"))
			})

			It("Should remove bootstrapStatus for running machines", func() {
				Expect(f).To(ExecuteSuccessfully())

				ic := f.KubernetesGlobalResource("Instance", "worker-ac32h")

				Expect(ic.Exists()).To(BeTrue())
				Expect(ic.Field(`status.bootstrapStatus.logsEndpoint`).String()).To(BeEmpty())
				Expect(ic.Field(`status.bootstrapStatus.description`).String()).To(BeEmpty())
			})
		})

		Context("Another status fields", func() {
			const (
				ic1 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  labels:
    node.deckhouse.io/group: "ng1"
  name: worker-dde21
  finalizers:
  - node-manager.hooks.deckhouse.io/instance-controller
status:
  classReference:
    kind: YandexInstanceClass
    name: worker
  currentStatus:
    lastUpdateTime: "2023-04-18T15:54:55Z"
    phase: Pending
  lastOperation:
    description: AAAA
    lastUpdateTime: "2023-04-18T15:54:55Z"
    state: Processing
    type: Create
  machineRef:
    kind: Machine
    apiVersion: machine.sapcloud.io/v1alpha1
    namespace: d8-cloud-instance-manager
    name: worker-dde21
`
				machine1 = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: worker-dde21
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: ng1-nova
    node: worker-dde21
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
    description: AAAA
    lastUpdateTime: "2023-04-18T15:54:55Z"
    state: Processing
    type: Create
`

				machine2 = `
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
				ic2 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  labels:
    node.deckhouse.io/group: "ng1"
  name: worker-ac32h
  finalizers:
  - node-manager.hooks.deckhouse.io/instance-controller
status:
  classReference:
    kind: YandexInstanceClass
    name: worker
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
  bootstrapStatus:
    logsEndpoint: 127.0.0.0:1111
    description: Use nc
`
			)

			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(ng + ic1 + machine1 + ic2 + machine2))
				f.RunHook()
			})

			It("Should keep another instances", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Exists()).To(BeTrue())
				ic := f.KubernetesGlobalResource("Instance", "worker-dde21")
				machine := f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-dde21")

				Expect(ic.Exists()).To(BeTrue())
				Expect(machine.Exists()).To(BeTrue())

				Expect(ic.Field(`metadata.labels.node\.deckhouse\.io/group`).String()).To(Equal("ng1"))

				Expect(ic.Field(`status.nodeRef.name`).Exists()).To(BeTrue())
				Expect(ic.Field(`status.nodeRef.name`).String()).To(Equal("worker-dde21"))

				assertMachineRef(f, "worker-dde21")
				assertCurrentStatus(f, "worker-dde21")
				assertLastOperation(f, "worker-dde21")
			})

			It("Should update instance status from machine status", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Exists()).To(BeTrue())
				ic := f.KubernetesGlobalResource("Instance", "worker-ac32h")
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

		Context("Machine with last operation 'Started Machine creation process'", func() {
			const (
				ic1 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  labels:
    node.deckhouse.io/group: "ng1"
  name: worker-dde21
  finalizers:
  - node-manager.hooks.deckhouse.io/instance-controller
status:
  classReference:
    kind: YandexInstanceClass
    name: worker
  currentStatus:
    lastUpdateTime: "2023-04-18T15:54:55Z"
    phase: Pending
  lastOperation:
    description: AAAA
    lastUpdateTime: "2023-04-18T15:54:55Z"
    state: Processing
    type: Create
  machineRef:
    kind: Machine
    apiVersion: machine.sapcloud.io/v1alpha1
    namespace: d8-cloud-instance-manager
    name: worker-dde21
`
				machine1 = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: worker-dde21
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: ng1-nova
    node: worker-dde21
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
    description: 'Started Machine creation process'
    lastUpdateTime: "2020-05-15T15:01:13Z"
    state: Failed
    type: Create
`
			)

			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(ng + ic1 + machine1))
				f.RunHook()
			})

			It("Should update instance status from machine status", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Exists()).To(BeTrue())
				ic := f.KubernetesGlobalResource("Instance", "worker-dde21")
				machine := f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-dde21")

				Expect(ic.Exists()).To(BeTrue())
				Expect(machine.Exists()).To(BeTrue())

				Expect(ic.Field("status.lastOperation.lastUpdateTime").Exists()).To(BeTrue())
				Expect(machine.Field("status.lastOperation.lastUpdateTime").Exists()).To(BeTrue())
				icTime, err := time.Parse(time.RFC3339, ic.Field("status.lastOperation.lastUpdateTime").String())
				Expect(err).ToNot(HaveOccurred())
				machineTime, err := time.Parse(time.RFC3339, "2023-04-18T15:54:55Z")
				Expect(err).ToNot(HaveOccurred())
				Expect(icTime.Equal(machineTime)).To(BeTrue())

				Expect(ic.Field("status.lastOperation.description").Exists()).To(BeTrue())
				Expect(ic.Field("status.lastOperation.description").String()).To(Equal("AAAA"))

				Expect(ic.Field("status.lastOperation.state").Exists()).To(BeTrue())
				Expect(ic.Field("status.lastOperation.state").String()).To(Equal("Processing"))

				Expect(ic.Field("status.lastOperation.type").Exists()).To(BeTrue())
				Expect(ic.Field("status.lastOperation.type").String()).To(Equal("Create"))
			})
		})
	})

	Context("Deleting instances (have instances but do not have machines)", func() {
		const (
			ic1 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  name: worker-ac32h
  finalizers:
  - node-manager.hooks.deckhouse.io/instance-controller
status:
  classReference:
    kind: YandexInstanceClass
    name: worker
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

		Context("does not start deletion instance (without deletion timestamp", func() {
			const ic2 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  name: worker-bg11u
  finalizers:
  - node-manager.hooks.deckhouse.io/instance-controller
status:
  classReference:
    kind: YandexInstanceClass
    name: worker
`
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(ng + ic1 + ic2 + machine))
				f.RunHook()
			})

			It("Should keep instance with machine", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Exists()).To(BeTrue())
				Expect(f.KubernetesGlobalResource("Instance", "worker-ac32h").Exists()).To(BeTrue())
				Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-ac32h").Exists()).To(BeTrue())
				assertFinalizersExists(f, "worker-ac32h")
			})

			It("Should delete instance without machine", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(f.KubernetesGlobalResource("Instance", "worker-bg11u").Exists()).To(BeFalse())
			})
		})

		Context("start deletion instance (with deletion timestamp)", func() {
			Context("does not have machine for deleted instance", func() {
				const ic2 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  name: worker-bg11u
  finalizers:
  - node-manager.hooks.deckhouse.io/instance-controller
  deletionTimestamp: "1970-01-01T00:00:00Z"
status:
  classReference:
    kind: YandexInstanceClass
    name: worker
`
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(ng + ic1 + ic2 + machine))
					f.RunHook()
				})

				It("Should keep instance with machine", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Exists()).To(BeTrue())
					Expect(f.KubernetesGlobalResource("Instance", "worker-ac32h").Exists()).To(BeTrue())
					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-ac32h").Exists()).To(BeTrue())
					assertFinalizersExists(f, "worker-ac32h")
				})

				It("Should remove finalizers from instance without machine", func() {
					Expect(f).To(ExecuteSuccessfully())

					ic := f.KubernetesGlobalResource("Instance", "worker-bg11u")
					Expect(ic.Exists()).To(BeTrue())
					Expect(ic.Field("metadata.finalizers").Array()).To(BeEmpty())
				})
			})

			Context("have machine for deleted instance", func() {
				const (
					ic2 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  name: worker-bg11u
  finalizers:
  - node-manager.hooks.deckhouse.io/instance-controller
  deletionTimestamp: "1970-01-01T00:00:00Z"
status:
  classReference:
    kind: YandexInstanceClass
    name: worker
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

				It("Should keep another instances", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Exists()).To(BeTrue())
					Expect(f.KubernetesGlobalResource("Instance", "worker-ac32h").Exists()).To(BeTrue())
					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-ac32h").Exists()).To(BeTrue())
					assertFinalizersExists(f, "worker-ac32h")
				})

				It("Should remove machine", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-bg11u").Exists()).To(BeFalse())
				})

				It("Should keep instance with finalizers", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(f.KubernetesGlobalResource("Instance", "worker-bg11u").Exists()).To(BeTrue())
					assertFinalizersExists(f, "worker-bg11u")
				})
			})
		})
	})

	const nodeGroupWithStaticInstances = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
  uid: 87233806-25b3-41b4-8c15-46b7212326b4
spec:
  nodeType: Static
  staticInstances:
    count: 1
`

	assertCurrentStatusClusterAPI := func(f *HookExecutionConfig, instanceName string) {
		ic := f.KubernetesGlobalResource("Instance", instanceName)
		machine := f.KubernetesResource("Machine", "d8-cloud-instance-manager", instanceName)

		Expect(ic.Field("status.currentStatus.lastUpdateTime").Exists()).To(BeTrue())
		Expect(machine.Field("status.lastUpdated").Exists()).To(BeTrue())
		icTime, err := time.Parse(time.RFC3339, ic.Field("status.currentStatus.lastUpdateTime").String())
		Expect(err).ToNot(HaveOccurred())
		machineTime, err := time.Parse(time.RFC3339, machine.Field("status.lastUpdated").String())
		Expect(err).ToNot(HaveOccurred())
		Expect(icTime.Equal(machineTime)).To(BeTrue())

		Expect(ic.Field("status.currentStatus.phase").Exists()).To(BeTrue())
		Expect(machine.Field("status.phase").Exists()).To(BeTrue())
		Expect(ic.Field("status.currentStatus.phase").String()).To(Equal(machine.Field("status.phase").String()))
	}

	assertLastOperationClusterAPI := func(f *HookExecutionConfig, instanceName string) {
		ic := f.KubernetesGlobalResource("Instance", instanceName)

		Expect(ic.Field("status.lastOperation.lastUpdateTime").String()).To(Equal(""))
		Expect(ic.Field("status.lastOperation.description").String()).To(Equal(""))
		Expect(ic.Field("status.lastOperation.state").String()).To(Equal(""))
		Expect(ic.Field("status.lastOperation.type").String()).To(Equal(""))
	}

	assertMachineRefClusterAPI := func(f *HookExecutionConfig, instanceName string) {
		ic := f.KubernetesGlobalResource("Instance", instanceName)

		Expect(ic.Field("status.machineRef.kind").Exists()).To(BeTrue())
		Expect(ic.Field("status.machineRef.kind").String()).To(Equal("Machine"))

		Expect(ic.Field("status.machineRef.apiVersion").Exists()).To(BeTrue())
		Expect(ic.Field("status.machineRef.apiVersion").String()).To(Equal("cluster.x-k8s.io/v1beta1"))

		Expect(ic.Field("status.machineRef.namespace").Exists()).To(BeTrue())
		Expect(ic.Field("status.machineRef.namespace").String()).To(Equal("d8-cloud-instance-manager"))

		Expect(ic.Field("status.machineRef.name").Exists()).To(BeTrue())
		Expect(ic.Field("status.machineRef.name").String()).To(Equal(instanceName))
	}

	Context("Cluster API", func() {
		Context("Adding instances", func() {
			const (
				ic1 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  name: worker-ac32h
  finalizers:
  - node-manager.hooks.deckhouse.io/instance-controller
status: {}
`
				machine1 = `
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Machine
metadata:
  name: worker-ac32h
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng1
spec: {}
status: {}
`

				machine2 = `
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Machine
metadata:
  name: worker-fac21
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng1
spec: {}
status:
  nodeRef:
    name: worker-fac21
  phase: Pending
  lastUpdated: "2023-04-18T15:54:55Z"
`
			)

			Context("does not have instance classes but have machine", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(nodeGroupWithStaticInstances + ic1 + machine1 + machine2))
					f.RunHook()
				})

				It("Should keep 'as is' instance with machine", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Exists()).To(BeTrue())
					ic := f.KubernetesGlobalResource("Instance", "worker-ac32h")
					machine := f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-ac32h")

					Expect(ic.Exists()).To(BeTrue())
					Expect(machine.Exists()).To(BeTrue())

					Expect(ic.ToYaml()).To(MatchYAML(ic1))
					Expect(machine.ToYaml()).To(MatchYAML(machine1))
				})

				It("Should create instance for machine", func() {
					Expect(f).To(ExecuteSuccessfully())

					ic := f.KubernetesGlobalResource("Instance", "worker-fac21")
					machine := f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-fac21")

					Expect(ic.Exists()).To(BeTrue())
					Expect(machine.Exists()).To(BeTrue())

					Expect(ic.Field(`metadata.labels.node\.deckhouse\.io/group`).String()).To(Equal("ng1"))
					assertCurrentStatusClusterAPI(f, "worker-fac21")
					assertLastOperationClusterAPI(f, "worker-fac21")
					assertMachineRefClusterAPI(f, "worker-fac21")

					Expect(ic.Field("metadata.ownerReferences").IsArray()).To(BeTrue())
					Expect(ic.Field("metadata.ownerReferences").Array()).To(HaveLen(1))

					Expect(ic.Field("metadata.ownerReferences.0.kind").String()).To(Equal("NodeGroup"))
					Expect(ic.Field("metadata.ownerReferences.0.name").String()).To(Equal("ng1"))
					Expect(ic.Field("metadata.ownerReferences.0.uid").String()).To(Equal("87233806-25b3-41b4-8c15-46b7212326b4"))
				})
			})
		})

		Context("Updating instances status", func() {
			Context("Phase is Running and bootstrapStatus is not empty", func() {
				const (
					ic1 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  labels:
    node.deckhouse.io/group: "ng1"
  name: worker-dde21
  finalizers:
  - node-manager.hooks.deckhouse.io/instance-controller
status:
  currentStatus:
    lastUpdateTime: "2023-04-18T15:54:55Z"
    phase: Pending
  machineRef:
    kind: Machine
    apiVersion: cluster.x-k8s.io/v1beta1
    namespace: d8-cloud-instance-manager
    name: worker-dde21
  bootstrapStatus:
    logsEndpoint: 127.0.0.0:1111
    description: Use nc
`
					machine1 = `
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Machine
metadata:
  name: worker-dde21
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng1
spec: {}
status:
  nodeRef:
    name: worker-dde21
  lastUpdated: "2023-04-18T15:54:55Z"
  phase: Pending
`
					ic2 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  labels:
    node.deckhouse.io/group: "ng1"
  name: worker-ac32h
  finalizers:
  - node-manager.hooks.deckhouse.io/instance-controller
status:
  currentStatus:
    lastUpdateTime: "2023-04-18T15:54:55Z"
    phase: Pending
  machineRef:
    kind: Machine
    apiVersion: cluster.x-k8s.io/v1beta1
    namespace: d8-cloud-instance-manager
    name: worker-ac32h
  bootstrapStatus:
    logsEndpoint: 127.0.0.0:1111
    description: Use nc
`
					machine2 = `
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Machine
metadata:
  name: worker-ac32h
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng1
spec: {}
status:
  nodeRef:
    name: worker-ac32h
  lastUpdated: "2023-04-19T15:54:55Z"
  phase: Running
`
				)
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(nodeGroupWithStaticInstances + ic1 + machine1 + ic2 + machine2))
					f.RunHook()
				})

				It("Should keep instance bootstrapStatus for none running machines", func() {
					Expect(f).To(ExecuteSuccessfully())

					ic := f.KubernetesGlobalResource("Instance", "worker-dde21")

					Expect(ic.Exists()).To(BeTrue())
					Expect(ic.Field(`status.bootstrapStatus.logsEndpoint`).String()).To(Equal("127.0.0.0:1111"))
					Expect(ic.Field(`status.bootstrapStatus.description`).String()).To(Equal("Use nc"))
				})

				It("Should remove bootstrapStatus for running machines", func() {
					Expect(f).To(ExecuteSuccessfully())

					ic := f.KubernetesGlobalResource("Instance", "worker-ac32h")

					Expect(ic.Exists()).To(BeTrue())
					Expect(ic.Field(`status.bootstrapStatus.logsEndpoint`).String()).To(BeEmpty())
					Expect(ic.Field(`status.bootstrapStatus.description`).String()).To(BeEmpty())
				})
			})

			Context("Another status fields", func() {
				const (
					ic1 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  labels:
    node.deckhouse.io/group: "ng1"
  name: worker-dde21
  finalizers:
  - node-manager.hooks.deckhouse.io/instance-controller
status:
  currentStatus:
    lastUpdateTime: "2023-04-18T15:54:55Z"
    phase: Pending
  machineRef:
    kind: Machine
    apiVersion: cluster.x-k8s.io/v1beta1
    namespace: d8-cloud-instance-manager
    name: worker-dde21
`
					machine1 = `
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Machine
metadata:
  name: worker-dde21
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng1
spec: {}
status:
  nodeRef:
    name: worker-dde21
  lastUpdated: "2023-04-18T15:54:55Z"
  phase: Pending
`

					machine2 = `
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Machine
metadata:
  name: worker-ac32h
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng1
spec: {}
status:
  nodeRef:
    name: worker-ac32h
  lastUpdated: "2023-04-19T15:54:55Z"
  phase: Running
`
					ic2 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  labels:
    node.deckhouse.io/group: "ng1"
  name: worker-ac32h
  finalizers:
  - node-manager.hooks.deckhouse.io/instance-controller
status:
  currentStatus:
    lastUpdateTime: "2023-04-18T15:54:55Z"
    phase: Pending
  machineRef:
    kind: Machine
    apiVersion: cluster.x-k8s.io/v1beta1
    namespace: d8-cloud-instance-manager
    name: worker-ac32h
  bootstrapStatus:
    logsEndpoint: 127.0.0.0:1111
    description: Use nc
`
				)

				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(nodeGroupWithStaticInstances + ic1 + machine1 + ic2 + machine2))
					f.RunHook()
				})

				It("Should keep another instances", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Exists()).To(BeTrue())
					ic := f.KubernetesGlobalResource("Instance", "worker-dde21")
					machine := f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-dde21")

					Expect(ic.Exists()).To(BeTrue())
					Expect(machine.Exists()).To(BeTrue())

					Expect(ic.Field(`metadata.labels.node\.deckhouse\.io/group`).String()).To(Equal("ng1"))

					Expect(ic.Field(`status.nodeRef.name`).Exists()).To(BeTrue())
					Expect(ic.Field(`status.nodeRef.name`).String()).To(Equal("worker-dde21"))

					assertMachineRefClusterAPI(f, "worker-dde21")
					assertCurrentStatusClusterAPI(f, "worker-dde21")
					assertLastOperationClusterAPI(f, "worker-dde21")
				})

				It("Should update instance status from machine status", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Exists()).To(BeTrue())
					ic := f.KubernetesGlobalResource("Instance", "worker-ac32h")
					machine := f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-ac32h")

					Expect(ic.Exists()).To(BeTrue())
					Expect(machine.Exists()).To(BeTrue())

					Expect(ic.Field(`metadata.labels.node\.deckhouse\.io/group`).String()).To(Equal("ng1"))

					Expect(ic.Field(`status.nodeRef.name`).Exists()).To(BeTrue())
					Expect(ic.Field(`status.nodeRef.name`).String()).To(Equal("worker-ac32h"))

					assertMachineRefClusterAPI(f, "worker-ac32h")
					assertCurrentStatusClusterAPI(f, "worker-ac32h")
					assertLastOperationClusterAPI(f, "worker-ac32h")
				})
			})

			Context("Machine with last operation 'Started Machine creation process'", func() {
				const (
					ic1 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  labels:
    node.deckhouse.io/group: "ng1"
  name: worker-dde21
  finalizers:
  - node-manager.hooks.deckhouse.io/instance-controller
status:
  currentStatus:
    lastUpdateTime: "2023-04-18T15:54:55Z"
    phase: Pending
  machineRef:
    kind: Machine
    apiVersion: cluster.x-k8s.io/v1beta1
    namespace: d8-cloud-instance-manager
    name: worker-dde21
`
					machine1 = `
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Machine
metadata:
  name: worker-dde21
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng1
spec: {}
status:
  nodeRef:
    name: worker-dde21
  lastUpdated: "2023-04-18T15:54:55Z"
  phase: Pending
`
				)

				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(nodeGroupWithStaticInstances + ic1 + machine1))
					f.RunHook()
				})

				It("Should not update lastOperation instance status from machine status", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Exists()).To(BeTrue())
					ic := f.KubernetesGlobalResource("Instance", "worker-dde21")
					machine := f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-dde21")

					Expect(ic.Exists()).To(BeTrue())
					Expect(machine.Exists()).To(BeTrue())

					Expect(ic.Field("status.lastOperation.lastUpdateTime").Exists()).To(BeFalse())
					Expect(machine.Field("status.lastUpdated").Exists()).To(BeTrue())
				})
			})
		})

		Context("Deleting instances (have instances but do not have machines)", func() {
			const (
				ic1 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  name: worker-ac32h
  finalizers:
  - node-manager.hooks.deckhouse.io/instance-controller
`
				machine = `
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Machine
metadata:
  name: worker-ac32h
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng1
spec: {}
`
			)

			Context("does not start deletion instance (without deletion timestamp", func() {
				const ic2 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  name: worker-bg11u
  finalizers:
  - node-manager.hooks.deckhouse.io/instance-controller
`
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(nodeGroupWithStaticInstances + ic1 + ic2 + machine))
					f.RunHook()
				})

				It("Should keep instance with machine", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Exists()).To(BeTrue())
					Expect(f.KubernetesGlobalResource("Instance", "worker-ac32h").Exists()).To(BeTrue())
					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-ac32h").Exists()).To(BeTrue())
					assertFinalizersExists(f, "worker-ac32h")
				})

				It("Should delete instance without machine", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(f.KubernetesGlobalResource("Instance", "worker-bg11u").Exists()).To(BeFalse())
				})
			})

			Context("start deletion instance (with deletion timestamp)", func() {
				Context("does not have machine for deleted instance", func() {
					const ic2 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  name: worker-bg11u
  finalizers:
  - node-manager.hooks.deckhouse.io/instance-controller
  deletionTimestamp: "1970-01-01T00:00:00Z"
`
					BeforeEach(func() {
						f.BindingContexts.Set(f.KubeStateSet(nodeGroupWithStaticInstances + ic1 + ic2 + machine))
						f.RunHook()
					})

					It("Should keep instance with machine", func() {
						Expect(f).To(ExecuteSuccessfully())

						Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Exists()).To(BeTrue())
						Expect(f.KubernetesGlobalResource("Instance", "worker-ac32h").Exists()).To(BeTrue())
						Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-ac32h").Exists()).To(BeTrue())
						assertFinalizersExists(f, "worker-ac32h")
					})

					It("Should remove finalizers from instance without machine", func() {
						Expect(f).To(ExecuteSuccessfully())

						ic := f.KubernetesGlobalResource("Instance", "worker-bg11u")
						Expect(ic.Exists()).To(BeTrue())
						Expect(ic.Field("metadata.finalizers").Array()).To(BeEmpty())
					})
				})

				Context("have machine for deleted instance", func() {
					const (
						ic2 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Instance
metadata:
  name: worker-bg11u
  finalizers:
  - node-manager.hooks.deckhouse.io/instance-controller
  deletionTimestamp: "1970-01-01T00:00:00Z"
`
						machine2 = `
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Machine
metadata:
  name: worker-bg11u
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng1
spec: {}
`
					)
					BeforeEach(func() {
						f.BindingContexts.Set(f.KubeStateSet(nodeGroupWithStaticInstances + ic1 + ic2 + machine + machine2))
						f.RunHook()
					})

					It("Should keep another instances", func() {
						Expect(f).To(ExecuteSuccessfully())

						Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Exists()).To(BeTrue())
						Expect(f.KubernetesGlobalResource("Instance", "worker-ac32h").Exists()).To(BeTrue())
						Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-ac32h").Exists()).To(BeTrue())
						assertFinalizersExists(f, "worker-ac32h")
					})

					It("Should remove machine", func() {
						Expect(f).To(ExecuteSuccessfully())

						Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-bg11u").Exists()).To(BeFalse())
					})

					It("Should keep instance with finalizers", func() {
						Expect(f).To(ExecuteSuccessfully())

						Expect(f.KubernetesGlobalResource("Instance", "worker-bg11u").Exists()).To(BeTrue())
						assertFinalizersExists(f, "worker-bg11u")
					})
				})
			})
		})
	})
})

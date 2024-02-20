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
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: update_node_group_status ::", func() {
	const (
		stateCloudNG1 = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
status:
  extra: thing
`
		stateCloudNG2 = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng-2
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 3
    minPerZone: 2
    zones: [a, b, c]
status:
  error: 'Wrong classReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass.'
`
		stateNG1 = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroups
metadata:
  name: ng1
spec:
  nodeType: Static
status:
  extra: thing
`
		stateNG2 = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroups
metadata:
  name: ng-2
spec:
  nodeType: Static
status:
  error: 'Wrong classReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass.'
`
		stateMDs = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-ng1
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng1
spec:
  replicas: 2
`
		stateMachines = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: machine-ng1-aaa
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
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: machine-ng1-bbb
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
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: machine-ng1-big-bbb
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: ng1-big-nova
spec:
  nodeTemplate:
    metadata:
      labels:
        node-role.kubernetes.io/ng1-big: ""
        node.deckhouse.io/group: ng1-big
        node.deckhouse.io/type: CloudEphemeral
`

		stateNodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-aaa
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - some: thing
  - status: "False"
    type: Ready
  - some: thing
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-bbb
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - some: thing
  - status: "True"
    type: Ready
`
		stateCloudProviderSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-node-manager-cloud-provider
  namespace: kube-system
data:
  zones: WyJub3ZhIl0= # ["nova"]
`
		configurationChecksums = `
---
apiVersion: v1
kind: Secret
metadata:
  name: configuration-checksums
  namespace: d8-cloud-instance-manager
data:
  ng1: YTY2NWE0NTkyMDQyMmY5ZDQxN2U0ODY3ZWZkYzRmYjhhMDRhMWYzZmZmMWZhMDdlOTk4ZTg2ZjdmN2EyN2FlMw== # sha256sum 123
  ng-2: OGQyM2NmNmM4NmU4MzRhN2FhNmVkZWQ1NGMyNmNlMmJiMmU3NDkwMzUzOGM2MWJkZDVkMjE5Nzk5N2FiMmY3Mg== # sha256sum 321
`

		failedMachineDeployment = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-failed-ng
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng-2
spec:
  replicas: 2
status:
  failedMachines:
  - lastOperation:
      description: 'Cloud provider message - rpc error: code = FailedPrecondition
        desc = Image not found #2.'
      lastUpdateTime: "2020-05-15T15:01:15Z"
      state: Failed
      type: Create
    name: machine-ng-2-aaa
    ownerRef: korker-3e52ee98-8649499f7
  - lastOperation:
      description: 'Cloud provider message - rpc error: code = FailedPrecondition
        desc = Image not found.'
      lastUpdateTime: "2020-05-15T15:01:13Z"
      state: Failed
      type: Create
    name: machine-ng-2-bbb
    ownerRef: korker-3e52ee98-8649499f7
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: machine-ng1-aaa
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: ng-2
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: machine-ng1-bbb
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: ng-2
`

		secondFailedMachineDeployment = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-second-failed-ng
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng-2
spec:
  replicas: 2
status:
  failedMachines:
  - lastOperation:
      description: 'Cloud provider message - rpc error: code = FailedPrecondition
        desc = Image not found #3.'
      lastUpdateTime: "2020-05-15T15:05:12Z"
      state: Failed
      type: Create
    name: machine-ng-2-ccc
    ownerRef: korker-3e52ee98-8649499f7
`
	)

	f := HookExecutionConfigInit(`{"global": {"discovery": {"kubernetesVersion": "1.29.1"}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineDeployment", true)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "Machine", true)
	f.RegisterCRD("cluster.x-k8s.io", "v1beta1", "Machine", true)

	const nowTime = "2023-03-03T16:49:52Z"
	err := os.Setenv("TEST_CONDITIONS_CALC_NOW_TIME", nowTime)
	if err != nil {
		panic(err)
	}

	const checkSum = "123123123123123"
	err = os.Setenv("TEST_CONDITIONS_CALC_CHKSUM", checkSum)
	if err != nil {
		panic(err)
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

	Context("A NG1 and zones Secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateCloudNG1+stateCloudProviderSecret, 1))
			f.RunHook()
		})

		It("Min and max must be filled", func() {
			Expect(f).To(ExecuteSuccessfully())
			const expected = `{
				"extra":"thing",
				"max":5,
				"min":1,"desired":1,
				"instances":0,
				"nodes":0,
				"ready":0,
				"upToDate": 0,
				"lastMachineFailures": [],
				"conditionSummary": {"statusMessage": "", "ready": "True"},
				"conditions": [
					{
						"lastTransitionTime": "2023-03-03T16:49:52Z",
						"status": "False",
						"type": "Ready"
					},
					{
						"lastTransitionTime": "2023-03-03T16:49:52Z",
						"status": "False",
						"type": "Updating"
					},
					{
						"lastTransitionTime": "2023-03-03T16:49:52Z",
						"status": "False",
						"type": "WaitingForDisruptiveApproval"
					},
					{
						"lastTransitionTime": "2023-03-03T16:49:52Z",
						"status": "False",
						"type": "Error"
					},
					{
						"lastTransitionTime": "2023-03-03T16:49:52Z",
						"status": "True",
						"type": "Scaling"
					}
				],
				"deckhouse": {
					"processed": {
						"checkSum": "123123123123123",
						"lastTimestamp": "2023-03-03T16:49:52Z"
					},
					"synced": "False"
				}
			}`

			Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Field("status").String()).To(MatchJSON(expected))
		})
	})

	Context("NGs MD, Machines, Nodes and zones Secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateCloudNG1+stateCloudNG2+stateMDs+stateMachines+stateNodes+stateCloudProviderSecret+configurationChecksums, 2))
			f.RunHook()
		})

		It("Min, max, desired, instances, nodes, ready must be filled", func() {
			Expect(f).To(ExecuteSuccessfully())
			const expectedNG1 = `
				{
					"extra": "thing",
					"max": 5,
					"min": 1,
					"desired": 2,
					"instances": 2,
					"nodes": 2,
					"ready": 1,
					"upToDate": 2,
					"lastMachineFailures": [],
					"conditionSummary": {
						"statusMessage": "",
						"ready": "True"
					},
					"conditions": [
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "True",
							"type": "Ready"
						},
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "False",
							"type": "Updating"
						},
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "False",
							"type": "WaitingForDisruptiveApproval"
						},
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "False",
							"type": "Error"
						},
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "False",
							"type": "Scaling"
						}
					],
					"deckhouse": {
						"processed": {
							"checkSum": "123123123123123",
							"lastTimestamp": "2023-03-03T16:49:52Z"
						},
						"synced": "False"
					}
				}
			`

			const expectedNG2 = `
				{
					"max": 9,
					"min": 6,
					"desired": 6,
					"instances": 0,
					"nodes": 0,
					"ready": 0,
					"upToDate": 0,
					"lastMachineFailures": [],
					"error": "Wrong classReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass.",
					"conditionSummary": {
						"statusMessage": "Machine creation failed. Check events for details.",
						"ready": "False"
					 },
					"conditions": [
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "False",
							"type": "Ready"
						},
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "False",
							"type": "Updating"
						},
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "False",
							"type": "WaitingForDisruptiveApproval"
						},
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "True",
							"type": "Error",
							"message": "Wrong classReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass."
						},
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "True",
							"type": "Scaling"
						}
					],
					"deckhouse": {
						"processed": {
							"checkSum": "123123123123123",
							"lastTimestamp": "2023-03-03T16:49:52Z"
						},
						"synced": "False"
					}
				}
			`
			Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Field("status").String()).To(MatchJSON(expectedNG1))
			Expect(f.KubernetesGlobalResource("NodeGroup", "ng-2").Field("status").String()).To(MatchJSON(expectedNG2))
		})
	})

	Context("NGs MD, Machines, Nodes and zones Secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateNG1+stateNG2+stateMDs+stateMachines+stateNodes+stateCloudProviderSecret+configurationChecksums, 2))
			f.RunHook()
		})

		It("Nodes, ready must be filled", func() {
			Expect(f).To(ExecuteSuccessfully())
			const expectedNG1 = `
				{
					"extra": "thing",
					"nodes": 2,
					"ready": 1,
					"upToDate": 2,
					"conditionSummary": {
						"statusMessage": "",
						"ready": "True"
					},
					"conditions": [
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "True",
							"type": "Ready"
						},
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "False",
							"type": "Updating"
						},
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "False",
							"type": "WaitingForDisruptiveApproval"
						},
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "False",
							"type": "Error"
						}
					],
					"deckhouse": {
						"processed": {
							"checkSum": "123123123123123",
							"lastTimestamp": "2023-03-03T16:49:52Z"
						},
						"synced": "False"
					}
				}
			`
			const expectedNG2 = `
				{
					"nodes": 0,
					"ready": 0,
					"upToDate": 0,
					"error": "Wrong classReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass.",
					"conditionSummary": {
						"statusMessage": "Machine creation failed. Check events for details.",
						"ready": "False"
					},
					"conditions": [
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "True",
							"type": "Ready"
						},
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "False",
							"type": "Updating"
						},
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "False",
							"type": "WaitingForDisruptiveApproval"
						},
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "True",
							"type": "Error",
							"message": "Wrong classReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass."
						}
					],
					"deckhouse": {
						"processed": {
							"checkSum": "123123123123123",
							"lastTimestamp": "2023-03-03T16:49:52Z"
						},
						"synced": "False"
					}
				}
			`
			Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Field("status").String()).To(MatchJSON(expectedNG1))
			Expect(f.KubernetesGlobalResource("NodeGroup", "ng-2").Field("status").String()).To(MatchJSON(expectedNG2))
			// MachineDeployment metrics should be set
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).To(HaveLen(1))
			Expect(metrics[0].Labels).To(BeEquivalentTo(map[string]string{"name": "md-ng1", "node_group": "ng1"}))
		})
	})

	Context("One failed NG MD, Machines, Nodes and zones Secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateCloudNG2+stateNodes+stateCloudProviderSecret+configurationChecksums+failedMachineDeployment, 1))
			f.RunHook()
		})

		It("NG's status.lastMachineFailures must be filled", func() {
			Expect(f).To(ExecuteSuccessfully())
			const expected = `
				{
					"max": 9,
					"min": 6,
					"desired": 6,
					"instances": 0,
					"nodes": 0,
					"ready": 0,
					"upToDate": 0,
					"lastMachineFailures": [
						{
							"lastOperation": {
								"description": "Cloud provider message - rpc error: code = FailedPrecondition desc = Image not found.",
								"lastUpdateTime": "2020-05-15T15:01:13Z",
								"state": "Failed",
								"type": "Create"
							},
							"name": "machine-ng-2-bbb",
							"ownerRef": "korker-3e52ee98-8649499f7"
						},
						{
							"lastOperation": {
								"description": "Cloud provider message - rpc error: code = FailedPrecondition desc = Image not found #2.",
								"lastUpdateTime": "2020-05-15T15:01:15Z",
								"state": "Failed",
								"type": "Create"
							},
							"name": "machine-ng-2-aaa",
							"ownerRef": "korker-3e52ee98-8649499f7"
						}
					],
					"error": "Wrong classReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass.",
					"conditionSummary": {
						"statusMessage": "Machine creation failed. Check events for details.",
						"ready": "False"
					},
					"conditions": [
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "False",
							"type": "Ready"
						},
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "False",
							"type": "Updating"
						},
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "False",
							"type": "WaitingForDisruptiveApproval"
						},
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "True",
							"type": "Error",
							"message": "Wrong classReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass.|Cloud provider message - rpc error: code = FailedPrecondition desc = Image not found #2."
						},
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "True",
							"type": "Scaling"
						}
						],
						"deckhouse": {
							"processed": {
								"checkSum": "123123123123123",
								"lastTimestamp": "2023-03-03T16:49:52Z"
							},
							"synced": "False"
						}
					}
				`
			Expect(f.KubernetesGlobalResource("NodeGroup", "ng-2").Field("status").String()).To(MatchJSON(expected))
		})
	})

	Context("One failed NG from two failed MDs, Machines, Nodes and zones Secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateCloudNG2+stateNodes+stateCloudProviderSecret+configurationChecksums+failedMachineDeployment+secondFailedMachineDeployment, 1))
			f.RunHook()
		})

		It("NG's status.lastMachineFailures must be filled", func() {
			Expect(f).To(ExecuteSuccessfully())
			const expected = `
				{
					"max": 9,
					"min": 6,
					"desired": 6,
					"instances": 0,
					"nodes": 0,
					"ready": 0,
					"upToDate": 0,
					"lastMachineFailures": [
						{
							"lastOperation": {
								"description": "Cloud provider message - rpc error: code = FailedPrecondition desc = Image not found.",
								"lastUpdateTime": "2020-05-15T15:01:13Z",
								"state": "Failed",
								"type": "Create"
							},
							"name": "machine-ng-2-bbb",
							"ownerRef": "korker-3e52ee98-8649499f7"
						},
						{
							"lastOperation": {
								"description": "Cloud provider message - rpc error: code = FailedPrecondition desc = Image not found #2.",
								"lastUpdateTime": "2020-05-15T15:01:15Z",
								"state": "Failed",
								"type": "Create"
							  },
							"name": "machine-ng-2-aaa",
							"ownerRef": "korker-3e52ee98-8649499f7"
						},
						{
							"lastOperation": {
								"description": "Cloud provider message - rpc error: code = FailedPrecondition desc = Image not found #3.",
								"lastUpdateTime": "2020-05-15T15:05:12Z",
								"state": "Failed",
								"type": "Create"
							},
							"name": "machine-ng-2-ccc",
							"ownerRef": "korker-3e52ee98-8649499f7"
						}
						],
						"error": "Wrong classReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass.",
						"conditionSummary": {
							"statusMessage": "Machine creation failed. Check events for details.",
							"ready": "False"
						},
						"conditions": [
							{
								"lastTransitionTime": "2023-03-03T16:49:52Z",
								"status": "False",
								"type": "Ready"
							},
							{
								"lastTransitionTime": "2023-03-03T16:49:52Z",
								"status": "False",
								"type": "Updating"
							},
							{
								"lastTransitionTime": "2023-03-03T16:49:52Z",
								"status": "False",
								"type": "WaitingForDisruptiveApproval"
						},
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "True",
							"type": "Error",
							"message": "Wrong classReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass.|Cloud provider message - rpc error: code = FailedPrecondition desc = Image not found #3."
						},
						{
							"lastTransitionTime": "2023-03-03T16:49:52Z",
							"status": "True",
							"type": "Scaling"
						}
					],
					"deckhouse": {
						"processed": {
							"checkSum": "123123123123123",
							"lastTimestamp": "2023-03-03T16:49:52Z"
						},
						"synced": "False"
					}
				}
			`
			Expect(f.KubernetesGlobalResource("NodeGroup", "ng-2").Field("status").String()).To(MatchJSON(expected))
		})
	})

	Context("Node group conditions", func() {
		assertCondition := func(f *HookExecutionConfig, t ngv1.NodeGroupConditionType, s ngv1.ConditionStatus, tt, m string) {
			conditions := f.KubernetesGlobalResource("NodeGroup", "ng1").Field("status.conditions").Array()
			hasCondition := false
			for _, c := range conditions {
				if c.Get("type").String() == string(t) {
					hasCondition = true
					Expect(c.Get("status").String()).To(Equal(string(s)))
					Expect(c.Get("message").String()).To(Equal(m))

					toExpectTime, err := time.Parse(time.RFC3339, c.Get("lastTransitionTime").String())
					Expect(err).ToNot(HaveOccurred())
					expectedTime, err := time.Parse(time.RFC3339, tt)
					Expect(err).ToNot(HaveOccurred())

					Expect(toExpectTime.Equal(expectedTime)).To(BeTrue())

					break
				}
			}

			Expect(hasCondition).To(BeTrue())
		}

		const (
			cloudNG1 = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
status:
  extra: thing
`
			machineDeploy = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-ng1
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng1
spec:
  replicas: 2
`
			machines = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: machine-ng1-aaa
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
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: machine-ng1-bbb
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
		Context("Ready condition", func() {
			assertReadyCondition := func(f *HookExecutionConfig, s ngv1.ConditionStatus) {
				assertCondition(f, ngv1.NodeGroupConditionTypeReady, s, nowTime, "")
			}

			Context("Have not nodes", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						cloudNG1+machineDeploy+machines+stateCloudProviderSecret+configurationChecksums, 2))
					f.RunHook()
				})

				It("Sets to False", func() {
					assertReadyCondition(f, ngv1.ConditionFalse)
				})
			})

			Context("All nodes in NG unschedulable but ready", func() {
				BeforeEach(func() {
					const nodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-aaa
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
spec:
  unschedulable: true
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-bbb
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
spec:
  unschedulable: true
status:
  conditions:
  - status: "True"
    type: Ready
`
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						cloudNG1+machineDeploy+machines+stateCloudProviderSecret+configurationChecksums+nodes, 2))
					f.RunHook()
				})

				It("Sets to False", func() {
					assertReadyCondition(f, ngv1.ConditionFalse)
				})
			})

			Context("One of two node is NotReady, but unready node lifetime great than 5min", func() {
				BeforeEach(func() {
					const nodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-aaa
  creationTimestamp: 2023-03-03T16:43:52Z
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "False"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-bbb
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "True"
    type: Ready
`
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						cloudNG1+machineDeploy+machines+stateCloudProviderSecret+configurationChecksums+nodes, 2))
					f.RunHook()
				})
				It("Sets to True", func() {
					assertReadyCondition(f, ngv1.ConditionTrue)
				})
			})

			Context("One of two node is NotReady, but unready node lifetime less 5min", func() {
				BeforeEach(func() {
					const nodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-aaa
  creationTimestamp: 2023-03-03T16:47:52Z
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "False"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-bbb
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "True"
    type: Ready
`
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						cloudNG1+machineDeploy+machines+stateCloudProviderSecret+configurationChecksums+nodes, 2))
					f.RunHook()
				})

				It("Sets to True", func() {
					assertReadyCondition(f, ngv1.ConditionTrue)
				})
			})

			Context("All nodes Schedulable and Ready", func() {
				BeforeEach(func() {
					const nodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-aaa
  creationTimestamp: 2023-03-03T16:47:52Z
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-bbb
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "True"
    type: Ready
`
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						cloudNG1+machineDeploy+machines+stateCloudProviderSecret+configurationChecksums+nodes, 2))
					f.RunHook()
				})

				It("Sets to True", func() {
					assertReadyCondition(f, ngv1.ConditionTrue)
				})
			})

			Context("Current condition is False but all nodes become Ready and Schedulable", func() {
				const (
					cloudWithCondition = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
status:
  conditions:
  - status: "False"
    type: Ready
    lastTransitionTime: "2023-03-03T16:47:40Z"
`
					nodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-aaa
  creationTimestamp: 2023-03-03T16:47:52Z
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-bbb
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "True"
    type: Ready
`
				)
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						cloudWithCondition+machineDeploy+machines+stateCloudProviderSecret+configurationChecksums+nodes, 2))
					f.RunHook()
				})
				It("Switch from False to True and sets new transition time", func() {
					assertCondition(f, ngv1.NodeGroupConditionTypeReady, ngv1.ConditionTrue, nowTime, "")
				})
			})

			Context("Current condition is True but some nodes (< 90%) become NotReady and Schedulable", func() {
				const (
					cloudWithCondition = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
status:
  conditions:
  - status: "True"
    type: Ready
    lastTransitionTime: "2023-03-03T16:47:40Z"
`
					nodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-aaa
  creationTimestamp: 2023-03-03T16:40:52Z
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "False"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-bbb
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
spec:
  unschedulable: true
status:
  conditions:
  - status: "True"
    type: Ready
`
				)
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						cloudWithCondition+machineDeploy+machines+stateCloudProviderSecret+configurationChecksums+nodes, 2))
					f.RunHook()
				})

				It("Switch from True to False and sets new transition time", func() {
					assertCondition(f, ngv1.NodeGroupConditionTypeReady, ngv1.ConditionFalse, nowTime, "")
				})
			})
		})

		Context("Updating condition", func() {
			const cloudNG1 = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
status:
  extra: thing
`

			annotations := []string{
				"update.node.deckhouse.io/approved",
				"update.node.deckhouse.io/waiting-for-approval",
				"update.node.deckhouse.io/disruption-required",
				drainingAnnotationKey,
				"update.node.deckhouse.io/disruption-approved",
				drainedAnnotationKey,
			}

			for _, a := range annotations {
				nodesTemp := func(annot string) string {
					return fmt.Sprintf(`
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-aaa
  creationTimestamp: 2023-03-03T16:47:52Z
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    %s: ""
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "False"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-bbb
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "True"
    type: Ready
`, annot)
				}

				Context(fmt.Sprintf("One node has '%s' annotation", a), func() {
					BeforeEach(func() {
						f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
							cloudNG1+machineDeploy+machines+stateCloudProviderSecret+configurationChecksums+nodesTemp(a), 2))
						f.RunHook()
					})
					It("Sets to True", func() {
						assertCondition(f, ngv1.NodeGroupConditionTypeUpdating, ngv1.ConditionTrue, nowTime, "")
					})
				})
			}

			Context("Nodes don't have update.node.deckhouse.io/* annotation", func() {
				const nodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-aaa
  creationTimestamp: 2023-03-03T16:47:52Z
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-bbb
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "True"
    type: Ready
`
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						cloudNG1+machineDeploy+machines+stateCloudProviderSecret+configurationChecksums+nodes, 2))
					f.RunHook()
				})

				It("Sets to False", func() {
					assertCondition(f, ngv1.NodeGroupConditionTypeUpdating, ngv1.ConditionFalse, nowTime, "")
				})
			})

			Context("Current status is True, nodes have not update.node.deckhouse.io/*", func() {
				const cloudWithCondition = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
status:
  conditions:
  - status: "True"
    type: Updating
    lastTransitionTime: "2023-03-03T16:47:40Z"
`
				const nodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-aaa
  creationTimestamp: 2023-03-03T16:47:52Z
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-bbb
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "True"
    type: Ready
`
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						cloudWithCondition+machineDeploy+machines+stateCloudProviderSecret+configurationChecksums+nodes, 2))
					f.RunHook()
				})

				It("Sets to True", func() {
					assertCondition(f, ngv1.NodeGroupConditionTypeUpdating, ngv1.ConditionFalse, nowTime, "")
				})
			})
		})

		Context("WaitingForDisruptiveApproval condition", func() {
			const cloudNG1 = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
`

			Context("Nodes have 'update.node.deckhouse.io/disruption-required' annotation", func() {
				const nodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-aaa
  creationTimestamp: 2023-03-03T16:47:52Z
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-bbb
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    update.node.deckhouse.io/disruption-required: ""
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "True"
    type: Ready
`
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						cloudNG1+machineDeploy+machines+stateCloudProviderSecret+configurationChecksums+nodes, 2))
					f.RunHook()
				})

				It("Sets to True", func() {
					assertCondition(f, ngv1.NodeGroupConditionTypeWaitingForDisruptiveApproval, ngv1.ConditionTrue, nowTime, "")
				})

				It("Sets Updating to True", func() {
					assertCondition(f, ngv1.NodeGroupConditionTypeUpdating, ngv1.ConditionTrue, nowTime, "")
				})
			})

			Context("Nodes have 'update.node.deckhouse.io/disruption-required' and 'update.node.deckhouse.io/disruption-approved' annotations both", func() {
				const nodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-aaa
  creationTimestamp: 2023-03-03T16:47:52Z
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-bbb
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    update.node.deckhouse.io/disruption-required: ""
    update.node.deckhouse.io/disruption-approved: ""
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "True"
    type: Ready
`
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						cloudNG1+machineDeploy+machines+stateCloudProviderSecret+configurationChecksums+nodes, 2))
					f.RunHook()
				})

				It("Sets to False", func() {
					assertCondition(f, ngv1.NodeGroupConditionTypeWaitingForDisruptiveApproval, ngv1.ConditionFalse, nowTime, "")
				})

				It("Sets Updating to True", func() {
					assertCondition(f, ngv1.NodeGroupConditionTypeUpdating, ngv1.ConditionTrue, nowTime, "")
				})
			})

			Context("Current status is True, nodes have not update.node.deckhouse.io/disruption-required annotation", func() {
				const cloudWithCondition = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
status:
  conditions:
  - status: "True"
    type: WaitingForDisruptiveApproval
    lastTransitionTime: "2023-03-03T16:47:40Z"
`
				const nodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-aaa
  creationTimestamp: 2023-03-03T16:47:52Z
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-bbb
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "True"
    type: Ready
`
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						cloudWithCondition+machineDeploy+machines+stateCloudProviderSecret+configurationChecksums+nodes, 2))
					f.RunHook()
				})

				It("Sets to True", func() {
					assertCondition(f, ngv1.NodeGroupConditionTypeWaitingForDisruptiveApproval, ngv1.ConditionFalse, nowTime, "")
				})

				It("Sets Updating to True", func() {
					assertCondition(f, ngv1.NodeGroupConditionTypeUpdating, ngv1.ConditionFalse, nowTime, "")
				})
			})
		})

		Context("Error condition", func() {
			const machines = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: machine-ng1-aaa
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: ng1
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: machine-ng1-bbb
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: ng1
`
			const ngWithError = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
status:
  error: "Node group error"
`
			Context("Node group status has 'error' field", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						ngWithError+machineDeploy+machines+stateCloudProviderSecret+configurationChecksums, 2))
					f.RunHook()
				})

				It("Sets to True and sets message", func() {
					assertCondition(f, ngv1.NodeGroupConditionTypeError, ngv1.ConditionTrue, nowTime, "Node group error")
				})
			})

			Context("Node group status has 'error' field and has machine deployment error", func() {
				const machineDeploymentErr = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-failed-ng
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng1
spec:
  replicas: 2
status:
  failedMachines:
  - lastOperation:
      description: 'Cloud provider message - rpc error: code = FailedPrecondition
        desc = Image not found #2.'
      lastUpdateTime: "2020-05-15T15:01:15Z"
      state: Failed
      type: Create
    name: machine-ng-2-aaa
    ownerRef: korker-3e52ee98-8649499f7
  - lastOperation:
      description: 'Cloud provider message - rpc error: code = FailedPrecondition
        desc = Image not found.'
      lastUpdateTime: "2020-05-15T15:01:13Z"
      state: Failed
      type: Create
    name: machine-ng-2-bbb
    ownerRef: korker-3e52ee98-8649499f7
`
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						ngWithError+machineDeploymentErr+machines+stateCloudProviderSecret+configurationChecksums, 2))
					f.RunHook()
				})

				It("Sets to True and sets message from ng error and machine deployment error", func() {
					assertCondition(f, ngv1.NodeGroupConditionTypeError, ngv1.ConditionTrue, nowTime, "Node group error|Cloud provider message - rpc error: code = FailedPrecondition desc = Image not found #2.")
				})
			})

			Context("Machine deployment has `Started Machine creation process` error but ng is not in error condition", func() {
				const machineDeploymentErr = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
status:
  conditions:
  - status: "False"
    type: Error
    lastTransitionTime: "2023-03-03T16:47:40Z"
    message: ""
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-failed-ng
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng1
spec:
  replicas: 2
status:
  failedMachines:
  - lastOperation:
      description: 'Started Machine creation process'
      lastUpdateTime: "2020-05-15T15:01:13Z"
      state: Failed
      type: Create
    name: machine-ng-2-bbb
    ownerRef: korker-3e52ee98-8649499f7

`
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						machineDeploymentErr+machines+stateCloudProviderSecret+configurationChecksums, 2))
					f.RunHook()
				})

				It("Sets to True and sets message from machine deployment error", func() {
					assertCondition(f, ngv1.NodeGroupConditionTypeError, ngv1.ConditionTrue, nowTime, "Started Machine creation process")
				})
			})

			Context("Machine deployment has `Started Machine creation process` error and ng is in error condition", func() {
				const machineDeploymentErr = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
status:
  conditions:
  - status: "True"
    type: Error
    lastTransitionTime: "2023-03-03T19:47:40+03:00"
    message: "Some error"
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-failed-ng
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng1
spec:
  replicas: 2
status:
  failedMachines:
  - lastOperation:
      description: 'Started Machine creation process'
      lastUpdateTime: "2020-05-15T15:01:13Z"
      state: Failed
      type: Create
    name: machine-ng-2-bbb
    ownerRef: korker-3e52ee98-8649499f7

`
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						machineDeploymentErr+machines+stateCloudProviderSecret+configurationChecksums, 2))
					f.RunHook()
				})

				It("Sets to True and sets message from machine deployment error", func() {
					assertCondition(f, ngv1.NodeGroupConditionTypeError, ngv1.ConditionTrue, "2023-03-03T19:47:40+03:00", "Some error")
				})
			})

			Context("Machine deployment is in the frozen state ng is in error condition", func() {
				const machineDeploymentErr = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
status:
  conditions:
  - status: "True"
    type: Error
    lastTransitionTime: "2023-03-03T19:47:40Z"
    message: "Some error"
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-failed-ng
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng1
spec:
  replicas: 2
status:
  conditions:
  - lastTransitionTime: "2023-04-06T15:15:07Z"
    lastUpdateTime: "2023-04-06T15:15:07Z"
    message: Deployment has minimum availability.
    reason: MinimumReplicasAvailable
    status: "True"
    type: Available
  - lastTransitionTime: "2023-04-11T12:03:02Z"
    lastUpdateTime: "2023-04-11T12:03:02Z"
    message: 'The number of machines backing MachineSet: sandbox-stage-8ef4a622-5f76f
      is 4 >= 4 which is the Max-ScaleUp-Limit'
    reason: OverShootingReplicaCount
    status: "True"
    type: Frozen
`
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						machineDeploymentErr+machines+stateCloudProviderSecret+configurationChecksums, 2))
					f.RunHook()
				})

				It("Sets to True and sets message from machine deployment error", func() {
					assertCondition(f, ngv1.NodeGroupConditionTypeError, ngv1.ConditionTrue, "2023-03-03T19:47:40Z", "Some error")
				})
			})

			Context("Machine deployment unfroze and has not error, ng is in error condition", func() {
				const machineDeploymentErr = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
status:
  conditions:
  - status: "True"
    type: Error
    lastTransitionTime: "2023-03-03T19:47:40+03:00"
    message: "Some error"
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-failed-ng
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng1
spec:
  replicas: 2
status:
  conditions:
  - lastTransitionTime: "2023-04-06T15:15:07Z"
    lastUpdateTime: "2023-04-06T15:15:07Z"
    message: Deployment has minimum availability.
    reason: MinimumReplicasAvailable
    status: "True"
    type: Available
`
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						machineDeploymentErr+machines+stateCloudProviderSecret+configurationChecksums, 2))
					f.RunHook()
				})

				It("Sets to False and clear error message", func() {
					assertCondition(f, ngv1.NodeGroupConditionTypeError, ngv1.ConditionFalse, nowTime, "")
				})
			})

			Context("Node group has error condition, but ng and machine deployment not contain errors", func() {
				const ngWithCondition = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
status:
  conditions:
  - status: "True"
    type: Error
    lastTransitionTime: "2023-03-03T16:47:40Z"
    message: "Some error"
`
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						ngWithCondition+machineDeploy+machines+stateCloudProviderSecret+configurationChecksums, 2))
					f.RunHook()
				})

				It("Sets to False and clears message", func() {
					assertCondition(f, ngv1.NodeGroupConditionTypeError, ngv1.ConditionFalse, nowTime, "")
				})
			})
		})

		Context("Scaling condition", func() {
			const ng = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
`
			Context("Cluster autoscaler set taint to delete node (scaling down)", func() {
				const nodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-aaa
  creationTimestamp: 2023-03-03T16:47:52Z
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-bbb
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
spec:
  taints:
    - effect: NoSchedule
      key: ToBeDeletedByClusterAutoscaler
status:
  conditions:
  - status: "True"
    type: Ready
`
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						ng+machineDeploy+machines+stateCloudProviderSecret+configurationChecksums+nodes, 2))
					f.RunHook()
				})

				It("Sets to True", func() {
					assertCondition(f, ngv1.NodeGroupConditionTypeScaling, ngv1.ConditionTrue, nowTime, "")
				})
			})

			const nodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-aaa
  creationTimestamp: 2023-03-03T16:47:52Z
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node-ng1-bbb
  labels:
    node.deckhouse.io/group: ng1
  annotations:
    node.deckhouse.io/configuration-checksum: a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3
status:
  conditions:
  - status: "True"
    type: Ready
`
			Context("Increase machine deployment replicas (scaling up)", func() {
				const machineDeployIncreased = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-ng1
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng1
spec:
  replicas: 3
`
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						ng+machineDeployIncreased+machines+stateCloudProviderSecret+configurationChecksums+nodes, 2))
					f.RunHook()
				})

				It("Sets to True and sets message", func() {
					assertCondition(f, ngv1.NodeGroupConditionTypeScaling, ngv1.ConditionTrue, nowTime, "")
				})
			})

			Context("Decrease machine deployment replicas (manual scaling down)", func() {
				const machineDeployIncreased = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-ng1
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng1
spec:
  replicas: 1
`
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						ng+machineDeployIncreased+machines+stateCloudProviderSecret+configurationChecksums+nodes, 2))
					f.RunHook()
				})

				It("Sets to True and sets message", func() {
					assertCondition(f, ngv1.NodeGroupConditionTypeScaling, ngv1.ConditionTrue, nowTime, "")
				})
			})

			Context("Node group has scaling condition, but all scaling precesses were done", func() {
				const ngWithCondition = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
status:
  conditions:
  - status: "True"
    type: Scaling
    lastTransitionTime: "2023-03-03T16:47:40Z"
`
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						ngWithCondition+machineDeploy+machines+stateCloudProviderSecret+configurationChecksums+nodes, 2))
					f.RunHook()
				})

				It("Sets to False", func() {
					assertCondition(f, ngv1.NodeGroupConditionTypeScaling, ngv1.ConditionFalse, nowTime, "")
				})
			})
			assertNoScalingCondition := func(f *HookExecutionConfig) {
				conditions := f.KubernetesGlobalResource("NodeGroup", "ng1").Field("status.conditions").Array()
				hasCondition := false
				for _, c := range conditions {
					if c.Get("type").String() == ngv1.NodeGroupConditionTypeScaling {
						hasCondition = true
						break
					}
				}

				Expect(hasCondition).To(BeFalse())
			}

			Context("Should not contains for Static ng", func() {
				const ng = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroups
metadata:
  name: ng1
spec:
  nodeType: Static
`
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						ng+stateCloudProviderSecret+configurationChecksums+nodes, 2))
					f.RunHook()
				})

				It("Sets to False", func() {
					assertNoScalingCondition(f)
				})
			})

			Context("Should not contains for CloudPermanent ng", func() {
				const ng = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroups
metadata:
  name: ng1
spec:
  nodeType: CloudPermanent
`
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						ng+stateCloudProviderSecret+configurationChecksums+nodes, 2))
					f.RunHook()
				})

				It("Sets to False", func() {
					assertNoScalingCondition(f)
				})
			})
		})
	})
})

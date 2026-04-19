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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: discover_standby_ng ::", func() {
	const (
		nodeGroupWithoutStandby = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: normal
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    minPerZone: 1
    maxPerZone: 5
status: {}
`
		nodeGroupWithZeroStandbyAsString = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: normal
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    standby: "0"
    minPerZone: 1
    maxPerZone: 5
status: {}
`
		nodeGroupWithZeroStandbyAsInt = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: normal
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    standby: 0
    minPerZone: 1
    maxPerZone: 5
status: {}
`
		nodeGroupStandbyAbsolute = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: standby-absolute
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 10
    minPerZone: 1
    zones:
      - zone1
      - zone2
    standby: 5
  nodeTemplate:
    taints:
    - effect: NoExecute
      key: ship-class
      value: frigate
status: {}
`
		nodeGroupStandbyAbsoluteOverprovisioningRate = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: standby-absolute
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 10
    minPerZone: 1
    zones:
      - zone1
      - zone2
    standby: 5
    standbyHolder:
      overprovisioningRate: 80
  nodeTemplate:
    taints:
    - effect: NoExecute
      key: ship-class
      value: frigate
status: {}
`

		nodeGroupStandbyAbsoluteTooBigStandby = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: standby-absolute
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 10
    minPerZone: 1
    zones:
      - zone1
      - zone2
    standby: 30
  nodeTemplate:
    taints:
    - effect: NoExecute
      key: ship-class
      value: frigate
status: {}
`
		nodeGroupStandbyPercent = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: standby-percent
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 20
    minPerZone: 1
    zones:
      - zone1
      - zone2
      - zone3
    standby: 20%
  nodeTemplate:
    taints:
    - effect: NoExecute
      key: ship-class
      value: frigate
status: {}
`
		nodeGroupStandbyMinEqMax = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: standby-absolute-min-eq-max
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 5
    zones:
      - zone1
      - zone2
    standby: 5
  nodeTemplate:
    taints:
    - effect: NoExecute
      key: ship-class
      value: frigate
status: {}
`
		nodeStandby4Cpu = `
---
apiVersion: v1
kind: Node
metadata:
  name: standby-holder-0
  labels:
    node.deckhouse.io/group: %s
status:
  allocatable:
    cpu: 4
    memory: 2063326004
  conditions:
  - status: "True"
    type: Ready
`
		nodeStandby6Cpu = `
---
apiVersion: v1
kind: Node
metadata:
  name: standby-holder-1
  labels:
    node.deckhouse.io/group: %s
status:
  allocatable:
    cpu: 6
    memory: 4126652008
  conditions:
  - status: "True"
    type: Ready
`

		nodesWithTimestamp = `
---
apiVersion: v1
kind: Node
metadata:
  creationTimestamp: "2021-01-01T06:02:26Z"
  name: standby-node-1
  labels:
    node.deckhouse.io/group: %[1]s
status:
  allocatable:
    cpu: 4
    memory: 8264994695
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  creationTimestamp: "2022-02-02T06:02:26Z"
  name: standby-node-2
  labels:
    node.deckhouse.io/group: %[1]s
status:
  allocatable:
    cpu: 2
    memory: 4126652008
  conditions:
  - status: "True"
    type: Ready
`

		podStandby0 = `
---
apiVersion: v1
kind: Pod
metadata:
  name: standby-ab7s2-d0s1a2-0
  namespace: d8-cloud-instance-manager
  labels:
    app: standby-holder
    ng: %s
status:
  allocatable:
    cpu: 6
    memory: 4126652008
  conditions:
  - status: "True"
    type: Ready
`
		podStandby1 = `
---
apiVersion: v1
kind: Pod
metadata:
  name: standby-ab7s2-d0s1a2-1
  namespace: d8-cloud-instance-manager
  labels:
    app: standby-holder
    ng: %s
status:
  allocatable:
    cpu: 6
    memory: 4126652008
  conditions:
  - status: "True"
    type: Ready
`

		nodeGroupWithoutZones = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: worker
    maxPerZone: 8
    minPerZone: 2
    standby: "3"
  nodeType: CloudEphemeral
`

		nodeWorkerTemplate = `
---
apiVersion: v1
kind: Node
metadata:
  name: worker-%d
  labels:
    node.deckhouse.io/group: worker
status:
  allocatable:
    cpu: 4
    memory: 2063326004
  conditions:
  - status: "True"
    type: Ready
`
	)

	f := HookExecutionConfigInit(`
{
	"global": {
		"discovery": {
			"kubernetesVersion": "1.16.15",
			"kubernetesVersions": [
				"1.16.15"
			],
			"clusterUUID": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		},
	},
	"nodeManager": {
		"internal": {}
	}
}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail; no standby NGs should be discovered", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups").Array()).To(BeEmpty())
		})
	})

	Context("Cluster with NG without standby", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupWithoutStandby))
			f.RunHook()
		})

		It("Hook must not fail; no standby NGs should be discovered", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups").Array()).To(BeEmpty())
		})
	})

	Context("Cluster with NG with standby as int but min == max", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupStandbyMinEqMax))
			f.RunHook()
		})

		It("Hook must not fail; no standby NGs should be discovered", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups").Array()).To(BeEmpty())
		})
	})

	Context("Cluster with NG with zero standby as string", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupWithZeroStandbyAsString))
			f.RunHook()
		})

		It("Hook must not fail; no standby NGs should be discovered", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups").Array()).To(BeEmpty())
		})
	})

	Context("Cluster with NG with zero standby as int", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupWithZeroStandbyAsInt))
			f.RunHook()
		})

		It("Hook must not fail; no standby NGs should be discovered", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups").Array()).To(BeEmpty())
		})
	})

	Context("Cluster with standby NG defined by absolute number and no nodes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupStandbyAbsolute))
			f.RunHook()
		})

		It("Hook must not fail; standby NG should be discovered", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups.0").String()).To(MatchJSON(`{"name":"standby-absolute","standby":5,"reserveCPU":"2","reserveMemory": "4096Mi","taints":[{"key":"ship-class","value":"frigate","effect":"NoExecute"}]}`))
		})
	})

	Context("Cluster with standby NG defined by absolute number and nodes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupStandbyAbsolute +
				fmt.Sprintf(nodeStandby4Cpu, "standby-absolute") +
				fmt.Sprintf(nodeStandby6Cpu, "standby-absolute")))
			f.RunHook()
		})

		It("Hook must not fail; standby NG should be discovered", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups.0").String()).To(MatchJSON(`{"name": "standby-absolute", "standby": 5, "reserveCPU": "3","reserveMemory": "1967Mi","taints":[{"effect":"NoExecute","key":"ship-class","value":"frigate"}]}`))
		})
	})

	Context("Cluster with standby NG defined by too big absolute number and nodes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupStandbyAbsoluteTooBigStandby +
				fmt.Sprintf(nodeStandby4Cpu, "standby-absolute") +
				fmt.Sprintf(nodeStandby6Cpu, "standby-absolute")))
			f.RunHook()
		})

		It("Hook must not fail; standby NG should be discovered", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups.0").String()).To(MatchJSON(`{"name": "standby-absolute", "standby": 18, "reserveCPU": "3","reserveMemory": "1967Mi","taints":[{"effect":"NoExecute","key":"ship-class","value":"frigate"}]}`))
		})
	})

	Context("Cluster with standby NG defined by percent and nodes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupStandbyPercent +
				fmt.Sprintf(nodeStandby6Cpu, "standby-percent")))
			f.RunHook()
		})

		It("Hook must not fail; standby NG should be discovered", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups.0").String()).To(MatchJSON(`{"name": "standby-percent", "standby": 12, "reserveCPU": "3","reserveMemory": "1967Mi","taints":[{"effect":"NoExecute","key":"ship-class","value":"frigate"}]}`))
		})
	})

	Context("Cluster with standby NGs defined both by percent and absolute value and nodes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupStandbyAbsolute + nodeGroupStandbyPercent +
				fmt.Sprintf(nodeStandby6Cpu, "standby-absolute") +
				fmt.Sprintf(nodeStandby4Cpu, "standby-percent")))
			f.RunHook()
		})

		It("Hook must not fail; standby NGs should be discovered", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups.0").String()).To(MatchJSON(`{"name":"standby-absolute","standby":5,"reserveCPU":"3","reserveMemory": "1967Mi","taints":[{"effect":"NoExecute","key":"ship-class","value":"frigate"}]}`))
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups.1").String()).To(MatchJSON(`{"name":"standby-percent","standby":12,"reserveCPU":"2","reserveMemory": "983Mi","taints":[{"effect":"NoExecute","key":"ship-class","value":"frigate"}]}`))
		})
	})

	Context("Cluster with standby NGs defined both by percent and absolute value and nodes and pod", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupStandbyAbsolute + nodeGroupStandbyPercent +
				fmt.Sprintf(nodeStandby6Cpu, "standby-absolute") +
				fmt.Sprintf(nodeStandby4Cpu, "standby-percent") +
				fmt.Sprintf(podStandby0, "standby-absolute") +
				fmt.Sprintf(podStandby1, "standby-absolute")))
			f.RunHook()
		})

		It("Hook must not fail; standby NGs should be discovered; status standby should be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups.0").String()).To(MatchJSON(`{"name":"standby-absolute","standby":5,"reserveCPU":"3","reserveMemory": "1967Mi","taints":[{"effect":"NoExecute","key":"ship-class","value":"frigate"}]}`))
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups.1").String()).To(MatchJSON(`{"name":"standby-percent","standby":12,"reserveCPU":"2","reserveMemory": "983Mi","taints":[{"effect":"NoExecute","key":"ship-class","value":"frigate"}]}`))

			Expect(f.KubernetesGlobalResource("NodeGroup", "standby-absolute").Field("status").String()).To(MatchJSON(`{"standby":2}`))
			Expect(f.KubernetesGlobalResource("NodeGroup", "standby-percent").Field("status").String()).To(MatchJSON(`{"standby":0}`))
		})
	})

	Context("Cluster with standby NGs defined by absolute value", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupStandbyAbsolute +
				fmt.Sprintf(nodeStandby6Cpu, "standby-absolute") +
				fmt.Sprintf(podStandby0, "standby-absolute")))
			f.RunHook()
		})

		It("Hook must not fail; standby NGs should be discovered; status standby should be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups.0").String()).To(MatchJSON(`{"name":"standby-absolute","standby":5,"reserveCPU":"3","reserveMemory": "1967Mi","taints": [{"key": "ship-class","value": "frigate","effect": "NoExecute"}]}`))

			Expect(f.KubernetesGlobalResource("NodeGroup", "standby-absolute").Field("status").String()).To(MatchJSON(`{"standby":1}`))
		})
	})

	Context("Cluster with standby NGs defined by absolute value, having overprovisioning rate and nodes and pod", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupStandbyAbsoluteOverprovisioningRate +
				fmt.Sprintf(nodeStandby6Cpu, "standby-absolute") +
				fmt.Sprintf(podStandby0, "standby-absolute")))
			f.RunHook()
		})

		It("Hook must not fail; standby NGs should be discovered; status standby should be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups.0").String()).To(MatchJSON(`{"name":"standby-absolute","standby":5,"reserveCPU":"4800m","reserveMemory": "3148Mi","taints": [{"key": "ship-class","value": "frigate","effect": "NoExecute"}]}`))

			Expect(f.KubernetesGlobalResource("NodeGroup", "standby-absolute").Field("status").String()).To(MatchJSON(`{"standby":1}`))
		})
	})

	Context("Cluster with standby NGs and two different nodes, simulates instance class recreation", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupStandbyAbsolute +
				fmt.Sprintf(nodesWithTimestamp, "standby-absolute") +
				fmt.Sprintf(podStandby0, "standby-absolute")))
			f.RunHook()
		})

		It("Hook must not fail; overprovisioning resources should be discovered from the latest node", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups.0").String()).To(MatchJSON(`{"name":"standby-absolute","standby":5,"reserveCPU":"1","reserveMemory": "1967Mi","taints": [{"key": "ship-class","value": "frigate","effect": "NoExecute"}]}`))

			Expect(f.KubernetesGlobalResource("NodeGroup", "standby-absolute").Field("status").String()).To(MatchJSON(`{"standby":1}`))
		})
	})

	Context("Cluster containing NG without zones defined, but having internal cloudProvider zones defined ", func() {
		BeforeEach(func() {
			state := nodeGroupWithoutZones
			for i := 1; i <= 12; i++ {
				state += fmt.Sprintf(nodeWorkerTemplate, i)
			}
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.ValuesSet("nodeManager.internal.cloudProvider.zones", []string{"zoneA", "zoneB", "zoneC"})
			f.RunHook()
		})

		It("Hook must not fail; standby NGs should be discovered", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups.0").String()).To(MatchJSON(`{"name":"worker","standby":3,"reserveCPU":"2","reserveMemory": "983Mi","taints":[]}`))
		})
	})
})

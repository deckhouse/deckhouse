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
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: normal
spec:
  nodeType: Cloud
status: {}
`
		nodeGroupStandbyAbsolute = `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: standby-absolute
spec:
  nodeType: Cloud
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
		nodeGroupStandbyAbsoluteTooBigStandby = `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: standby-absolute
spec:
  nodeType: Cloud
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
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: standby-percent
spec:
  nodeType: Cloud
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
		nodeGroupStandbyAbsoluteNotHeldResources = `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: standby-absolute
spec:
  nodeType: Cloud
  cloudInstances:
    maxPerZone: 10
    minPerZone: 1
    zones:
      - zone1
    standby: 2
    standbyHolder:
      notHeldResources:
        cpu: 600m
        memory: 750Mi
status: {}
`
	)

	f := HookExecutionConfigInit(`
{
	"global": {
        "modules": {
            "resourcesRequests": {
			    "everyNode": {
				    "cpu": "300m",
				    "memory": "512Mi"
			    }
		    }
        },
		"discovery": {
			"kubernetesVersion": "1.16.15",
			"kubernetesVersions": [
				"1.16.15"
			]
		},
		"clusterUUID": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	},
	"nodeManager": {
		"internal": {}
	}
}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "NodeGroup", false)

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

	Context("Cluster with standby NG defined by absolute number and no nodes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupStandbyAbsolute))
			f.RunHook()
		})

		It("Hook must not fail; standby NG should be discovered", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups.0").String()).To(MatchJSON(`{"name":"standby-absolute","standby":5,"reserveCPU":"10m","reserveMemory": "10Mi","taints":[{"effect":"NoExecute","key":"ship-class","value":"frigate"}]}`))
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
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups.0").String()).To(MatchJSON(`{"name": "standby-absolute", "standby": 5, "reserveCPU": "3700m","reserveMemory": "1455Mi","taints":[{"effect":"NoExecute","key":"ship-class","value":"frigate"}]}`))
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
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups.0").String()).To(MatchJSON(`{"name": "standby-absolute", "standby": 18, "reserveCPU": "3700m","reserveMemory": "1455Mi","taints":[{"effect":"NoExecute","key":"ship-class","value":"frigate"}]}`))
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
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups.0").String()).To(MatchJSON(`{"name": "standby-percent", "standby": 12, "reserveCPU": "5700m","reserveMemory": "3423Mi","taints":[{"effect":"NoExecute","key":"ship-class","value":"frigate"}]}`))
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
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups.0").String()).To(MatchJSON(`{"name":"standby-absolute","standby":5,"reserveCPU":"5700m","reserveMemory": "3423Mi","taints":[{"effect":"NoExecute","key":"ship-class","value":"frigate"}]}`))
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups.1").String()).To(MatchJSON(`{"name":"standby-percent","standby":12,"reserveCPU":"3700m","reserveMemory": "1455Mi","taints":[{"effect":"NoExecute","key":"ship-class","value":"frigate"}]}`))
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
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups.0").String()).To(MatchJSON(`{"name":"standby-absolute","standby":5,"reserveCPU":"5700m","reserveMemory": "3423Mi","taints":[{"effect":"NoExecute","key":"ship-class","value":"frigate"}]}`))
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups.1").String()).To(MatchJSON(`{"name":"standby-percent","standby":12,"reserveCPU":"3700m","reserveMemory": "1455Mi","taints":[{"effect":"NoExecute","key":"ship-class","value":"frigate"}]}`))

			Expect(f.KubernetesGlobalResource("NodeGroup", "standby-absolute").Field("status").String()).To(MatchJSON(`{"standby":2}`))
			Expect(f.KubernetesGlobalResource("NodeGroup", "standby-percent").Field("status").String()).To(MatchJSON(`{"standby":0}`))
		})
	})

	Context("Cluster with standby NGs defined by absolute value, having not held resources parameter and nodes and pod", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupStandbyAbsoluteNotHeldResources +
				fmt.Sprintf(nodeStandby6Cpu, "standby-absolute") +
				fmt.Sprintf(podStandby0, "standby-absolute")))
			f.RunHook()
		})

		It("Hook must not fail; standby NGs should be discovered; status standby should be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.standbyNodeGroups.0").String()).To(MatchJSON(`{"name":"standby-absolute","standby":2,"reserveCPU":"5100m","reserveMemory": "2673Mi","taints":[]}`))

			Expect(f.KubernetesGlobalResource("NodeGroup", "standby-absolute").Field("status").String()).To(MatchJSON(`{"standby":1}`))
		})
	})

})

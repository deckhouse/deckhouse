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
	"maps"
	"slices"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// get_crds builds the nodeManager.internal.nodeGroups blob: name, nodeType, engine, gpu,
// fencing and the three read cloudInstances fields, with zones defaulted for CloudEphemeral.
// Everything else about a NodeGroup is owned by node-controller and is not tested here.
var _ = Describe("Modules :: node-manager :: hooks :: get_crds ::", func() {
	const (
		stateNGProper = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: proper1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper1
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: proper2
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper2
    zones: [a,b]
`
		stateNGStaticAndCloudPermanent = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: static1
spec:
  nodeType: Static
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: cp1
spec:
  nodeType: CloudPermanent
`
		stateCloudProviderSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-node-manager-cloud-provider
  namespace: kube-system
data:
  zones: WyJhIiwiYiIsImMiXQ== # ["a","b","c"]
`
		machineDeployments = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  annotations:
    zone: a
  labels:
    heritage: deckhouse
  name: proper1-aaa
  namespace: d8-cloud-instance-manager
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  annotations:
    zone: b
  labels:
    heritage: deckhouse
  name: proper2-bbb
  namespace: d8-cloud-instance-manager
`
	)

	f := HookExecutionConfigInit(`{"global":{"discovery":{"kubernetesVersion": "1.32.5", "kubernetesVersions":["1.32.5"], "clusterUUID":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"},},"nodeManager":{"internal": {"static": {"internalNetworkCIDRs":["172.18.200.0/24"]}}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineDeployment", true)

	Context("Cluster with NGs, MDs and provider secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + machineDeployments + stateCloudProviderSecret))
			f.RunHook()
		})

		It("Hook must not fail; zones must be defaulted from provider secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			// proper1 has no zones -> defaulted to all known zones.
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.cloudInstances.zones").String()).To(MatchJSON(`["a","b","c"]`))
			// proper2 has explicit zones -> kept as is.
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.1.cloudInstances.zones").String()).To(MatchJSON(`["a","b"]`))
		})
	})

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with NG only, no provider", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper))
			f.RunHook()
		})

		It("Blob must be a thin passthrough with name, engine and zones", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectedJSON := `
				[
				  {
				    "nodeType": "CloudEphemeral",
				    "cloudInstances": {
				      "zones": []
				    },
				    "engine": "None",
				    "name": "proper1"
				  },
				  {
				    "nodeType": "CloudEphemeral",
				    "cloudInstances": {
				      "zones": ["a","b"]
				    },
				    "engine": "None",
				    "name": "proper2"
				  }
				]
`
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))
		})
	})

	Context("Static and CloudPermanent NodeGroups", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGStaticAndCloudPermanent))
			f.RunHook()
		})

		It("Must publish name, nodeType and engine and no static overlay", func() {
			Expect(f).To(ExecuteSuccessfully())

			// NodeGroups are listed sorted by name: cp1 (index 0), static1 (index 1).
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.name").String()).To(Equal("cp1"))
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.nodeType").String()).To(Equal("CloudPermanent"))
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.engine").String()).To(Equal("None"))
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.static").Exists()).To(BeFalse())

			// nodeManager.internal.static is set in the initial values, the Static NG must not
			// pick it up.
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.1.name").String()).To(Equal("static1"))
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.1.nodeType").String()).To(Equal("Static"))
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.1.engine").String()).To(Equal("None"))
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.1.static").Exists()).To(BeFalse())
		})
	})

	Context("Engine defaulting from cloud provider", func() {
		BeforeEach(func() {
			f.ValuesSet("nodeManager.internal.cloudProvider.machineClassKind", "AWSInstanceClass")
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper))
			f.RunHook()
		})

		AfterEach(func() {
			f.ValuesDelete("nodeManager.internal.cloudProvider")
		})

		It("CloudEphemeral NGs must get MCM engine", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.engine").String()).To(Equal("MCM"))
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.1.engine").String()).To(Equal("MCM"))
		})
	})

	Context("Static instances", func() {
		const staticNodeGroupWithStaticInstances = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  staticInstances:
    labelSelector:
      matchLabels:
        node-group: worker
`
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(staticNodeGroupWithStaticInstances))
			f.RunHook()
		})

		It("engine must be CAPI and staticInstances must not be published", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.engine").String()).To(Equal("CAPI"))
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.staticInstances").Exists()).To(BeFalse())
		})
	})

	Context("Static instances with fencing", func() {
		const staticNodeGroupWithFencing = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  staticInstances:
    labelSelector:
      matchLabels:
        node-group: worker
  fencing:
    mode: Watchdog
`
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(staticNodeGroupWithFencing))
			f.RunHook()
		})

		It("Fencing values must be passed through", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.fencing.mode").Value()).To(Equal("Watchdog"))
		})
	})

	Context("NG referencing an unknown instance class kind", func() {
		BeforeEach(func() {
			ng := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: improper
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: ImproperInstanceClass
      name: improper
`
			f.BindingContexts.Set(f.KubeStateSet(ng))
			f.RunHook()
		})

		It("must reach helm values instead of being replaced by a last known good value", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.name").String()).To(Equal("improper"))
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.nodeType").String()).To(Equal("CloudEphemeral"))
		})
	})

	Context("Published key set", func() {
		const ngWithEverySpecField = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: everything
  annotations:
    manual-rollout-id: "1"
spec:
  nodeType: CloudEphemeral
  systemType: Immutable
  nodeDrainTimeoutSecond: 300
  cri:
    type: Containerd
  gpu:
    sharing: mig
    mig:
      partedConfig: all-1g.5gb
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: everything
    minPerZone: 1
    maxPerZone: 3
    maxUnavailablePerZone: 1
    maxSurgePerZone: 1
    quickShutdown: true
    priority: 5
    zones: [a]
  nodeTemplate:
    labels:
      node-role.deckhouse.io/system: ""
  chaos:
    mode: DrainAndDelete
  operatingSystem:
    manageKernel: false
  disruptions:
    approvalMode: Manual
  kubelet:
    maxPods: 100
  fencing:
    mode: Watchdog
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: staticng
spec:
  nodeType: Static
  staticInstances:
    count: 1
    labelSelector:
      matchLabels:
        node-group: staticng
`
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(ngWithEverySpecField))
			f.RunHook()
		})

		// Guard against the blob growing back: the published element carries exactly the
		// keys its helm-template and Go consumers read.
		It("CloudEphemeral element must publish exactly the keys the consumers read", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(slices.Collect(maps.Keys(f.ValuesGet("nodeManager.internal.nodeGroups.0").Map()))).To(
				ConsistOf("name", "nodeType", "engine", "gpu", "cloudInstances", "fencing"))
			Expect(slices.Collect(maps.Keys(f.ValuesGet("nodeManager.internal.nodeGroups.0.cloudInstances").Map()))).To(
				ConsistOf("minPerZone", "maxPerZone", "zones"))
		})

		It("Static element must publish exactly the keys the consumers read", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(slices.Collect(maps.Keys(f.ValuesGet("nodeManager.internal.nodeGroups.1").Map()))).To(
				ConsistOf("name", "nodeType", "engine"))
		})
	})
})

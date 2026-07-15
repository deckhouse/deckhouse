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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// get_crds builds the thin nodeManager.internal.nodeGroups blob: a passthrough of the
// NodeGroup spec enriched with name, engine and defaulted cloudInstances.zones. Validation,
// status, instanceClass overlay, capacity, CRI/kubernetesVersion resolution, updateEpoch and
// serialized labels/taints are owned by node-controller and are intentionally NOT tested here.
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
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper1"
				      },
				      "zones": []
				    },
				    "kubelet": {
				      "containerLogMaxSize": "50Mi",
				      "containerLogMaxFiles": 4,
				      "resourceReservation": {
				        "mode": "Auto"
				      },
				      "topologyManager": {}
				    },
				    "engine": "None",
				    "name": "proper1"
				  },
				  {
				    "nodeType": "CloudEphemeral",
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper2"
				      },
				      "zones": ["a","b"]
				    },
				    "kubelet": {
				      "containerLogMaxSize": "50Mi",
				      "containerLogMaxFiles": 4,
				      "resourceReservation": {
				        "mode": "Auto"
				      },
				      "topologyManager": {}
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

		It("Must pass through and overlay static settings for Static nodeType", func() {
			Expect(f).To(ExecuteSuccessfully())

			// Static NG carries the internal static overlay and engine None (no staticInstances).
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.name").String()).To(Equal("static1"))
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.nodeType").String()).To(Equal("Static"))
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.engine").String()).To(Equal("None"))
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.static.internalNetworkCIDRs").String()).To(MatchJSON(`["172.18.200.0/24"]`))

			// CloudPermanent NG is a plain passthrough without zones or static overlay.
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.1.name").String()).To(Equal("cp1"))
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.1.nodeType").String()).To(Equal("CloudPermanent"))
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

		It("staticInstances must be passed through and engine must be CAPI", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.staticInstances").Exists()).To(BeTrue())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.engine").String()).To(Equal("CAPI"))
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

	Context("NodeTemplate labels passthrough", func() {
		BeforeEach(func() {
			ng := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: CloudEphemeral
  nodeTemplate:
    labels:
      node-role.deckhouse.io/system: ""
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper1
    zones: [a,b]
`
			f.BindingContexts.Set(f.KubeStateSet(ng))
			f.RunHook()
		})

		It("nodeTemplate labels must be passed through unchanged", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.nodeTemplate.labels").String()).To(MatchJSON(`{"node-role.deckhouse.io/system": ""}`))
		})
	})

	Context("NG referencing an unknown instance class", func() {
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

		It("Hook must not validate; NG passes through (validation owned by node-controller)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.name").String()).To(Equal("improper"))
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.cloudInstances.classReference.kind").String()).To(Equal("ImproperInstanceClass"))
		})
	})
})

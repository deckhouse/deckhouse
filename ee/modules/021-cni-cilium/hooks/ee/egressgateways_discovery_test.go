/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("cni-cilium :: hooks :: egress_discovery ::", func() {
	f := HookExecutionConfigInit(`{"cniCilium":{"internal": {"egressGatewaysMap": {}}}}`, "")
	f.RegisterCRD("network.deckhouse.io", "v1alpha1", "EgressGateway", false)
	f.RegisterCRD("internal.network.deckhouse.io", "v1alpha1", "SDNInternalEgressGatewayInstance", false)

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})
	})

	Context("Adding labels to single node", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: egg-dev
spec:
  nodeSelector:
    node-role: egress
  sourceIP:
    mode: VirtualIPAddress
    virtualIPAddress:
      ip: 10.2.2.8
      routingTableName: external
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-1
  labels:
    node-role: egress
spec:
  podCIDR: 10.111.1.0/24
  podCIDRs:
  - 10.111.1.0/24
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-2
  labels:
   node-role: ingress
spec:
  podCIDR: 10.111.2.0/24
  podCIDRs:
  - 10.111.2.0/24
`))
			f.RunHook()
		})

		It("Node should be labeled", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.KubernetesGlobalResource("Node", "frontend-1").Field("metadata.labels").String()).To(MatchJSON(`
{
"egress-gateway.network.deckhouse.io/member": "",
"node-role": "egress"}
`))

			Expect(f.KubernetesGlobalResource("Node", "frontend-2").Field("metadata.labels").String()).To(MatchJSON(`
{
"node-role": "ingress"}
`))
		})
	})

	Context("Adding labels to single node and making active node", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: egg-dev
spec:
  nodeSelector:
    node-role: egress
  sourceIP:
    mode: VirtualIPAddress
    virtualIPAddress:
      ip: 10.2.2.8
      routingTableName: external
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-1
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    node-role: egress
spec:
  podCIDR: 10.111.1.0/24
  podCIDRs:
  - 10.111.1.0/24
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-2
  labels:
   node-role: ingress
spec:
  podCIDR: 10.111.2.0/24
  podCIDRs:
  - 10.111.2.0/24
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-3
  labels:
    node-role: egress
spec:
  podCIDR: 10.111.3.0/24
  podCIDRs:
  - 10.111.3.0/24
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-5clwf
  namespace: d8-cni-cilium
  labels:
    app: agent
    module: cni-cilium
spec:
  nodeName: frontend-1
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: egress-agent-5clwf
  namespace: d8-cni-cilium
  labels:
    app: egress-gateway-agent
spec:
  nodeName: frontend-1
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
`))
			f.RunHook()
		})

		It("Nodes should be labeled as active node and member", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.KubernetesGlobalResource("Node", "frontend-1").Field("metadata.labels").String()).To(MatchJSON(`
{"egress-gateway.network.deckhouse.io/active-for-egg-dev": "",
"egress-gateway.network.deckhouse.io/member": "",
"node-role": "egress"}
`))
			Expect(f.KubernetesGlobalResource("Node", "frontend-3").Field("metadata.labels").String()).To(MatchJSON(`
{"egress-gateway.network.deckhouse.io/member": "",
"node-role": "egress"}
`))
		})
	})

	Context("Remove labels from active node after cordon", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: egg-dev
spec:
  nodeSelector:
    node-role: egress
  sourceIP:
    mode: VirtualIPAddress
    virtualIPAddress:
      ip: 10.2.2.8
      routingTableName: external
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-1
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    egress-gateway.network.deckhouse.io/active-for-egg-dev: ""
    node-role: egress
spec:
  unschedulable: true
  podCIDR: 10.111.1.0/24
  podCIDRs:
  - 10.111.1.0/24
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-2
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    node-role: egress
spec:
  podCIDR: 10.111.2.0/24
  podCIDRs:
  - 10.111.2.0/24
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-3
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    node-role: egress
spec:
  podCIDR: 10.111.3.0/24
  podCIDRs:
  - 10.111.3.0/24
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-5clwf
  namespace: d8-cni-cilium
  labels:
    app: agent
    module: cni-cilium
spec:
  nodeName: frontend-1
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-6clwf
  namespace: d8-cni-cilium
  labels:
    app: agent
    module: cni-cilium
spec:
  nodeName: frontend-2
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-7clwf
  namespace: d8-cni-cilium
  labels:
    app: agent
    module: cni-cilium
spec:
  nodeName: frontend-3
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: egress-agent-5clwf
  namespace: d8-cni-cilium
  labels:
    app: egress-gateway-agent
spec:
  nodeName: frontend-1
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: egress-agent-6clwf
  namespace: d8-cni-cilium
  labels:
    app: egress-gateway-agent
spec:
  nodeName: frontend-2
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: egress-agent-7clwf
  namespace: d8-cni-cilium
  labels:
    app: egress-gateway-agent
spec:
  nodeName: frontend-3
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
`))
			f.RunHook()
		})

		It("Node frontend-3 should be labeled as active node, active node label - removed active node and member labels", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.KubernetesGlobalResource("Node", "frontend-1").Field("metadata.labels").String()).To(MatchJSON(`
{"node-role": "egress"}
`))

			Expect(f.KubernetesGlobalResource("Node", "frontend-3").Field("metadata.labels").String()).To(MatchJSON(`
{
"egress-gateway.network.deckhouse.io/active-for-egg-dev": "",
"egress-gateway.network.deckhouse.io/member": "",
"node-role": "egress"}
`))
		})
	})

	Context("Remove labels from active node while pod is NotReady (case 1)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: egg-dev
spec:
  nodeSelector:
    node-role: egress
  sourceIP:
    mode: VirtualIPAddress
    virtualIPAddress:
      ip: 10.2.2.8
      routingTableName: external
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-1
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    egress-gateway.network.deckhouse.io/active-for-egg-dev: ""
    node-role: egress
spec:
  podCIDR: 10.111.1.0/24
  podCIDRs:
  - 10.111.1.0/24
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-2
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    node-role: egress
spec:
  podCIDR: 10.111.2.0/24
  podCIDRs:
  - 10.111.2.0/24
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-3
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    node-role: egress
spec:
  podCIDR: 10.111.3.0/24
  podCIDRs:
  - 10.111.3.0/24
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-5clwf
  namespace: d8-cni-cilium
  labels:
    app: agent
    module: cni-cilium
spec:
  nodeName: frontend-1
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "False"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-6clwf
  namespace: d8-cni-cilium
  labels:
    app: agent
    module: cni-cilium
spec:
  nodeName: frontend-2
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-7clwf
  namespace: d8-cni-cilium
  labels:
    app: agent
    module: cni-cilium
spec:
  nodeName: frontend-3
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: egress-agent-6clwf
  namespace: d8-cni-cilium
  labels:
    app: egress-gateway-agent
spec:
  nodeName: frontend-2
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: egress-agent-7clwf
  namespace: d8-cni-cilium
  labels:
    app: egress-gateway-agent
spec:
  nodeName: frontend-3
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
`))
			f.RunHook()
		})

		It("Node frontend-3 should be labeled as active node, active node label - removed active node and member labels", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.KubernetesGlobalResource("Node", "frontend-1").Field("metadata.labels").String()).To(MatchJSON(`
{
"egress-gateway.network.deckhouse.io/member": "",
"node-role": "egress"}
`))

			Expect(f.KubernetesGlobalResource("Node", "frontend-3").Field("metadata.labels").String()).To(MatchJSON(`
{
"egress-gateway.network.deckhouse.io/active-for-egg-dev": "",
"egress-gateway.network.deckhouse.io/member": "",
"node-role": "egress"}
`))
		})
	})

	Context("Remove labels from active node while pod is NotReady (case 2)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: egg-dev
spec:
  nodeSelector:
    node-role: egress
  sourceIP:
    mode: VirtualIPAddress
    virtualIPAddress:
      ip: 10.2.2.8
      routingTableName: external
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-1
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    egress-gateway.network.deckhouse.io/active-for-egg-dev: ""
    node-role: egress
spec:
  podCIDR: 10.111.1.0/24
  podCIDRs:
  - 10.111.1.0/24
status:
  conditions:
  - reason: KubeletNotReady
    status: "False"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-2
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    node-role: egress
spec:
  podCIDR: 10.111.2.0/24
  podCIDRs:
  - 10.111.2.0/24
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-3
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    node-role: egress
spec:
  unschedulable: true
  podCIDR: 10.111.3.0/24
  podCIDRs:
  - 10.111.3.0/24
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-5clwf
  namespace: d8-cni-cilium
  labels:
    app: agent
    module: cni-cilium
spec:
  nodeName: frontend-1
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-6clwf
  namespace: d8-cni-cilium
  labels:
    app: agent
    module: cni-cilium
spec:
  nodeName: frontend-2
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "False"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-7clwf
  namespace: d8-cni-cilium
  labels:
    app: agent
    module: cni-cilium
spec:
  nodeName: frontend-3
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
`))
			f.RunHook()
		})

		It("Remove labels from all nodes except frontend-2 (only member label)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.KubernetesGlobalResource("Node", "frontend-1").Field("metadata.labels").String()).To(MatchJSON(`
{
"node-role": "egress"}
`))
			Expect(f.KubernetesGlobalResource("Node", "frontend-2").Field("metadata.labels").String()).To(MatchJSON(`
{
"egress-gateway.network.deckhouse.io/member": "",
"node-role": "egress"}
`))
			Expect(f.KubernetesGlobalResource("Node", "frontend-3").Field("metadata.labels").String()).To(MatchJSON(`
{
"node-role": "egress"}
`))
		})
	})

	Context("Remove labels from active node while pod is NotReady (case 3)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: egg-dev
spec:
  nodeSelector:
    node-role: egress
  sourceIP:
    mode: VirtualIPAddress
    virtualIPAddress:
      ip: 10.2.2.8
      routingTableName: external
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-1
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    egress-gateway.network.deckhouse.io/active-for-egg-dev: ""
    node-role: egress
spec:
  podCIDR: 10.111.1.0/24
  podCIDRs:
  - 10.111.1.0/24
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-2
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    node-role: egress
spec:
  podCIDR: 10.111.2.0/24
  podCIDRs:
  - 10.111.2.0/24
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-3
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    node-role: egress
spec:
  podCIDR: 10.111.3.0/24
  podCIDRs:
  - 10.111.3.0/24
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-5clwf
  namespace: d8-cni-cilium
  labels:
    app: agent
    module: cni-cilium
spec:
  nodeName: frontend-1
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-6clwf
  namespace: d8-cni-cilium
  labels:
    app: agent
    module: cni-cilium
spec:
  nodeName: frontend-2
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "False"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-7clwf
  namespace: d8-cni-cilium
  labels:
    app: agent
    module: cni-cilium
spec:
  nodeName: frontend-3
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-egress-5clwf
  namespace: d8-cni-cilium
  labels:
    app: egress-gateway-agent
spec:
  nodeName: frontend-1
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-egress-6clwf
  namespace: d8-cni-cilium
  labels:
    app: egress-gateway-agent
spec:
  nodeName: frontend-2
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "False"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-egress-7clwf
  namespace: d8-cni-cilium
  labels:
    app: egress-gateway-agent
spec:
  nodeName: frontend-3
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
`))
			f.RunHook()
		})

		It("There are two ready egress agent pods", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.KubernetesGlobalResource("EgressGateway", "egg-dev").Field("status.readyNodes").Int()).To(Equal(int64(2)))
			Expect(f.KubernetesGlobalResource("EgressGateway", "egg-dev").Field("status.activeNodeName").String()).To(Equal("frontend-3"))
		})
	})

	Context("Remove labels from active node while pod is NotReady (case 4)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: egg-dev
spec:
  nodeSelector:
    node-role: egress
  sourceIP:
    mode: VirtualIPAddress
    virtualIPAddress:
      ip: 10.2.2.8
      routingTableName: external
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-1
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    egress-gateway.network.deckhouse.io/active-for-egg-dev: ""
    node-role: egress
spec:
  podCIDR: 10.111.1.0/24
  podCIDRs:
  - 10.111.1.0/24
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-2
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    node-role: egress
spec:
  podCIDR: 10.111.2.0/24
  podCIDRs:
  - 10.111.2.0/24
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-3
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    node-role: egress
spec:
  podCIDR: 10.111.3.0/24
  podCIDRs:
  - 10.111.3.0/24
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-5clwf
  namespace: d8-cni-cilium
  labels:
    app: agent
    module: cni-cilium
spec:
  nodeName: frontend-1
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-6clwf
  namespace: d8-cni-cilium
  labels:
    app: agent
    module: cni-cilium
spec:
  nodeName: frontend-2
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "False"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-7clwf
  namespace: d8-cni-cilium
  labels:
    app: agent
    module: cni-cilium
spec:
  nodeName: frontend-3
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-egress-5clwf
  namespace: d8-cni-cilium
  labels:
    app: egress-gateway-agent
spec:
  nodeName: frontend-1
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "False"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-egress-6clwf
  namespace: d8-cni-cilium
  labels:
    app: egress-gateway-agent
spec:
  nodeName: frontend-2
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "False"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-egress-7clwf
  namespace: d8-cni-cilium
  labels:
    app: egress-gateway-agent
spec:
  nodeName: frontend-3
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "False"
    type: Ready
`))
			f.RunHook()
		})

		It("There are no ready egress agent pods", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.KubernetesGlobalResource("EgressGateway", "egg-dev").Field("status.readyNodes").Int()).To(BeZero())
			Expect(f.KubernetesGlobalResource("EgressGateway", "egg-dev").Field("status.activeNodeName").String()).To(BeEmpty())
		})
	})

	Context("Sync status with egress gateway instances", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: egg-dev
  uid: bc8a4fc5-94cb-46c5-98cc-03a051c36d0b
spec:
  nodeSelector:
    node-role: egress
  sourceIP:
    mode: VirtualIPAddress
    virtualIPAddress:
      ip: 10.2.2.8
      routingTableName: external
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-1
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    egress-gateway.network.deckhouse.io/active-for-egg-dev: ""
    node-role: egress
spec:
  podCIDR: 10.111.1.0/24
  podCIDRs:
  - 10.111.1.0/24
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-2
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    node-role: egress
spec:
  podCIDR: 10.111.2.0/24
  podCIDRs:
  - 10.111.2.0/24
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-5clwf
  namespace: d8-cni-cilium
  labels:
    app: agent
    module: cni-cilium
spec:
  nodeName: frontend-1
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Pod
metadata:
  name: egress-agent-5clwf
  namespace: d8-cni-cilium
  labels:
    app: egress-gateway-agent
spec:
  nodeName: frontend-1
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
---
apiVersion: internal.network.deckhouse.io/v1alpha1
kind: SDNInternalEgressGatewayInstance
metadata:
  annotations:
    meta.helm.sh/release-name: cni-cilium
    meta.helm.sh/release-namespace: d8-system
  creationTimestamp: "2024-05-07T11:10:59Z"
  finalizers:
  - egress-gateway.network.deckhouse.io
  generation: 1
  labels:
    app.kubernetes.io/managed-by: Helm
    egress-gateway.network.deckhouse.io/node-name: frontend-1
    heritage: deckhouse
    module: cni-cilium
  name: egg-dev-11bedb80fb
  ownerReferences:
  - apiVersion: network.deckhouse.io/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: EgressGateway
    name: egg-dev
    uid: bc8a4fc5-94cb-46c5-98cc-03a051c36d0b
  resourceVersion: "32731249"
  uid: 89fc9368-4f7f-4f78-abe9-f24c30298ced
spec:
  nodeName: frontend-1
  sourceIP:
    mode: VirtualIPAddress
    virtualIPAddress:
      ip: 10.2.2.8
      routingTableName: external
status:
  conditions:
  - lastHeartbeatTime: "2024-05-07T11:34:17Z"
    lastTransitionTime: "2024-05-07T11:10:59Z"
    message: Announcing 1 Virtual IPs
    reason: AnnouncingSucceed
    status: "True"
    type: Ready
  observedGeneration: 1
`))
			f.RunHook()
		})

		It("EG status should be synced with EGI", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			status := f.KubernetesGlobalResource("EgressGateway", "egg-dev").Field("status").Array()
			Expect(len(status)).Should(BeNumerically(">", 0))
			Expect(status[0].String()).To(ContainSubstring("ElectionSucceedAndVirtualIPAnnounced"))
		})
	})

	Context("Sync status with egress gateway instances (case 2)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: egg-dev
  uid: bc8a4fc5-94cb-46c5-98cc-03a051c36d0b
spec:
  nodeSelector:
    node-role: egress
  sourceIP:
    mode: VirtualIPAddress
    virtualIPAddress:
      ip: 10.2.2.8
      routingTableName: external
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-1
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    egress-gateway.network.deckhouse.io/active-for-egg-dev: ""
    node-role: egress
spec:
  podCIDR: 10.111.1.0/24
  podCIDRs:
  - 10.111.1.0/24
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-2
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    node-role: egress
spec:
  podCIDR: 10.111.2.0/24
  podCIDRs:
  - 10.111.2.0/24
---
apiVersion: v1
kind: Pod
metadata:
  name: agent-5clwf
  namespace: d8-cni-cilium
  labels:
    app: agent
    module: cni-cilium
spec:
  nodeName: frontend-1
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-12T11:57:00Z"
    status: "True"
    type: Ready
---
apiVersion: internal.network.deckhouse.io/v1alpha1
kind: SDNInternalEgressGatewayInstance
metadata:
  annotations:
    meta.helm.sh/release-name: cni-cilium
    meta.helm.sh/release-namespace: d8-system
  creationTimestamp: "2024-05-07T11:10:59Z"
  finalizers:
  - egress-gateway.network.deckhouse.io
  generation: 1
  labels:
    app.kubernetes.io/managed-by: Helm
    egress-gateway.network.deckhouse.io/node-name: frontend-1
    heritage: deckhouse
    module: cni-cilium
  name: egg-dev-11bedb80fb
  ownerReferences:
  - apiVersion: network.deckhouse.io/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: EgressGateway
    name: egg-dev
    uid: bc8a4fc5-94cb-46c5-98cc-03a051c36d0b
  resourceVersion: "32731249"
  uid: 89fc9368-4f7f-4f78-abe9-f24c30298ced
spec:
  nodeName: frontend-1
  sourceIP:
    mode: VirtualIPAddress
    virtualIPAddress:
      ip: 10.2.2.8
      routingTableName: external
status:
  conditions:
  - lastHeartbeatTime: "2024-05-07T11:34:17Z"
    lastTransitionTime: "2024-05-07T11:10:59Z"
    message: Announcing Virtual IPs failed
    reason: AnnouncingFailed
    status: "False"
    type: Ready
  observedGeneration: 1
`))
			f.RunHook()
		})

		It("EG status should be synced with EGI", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			status := f.KubernetesGlobalResource("EgressGateway", "egg-dev").Field("status").Array()
			Expect(len(status)).Should(BeNumerically(">", 0))
			Expect(status[0].String()).To(ContainSubstring("VirtualIPAnnouncingFailed"))
		})
	})
})

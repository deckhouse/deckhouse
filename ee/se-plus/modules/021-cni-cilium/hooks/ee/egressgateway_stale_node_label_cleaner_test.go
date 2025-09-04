/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var eg = `
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
`

var nodeWithNodeRole = `
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
`

var nodeWithoutNodeRoleButWithLabels = `
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-2
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    egress-gateway.network.deckhouse.io/active-for-egg-dev: ""
spec:
  podCIDR: 10.111.2.0/24
  podCIDRs:
    - 10.111.2.0/24
`

var nodeWithoutNodeRoleWithoutLabels = `
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-3
spec:
  podCIDR: 10.111.3.0/24
  podCIDRs:
    - 10.111.3.0/24
`

var nodeWithMultipleActiveLabels = `
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-4
  labels:
    egress-gateway.network.deckhouse.io/member: ""
    egress-gateway.network.deckhouse.io/active-for-egg-dev: ""
    egress-gateway.network.deckhouse.io/active-for-egg-prod: ""
spec:
  podCIDR: 10.111.4.0/24
  podCIDRs:
    - 10.111.4.0/24
`

var _ = Describe("Modules :: cni-cilium :: hooks :: egress_label_cleaner", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)
	f.RegisterCRD("network.deckhouse.io", "v1alpha1", "EgressGateway", false)
	f.RegisterCRD("cilium.io", "v2", "CiliumNode", false)

	Context("Node with correct NodeSelector and labels", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(eg + nodeWithNodeRole))
			f.RunGoHook()
		})

		It("should keep egress labels", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Node", "frontend-1").Field("metadata.labels").Map()).To(HaveKey("egress-gateway.network.deckhouse.io/member"))
			Expect(f.KubernetesGlobalResource("Node", "frontend-1").Field("metadata.labels").Map()).To(HaveKey("egress-gateway.network.deckhouse.io/active-for-egg-dev"))
			// nodeObj := f.KubernetesResource("Node", "", "frontend-1")
			// fmt.Printf("%+v\n", nodeObj.ToYaml())
		})
	})

	Context("Node without NodeSelector but with egress labels", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(eg + nodeWithoutNodeRoleButWithLabels))
			f.RunGoHook()
		})

		It("should remove egress labels", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Node", "frontend-2").Field("metadata.labels").Map()).ToNot(HaveKey("egress-gateway.network.deckhouse.io/member"))
			Expect(f.KubernetesGlobalResource("Node", "frontend-2").Field("metadata.labels").Map()).ToNot(HaveKey("egress-gateway.network.deckhouse.io/active-for-egg-dev"))
		})
	})

	Context("Node without NodeSelector and without egress labels", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(eg + nodeWithoutNodeRoleWithoutLabels))
			f.RunGoHook()
		})

		It("should not change anything", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Node", "frontend-3").Field("metadata.labels").Map()).To(BeEmpty())
		})
	})

	Context("Node with multiple active-for-* labels", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(eg + nodeWithoutNodeRoleWithoutLabels + nodeWithMultipleActiveLabels))
			f.RunGoHook()
		})

		It("should remove all active-for-* labels", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Node", "frontend-4").Field("metadata.labels").Map()).ToNot(HaveKey("egress-gateway.network.deckhouse.io/active-for-egg-dev"))
			Expect(f.KubernetesGlobalResource("Node", "frontend-4").Field("metadata.labels").Map()).ToNot(HaveKey("egress-gateway.network.deckhouse.io/active-for-egg-prod"))
			Expect(f.KubernetesGlobalResource("Node", "frontend-4").Field("metadata.labels").Map()).ToNot(HaveKey("egress-gateway.network.deckhouse.io/member"))
		})
	})
})

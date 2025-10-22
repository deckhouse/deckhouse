/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Metallb :: hooks :: update_node_labels ::", func() {
	f := HookExecutionConfigInit(`{"metallb":{"internal": {"l2lbservices": [{}]}}}`, "")
	f.RegisterCRD("network.deckhouse.io", "v1alpha1", "MetalLoadBalancerClass", false)

	Context("Empty Cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})
	})

	Context("There are cluster with 1 service and 1 MetalLoadBalancerClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Service
metadata:
  name: nginx
  namespace: nginx
  annotations:
    network.deckhouse.io/l2-load-balancer-external-ips-count: "3"
spec:
  clusterIP: 1.2.3.4
  ports:
  - port: 7473
    protocol: TCP
    targetPort: 7473
  selector:
    app: nginx
  type: LoadBalancer
---
apiVersion: network.deckhouse.io/v1alpha1
kind: MetalLoadBalancerClass
metadata:
  name: default-ingress
spec:
  isDefault: true
  type: L2
  l2:
    interfaces:
    - eno1
    - eth0.vlan300
  addressPool:
  - 192.168.2.100-192.168.2.150
  nodeSelector:
    node-role.kubernetes.io/frontend: ""
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-1
  labels:
    node-role.kubernetes.io/frontend: ""
    l2-load-balancer.network.deckhouse.io/member: ""
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-2
  labels:
    node-role.kubernetes.io/frontend: ""
    l2-load-balancer.network.deckhouse.io/member: ""
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-3
  labels:
    l2-load-balancer.network.deckhouse.io/member: ""
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-4
  labels:
    node-role.kubernetes.io/frontend: ""
`))
			f.RunHook()
		})

		It("L2LBServices must be present in internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			frontend3 := f.KubernetesGlobalResource("Node", "frontend-3")
			frontend4 := f.KubernetesGlobalResource("Node", "frontend-4")

			Expect(frontend4.Field("metadata").String()).To(MatchJSON(`
{
	"labels": {
		"l2-load-balancer.network.deckhouse.io/member": "",
		"node-role.kubernetes.io/frontend": ""
	},
	"name": "frontend-4"
}`))

			Expect(frontend3.Field("metadata").String()).To(MatchJSON(`
{
	"labels": {},
	"name": "frontend-3"
}`))
		})
	})
})

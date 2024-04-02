/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("l2-load-balancer :: hooks :: get_load_balancers ::", func() {
	f := HookExecutionConfigInit(`{"l2LoadBalancer":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "L2LoadBalancer", true)

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})

		Context("After adding load balancer", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: L2LoadBalancer
metadata:
  name: test
  namespace: test
spec:
  addressPool: mypool
  nodeSelector:
    role: worker
  service:
    sourceRanges:
    - 10.0.0.0/24
    externalTrafficPolicy: Local
    selector:
      app: test
    ports:
    - name: http
      protocol: TCP
      port: 8081
      targetPort: 80
`))
				f.RunHook()
			})

			It("Should store load balancer crds to values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("l2LoadBalancer.internal.l2LoadBalancers").String()).To(MatchJSON(`[{
"name": "test",
"namespace": "test",
"addressPool": "mypool",
"externalTrafficPolicy": "Local",
"sourceRanges": ["10.0.0.0/24"],
"nodeSelector": {
	"role": "worker"
},
"selector": {
	"app": "test"
},
"ports": [
	{
		"name": "http",
		"protocol": "TCP",
		"port": 8081,
		"targetPort": 80
	}
],
"nodes": []
}]`))
			})
		})
	})

	Context("With L2 Load Balancer nodeSelector", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-1
  labels:
    node-role.kubernetes.io/frontend: ""
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
    node-role.kubernetes.io/frontend: ""
spec:
  podCIDR: 10.111.2.0/24
  podCIDRs:
  - 10.111.2.0/24
---
apiVersion: v1
kind: Node
metadata:
  name: worker-1
  labels:
    node-role.kubernetes.io/worker: ""
spec:
  podCIDR: 10.111.3.0/24
  podCIDRs:
  - 10.111.3.0/24
---
apiVersion: deckhouse.io/v1alpha1
kind: L2LoadBalancer
metadata:
  name: test
  namespace: test
spec:
  addressPool: mypool
  nodeSelector:
    node-role.kubernetes.io/frontend: ""
  service:
    selector:
      app: test
    externalTrafficPolicy: Local
    ports:
    - name: http
      protocol: TCP
      port: 8081
      targetPort: 80
`))
			f.RunHook()
		})
		It("Should store load balancer crds to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("l2LoadBalancer.internal.speakerNodes").Array()).To(HaveLen(2))
			Expect(f.ValuesGet("l2LoadBalancer.internal.l2LoadBalancers.0.nodes").Array()).To(HaveLen(2))

			Expect(f.ValuesGet("l2LoadBalancer.internal.l2LoadBalancers.0").String()).To(MatchJSON(`{
"name": "test",
"namespace": "test",
"addressPool": "mypool",
"externalTrafficPolicy": "Local",
"nodeSelector": {
	"node-role.kubernetes.io/frontend": ""
},
"selector": {
	"app": "test"
},
"ports": [
	{
		"name": "http",
		"protocol": "TCP",
		"port": 8081,
		"targetPort": 80
	}
],
"nodes": [
	{"name": "frontend-1"},
	{"name": "frontend-2"}
]
}`))
			Expect(f.ValuesGet("l2LoadBalancer.internal.speakerNodes").String()).To(Or(MatchJSON(`[
"frontend-1",
"frontend-2"
]`), MatchJSON(`[
"frontend-2",
"frontend-1"
]`)))
		})
	})

	Context("With L2 Load Balancer add and remove labels", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-1
  labels:
    node-role.kubernetes.io/frontend: ""
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
    node-role.kubernetes.io/frontend: ""
spec:
  podCIDR: 10.111.2.0/24
  podCIDRs:
  - 10.111.2.0/24
---
apiVersion: v1
kind: Node
metadata:
  name: worker-1
  labels:
    node-role.kubernetes.io/worker: ""
spec:
  podCIDR: 10.111.3.0/24
  podCIDRs:
  - 10.111.3.0/24
---
apiVersion: v1
kind: Node
metadata:
  name: broken-1
  labels:
    node-role.kubernetes.io/worker: ""
    l2-load-balancer.network.deckhouse.io/member: ""
spec:
  podCIDR: 10.111.4.0/24
  podCIDRs:
  - 10.111.4.0/24
---
apiVersion: deckhouse.io/v1alpha1
kind: L2LoadBalancer
metadata:
  name: test
  namespace: test
spec:
  addressPool: mypool
  nodeSelector:
    node-role.kubernetes.io/frontend: ""
  service:
    selector:
      app: test
    ports:
    - name: http
      protocol: TCP
      port: 8081
      targetPort: 80
`, 1))
			f.RunHook()
		})
		It("Should label nodes", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			// Add label
			Expect(f.KubernetesGlobalResource("Node", "frontend-1").Field("metadata.labels").String()).To(MatchJSON(`{"l2-load-balancer.network.deckhouse.io/member": "","node-role.kubernetes.io/frontend": ""}`))
			Expect(f.KubernetesGlobalResource("Node", "frontend-2").Field("metadata.labels").String()).To(Not(MatchJSON(`{"node-role.kubernetes.io/frontend": ""}`)))

			// Ignore node
			Expect(f.KubernetesGlobalResource("Node", "worker-1").Field("metadata.labels").String()).To(MatchJSON(`{"node-role.kubernetes.io/worker": ""}`))

			// Remove label
			Expect(f.KubernetesGlobalResource("Node", "broken-1").Field("metadata.labels").String()).To(MatchJSON(`{"node-role.kubernetes.io/worker": ""}`))
		})
	})
})

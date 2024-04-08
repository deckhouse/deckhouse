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

var _ = Describe("l2-load-balancer :: hooks :: update_load_balancers_status ::", func() {
	f := HookExecutionConfigInit(`{"l2LoadBalancer":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "L2LoadBalancer", true)

	Context("Update L2 LBs status", func() {
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
---
apiVersion: deckhouse.io/v1alpha1
kind: L2LoadBalancer
metadata:
  name: test2
  namespace: test2
spec:
  addressPool: mypool
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: l2-load-balancer
    heritage: deckhouse
    instance: test
  name: d8-l2-load-balancer-test-0
  namespace: test
spec: {}
status:
  loadBalancer:
    ingress:
    - ip: 192.168.122.100
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: l2-load-balancer
    heritage: deckhouse
    instance: test
  name: d8-l2-load-balancer-test-1
  namespace: test
spec: {}
status:
  loadBalancer:
    ingress:
    - ip: 192.168.122.101
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: l2-load-balancer
    heritage: deckhouse
    instance: test2
  name: d8-l2-load-balancer-test2-0
  namespace: test2
spec: {}
status:
  loadBalancer:
    ingress:
    - ip: 192.168.122.102
`))
				f.RunHook()
			})

			It("Should store load balancer crds to values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.KubernetesResource("L2LoadBalancer", "test", "test").Field("status.publicAddresses").Array()).To(HaveLen(2))
				Expect(f.KubernetesResource("L2LoadBalancer", "test2", "test2").Field("status.publicAddresses").Array()).To(HaveLen(1))

				Expect(f.KubernetesResource("L2LoadBalancer", "test2", "test2").Field("status").String()).To(MatchJSON(`{
"publicAddresses": ["192.168.122.102"]
}`))
			})
		})

		Context("After change service external IP", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: L2LoadBalancer
metadata:
  name: test3
  namespace: test3
spec:
  addressPool: mypool
status:
  publicAddresses:
  - 192.168.122.100
  - 192.168.122.101
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: l2-load-balancer
    heritage: deckhouse
    instance: test3
  name: d8-l2-load-balancer-test-0
  namespace: test
spec: {}
status:
  loadBalancer:
    ingress:
    - ip: 192.168.122.100
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: l2-load-balancer
    heritage: deckhouse
    instance: test3
  name: d8-l2-load-balancer-test-1
  namespace: test
spec: {}
status:
  loadBalancer:
    ingress:
    - ip: 192.168.122.102
`))
				f.RunHook()
			})

			It("Should store load balancer crds to values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				// Existing 192.168.122.101 hasn't been confirmed, but new one 192.168.122.101 has been added
				Expect(f.KubernetesResource("L2LoadBalancer", "test3", "test3").Field("status.publicAddresses").Array()).To(HaveLen(2))
				Expect(f.KubernetesResource("L2LoadBalancer", "test3", "test3").Field("status").String()).To(MatchJSON(`{
"publicAddresses": ["192.168.122.100", "192.168.122.102"]
}`))
			})
		})
	})
})

/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("l2-load-balancer :: hooks :: update_service_status ::", func() {
	f := HookExecutionConfigInit(`{"l2LoadBalancer":{"internal": {"l2lbservices": [{}]}}}`, "")
	f.RegisterCRD("internal.network.deckhouse.io", "v1alpha1", "SDNInternalL2LBService", true)

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

	Context("Cluster with 1 service and 2 L2LBServices", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Service
metadata:
  name: nginx
  namespace: nginx
  annotations:
    network.deckhouse.io/l2-load-balancer-name: "ingress"
    network.deckhouse.io/l2-load-balancer-external-ips-count: "3"
spec:
  ports:
  - port: 7473
    protocol: TCP
    targetPort: 7473
  selector:
    app: nginx
  type: LoadBalancer
  loadBalancerClass: my-lb-class
---
apiVersion: internal.network.deckhouse.io/v1alpha1
kind: SDNInternalL2LBService
metadata:
  name: nginx-0
  namespace: nginx
spec:
  serviceRef:
    name: nginx
    namespace: nginx
status:
  loadBalancer:
    ingress:
    - ip: 10.0.0.1
---
apiVersion: internal.network.deckhouse.io/v1alpha1
kind: SDNInternalL2LBService
metadata:
  name: nginx-1
  namespace: nginx
spec:
  serviceRef:
    name: nginx
    namespace: nginx
status:
  loadBalancer:
    ingress:
    - ip: 10.0.0.2
---
`))
			f.RunHook()
		})

		It("L2LBServices must be present in internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			svc := f.KubernetesResource("Service", "nginx", "nginx")
			fmt.Printf("%+v\n", svc)
			Expect(svc.Field("status").String()).To(MatchJSON(`{
"conditions": [
	{
		"message": "2 public IPs from 2 were assigned",
		"reason": "AllIPsAssigned",
		"status": "True",
		"type": "AllPublicIPsAssigned"
	}
],
"loadBalancer": {
	"ingress": [
		{
			"ip": "10.0.0.1"
		},
		{
			"ip": "10.0.0.2"
		}
	]
}
}`))
		})
	})

	Context("Cluster with 1 service and 3 L2LBServices (one is not ready)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Service
metadata:
  name: nginx
  namespace: nginx
  annotations:
    network.deckhouse.io/l2-load-balancer-name: "ingress"
    network.deckhouse.io/l2-load-balancer-external-ips-count: "3"
spec:
  ports:
  - port: 7473
    protocol: TCP
    targetPort: 7473
  selector:
    app: nginx
  type: LoadBalancer
  loadBalancerClass: my-lb-class
---
apiVersion: internal.network.deckhouse.io/v1alpha1
kind: SDNInternalL2LBService
metadata:
  name: nginx-0
  namespace: nginx
spec:
  serviceRef:
    name: nginx
    namespace: nginx
status:
  loadBalancer:
    ingress:
    - ip: 10.0.0.1
---
apiVersion: internal.network.deckhouse.io/v1alpha1
kind: SDNInternalL2LBService
metadata:
  name: nginx-1
  namespace: nginx
spec:
  serviceRef:
    name: nginx
    namespace: nginx
status:
  loadBalancer:
    ingress:
    - ip: 10.0.0.2
---
apiVersion: internal.network.deckhouse.io/v1alpha1
kind: SDNInternalL2LBService
metadata:
  name: nginx-2
  namespace: nginx
spec:
  serviceRef:
    name: nginx
    namespace: nginx
---
`))
			f.RunHook()
		})

		It("L2LBServices must be present in internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			svc := f.KubernetesResource("Service", "nginx", "nginx")
			fmt.Printf("%+v\n", svc)
			Expect(svc.Field("status").String()).To(MatchJSON(`{
"conditions": [
	{
		"message": "2 public IPs from 3 were assigned",
		"reason": "NotAllIPsAssigned",
		"status": "False",
		"type": "AllPublicIPsAssigned"
	}
],
"loadBalancer": {
	"ingress": [
		{
			"ip": "10.0.0.1"
		},
		{
			"ip": "10.0.0.2"
		}
	]
}
}`))
		})
	})
})

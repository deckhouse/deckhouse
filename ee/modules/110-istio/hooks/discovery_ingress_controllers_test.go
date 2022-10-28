/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = FDescribe("Istio hooks :: discovery_ingress_controllers :: ::", func() {
	f := HookExecutionConfigInit(`{"istio":{ "internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IngressIstioController", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})
	})

	Context("Controller with inlet LoadBalancer", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IngressIstioController
metadata:
  name: test
spec:
  ingressGatewayClass: test
  inlet: LoadBalancer
`))
			f.RunHook()
		})

		It("Should store ingress controller crds to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.ValuesGet("istio.internal.ingressControllers").String()).To(MatchJSON(`
[{
  "name": "test",
  "spec": {
     "ingressGatewayClass": "test",
     "inlet": "LoadBalancer"
  }
 }]`))
		})
	})

})

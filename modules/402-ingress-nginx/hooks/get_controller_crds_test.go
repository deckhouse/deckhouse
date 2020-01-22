package hooks

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Ingress nginx hooks :: get controller crds ::", func() {
	f := HookExecutionConfigInit(`{"ingressNginx":{"defaultControllerVersion": 0.25, "internal": {"webhookCertificates":{}}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IngressNginxController", true)

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})

		Context("After adding ingress nginx controller object and webhook certificate", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
  name: test
spec:
  ingressClass: nginx
  inlet: LoadBalancer
`))
				f.RunHook()
			})

			It("Should store ingress controller crds to values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("ingressNginx.internal.ingressControllerCRDs").String()).To(MatchJSON(`[{
"name": "test",
"spec": {
  "config": {},
  "ingressClass": "nginx",
  "controllerVersion": "0.25",
  "inlet": "LoadBalancer",
  "loadBalancer": {}
}
}]`))
			})
		})
	})

	Context("With Ingress Nginx Controller resource", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
  name: test
spec:
  ingressClass: nginx
  inlet: LoadBalancer
`))
			f.RunHook()
		})
		It("Should store ingress controller crds to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("ingressNginx.internal.ingressControllerCRDs.0.name").String()).To(Equal("test"))
			Expect(f.ValuesGet("ingressNginx.internal.ingressControllerCRDs.0.spec").String()).To(MatchJSON(`{
"config": {},
"ingressClass": "nginx",
"controllerVersion": "0.25",
"inlet": "LoadBalancer",
"loadBalancer": {}
}`))
		})
	})
})

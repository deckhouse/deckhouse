package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: ingress-nginx :: hooks :: migrate_domain_resources ::", func() {
	const (
		properResources = `
---
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
  name: main-0
spec:
  nodeSelector:
    node-role/loadbalancer: ""
  tolerations:
  - effect: NoExecute
    key: node-role/loadbalancer
    value: ""
---
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
  name: main-1
  labels:
    heritage: deckhouse
spec:
  nodeSelector:
    node-role.flant.com/frontend: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.flant.com
    value: "frontend"
`
		resourcesWithOldNodeSelector = `
---
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
  name: main
spec:
  nodeSelector:
    node-role.flant.com/frontend: ""
`
		resourcesWithOldTolerations = `
---
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
  name: main
spec:
  tolerations:
  - effect: NoExecute
    key: dedicated.flant.com
    value: "frontend"
`
	)
	f := HookExecutionConfigInit(`{"ingressNginx":{"defaultControllerVersion": 0.25, "internal": {"webhookCertificates":{}}}}`, "")

	f.RegisterCRD("deckhouse.io", "v1alpha1", "IngressNginxController", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster containing proper resources", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(properResources))
			f.RunHook()
		})

		It("Hook must not fail, no metrics should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.ingressnginxcontrollers.0.filterResult.labels").Exists()).To(BeFalse())
		})
	})

	Context("Cluster with resources having old NodeSelector", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(resourcesWithOldNodeSelector))
			f.RunHook()
		})

		It("Hook must not fail, metrics must render", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.ingressnginxcontrollers.0.filterResult.labels").String()).To(MatchJSON(`{"name":"main","kind":"IngressNginxController","namespace":"default"}`))
		})
	})

	Context("Cluster with resources having old tolerations", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(resourcesWithOldTolerations))
			f.RunHook()
		})

		It("Hook must not fail, metrics must render", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.ingressnginxcontrollers.0.filterResult.labels").String()).To(MatchJSON(`{"name":"main","kind":"IngressNginxController","namespace":"default"}`))
		})
	})

})

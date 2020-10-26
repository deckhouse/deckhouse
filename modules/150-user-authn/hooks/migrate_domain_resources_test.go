package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: migrate_domain_resources ::", func() {
	const (
		properResources = `
---
apiVersion: deckhouse.io/v1alpha1
kind: DexAuthenticator
metadata:
  name: main
  namespace: default
spec:
  nodeSelector:
    node-role/security: ""
  tolerations:
  - effect: NoExecute
    key: node-role/security
    value: ""
`
		resourcesWithOldNodeSelector = `
---
apiVersion: deckhouse.io/v1alpha1
kind: DexAuthenticator
metadata:
  name: main
spec:
  nodeSelector:
    node-role.flant.com/frontend: ""
`
		resourcesWithOldTolerations = `
---
apiVersion: deckhouse.io/v1alpha1
kind: DexAuthenticator
metadata:
  name: main
spec:
  tolerations:
  - effect: NoExecute
    key: dedicated.flant.com
    value: "frontend"
`
	)
	f := HookExecutionConfigInit(
		`{"userAuthn":{"internal": {}}}`,
		`{}`,
	)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "DexAuthenticator", true)

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
			Expect(f.BindingContexts.Get("0.snapshots.dexauthenticators.0.filterResult.labels").Exists()).To(BeFalse())
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
			Expect(f.BindingContexts.Get("0.snapshots.dexauthenticators.0.filterResult.labels").String()).To(MatchJSON(`{"name":"main","kind":"DexAuthenticator","namespace":"default"}`))
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
			Expect(f.BindingContexts.Get("0.snapshots.dexauthenticators.0.filterResult.labels").String()).To(MatchJSON(`{"name":"main","kind":"DexAuthenticator","namespace":"default"}`))
		})
	})

})

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

		It("Hook must not fail, no selector and toleration should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.dexauthenticators.0.filterResult").String()).To(MatchJSON(`
{
  "kind": "DexAuthenticator",
  "name": "main",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": []
}
`))
		})
	})

	Context("Cluster with resources having old NodeSelector", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(resourcesWithOldNodeSelector))
			f.RunHook()
		})

		It("Hook must not fail, node selector should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.dexauthenticators.0.filterResult").String()).To(MatchJSON(`
{
  "kind": "DexAuthenticator",
  "name": "main",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": [
	"frontend"
  ]
}
`))
		})
	})

	Context("Cluster with resources having old tolerations", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(resourcesWithOldTolerations))
			f.RunHook()
		})

		It("Hook must not fail, toleration should be selected", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.BindingContexts.Get("0.snapshots.dexauthenticators.0.filterResult").String()).To(MatchJSON(`
{
  "kind": "DexAuthenticator",
  "name": "main",
  "namespace": "default",
  "usedNodeSelectorsAndTolerations": [
	"frontend"
  ]
}
`))
		})
	})

})

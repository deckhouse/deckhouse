package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: discover_cloud_provider ::", func() {
	const (
		stateSecret = `
---
apiVersion: v1
data:
  b64String: YWJj               # abc
  b64JSON: eyJwYXJzZSI6Im1lIn0= # {"parse":"me"}
kind: Secret
metadata:
  name: d8-node-manager-cloud-provider
  namespace: kube-system
`
		stateSecretModified = `
---
apiVersion: v1
data:
  b64String: eHl6                       # xyz
  b64JSON: eyJwYXJzZSI6InlvdXJzZWxmIn0= # {"parse":"yourself"}
kind: Secret
metadata:
  name: d8-node-manager-cloud-provider
  namespace: kube-system
`
	)

	f := HookExecutionConfigInit(`{"nodeManager":{"internal": {}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		Context("Someone added d8-node-manager-cloud-provider", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateSecret))
				f.RunHook()
			})

			It("`nodeManager.internal.cloudProvider must be filled with data from secret", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("nodeManager.internal.cloudProvider.b64String").String()).To(Equal("abc"))
				Expect(f.ValuesGet("nodeManager.internal.cloudProvider.b64JSON.parse").String()).To(Equal("me"))
			})
		})
	})

	Context("Secret d8-node-manager-cloud-provider is in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateSecret))
			f.RunHook()
		})

		It("`nodeManager.internal.cloudProvider must be filled with data from secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.cloudProvider.b64String").String()).To(Equal("abc"))
			Expect(f.ValuesGet("nodeManager.internal.cloudProvider.b64JSON.parse").String()).To(Equal("me"))
		})

		Context("Secret d8-node-manager-cloud-provider was modified", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateSecretModified))
				f.RunHook()
			})

			It("`nodeManager.internal.cloudProvider must be filled with data from secret", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("nodeManager.internal.cloudProvider.b64String").String()).To(Equal("xyz"))
				Expect(f.ValuesGet("nodeManager.internal.cloudProvider.b64JSON.parse").String()).To(Equal("yourself"))
			})
		})
	})
})

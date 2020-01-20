package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: discover apiserver endpoints ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"publishAPI":{"enable": true},"internal": {}}}`, "")

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Service
metadata:
  name: kubernetes
spec:
  ports:
  - targetPort: 6443
---
apiVersion: v1
kind: Endpoints
metadata:
  name: kubernetes
subsets:
- addresses:
  - ip: 192.168.1.1
`))
			f.RunHook()
		})

		It("Should fill internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.kubernetesApiserverTargetPort").String()).To(Equal("6443"))
			Expect(f.ValuesGet("userAuthn.internal.kubernetesApiserverAddresses").String()).To(Equal(`["192.168.1.1"]`))
		})

		Context("Change to multi-master and change apiserver targetPort", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Service
metadata:
  name: kubernetes
spec:
  ports:
  - targetPort: 443
---
apiVersion: v1
kind: Endpoints
metadata:
  name: kubernetes
subsets:
- addresses:
  - ip: 192.168.1.1
  - ip: 192.168.1.2
  - ip: 192.168.1.3
`))
				f.RunHook()
			})

			It("Should update internal values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("userAuthn.internal.kubernetesApiserverTargetPort").String()).To(Equal("443"))
				Expect(f.ValuesGet("userAuthn.internal.kubernetesApiserverAddresses").String()).To(MatchJSON(`["192.168.1.1","192.168.1.2","192.168.1.3"]`))
			})
		})
	})
})

/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: generate kubernetes dex client app secret ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")

	var clientAppSecret string
	var testSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: kubernetes-dex-client-app-secret
  namespace: d8-user-authn
data:
  secret: QUJD # ABC
`
	Context("With secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(testSecret))
			f.RunHook()
		})

		It("Should fill internal values from secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.kubernetesDexClientAppSecret").String()).To(Equal("ABC"))
		})

		Context("With  empty value", func() {
			BeforeEach(func() {
				f.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "")

				f.RunHook()
			})

			It("Should fill internal values from secret", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("userAuthn.internal.kubernetesDexClientAppSecret").String()).To(Equal("ABC"))
			})
		})
	})

	Context("With empty secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: kubernetes-dex-client-app-secret
  namespace: d8-user-authn
data: {}
`))
			f.RunHook()
		})

		It("Should generate new secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.kubernetesDexClientAppSecret").String()).To(HaveLen(20))
		})
	})

	Context("With empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Should fill internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.kubernetesDexClientAppSecret").Exists()).To(BeTrue())
			v := f.ValuesGet("userAuthn.internal.kubernetesDexClientAppSecret").String()
			Expect(v).NotTo(BeEmpty())
		})

		It("Should fill internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.kubernetesDexClientAppSecret").Exists()).To(BeTrue())
			v := f.ValuesGet("userAuthn.internal.kubernetesDexClientAppSecret").String()
			Expect(v).NotTo(BeEmpty())
		})

		Context("With another run", func() {
			BeforeEach(func() {
				clientAppSecret = f.ValuesGet("userAuthn.internal.kubernetesDexClientAppSecret").String()

				f.BindingContexts.Set(f.KubeStateSet(testSecret))
				f.RunHook()
			})

			It("Do not change the values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("userAuthn.internal.kubernetesDexClientAppSecret").Exists()).To(BeTrue())
				Expect(f.ValuesGet("userAuthn.internal.kubernetesDexClientAppSecret").String()).To(Equal(clientAppSecret))
			})
		})
	})

	Context("With default empty value", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "")

			f.RunHook()
		})

		It("Should fill non empty internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.kubernetesDexClientAppSecret").Exists()).To(BeTrue())
			Expect(f.ValuesGet("userAuthn.internal.kubernetesDexClientAppSecret").String()).NotTo(BeEmpty())
		})
	})
})

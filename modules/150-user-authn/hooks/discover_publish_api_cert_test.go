/*
Copyright 2021 Flant CJSC

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

var _ = Describe("User Authn hooks :: discover publish api cert ::", func() {
	f := HookExecutionConfigInit(
		`{"userAuthn":{"internal": {}, "https": {"mode": "CertManager"}}}`,
		"",
	)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})

		Context("After adding secret", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
apiVersion: v1
kind: Secret
metadata:
  name: kubernetes-tls
  namespace: d8-user-authn
data:
  ca.crt: dGVzdA==
`, 2))
				f.RunHook()
			})

			It("Should add internal values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
				Expect(f.ValuesGet("userAuthn.internal.publishedAPIKubeconfigGeneratorMasterCA").String()).To(Equal("test"))
			})

			Context("After updating secret", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
apiVersion: v1
kind: Secret
metadata:
  name: kubernetes-tls
  namespace: d8-user-authn
data:
  ca.crt: dGVzdC1uZXh0
`, 2))
					f.RunHook()
				})

				It("Should update internal values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

					Expect(f.ValuesGet("userAuthn.internal.publishedAPIKubeconfigGeneratorMasterCA").String()).To(Equal("test-next"))
				})
			})
		})
	})

	Context("Cluster with secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
apiVersion: v1
kind: Secret
metadata:
  name: kubernetes-tls
  namespace: d8-user-authn
data:
  ca.crt: dGVzdA==
`, 2))
			f.RunHook()
		})
		It("Should add internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.ValuesGet("userAuthn.internal.publishedAPIKubeconfigGeneratorMasterCA").String()).To(Equal("test"))
		})
	})

	Context("Cluster with secret with OnlyInURI mode", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
apiVersion: v1
kind: Secret
metadata:
  name: kubernetes-tls
  namespace: d8-user-authn
data:
  ca.crt: dGVzdA==
`, 2))
			f.ValuesSet("userAuthn.https.mode", "OnlyInURI")
			f.RunHook()
		})

		It("Should not add values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.ValuesGet("userAuthn.internal.publishedAPIKubeconfigGeneratorMasterCA").String()).To(Equal(""))
		})
	})
})

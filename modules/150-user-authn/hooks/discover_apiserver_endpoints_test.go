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

var _ = Describe("User Authn hooks :: discover apiserver endpoints ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"publishAPI":{"enabled": true},"internal": {}}}`, "")

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Service
metadata:
  name: kubernetes
  namespace: default
spec:
  ports:
  - targetPort: 6443
---
apiVersion: v1
kind: Endpoints
metadata:
  name: kubernetes
  namespace: default
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
  namespace: default
spec:
  ports:
  - targetPort: 443
---
apiVersion: v1
kind: Endpoints
metadata:
  name: kubernetes
  namespace: default
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
			Context("Test before helm ", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.GenerateBeforeHelmContext())
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
})

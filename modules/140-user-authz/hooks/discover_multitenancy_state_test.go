/*
Copyright 2026 Flant JSC

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

/*

User-stories:
The multitenancy.py validating webhook needs the effective (schema-defaults
merged) value of userAuthz.enableMultiTenancy, which is not visible in
ModuleConfig when it's left unset and defaulted by the edition's
config-schema (e.g. CSE). This hook mirrors input.Values into a ConfigMap so
the webhook can read it as a real Kubernetes object.

*/

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authz hooks :: discover multitenancy state ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"enableMultiTenancy": false, "internal":{}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("enableMultiTenancy is false, onBeforeHelm", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.ValuesSet("userAuthz.enableMultiTenancy", false)
				f.RunHook()
			})

			It("publishes enableMultiTenancy=false to the ConfigMap", func() {
				Expect(f).To(ExecuteSuccessfully())
				cm := f.KubernetesResource("ConfigMap", "d8-user-authz", "d8-user-authz-multitenancy-state")
				Expect(cm.Exists()).To(BeTrue())
				Expect(cm.Field("data.enableMultiTenancy").String()).To(Equal("false"))
			})
		})

		Context("enableMultiTenancy is true (explicitly set, or defaulted by the edition's schema), onBeforeHelm", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.ValuesSet("userAuthz.enableMultiTenancy", true)
				f.RunHook()
			})

			It("publishes enableMultiTenancy=true to the ConfigMap", func() {
				Expect(f).To(ExecuteSuccessfully())
				cm := f.KubernetesResource("ConfigMap", "d8-user-authz", "d8-user-authz-multitenancy-state")
				Expect(cm.Exists()).To(BeTrue())
				Expect(cm.Field("data.enableMultiTenancy").String()).To(Equal("true"))
			})

			Context("enableMultiTenancy then flips to false, onBeforeHelm", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.GenerateBeforeHelmContext())
					f.ValuesSet("userAuthz.enableMultiTenancy", false)
					f.RunHook()
				})

				It("updates the ConfigMap to enableMultiTenancy=false", func() {
					Expect(f).To(ExecuteSuccessfully())
					cm := f.KubernetesResource("ConfigMap", "d8-user-authz", "d8-user-authz-multitenancy-state")
					Expect(cm.Exists()).To(BeTrue())
					Expect(cm.Field("data.enableMultiTenancy").String()).To(Equal("false"))
				})
			})
		})
	})
})

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

The ConfigMap lives in the module's own d8-user-authz namespace — but that
namespace only exists when enableMultiTenancy is true, so this hook must
never assume it's there (regression: it used to try to create the ConfigMap
unconditionally and broke the module's reconcile loop whenever
enableMultiTenancy was false).

*/

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const stateNamespaceExists = `
apiVersion: v1
kind: Namespace
metadata:
  name: d8-user-authz
`

var _ = Describe("User Authz hooks :: discover multitenancy state ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"enableMultiTenancy": false, "internal":{}}}`, `{}`)

	Context("d8-user-authz namespace does not exist (enableMultiTenancy is false)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("executes successfully without the namespace", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("does not create the ConfigMap", func() {
			cm := f.KubernetesResource("ConfigMap", "d8-user-authz", "d8-user-authz-multitenancy-state")
			Expect(cm.Exists()).To(BeFalse())
		})

		Context("onBeforeHelm fires anyway (e.g. an unrelated Values recalculation)", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.ValuesSet("userAuthz.enableMultiTenancy", true)
				f.RunHook()
			})

			It("still executes successfully and still does not create the ConfigMap", func() {
				Expect(f).To(ExecuteSuccessfully())
				cm := f.KubernetesResource("ConfigMap", "d8-user-authz", "d8-user-authz-multitenancy-state")
				Expect(cm.Exists()).To(BeFalse())
			})
		})

		Context("the namespace then appears (the chart created it after enableMultiTenancy flipped to true)", func() {
			BeforeEach(func() {
				f.ValuesSet("userAuthz.enableMultiTenancy", true)
				f.BindingContexts.Set(f.KubeStateSet(stateNamespaceExists))
				f.RunHook()
			})

			It("publishes enableMultiTenancy=true as soon as the namespace shows up, without waiting for onBeforeHelm", func() {
				Expect(f).To(ExecuteSuccessfully())
				cm := f.KubernetesResource("ConfigMap", "d8-user-authz", "d8-user-authz-multitenancy-state")
				Expect(cm.Exists()).To(BeTrue())
				Expect(cm.Field("data.enableMultiTenancy").String()).To(Equal("true"))
			})
		})
	})

	Context("d8-user-authz namespace exists (enableMultiTenancy is true)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNamespaceExists))
			f.RunHook()
		})

		Context("enableMultiTenancy is true, onBeforeHelm", func() {
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

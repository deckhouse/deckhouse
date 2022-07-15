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

var _ = Describe("Modules :: upmeter :: hooks :: generate_password", func() {
	for _, app := range []string{"status", "webui"} {
		Context(app, func() {
			var (
				authKey         = "upmeter.auth." + app
				passwordKey     = "upmeter.auth." + app + ".password"
				externalAuthKey = "upmeter.auth." + app + ".externalAuthentication"
			)

			f := HookExecutionConfigInit(
				`{"upmeter": {"internal": {}} }`,
				`{"upmeter":{}}`,
			)

			Context("without external auth", func() {
				BeforeEach(func() {
					f.KubeStateSet("")
					f.BindingContexts.Set(f.GenerateBeforeHelmContext())
					f.RunHook()
				})

				It("should generate new password", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ConfigValuesGet(passwordKey).String()).ShouldNot(BeEmpty())
				})
			})

			Context("with existing password", func() {
				BeforeEach(func() {
					f.KubeStateSet("")
					f.BindingContexts.Set(f.GenerateBeforeHelmContext())
					f.ValuesSet(passwordKey, "zxczxczxc")
					f.RunHook()
				})

				It("should generate new password", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet(passwordKey).String()).Should(BeEquivalentTo("zxczxczxc"))
				})
			})

			Context("with external auth", func() {
				BeforeEach(func() {
					f.KubeStateSet("")
					f.BindingContexts.Set(f.GenerateBeforeHelmContext())
					f.ValuesSetFromYaml(externalAuthKey, []byte(`{"authURL": "test"}`))
					f.RunHook()
				})

				It("should run without error", func() {
					Expect(f).To(ExecuteSuccessfully())
				})

				It("should clean auth data", func() {
					Expect(f.ValuesGet(passwordKey).String()).Should(BeEmpty())
					Expect(f.ConfigValuesGet(authKey).Exists()).Should(BeFalse())
				})
			})
		})
	}

	Context("both", func() {
		f := HookExecutionConfigInit(
			`{"upmeter": {"internal": {}} }`,
			`{"upmeter":{}}`,
		)

		var (
			rootKey = "upmeter.auth"

			authKey1         = "upmeter.auth.status"
			passwordKey1     = "upmeter.auth.status.password"
			externalAuthKey1 = "upmeter.auth.status.externalAuthentication"

			authKey2         = "upmeter.auth.webui"
			passwordKey2     = "upmeter.auth.webui.password"
			externalAuthKey2 = "upmeter.auth.webui.externalAuthentication"
		)

		Context("without external auth", func() {
			BeforeEach(func() {
				f.KubeStateSet("")
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.RunHook()
			})

			It("should generate new password", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(f.ConfigValuesGet(passwordKey1).String()).ShouldNot(BeEmpty())
				Expect(f.ConfigValuesGet(passwordKey2).String()).ShouldNot(BeEmpty())
			})
		})

		Context("with existing password", func() {
			BeforeEach(func() {
				f.KubeStateSet("")
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())

				f.ValuesSet(passwordKey1, "xxx")
				f.ValuesSet(passwordKey2, "ooo")

				f.RunHook()
			})

			It("should generate new password", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet(passwordKey1).String()).Should(BeEquivalentTo("xxx"))
				Expect(f.ValuesGet(passwordKey2).String()).Should(BeEquivalentTo("ooo"))
			})
		})

		Context("with external auth", func() {
			BeforeEach(func() {
				f.KubeStateSet("")
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())

				extAuth := []byte(`{"authURL": "test"}`)

				f.ValuesSetFromYaml(externalAuthKey1, extAuth)
				f.ValuesSetFromYaml(externalAuthKey2, extAuth)
				f.RunHook()
			})

			It("should run without error", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("should clean auth data for both", func() {
				Expect(f.ValuesGet(passwordKey1).String()).Should(BeEmpty())
				Expect(f.ValuesGet(passwordKey2).String()).Should(BeEmpty())

				Expect(f.ConfigValuesGet(authKey1).Exists()).Should(BeFalse())
				Expect(f.ConfigValuesGet(authKey2).Exists()).Should(BeFalse())
			})

			It("should not set root value", func() {
				Expect(f.ConfigValuesGet(rootKey).Exists()).Should(BeFalse())
			})
		})
	})
})

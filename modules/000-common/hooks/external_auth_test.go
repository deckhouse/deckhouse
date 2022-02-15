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

var _ = Describe("Modules :: common :: hooks :: external_auth", func() {
	f := HookExecutionConfigInit(`
common:
  auth: {}
  internal: {}
global:
  enabledModules: ["user-authn"] # Dex enabled
  discovery:
    clusterDomain: cluster.local
`, `
common:
  auth: {}
  internal: {}
`)
	Context("with disabled dex", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[]`))
			f.RunHook()
		})
		It("Should not add anything", func() {
			Expect(f.ValuesGet("common.internal.deployDexAuthenticator").Exists()).To(BeFalse())
			Expect(f.ValuesGet("common.auth.externalAuthentication").Exists()).To(BeFalse())
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("fresh start Dex enabled", func() {
		BeforeEach(func() {
			f.RunHook()
		})

		It("Add dex values to the external auth", func() {
			Expect(f.ValuesGet("common.internal.deployDexAuthenticator").Bool()).To(BeTrue())
			Expect(f.ValuesGet("common.auth.externalAuthentication.authURL").String()).To(Equal("https://test.cluster.local/test"))
			Expect(f.ValuesGet("common.auth.externalAuthentication.authSignInURL").String()).To(Equal("https://test/sign_in"))
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("disable dex", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global.enabledModules", []byte(`[]`))
				f.RunHook()
			})

			It("Remove dex external auth", func() {
				Expect(f.ValuesGet("common.internal.deployDexAuthenticator").Exists()).To(BeFalse())
				Expect(f.ValuesGet("common.auth.externalAuthentication").Exists()).To(BeFalse())
				Expect(f).To(ExecuteSuccessfully())
			})
		})
	})

	Context("with external auth", func() {
		BeforeEach(func() {
			f.ConfigValuesSetFromYaml("common.auth.externalAuthentication", []byte(`
authURL: external-test
authSignInURL: external-signin-test
`))
			f.RunHook()
		})
		It("Should not add anything", func() {
			Expect(f.ValuesGet("common.auth.externalAuthentication.authURL").String()).To(Equal("external-test"))
			Expect(f.ValuesGet("common.auth.externalAuthentication.authSignInURL").String()).To(Equal("external-signin-test"))
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("disable dex", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global.enabledModules", []byte(`[]`))
				f.RunHook()
			})
			It("Should not add anything", func() {
				Expect(f.ValuesGet("common.auth.externalAuthentication.authURL").String()).To(Equal("external-test"))
				Expect(f.ValuesGet("common.auth.externalAuthentication.authSignInURL").String()).To(Equal("external-signin-test"))
				Expect(f).To(ExecuteSuccessfully())
			})
		})

		Context("disable external auth", func() {
			BeforeEach(func() {
				f.ConfigValuesSetFromYaml("common.auth", []byte("{}"))
				f.RunHook()
			})
			It("Should switch to dex", func() {
				Expect(f.ValuesGet("common.internal.deployDexAuthenticator").String()).To(Equal("true"))
				Expect(f.ValuesGet("common.auth.externalAuthentication.authURL").String()).To(Equal("https://test.cluster.local/test"))
				Expect(f.ValuesGet("common.auth.externalAuthentication.authSignInURL").String()).To(Equal("https://test/sign_in"))
				Expect(f).To(ExecuteSuccessfully())
			})
		})
	})
})

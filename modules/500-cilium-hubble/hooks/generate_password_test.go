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

var _ = Describe("Modules :: cilium-hubble :: hooks :: generate_password", func() {

	const (
		authKey         = "ciliumHubble.auth"
		passwordKey     = "ciliumHubble.auth.password"
		externalAuthKey = "ciliumHubble.auth.externalAuthentication"
	)
	f := HookExecutionConfigInit(
		`{"ciliumHubble": {"internal": {}} }`,
		`{"ciliumHubble":{}}`,
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
		It("should get existing password", func() {
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

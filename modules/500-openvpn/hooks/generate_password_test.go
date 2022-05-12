/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: openvpn :: hooks :: generate_password ", func() {
	f := HookExecutionConfigInit(`{"openvpn":{"internal":{}, "auth": {}}}`, `{"openvpn":{}}`)
	Context("without external auth", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("should generate new password", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ConfigValuesGet("openvpn.auth.password").String()).ShouldNot(BeEmpty())
		})
	})

	Context("with extisting password", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("openvpn.auth.password", "zxczxczxc")
			f.RunHook()
		})

		It("should generate new password", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("openvpn.auth.password").String()).Should(BeEquivalentTo("zxczxczxc"))
		})
	})

	Context("with external auth", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSetFromYaml("openvpn.auth.externalAuthentication", []byte(`{"authURL": "test"}`))
			f.RunHook()
		})

		It("should generate new password", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("openvpn.auth.password").String()).Should(BeEmpty())
			Expect(f.ConfigValuesGet("openvpn.auth").Exists()).Should(BeFalse())
		})
	})
})

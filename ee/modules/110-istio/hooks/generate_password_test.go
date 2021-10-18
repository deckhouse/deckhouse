/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: istio :: hooks :: generate_password ", func() {
	f := HookExecutionConfigInit(`{"istio":{"internal":{}, "auth": {}}}`, `{"istio":{}}`)
	Context("without external auth", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("should generate new password", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ConfigValuesGet("istio.auth.password").String()).ToNot(BeEmpty())
		})
	})

	Context("with extisting password", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("istio.auth.password", "zxczxczxc")
			f.RunHook()
		})

		It("should not change the password", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.auth.password").String()).To(BeEquivalentTo("zxczxczxc"))
		})
	})

	Context("with external auth", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSetFromYaml("istio.auth.externalAuthentication", json.RawMessage(`{"authURL": "test"}`))
			f.RunHook()
		})

		It("should not generate new password", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ConfigValuesGet("istio.auth.password").String()).To(BeEmpty())
		})
	})
})

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

var _ = Describe("Modules :: basic-auth :: hooks :: generate_password", func() {

	const (
		locationsKey = "basicAuth.locations"
		passwordKey  = "basicAuth.locations.0.users.admin"
	)
	f := HookExecutionConfigInit(
		`{"basicAuth":{}}`,
		`{"basicAuth":{}}`,
	)
	Context("Without locations", func() {
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
	Context("with existing locations", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSetFromYaml(locationsKey, []byte(`[{"location": "/", "users": {"admin": "secret"}}]`))
			f.RunHook()
		})
		It("should get existing password", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(passwordKey).String()).Should(BeEquivalentTo("secret"))
		})
	})
})

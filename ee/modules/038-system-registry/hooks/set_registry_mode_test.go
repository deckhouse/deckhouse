/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("System Registry :: hooks :: set registry mode ::", func() {
	f := HookExecutionConfigInit(`{"systemRegistry": {"internal": {}}}`,
		`{}`)

	Context("Registry mode is not set", func() {
		BeforeEach(func() {
			f.RunHook()
		})

		It("`systemRegistry.internal.registryMode` must be 'Direct'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("systemRegistry.internal.registryMode").String()).To(Equal("Direct"))
		})
	})

	Context("Registry mode is set to Direct", func() {
		BeforeEach(func() {
			f.ValuesSet("systemRegistry.registryMode", "Direct")
			f.RunHook()
		})

		It("`systemRegistry.internal.registryMode` must be 'Direct'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("systemRegistry.internal.registryMode").String()).To(Equal("Direct"))
		})
	})

	Context("Registry mode is set to Proxy", func() {
		BeforeEach(func() {
			f.ValuesSet("systemRegistry.registryMode", "Proxy")
			f.RunHook()
		})

		It("`systemRegistry.internal.registryMode` must be 'Proxy'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("systemRegistry.internal.registryMode").String()).To(Equal("Proxy"))
		})
	})

	Context("Registry mode is set to Detached", func() {
		BeforeEach(func() {
			f.ValuesSet("systemRegistry.registryMode", "Detached")
			f.RunHook()
		})

		It("`systemRegistry.internal.registryMode` must be 'Detached'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("systemRegistry.internal.registryMode").String()).To(Equal("Detached"))
		})
	})
})

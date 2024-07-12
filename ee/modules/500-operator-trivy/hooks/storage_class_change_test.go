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

var _ = Describe("Modules :: operator-trivy :: hooks :: storage_class_change ::", func() {
	f := HookExecutionConfigInit(`{"operatorTrivy":{"internal":{}}}`, "")

	Context("Storage class is not set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("Should set effectiveClass to false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("operatorTrivy.internal.effectiveStorageClass").String()).To(Equal("false"))
		})
	})

	Context("Global storage class is set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.ConfigValuesSet("global.storageClass", "test")
			f.RunHook()
		})
		It("Should set effectiveClass to test", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("operatorTrivy.internal.effectiveStorageClass").String()).To(Equal("test"))
		})
	})

	Context("Storage class is set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.ConfigValuesSet("operatorTrivy.storageClass", "test1")
			f.RunHook()
		})
		It("Should set effectiveClass to test1", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("operatorTrivy.internal.effectiveStorageClass").String()).To(Equal("test1"))
		})
	})
})

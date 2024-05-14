/*
Copyright 2024 Flant JSC

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

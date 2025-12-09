/*
Copyright 2025 Flant JSC

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
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func TestStorageClassChange(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "storage class change hook")
}

var _ = Describe("Modules :: ingress-nginx :: hooks :: storage_class_change ::", func() {
	f := HookExecutionConfigInit(`{"ingressNginx":{"internal":{}}}`, "{}")

	Context("StorageClass is not provided", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("should keep effectiveStorageClass equal to false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("ingressNginx.internal.effectiveStorageClass").String()).To(Equal("false"))
		})
	})

	Context("Global storageClass is configured", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.ConfigValuesSet("global.modules.storageClass", "fast-sc")
			f.RunHook()
		})

		AfterEach(func() {
			f.ConfigValuesDelete("global.modules.storageClass")
		})

		It("should propagate configured storageClass value", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("ingressNginx.internal.effectiveStorageClass").String()).To(Equal("fast-sc"))
		})
	})
})

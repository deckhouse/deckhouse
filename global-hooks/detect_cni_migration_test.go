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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: detect_cni_migration ::", func() {
	f := HookExecutionConfigInit(`{"global":{"internal":{}}}`, `{}`)
	f.RegisterCRD("network.deckhouse.io", "v1alpha1", "CNIMigration", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("global.internal.cniMigrationEnabled should not be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.internal.cniMigrationEnabled").Exists()).To(BeFalse())
		})
	})

	Context("CNIMigration resource exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: network.deckhouse.io/v1alpha1
kind: CNIMigration
metadata:
  name: test-migration
spec:
  targetCNI: cilium
`))
			f.RunHook()
		})

		It("global.internal.cniMigrationEnabled should be true", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.internal.cniMigrationEnabled").Bool()).To(BeTrue())
		})
	})

	Context("CNIMigration resource deleted", func() {
		BeforeEach(func() {
			f.ValuesSet("global.internal.cniMigrationEnabled", true)
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("global.internal.cniMigrationEnabled should be removed", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.internal.cniMigrationEnabled").Exists()).To(BeFalse())
		})
	})
})

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
			Expect(f.ValuesGet("global.internal.cniMigrationName").String()).To(Equal("test-migration"))
			Expect(f.ValuesGet("global.internal.cniMigrationWebhooksDisable").Bool()).To(BeTrue())
		})
	})

	Context("Multiple CNIMigration resources exist", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: network.deckhouse.io/v1alpha1
kind: CNIMigration
metadata:
  name: migration-newer
  creationTimestamp: "2024-01-02T00:00:00Z"
spec:
  targetCNI: cilium
---
apiVersion: network.deckhouse.io/v1alpha1
kind: CNIMigration
metadata:
  name: migration-older
  creationTimestamp: "2024-01-01T00:00:00Z"
spec:
  targetCNI: flannel
`))
			f.RunHook()
		})

		It("should select the oldest migration", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.internal.cniMigrationName").String()).To(Equal("migration-older"))
		})
	})

	Context("Multiple CNIMigration resources with same timestamp", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: network.deckhouse.io/v1alpha1
kind: CNIMigration
metadata:
  name: migration-b
  creationTimestamp: "2024-01-01T00:00:00Z"
spec:
  targetCNI: cilium
---
apiVersion: network.deckhouse.io/v1alpha1
kind: CNIMigration
metadata:
  name: migration-a
  creationTimestamp: "2024-01-01T00:00:00Z"
spec:
  targetCNI: flannel
`))
			f.RunHook()
		})

		It("should select migration with lexicographically smaller name", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.internal.cniMigrationName").String()).To(Equal("migration-a"))
		})
	})

	Context("CNIMigration resource succeeded", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: network.deckhouse.io/v1alpha1
kind: CNIMigration
metadata:
  name: test-migration-success
spec:
  targetCNI: cilium
status:
  conditions:
  - type: Succeeded
    status: "True"
`))
			f.RunHook()
		})

		It("global.internal.cniMigrationEnabled should be true and validation ignore removed", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.internal.cniMigrationEnabled").Bool()).To(BeTrue())
			Expect(f.ValuesGet("global.internal.cniMigrationName").String()).To(Equal("test-migration-success"))
			Expect(f.ValuesGet("global.internal.cniMigrationWebhooksDisable").Exists()).To(BeFalse())
		})
	})

	Context("CNIMigration resource deleted", func() {
		BeforeEach(func() {
			f.ValuesSet("global.internal.cniMigrationEnabled", true)
			f.ValuesSet("global.internal.cniMigrationName", "old-migration")
			f.ValuesSet("global.internal.cniMigrationWebhooksDisable", true)
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("global.internal.cniMigrationEnabled should be removed", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.internal.cniMigrationEnabled").Exists()).To(BeFalse())
			Expect(f.ValuesGet("global.internal.cniMigrationName").Exists()).To(BeFalse())
			Expect(f.ValuesGet("global.internal.cniMigrationWebhooksDisable").Exists()).To(BeFalse())
		})
	})
})

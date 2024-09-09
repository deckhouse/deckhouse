// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	moduleConfigNoStorageClass = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  settings:
    highAvailability: false
    modules:
      publicDomainTemplate: '%s.domain.example.com'
  version: 1
`

moduleConfigGlobalStorageClass = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  settings:
    highAvailability: false
    modules:
      publicDomainTemplate: '%s.domain.example.com'
    storageClass: some-storage-class
  version: 1
`

moduleConfigGlobalAndModulesStorageClass = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  settings:
    highAvailability: false
    modules:
      publicDomainTemplate: '%s.domain.example.com'
      storageClass: different-storage-class
    storageClass: some-storage-class
  version: 1
`
)

var _ = Describe("Global hooks :: migrate_global_storage_class ::", func() {
	f := HookExecutionConfigInit(`{"global": {"discovery": {}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	Context("Cluster has no `global` module config", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster has no `global.storageClass`", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(moduleConfigNoStorageClass))
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("MC `global` should be created without `storageClass`", func() {
			mc := f.KubernetesGlobalResource("ModuleConfig", "global")
			Expect(mc.Exists()).To(BeTrue())
			Expect(mc.Field("spec.settings.storageClass").String()).To(BeEmpty())
			Expect(mc.Field("spec.settings.modules.storageClass").String()).To(BeEmpty())
		})
	})

	Context("Cluster has `global.storageClass` defined", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(moduleConfigGlobalStorageClass))
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("`global.storageClass` should be moved to `global.modules.storageClass`", func() {
			mc := f.KubernetesGlobalResource("ModuleConfig", "global")
			Expect(mc.Exists()).To(BeTrue())
			Expect(mc.Field("spec.settings.storageClass").Exists()).To(BeFalse())
			Expect(mc.Field("spec.settings.modules.storageClass").String()).To(Equal("some-storage-class"))
		})
	})

	Context("Cluster has `global.storageClass` and `global.modules.storageClass` defined", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(moduleConfigGlobalAndModulesStorageClass))
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("`global.modules.storageClass` should remain the same", func() {
			mc := f.KubernetesGlobalResource("ModuleConfig", "global")
			Expect(mc.Exists()).To(BeTrue())
			Expect(mc.Field("spec.settings.storageClass").Exists()).To(BeFalse())
			Expect(mc.Field("spec.settings.modules.storageClass").String()).To(Equal("different-storage-class"))
		})
	})
})

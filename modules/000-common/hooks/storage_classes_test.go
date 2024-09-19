/*
Copyright 2021 Flant JSC

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
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/set"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: common :: hooks :: storage_classes ::", func() {
	const defaultSCPath = "cloudProviderFake.internal.defaultStorageClass"

	assertStorageClassesInValues := func(f *HookExecutionConfig, mustInValues ...string) {
		raw := f.ValuesGet("cloudProviderFake.internal.storageClasses").String()

		var scInValues []SC
		err := json.Unmarshal([]byte(raw), &scInValues)
		Expect(err).ToNot(HaveOccurred())

		expectedSCSet := set.New(mustInValues...)
		for _, sc := range scInValues {
			Expect(expectedSCSet.Has(sc.Name)).To(BeTrue())

			var expectSc *SC
			for _, supportedSc := range storageClassesConfig {
				if supportedSc.GetName() == sc.Name {
					expectSc = supportedSc.(*SC)
					break
				}
			}

			Expect(expectSc).NotTo(BeNil())
			Expect(*expectSc).To(Equal(sc))
		}
	}

	f := HookExecutionConfigInit(`{"cloudProviderFake":{"internal": {}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		Context("Empty values", func() {
			It("Should discover all supported storage classes", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertStorageClassesInValues(f, "first-hdd", "second-hdd", "third-ssd")
			})

			It("Should not set default class", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(f.ValuesGet(defaultSCPath).Exists()).To(BeFalse())
			})
		})

		Context("DEPRECATED: Set default storage class into values", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("cloudProviderFake.storageClass", []byte(`{"default": "first-hdd"}`))

				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.RunHook()
			})

			It("Should discover all supported storage classes", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertStorageClassesInValues(f, "first-hdd", "second-hdd", "third-ssd")
			})

			It("DEPRECATED: Should set default class", func() {
				Expect(f).To(ExecuteSuccessfully())

				// Expect(f.ValuesGet(defaultSCPath).String()).To(Equal("first-hdd"))
				// changed because cloudProvider's storageClass.default was deprecated in favor of `global.defaultClusterStorageClass`
				Expect(f.ValuesGet(defaultSCPath).Exists()).To(BeFalse())
			})

			Context("Remove default storage class from values", func() {
				BeforeEach(func() {
					f.ValuesSetFromYaml("cloudProviderFake.storageClass", []byte(`{"storageClass": {}}`))

					f.BindingContexts.Set(f.GenerateBeforeHelmContext())
					f.RunHook()
				})

				It("Should discover all supported storage classes", func() {
					Expect(f).To(ExecuteSuccessfully())

					assertStorageClassesInValues(f, "first-hdd", "second-hdd", "third-ssd")
				})

				It("Should not set default class", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(f.ValuesGet(defaultSCPath).Exists()).To(BeFalse())
				})
			})
		})

		Context("Set exclude rule", func() {
			Context("by regexp", func() {
				BeforeEach(func() {
					f.ValuesSetFromYaml("cloudProviderFake.storageClass", []byte(`{"exclude": [".*-hdd"]}`))

					f.BindingContexts.Set(f.GenerateBeforeHelmContext())
					f.RunHook()
				})

				It("Should filter supported storage classes", func() {
					Expect(f).To(ExecuteSuccessfully())

					assertStorageClassesInValues(f, "third-ssd")
				})
			})

			Context("by name", func() {
				BeforeEach(func() {
					f.ValuesSetFromYaml("cloudProviderFake.storageClass", []byte(`{"exclude": ["third-ssd"]}`))

					f.BindingContexts.Set(f.GenerateBeforeHelmContext())
					f.RunHook()
				})

				It("Should filter supported storage classes", func() {
					Expect(f).To(ExecuteSuccessfully())

					assertStorageClassesInValues(f, "first-hdd", "second-hdd")
				})
			})
		})

		Context("Set exclude rule both with default class", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("cloudProviderFake.storageClass", []byte(`
{
  "exclude": ["first-.*", "second-hdd"],
  "default": "third-ssd"
}
`))

				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.RunHook()
			})

			It("Should filter supported storage classes", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertStorageClassesInValues(f, "third-ssd")
			})

			It("DEPRECATED: Should set default class", func() {
				Expect(f).To(ExecuteSuccessfully())

				// Expect(f.ValuesGet(defaultSCPath).String()).To(Equal("third-ssd"))
				// changed because cloudProvider's storageClass.default was deprecated in favor of `global.defaultClusterStorageClass`
				Expect(f.ValuesGet(defaultSCPath).Exists()).To(BeFalse())
			})

			Context("Remove excluding", func() {
				BeforeEach(func() {
					f.ValuesSetFromYaml("cloudProviderFake.storageClass", []byte(`{"default": "third-ssd"}`))

					f.BindingContexts.Set(f.GenerateBeforeHelmContext())
					f.RunHook()
				})

				It("Should discover all supported storage classes", func() {
					Expect(f).To(ExecuteSuccessfully())

					assertStorageClassesInValues(f, "first-hdd", "second-hdd", "third-ssd")
				})

				It("Should set default class", func() {
					Expect(f).To(ExecuteSuccessfully())

					// Expect(f.ValuesGet(defaultSCPath).String()).To(Equal("third-ssd"))
					// changed because cloudProvider's storageClass.default was deprecated in favor of `global.defaultClusterStorageClass`
					Expect(f.ValuesGet(defaultSCPath).Exists()).To(BeFalse())
				})
			})
		})
	})
})

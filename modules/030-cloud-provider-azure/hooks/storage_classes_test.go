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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-azure :: hooks :: storage_classes ::", func() {
	const (
		initValuesString = `
cloudProviderAzure:
  internal: {}
  storageClass:
    provision:
    - name: managed-ultra-ssd
      type: UltraSSD_LRS
      diskIOPSReadWrite: 600
      diskMBpsReadWrite: 150
      tags:
      - key: key1
        value: value1
      - key: key2
        value: value2
    exclude:
    - sc\d+
    - bar
    - managed-standard-large
    default: other-bar
`
		initValuesExcludeAllString = `
cloudProviderAzure:
  internal: {}
  storageClass:
    exclude:
    - ".*"
`
	)

	f := HookExecutionConfigInit(initValuesString, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should discover storageClasses with default set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderAzure.internal.storageClasses").String()).To(MatchJSON(`
[
  {
    "name": "managed-standard-ssd",
    "type": "StandardSSD_LRS"
  },
  {
    "name": "managed-standard",
    "type": "Standard_LRS"
  },
  {
    "name": "managed-premium",
    "type": "Premium_LRS"
  },
  {
    "cachingMode": "None",
    "name": "managed-standard-ssd-large",
    "type": "StandardSSD_LRS"
  },
  {
    "cachingMode": "None",
    "name": "managed-premium-large",
    "type": "Premium_LRS"
  },
  {
    "name": "managed-ultra-ssd",
    "type": "UltraSSD_LRS",
    "diskIOPSReadWrite": 600,
    "diskMBpsReadWrite": 150,
    "tags": "key1=value1,key2=value2"
  }
]
`))
			Expect(f.ValuesGet("cloudProviderAzure.internal.defaultStorageClass").String()).To(Equal(`other-bar`))
		})

	})

	fb := HookExecutionConfigInit(initValuesExcludeAllString, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			fb.BindingContexts.Set(fb.GenerateBeforeHelmContext())
			fb.RunHook()
		})

		It("Should discover no storageClasses with no default is set", func() {
			Expect(fb).To(ExecuteSuccessfully())
			Expect(fb.ValuesGet("cloudProviderAzure.internal.storageClasses").String()).To(MatchJSON(`[]`))
			Expect(fb.ValuesGet("cloudProviderAzure.internal.defaultStorageClass").Exists()).To(BeFalse())
		})

	})

})

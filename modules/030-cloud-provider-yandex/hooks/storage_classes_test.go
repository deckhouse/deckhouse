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

var _ = Describe("Modules :: cloud-provider-yandex :: hooks :: storage_classes ::", func() {
	const (
		initValuesString = `
global:
  discovery: {}
cloudProviderYandex:
  internal: {}
`

		initValuesStringExcludeHdd = `
global:
  discovery: {}
cloudProviderYandex:
  internal: {}
  storageClass:
    exclude:
    - .*-hdd
    - bar
`
	)

	f := HookExecutionConfigInit(initValuesString, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should discover storageClasses with default storageClass set to network-hdd", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderYandex.internal.storageClasses").String()).To(MatchJSON(`
[
  {
	"name": "network-hdd",
	"type": "network-hdd"
  },
  {
	"name": "network-ssd",
	"type": "network-ssd"
  },
  {
	"name": "network-ssd-nonreplicated",
	"type": "network-ssd-nonreplicated"
  },
  {
	"name": "network-ssd-io-m3",
	"type": "network-ssd-io-m3"
  }
]
`))
		})
	})

	b := HookExecutionConfigInit(initValuesStringExcludeHdd, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.GenerateBeforeHelmContext())
			b.RunHook()
		})

		It("Should discover storageClasses with default NOT set", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("cloudProviderYandex.internal.storageClasses").String()).To(MatchJSON(`
[
  {
	"name": "network-ssd",
	"type": "network-ssd"
  },
  {
	"name": "network-ssd-nonreplicated",
	"type": "network-ssd-nonreplicated"
  },
  {
	"name": "network-ssd-io-m3",
	"type": "network-ssd-io-m3"
  }
]
`))
		})
	})
})

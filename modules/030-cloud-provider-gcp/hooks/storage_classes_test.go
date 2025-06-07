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
//nolint:unused // TODO: fix unused linter
package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-gcp :: hooks :: storage_classes ::", func() {
	const (
		initValuesString = `
cloudProviderGcp:
  internal: {}
  storageClass:
    exclude:
    - .*standard.*
    - bar
`

		initValuesWithDefaultClusterStorageClass = `
global:
  defaultClusterStorageClass: default-cluster-sc
cloudProviderGcp:
  internal: {}
  storageClass:
    exclude:
    - .*standard.*
    - bar
`

		initValuesWithEmptyDefaultClusterStorageClass = `
global:
  defaultClusterStorageClass: ""
cloudProviderGcp:
  internal: {}
  storageClass:
    exclude:
    - .*standard.*
    - bar
`
	)

	f := HookExecutionConfigInit(initValuesString, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should discover storageClasses", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderGcp.internal.storageClasses").String()).To(MatchJSON(`
[
  {
	"name": "pd-balanced-not-replicated",
	"replicationType": "none",
	"type": "pd-balanced"
  },
  {
	"name": "pd-balanced-replicated",
	"replicationType": "regional-pd",
	"type": "pd-balanced"
  },
  {
	"name": "pd-ssd-not-replicated",
	"replicationType": "none",
	"type": "pd-ssd"
  },
  {
	"name": "pd-ssd-replicated",
	"replicationType": "regional-pd",
	"type": "pd-ssd"
  }
]
`))
		})

	})
})

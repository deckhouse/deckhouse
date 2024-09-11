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

var _ = Describe("Modules :: cloud-provider-gcp :: hooks :: storage_classes ::", func() {
	const (
		initValuesString = `
cloudProviderGcp:
  internal: {}
  storageClass:
    exclude:
    - .*standard.*
    - bar
    default: pd-ssd-replicated
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
    default: pd-ssd-replicated
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
    default: pd-ssd-replicated
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
			Expect(f.ValuesGet("cloudProviderGcp.internal.defaultStorageClass").String()).To(Equal(`pd-ssd-replicated`))
		})

	})

	a := HookExecutionConfigInit(initValuesWithDefaultClusterStorageClass, `{}`)

	Context("Cluster with `global.defaultClusterStorageClass`", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.GenerateBeforeHelmContext())
			a.RunHook()
		})

		It("Default storage class should be overrided by `global.defaultClusterStorageClass`", func() {
			Expect(a).To(ExecuteSuccessfully())
			Expect(a.ValuesGet("cloudProviderGcp.internal.defaultStorageClass").String()).To(Equal(`default-cluster-sc`))
		})
	})

	b := HookExecutionConfigInit(initValuesWithEmptyDefaultClusterStorageClass, `{}`)

	Context("Cluster with empty `global.defaultClusterStorageClass`", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.GenerateBeforeHelmContext())
			b.RunHook()
		})

		It("Default storage class should be `pd-ssd-replicated` if `global.defaultClusterStorageClass` is empty", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("cloudProviderGcp.internal.defaultStorageClass").String()).To(Equal(`pd-ssd-replicated`))
		})
	})

})

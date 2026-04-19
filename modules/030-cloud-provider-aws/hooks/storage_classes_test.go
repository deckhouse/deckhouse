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

var _ = Describe("Modules :: cloud-provider-aws :: hooks :: storage_classes ::", func() {
	const (
		initValuesString = `
cloudProviderAws:
  internal: {}
  storageClass:
    provision:
    - iopsPerGB: "5"
      name: iops-foo
      type: io1
    - name: gp3
      type: gp3
      iops: "6000"
      throughput: "300"
    exclude:
    - sc\d+
    - bar
`
		initValuesExcludeAllString = `
cloudProviderAws:
  internal: {}
  storageClass:
    exclude:
    - ".*"
`

		initValuesWithDefaultClusterStorageClass = `
global:
  defaultClusterStorageClass: default-cluster-sc
cloudProviderAws:
  internal: {}
  storageClass:
    provision:
    - iopsPerGB: "5"
      name: iops-foo
      type: io1
    - name: gp3
      type: gp3
      iops: "6000"
      throughput: "300"
    exclude:
    - sc\d+
    - bar
`

		initValuesWithEmptyDefaultClusterStorageClass = `
global:
  defaultClusterStorageClass: ""
cloudProviderAws:
  internal: {}
  storageClass:
    provision:
    - iopsPerGB: "5"
      name: iops-foo
      type: io1
    - name: gp3
      type: gp3
      iops: "6000"
      throughput: "300"
    exclude:
    - sc\d+
    - bar
`

		storageClass = `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: gp3
  labels:
    heritage: deckhouse
parameters:
  type: gp3
  iops: "6000"
  throughput: "300"
`
	)

	f := HookExecutionConfigInit(initValuesString, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext(), f.KubeStateSet(storageClass))
			f.RunHook()
		})

		It("Should discover storageClasses", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderAws.internal.storageClasses").String()).To(MatchJSON(`
[
  {
	"name": "gp2",
	"type": "gp2"
  },
  {
	"name": "gp3",
	"type": "gp3",
	"iops": "6000",
	"throughput": "300"
  },
  {
	"iopsPerGB": "5",
	"name": "iops-foo",
	"type": "io1"
  },
  {
	"name": "st1",
	"type": "st1"
  }
]
`))
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
			Expect(fb.ValuesGet("cloudProviderAws.internal.storageClasses").String()).To(MatchJSON(`[]`))
		})
	})
})

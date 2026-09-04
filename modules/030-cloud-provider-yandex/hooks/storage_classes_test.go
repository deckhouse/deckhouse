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
  storage:
    parameters:
      excludedStorageClasses:
      - .*-hdd
      - bar
`

		initValuesStringProvision = `
global:
  discovery: {}
cloudProviderYandex:
  internal: {}
  storage:
    parameters:
      provisionedStorageClasses:
      - name: network-ssd-64k
        type: network-ssd
        blockSize: 64Ki
      - name: network-ssd-io-m3
        type: network-ssd-io-m3
        blockSize: 128Ki
      excludedStorageClasses:
      - .*-hdd
`

		initValuesStringProvisionOverride = `
global:
  discovery: {}
cloudProviderYandex:
  internal: {}
  storage:
    parameters:
      provisionedStorageClasses:
      - name: network-ssd
        type: network-ssd
        blockSize: 64Ki
`

		initValuesStringProvisionExcluded = `
global:
  discovery: {}
cloudProviderYandex:
  internal: {}
  storage:
    parameters:
      provisionedStorageClasses:
      - name: network-ssd-64k
        type: network-ssd
        blockSize: 64Ki
      excludedStorageClasses:
      - network-ssd.*
`

		modifiedStorageClass = `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: network-ssd-64k
  labels:
    heritage: deckhouse
parameters:
  typeID: network-ssd
  blockSize: 32Ki
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: network-ssd
  labels:
    heritage: deckhouse
parameters:
  typeID: network-ssd
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
	"name": "network-ssd-io-m3",
	"type": "network-ssd-io-m3"
  },
  {
	"name": "network-ssd-nonreplicated",
	"type": "network-ssd-nonreplicated"
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
	"name": "network-ssd-io-m3",
	"type": "network-ssd-io-m3"
  },
  {
	"name": "network-ssd-nonreplicated",
	"type": "network-ssd-nonreplicated"
  }
]
`))
		})
	})

	p := HookExecutionConfigInit(initValuesStringProvision, `{}`)

	Context("Cluster with provisioned storageClasses", func() {
		BeforeEach(func() {
			p.BindingContexts.Set(p.GenerateBeforeHelmContext(), p.KubeStateSet(modifiedStorageClass))
			p.RunHook()
		})

		It("Should add the provisioned storageClasses and override the default ones with the same name", func() {
			Expect(p).To(ExecuteSuccessfully())
			Expect(p.ValuesGet("cloudProviderYandex.internal.storageClasses").String()).To(MatchJSON(`
[
  {
	"name": "network-ssd",
	"type": "network-ssd"
  },
  {
	"name": "network-ssd-64k",
	"type": "network-ssd",
	"blockSize": "64Ki"
  },
  {
	"name": "network-ssd-io-m3",
	"type": "network-ssd-io-m3",
	"blockSize": "128Ki"
  },
  {
	"name": "network-ssd-nonreplicated",
	"type": "network-ssd-nonreplicated"
  }
]
`))
		})

		It("Should delete the storageClass with changed parameters", func() {
			Expect(p).To(ExecuteSuccessfully())
			Expect(p.KubernetesGlobalResource("StorageClass", "network-ssd-64k").Exists()).To(BeFalse())
			Expect(p.KubernetesGlobalResource("StorageClass", "network-ssd").Exists()).To(BeTrue())
		})
	})

	o := HookExecutionConfigInit(initValuesStringProvisionOverride, `{}`)

	Context("Cluster with a default storageClass overridden by provision", func() {
		BeforeEach(func() {
			o.BindingContexts.Set(o.GenerateBeforeHelmContext())
			o.RunHook()
		})

		It("Should override only the storageClass with exactly the same name", func() {
			Expect(o).To(ExecuteSuccessfully())
			Expect(o.ValuesGet("cloudProviderYandex.internal.storageClasses").String()).To(MatchJSON(`
[
  {
	"name": "network-hdd",
	"type": "network-hdd"
  },
  {
	"name": "network-ssd",
	"type": "network-ssd",
	"blockSize": "64Ki"
  },
  {
	"name": "network-ssd-io-m3",
	"type": "network-ssd-io-m3"
  },
  {
	"name": "network-ssd-nonreplicated",
	"type": "network-ssd-nonreplicated"
  }
]
`))
		})
	})

	e := HookExecutionConfigInit(initValuesStringProvisionExcluded, `{}`)

	Context("Cluster where excludedStorageClasses matches a provisioned storageClass", func() {
		BeforeEach(func() {
			e.BindingContexts.Set(e.GenerateBeforeHelmContext())
			e.RunHook()
		})

		// exclude is applied after provision, so it filters the provisioned classes too.
		It("Should exclude the provisioned storageClass as well as the default ones", func() {
			Expect(e).To(ExecuteSuccessfully())
			Expect(e.ValuesGet("cloudProviderYandex.internal.storageClasses").String()).To(MatchJSON(`
[
  {
	"name": "network-hdd",
	"type": "network-hdd"
  }
]
`))
		})
	})
})

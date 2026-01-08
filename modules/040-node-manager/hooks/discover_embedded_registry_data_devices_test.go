/*
Copyright 2024 Flant JSC

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

var _ = Describe("Modules :: node-manager :: hooks :: discover_registry_data_devices ::", func() {
	const (
		embeddedRegistryDataDevicesSecret = `
---
apiVersion: v1
data:
  prefix-master-node-0: L2Rldi92ZGMK # /dev/vdc
  prefix-master-node-1: L2Rldi92ZGMK # /dev/vdc
  prefix-master-node-2: L2Rldi92ZGMK # /dev/vdc
kind: Secret
metadata:
  name: d8-masters-system-registry-data-device-path
  namespace: d8-system
`
	)

	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{"systemRegistry":{}}}}`, `{}`)

	Context("Secret d8-masters-system-registry-data-device-path is not exist", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("`nodeManager.internal.systemRegistry.dataDevices must be empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.systemRegistry.dataDevices").Array()).To(BeEmpty())
		})

		Context("Someone added d8-masters-system-registry-data-device-path", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(embeddedRegistryDataDevicesSecret))
				f.RunHook()
			})

			It("`nodeManager.internal.systemRegistry.dataDevices must be filled with data from secret", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("nodeManager.internal.systemRegistry.dataDevices").String()).To(MatchJSON(`
[{
    "nodeName": "prefix-master-node-0",
    "deviceName": "/dev/vdc"
},
{
    "nodeName": "prefix-master-node-1",
    "deviceName": "/dev/vdc"
},
{
    "nodeName": "prefix-master-node-2",
    "deviceName": "/dev/vdc"
}
]`))
			})
		})
	})

	Context("Secret d8-masters-system-registry-data-device-path is in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(embeddedRegistryDataDevicesSecret))
			f.RunHook()
		})

		It("`nodeManager.internal.systemRegistry.dataDevices must be filled with data from secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.systemRegistry.dataDevices").String()).To(MatchJSON(`
[{
    "nodeName": "prefix-master-node-0",
    "deviceName": "/dev/vdc"
},
{
    "nodeName": "prefix-master-node-1",
    "deviceName": "/dev/vdc"
},
{
    "nodeName": "prefix-master-node-2",
    "deviceName": "/dev/vdc"
}
]`))
		})

		Context("Secret d8-masters-system-registry-data-device-path was modified", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
data:
  prefix-master-node-0: "" # empty
  prefix-master-node-1: "" # empty
  prefix-master-node-2: "" # empty
kind: Secret
metadata:
  name: d8-masters-system-registry-data-device-path
  namespace: d8-system
`))
				f.RunHook()
			})

			It("`nodeManager.internal.systemRegistry.dataDevices must be filled with data from secret", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("nodeManager.internal.systemRegistry.dataDevices").String()).To(MatchJSON(`
[{
    "nodeName": "prefix-master-node-0",
    "deviceName": ""
},
{
    "nodeName": "prefix-master-node-1",
    "deviceName": ""
},
{
    "nodeName": "prefix-master-node-2",
    "deviceName": ""
}
]`))
			})
		})

		Context("Secret d8-masters-system-registry-data-device-path was modified", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
data:
  prefix-master-node-0: "" # empty
  prefix-master-node-2: "" # empty
kind: Secret
metadata:
  name: d8-masters-system-registry-data-device-path
  namespace: d8-system
`))
				f.RunHook()
			})

			It("`nodeManager.internal.systemRegistry.dataDevices must be filled with data from secret", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("nodeManager.internal.systemRegistry.dataDevices").String()).To(MatchJSON(`
[{
    "nodeName": "prefix-master-node-0",
    "deviceName": ""
},
{
    "nodeName": "prefix-master-node-2",
    "deviceName": ""
}
]`))
			})
		})

		Context("Secret d8-masters-system-registry-data-device-path was modified", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
data: {}
kind: Secret
metadata:
  name: d8-masters-system-registry-data-device-path
  namespace: d8-system
`))
				f.RunHook()
			})

			It("`nodeManager.internal.systemRegistry.dataDevices must be empty", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("nodeManager.internal.systemRegistry.dataDevices").Array()).To(BeEmpty())
			})
		})
	})
})

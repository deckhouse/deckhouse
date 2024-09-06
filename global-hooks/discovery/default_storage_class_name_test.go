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

/*

User-stories:
1. There is CM kube-system/extension-apiserver-authentication with CA for verification requests to our custom modules from clients inside cluster, hook must store it to `global.discovery.extensionAPIServerAuthenticationRequestheaderClientCA`.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery :: default_storage_class_name_test ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		stateDefinedDefaultStorageClassName = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-default-storage-class
  namespace: d8-system
data:
  default-storage-class-name: "network-hdd"
`
		stateEmptyDefaultStorageClassName = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-default-storage-class
  namespace: d8-system
data:
  default-storage-class-name: ""
`

		stateNoConfigMapKeyDefined = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-default-storage-class
  namespace: d8-system
data: {}
`

	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("cluster has no defined global.defaultStorageClassName (as default)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("`global.discovery.defaultStorageClassName` must not be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.defaultStorageClassName").Exists()).To(BeFalse())
		})

		Context("Then user was defined global.defaultStorageClassName", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateDefinedDefaultStorageClassName))
				f.RunHook()
			})

			It("filterResult must be true, `global.discovery.defaultStorageClassName` must be 'network-hdd'", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.defaultStorageClassName").String()).To(Equal("network-hdd"))
			})
		})

		Context("And now user was unset global.defaultStorageClassName", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateEmptyDefaultStorageClassName))
				f.RunHook()
			})

			It("filterResult must be false, `global.discovery.defaultStorageClassName` must not be set", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.defaultStorageClassName").Exists()).To(BeFalse())
			})
		})
	})

	Context("cluster has configmap without key `default-storage-class-name`", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNoConfigMapKeyDefined))
			f.RunHook()
		})

		It("`global.discovery.defaultStorageClassName` must not be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.defaultStorageClassName").Exists()).To(BeFalse())
		})
	})
})

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
1. There are StorageClasses cluster. They could have special annotations which make them default SC. Hook must find first SC with annotation and store it to `global.discovery.defaultStorageClass` else — unset it.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery :: default_storage_class_name ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		stateOneNotDefaultSC = `
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: sc0
`
		stateOneDefaultSC = `
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  annotations:
    storageclass.beta.kubernetes.io/is-default-class: "true"
  name: sc0
`
		stateOneNotDefaultAndOneDefaultSC = `
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: sc0
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
  name: sc1
`
		stateTwoNotDefaultSC = `
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: sc0
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: sc1
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("cluster has no SC", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("`global.discovery.defaultStorageClass` must not be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.defaultStorageClass").Exists()).To(BeFalse())
		})

		Context("One non-default SC added", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateOneNotDefaultSC))
				f.RunHook()
			})

			It("filterResult must be false, `global.discovery.defaultStorageClass` must not be set", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.defaultStorageClass").Exists()).To(BeFalse())
			})

			Context("Single SC was set as default", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(stateOneDefaultSC))
					f.RunHook()
				})

				It("filterResult must be true, `global.discovery.defaultStorageClass` must be 'sc0'", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("global.discovery.defaultStorageClass").String()).To(Equal("sc0"))
				})
			})

			Context("One default SC was added", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(stateOneNotDefaultAndOneDefaultSC))
					f.RunHook()
				})

				It("filterResult must be true, `global.discovery.defaultStorageClass` must be 'sc1'", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("global.discovery.defaultStorageClass").String()).To(Equal("sc1"))
				})
			})
		})
	})

	Context("One default SC and one non-default", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateOneNotDefaultAndOneDefaultSC))
			f.RunHook()
		})

		It("filterResult.isDefault must be true and false, `global.discovery.defaultStorageClass` must be 'sc1'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.defaultStorageClass").String()).To(Equal("sc1"))
		})

		Context("Both SC become non-default", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateTwoNotDefaultSC))
				f.RunHook()
			})

			It("filterResults must be false and false, `global.discovery.defaultStorageClass` must not be set", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.defaultStorageClass").Exists()).To(BeFalse())
			})
		})
	})
})
